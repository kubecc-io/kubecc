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

package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/apps/consumer"
	"github.com/kubecc-io/kubecc/pkg/identity"
	. "github.com/kubecc-io/kubecc/pkg/kubecc/internal"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var ConsumerNames = []string{
	"gcc",
	"g++",
	"c++",
	"clang",
	"clang++",
	"cc",
}

func run() {
	conf := CLIConfigProvider.Load().Consumer
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
