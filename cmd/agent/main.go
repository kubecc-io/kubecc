package main

import (
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

var (
	log *zap.SugaredLogger
)

func init() {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	lg, err := cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		panic(err)
	}
	log = lg.Sugar()
}

func main() {
	srv := grpc.NewServer()
	listener, err := net.Listen("tcp", fmt.Sprintf(":9090"))
	if err != nil {
		log.With(
			zap.Error(err),
		).Fatal("Error listening on socket")
	}
	agent := &agentServer{}
	types.RegisterAgentServer(srv, agent)
	connectToScheduler()
	err = srv.Serve(listener)
	if err != nil {
		log.With(zap.Error(err)).Error("GRPC error")
	}
}
