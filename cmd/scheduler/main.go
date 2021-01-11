package main

import (
	"net"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

var (
	scheme = runtime.NewScheme()
	config *rest.Config
	log    *zap.SugaredLogger
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

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	log.Info("Server starting")

	cfg, err := rest.InClusterConfig()

	if err != nil {
		panic(err.Error())
	}

	config = cfg

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
