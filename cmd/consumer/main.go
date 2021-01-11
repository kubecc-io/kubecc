package main

import (
	"fmt"

	"github.com/cobalt77/kubecc/internal/consumer"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	log *zap.Logger
)

func init() {
	conf := zap.NewDevelopmentConfig()
	conf.OutputPaths = []string{"/tmp/consumer.log"}
	conf.ErrorOutputPaths = []string{"/tmp/consumer.log"}

	lg, err := conf.Build(zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		panic(err)
	}
	log = lg
}

func main() {
	consumer.InitConfig()
	c, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", viper.GetInt("port")),
		grpc.WithInsecure())
	if err != nil {
		log.With(zap.Error(err)).Fatal("Error connecting to leader")
	}
	dispatchAndWait(c)
}
