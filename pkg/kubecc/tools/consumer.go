package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/consumer"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var ConsumerNames = []string{
	"gcc",
	"g++",
	"clang",
	"clang++",
	"cc",
}

func run() {
	conf := (&config.ConfigMapProvider{}).Load().Consumer
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumer)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumer,
				logkc.WithOutputPaths([]string{"/tmp/consumer.log"}),
				logkc.WithErrorOutputPaths([]string{"/tmp/consumer.log"}),
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	cc, err := servers.Dial(ctx, conf.ConsumerdAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error connecting to consumerd")
	}
	consumer.DispatchAndWait(ctx, cc)
}

var ConsumerCmd = &cobra.Command{
	Use:   "consumer",
	Short: "A compiler shim used as an entrypoint into kubecc",
	Long: fmt.Sprintf(`This tool will send the executable name and arguments to the running consumerd,
to be invoked either locally or remotely. This will automatically be run if the
kubecc executable name is one of the following:
%s`, strings.Join(ConsumerNames, "\n")),
	PreRun: func(_ *cobra.Command, args []string) {
		// when run from cobra
		os.Args = append([]string{os.Args[0]}, args...)
	},
	Run: func(_ *cobra.Command, args []string) {
		run()
	},
}
