/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
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
	zapf "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/controllers"
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/templates"
	"github.com/cobalt77/kubecc/pkg/types"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	lg     *zap.SugaredLogger
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	logkc.NewFromContext(context.Background(), types.Controller)
	logkc.PrintHeader()

	var (
		configFile string
		tmplPrefix string
	)
	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	flag.StringVar(&tmplPrefix, "templates-path", "/templates",
		"Path prefix for resource templates")
	opts := zapf.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zapf.New(zapf.UseFlagOptions(&opts)))
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

	if err = (&controllers.ToolchainReconciler{
		Client: mgr.GetClient(),
		Log:    lg.Named("Toolchain"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		lg.With(zap.Error(err)).Error("unable to create controller", "controller", "Toolchain")
		os.Exit(1)
	}
	if err = (&controllers.BuildClusterReconciler{
		Client: mgr.GetClient(),
		Log:    lg.Named("BuildCluster"),
		Scheme: mgr.GetScheme(),
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
