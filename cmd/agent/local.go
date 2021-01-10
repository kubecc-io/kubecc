package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type localAgentServer struct {
	types.LocalAgentServer

	schedulerClient types.SchedulerClient
}

var (
	scheduler *Scheduler
)

func (s *localAgentServer) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	os.Chdir(req.WorkDir)
	log.Debug("Running agent request")
	info := cc.NewArgsInfo(req.Command, req.Args, log)
	info.Parse()

	var outputPath string
	if info.OutputArgIndex >= 0 && info.OutputArgIndex < len(info.Args) {
		outputPath = info.Args[info.OutputArgIndex]
	}

	if s.schedulerClient != nil {
		log.Debug("No remote scheduler, running locally")

		buf := new(bytes.Buffer)
		// Always run locally
		data, err := cc.Run(info,
			cc.WithLogOutput(buf),
			cc.WithEnv(req.Env),
			cc.WithWorkDir(req.WorkDir),
		)
		if err != nil {
			log.With(zap.Error(err)).Debug("Compiler error")
			return nil, errors.New(string(buf.Bytes()))
		}
		if outputPath != "" {
			log.With(zap.String("path", outputPath)).
				Debug("Writing file")
			err = ioutil.WriteFile(outputPath, data, 0777)
			if err != nil {
				log.With(zap.Error(err)).Debug("Failed to write file")
				return nil, err
			}
		}
		log.With(zap.Error(err)).Debug("Local run success")
		return &types.RunResponse{}, nil
	}

	info.SubstitutePreprocessorOptions()

	var preprocessedSource []byte
	switch opt := info.ActionOpt(); opt {
	case cc.Compile:
	case cc.GenAssembly:
		log.Debug("Preprocessing")
		info.SetActionOpt(cc.Preprocess)
		var err error
		preprocessedSource, err = cc.Run(info, cc.WithCompressOutput())
		if err != nil {
			return nil, err
		}
		info.SetActionOpt(opt)
	case cc.Preprocess:
		log.Debug("Preprocess requested originally")
		f, err := os.Open(info.Args[info.InputArgIndex])
		if err != nil {
			return nil, err
		}
		preprocessedSource, err = ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}
	}

	info.ReplaceInputPath("-") // Read from stdin

	sourceReader, err := gzip.NewReader(bytes.NewReader(preprocessedSource))
	if err != nil {
		panic(err)
	}

	for i := 0; ; i++ {
		outBuf := new(bytes.Buffer)
		err := scheduler.Run(ctx,
			FailFast(i == 0),
			WithCommand(info.Compiler),
			WithArgs(info.Args),
			WithEnv(req.Env),
			WithWorkDir(req.WorkDir),
			WithInputStream(sourceReader),
			WithOutputStreams(outBuf, os.Stderr),
		)
		if _, ok := err.(*AllThreadsBusy); ok {
			log.Debug("All threads busy")

			_, err := s.schedulerClient.Schedule(
				ctx, &types.ScheduleRequest{})
			if err != nil {
				log.Debug("Schedule failed")
				// Schedule failed, try local again
				continue
			}
			// Compile remote
			info.RemoveLocalArgs()
			log.Debug("Starting remote compile")
			resp, err := s.schedulerClient.Compile(ctx, &types.CompileRequest{
				Command:            info.Compiler,
				Args:               info.Args,
				PreprocessedSource: preprocessedSource,
			})
			if err != nil {
				log.With(zap.Error(err)).Debug("Remote compile failed")
				return nil, err
			}
			log.Debug("Remote compile success")
			f, err := os.OpenFile(outputPath,
				os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0777)

			if err != nil {
				log.With(zap.Error(err)).Debug("Failed to write file")
				return nil, err
			}
			reader, err := gzip.NewReader(bytes.NewReader(
				resp.CompiledSource))
			if err != nil {
				log.Error("Invalid data returned from the remote agent")
				return nil, err
			}
			_, err = io.Copy(f, reader)
			if err != nil {
				log.With(zap.Error(err)).Debug("Copy failed")
				return nil, err
			}
			return &types.RunResponse{}, nil
		}
		return &types.RunResponse{}, err
	}
}

func (s *localAgentServer) connectToRemote() {
	conn, err := grpc.Dial(
		viper.GetString("remoteAddress"),
		grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				InsecureSkipVerify: false,
			},
		)))
	if err != nil {
		log.With(zap.Error(err)).Info("Remote compilation unavailable")
	} else {
		s.schedulerClient = types.NewSchedulerClient(conn)
	}
	scheduler = NewScheduler()
}

func startLocalAgent() {
	agent := &localAgentServer{}
	go agent.connectToRemote()

	port := viper.GetInt("agentPort")
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.With(zap.Error(err)).Fatal("Could not start local agent")
	}
	srv := grpc.NewServer()

	types.RegisterLocalAgentServer(srv, agent)

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err.Error())
	}
}

func dispatchToAgent(cc *grpc.ClientConn) {
	agent := types.NewLocalAgentClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err.Error())
	}
	ctx, _ := cluster.NewAgentContext()
	_, err = agent.Run(ctx, &types.RunRequest{
		WorkDir: wd,
		Command: os.Args[0],
		Args:    os.Args[1:],
		Env:     os.Environ(),
	}, grpc.WaitForReady(true))
	if err != nil {
		log.Error("Dispatch error")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log.Info("Dispatch Success")
	os.Exit(0)
}

func connectOrFork() {
	port := viper.GetInt("agentPort")
	// Test if the connection is open
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))

	if err == nil {
		listener.Close()
		log.Info("Starting agent")
		self, err := os.Executable()
		if err != nil {
			log.Fatal(err.Error())
		}
		orig, err := filepath.EvalSymlinks(self)
		if err != nil {
			log.Fatal(err.Error())
		}
		workdir, err := os.Getwd()
		if err != nil {
			log.Fatal(err.Error())
		}
		_, err = syscall.ForkExec(orig, []string{
			"agent", "run", "--local",
		}, &syscall.ProcAttr{
			Dir: workdir,
			Env: os.Environ(),
			Sys: &syscall.SysProcAttr{
				Setsid: true,
			},
			Files: []uintptr{
				os.Stdin.Fd(),
				os.Stdout.Fd(),
				os.Stderr.Fd(),
			},
		})
		if err != nil {
			log.With(zap.Error(err)).Fatal("Error starting local agent")
		}
	}

	log.Info("Dispatching to agent")
	// Connect to the local agent
	c, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithInsecure())
	if err != nil {
		log.With(zap.Error(err)).Fatal("Error connecting to local agent")
	}
	dispatchToAgent(c)
}
