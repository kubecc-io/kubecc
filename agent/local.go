package agent

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

	"github.com/cobalt77/kube-cc/cc"
	types "github.com/cobalt77/kube-cc/types"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type localAgentServer struct {
	types.LocalAgentServer

	mgrClient types.MgrClient
}

var (
	scheduler *Scheduler
)

func (s *localAgentServer) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	os.Chdir(req.WorkDir)

	info := cc.NewArgsInfo(req.Command, req.Args)
	info.Parse()
	outputPath := info.Args[info.OutputArgIndex]

	if s.mgrClient != nil {
		// Always run locally
		data, err := cc.Run(info)
		if err != nil {
			return nil, err
		}
		ioutil.WriteFile(outputPath, data, 0644)
		return &types.RunResponse{}, nil
	}

	info.SubstitutePreprocessorOptions()

	var preprocessedSource []byte
	switch opt := info.ActionOpt(); opt {
	case cc.Compile:
	case cc.GenAssembly:
		info.SetActionOpt(cc.Preprocess)
		var err error
		preprocessedSource, err = cc.Run(info, cc.WithCompressOutput())
		if err != nil {
			return nil, err
		}
		info.SetActionOpt(opt)
	case cc.Preprocess:
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
			_, err := s.mgrClient.Schedule(
				ctx, &types.ScheduleRequest{})
			if err != nil {
				// Schedule failed, try local again
				continue
			}
			// Compile remote
			info.RemoveLocalArgs()
			resp, err := s.mgrClient.Compile(ctx, &types.CompileRequest{
				Command:            info.Compiler,
				Args:               info.Args,
				PreprocessedSource: preprocessedSource,
			})
			if err != nil {
				return nil, err
			}
			f, err := os.Create(outputPath)
			if err != nil {
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
				return nil, err
			}
			return &types.RunResponse{}, nil
		}
		return &types.RunResponse{}, err
	}
}

func startAgent() {
	srv := grpc.NewServer()
	port := viper.GetInt("agentPort")
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Fatal(err)
	}
	agent := &localAgentServer{}
	types.RegisterLocalAgentServer(srv, agent)

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:443", viper.GetString("remoteAddress")),
		grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				InsecureSkipVerify: false,
			},
		)))
	if err != nil {
		log.Infof("Remote compilation unavailable: %s", err.Error())
	} else {
		agent.mgrClient = types.NewMgrClient(conn)
	}
	scheduler = NewScheduler()

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err)
	}
	// log.SetLevel(log.DebugLevel)
	// log.Debug("Starting agent ")
}

func dispatchToAgent(cc *grpc.ClientConn) {
	agent := types.NewLocalAgentClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	_, err = agent.Run(context.Background(), &types.RunRequest{
		WorkDir: wd,
		Args:    os.Args,
		Env:     os.Environ(),
	})
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func StartAgentOrDispatch() {
	port := viper.GetInt("agentPort")

	c, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithInsecure())
	if err != nil {
		// No agent running, become the agent
		startAgent()
	} else {
		// Connect to the local agent
		dispatchToAgent(c)
	}
}
