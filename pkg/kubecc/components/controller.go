/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package components

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
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/spf13/cobra"
	// +kubebuilder:scaffold:imports
)

var (
	scheme     = runtime.NewScheme()
	lg         *zap.SugaredLogger
	configFile string
)

func runController(cmd *cobra.Command, args []string) {
	mctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Controller)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
	)
	lg = meta.Log(mctx).Desugar().WithOptions(zap.WithCaller(false)).Sugar()
	flag.Parse()

	ctrl.SetLogger(util.ZapfLogShim{ZapLogger: lg})

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

var ControllerCmd = &cobra.Command{
	Use:   "controller",
	Short: "Run the Kubernetes operator (controller)",
	PreRun: func(cmd *cobra.Command, args []string) {
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(v1alpha1.AddToScheme(scheme))
		// +kubebuilder:scaffold:scheme
	},
	Run: runController,
}

func init() {
	AgentCmd.Flags().StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
}
