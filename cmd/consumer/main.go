package main

import (
	"context"
	"fmt"

	internal "github.com/cobalt77/kubecc/internal/consumer"
	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/apps/consumer"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var lg *zap.SugaredLogger

func main() {
	ctx := lll.NewFromContext(context.Background(), lll.Consumer,
		lll.WithOutputPaths([]string{"/tmp/consumer.log"}),
		lll.WithErrorOutputPaths([]string{"/tmp/consumer.log"}),
	)
	lg = lll.LogFromContext(ctx)

	internal.InitConfig()

	cc, err := servers.Dial(
		ctx, fmt.Sprintf("127.0.0.1:%d", viper.GetInt("port")))
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error connecting to leader")
	}
	consumer.DispatchAndWait(ctx, cc)
}
