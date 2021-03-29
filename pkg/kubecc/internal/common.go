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

package internal

import (
	"context"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	CLIContext context.Context
	CLILog     *zap.SugaredLogger
	CLIConfig  config.KcctlSpec
)

func InitCLI(_ *cobra.Command, _ []string) {
	CLIConfig = (&config.ConfigMapProvider{}).Load().Kcctl

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.CLI,
				logkc.WithLogLevel(CLIConfig.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	CLIContext = ctx
	CLILog = lg
}
