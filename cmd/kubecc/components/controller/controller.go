package commands

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/controllers"
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/templates"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/spf13/cobra"
	// +kubebuilder:scaffold:imports
)

var (
	scheme     = runtime.NewScheme()
	lg         *zap.SugaredLogger
	configFile string
	tmplPrefix string
)

func run(cmd *cobra.Command, args []string) {
	mctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Controller)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
	)
	lg = meta.Log(mctx).Desugar().WithOptions(zap.WithCaller(false)).Sugar()
	flag.Parse()

	ctrl.SetLogger(util.ZapfLogShim{ZapLogger: lg})
	templates.SetPathPrefix(tmplPrefix)

	var err error
	options := ctrl.Options{Scheme: scheme}
	if configFile != "" {
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile))
		if err != nil {
			lg.With(zap.Error(err)).Error("unable to load the config file")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		lg.With(zap.Error(err)).Error("unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.BuildClusterReconciler{
		Context: mctx,
		Client:  mgr.GetClient(),
		Log:     lg.Named("BuildCluster"),
		Scheme:  mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		lg.With(zap.Error(err)).Error("unable to create controller", "controller", "BuildCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		lg.With(zap.Error(err)).Error("unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		lg.With(zap.Error(err)).Error("unable to set up ready check")
		os.Exit(1)
	}

	lg.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		lg.With(zap.Error(err)).Error("problem running manager")
		os.Exit(1)
	}
}

var Command = &cobra.Command{
	Use:   "controller",
	Short: "Run the Kubernetes operator (controller)",
	Run:   run,
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	Command.Flags().StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	Command.Flags().StringVar(&tmplPrefix, "templates-path", "/templates",
		"Path prefix for resource templates")
}
