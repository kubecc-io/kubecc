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
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type consumerLeader struct {
	types.ConsumerServer

	schedulerClient types.SchedulerClient
}

var (
	executor *Executor
)

func (s *consumerLeader) Run(
	ctx context.Context,
	req *types.DispatchRequest,
) (*types.Empty, error) {
	os.Chdir(req.WorkDir)
	log.Debug("Running follower request")
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
		return &types.Empty{}, nil
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
		err := executor.Exec(ctx,
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
				ctx, &types.ScheduleRequest{},
				grpc.WaitForReady(false))
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
			log.Debug("Remote compile completed")
			switch resp.CompileResult {
			case types.CompileResponse_Success:
				f, err := os.OpenFile(outputPath,
					os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0777)

				if err != nil {
					log.With(zap.Error(err)).Debug("Failed to write file")
					return nil, err
				}
				reader, err := gzip.NewReader(bytes.NewReader(
					resp.GetCompiledSource()))
				if err != nil {
					log.Error("Invalid data returned from the remote agent")
					return nil, err
				}
				_, err = io.Copy(f, reader)
				if err != nil {
					log.With(zap.Error(err)).Debug("Copy failed")
					return nil, err
				}
				return &types.Empty{}, nil
			case types.CompileResponse_Fail:
				log.Error(resp.GetError())
				return &types.Empty{}, errors.New(resp.GetError())
			}
		}
		return &types.Empty{}, err
	}
}

func (s *consumerLeader) connectToRemote() {
	addr := viper.GetString("schedulerAddress")
	if addr == "" {
		log.Debug("Remote compilation unavailable: scheduler address not configured")
		return
	}

	conn, err := grpc.Dial(addr,
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
	executor = NewExecutor()
}

func runLeader() {
	agent := &consumerLeader{}
	go agent.connectToRemote()

	port := viper.GetInt("port")
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.With(zap.Error(err)).Fatal("Could not start local agent")
	}
	srv := grpc.NewServer()

	types.RegisterConsumerServer(srv, agent)

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err.Error())
	}
}

func becomeLeader() {
	log.Info("Becoming leader")
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
		"--become-leader",
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
		log.With(zap.Error(err)).Fatal("Error becoming leader")
	}
}
