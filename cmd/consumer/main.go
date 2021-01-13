package main

import (
	"fmt"

	"github.com/cobalt77/kubecc/internal/consumer"
	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	lll.Setup("C",
		lll.WithOutputPaths([]string{"/tmp/consumer.log"}),
		lll.WithErrorOutputPaths([]string{"/tmp/consumer.log"}),
	)

	consumer.InitConfig()
	c, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", viper.GetInt("port")),
		grpc.WithInsecure())
	if err != nil {
		lll.With(zap.Error(err)).Fatal("Error connecting to leader")
	}
	dispatchAndWait(c)
}
