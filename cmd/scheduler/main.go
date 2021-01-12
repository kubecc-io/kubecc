package main

import (
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
	log.Info("Server starting")
	initConfig()

	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err.Error())
	}
	log.With("addr", listener.Addr().String()).
		Info("Server listening")

	grpcServer := grpc.NewServer()

	srv := NewSchedulerServer()
	types.RegisterSchedulerServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
