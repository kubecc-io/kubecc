package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/zapkc"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	cliContext context.Context
	cliLog     *zap.SugaredLogger
	cliConfig  config.KcctlSpec
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "kcctl",
	Short: "A brief description of your application",
	Long: fmt.Sprintf("%s\n%s", zapkc.Yellow.Add(logkc.BigAsciiTextColored), `
The kubecc CLI utility`),
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	cliContext = ctx
	cliLog = lg

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

// initConfig reads in the config file.
func initConfig() {
	cliConfig = (&config.ConfigMapProvider{}).Load(cliContext).Kcctl
}
