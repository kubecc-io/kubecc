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

package commands

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	. "github.com/cobalt77/kubecc/pkg/kubecc/internal"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/ui"
	"github.com/spf13/cobra"
)

// StatusCmd represents the status command.
var StatusCmd = &cobra.Command{
	Use:              "status",
	Short:            "Show overall cluster status",
	PersistentPreRun: InitCLI,
	Run: func(cmd *cobra.Command, args []string) {
		cc, err := servers.Dial(CLIContext, CLIConfig.MonitorAddress,
			servers.WithTLS(!CLIConfig.DisableTLS))
		if err != nil {
			CLILog.Fatal(err)
		}
		client := types.NewMonitorClient(cc)
		listener := clients.NewListener(CLIContext, client)
		display := ui.NewStatusDisplay()

		listener.OnProviderAdded(func(pctx context.Context, uuid string) {
			info, err := client.Whois(CLIContext, &types.WhoisRequest{
				UUID: uuid,
			})
			if err != nil {
				return
			}

			display.AddAgent(pctx, info)
			listener.OnValueChanged(uuid, func(qp *metrics.UsageLimits) {
				display.Update(uuid, qp)
			})
			listener.OnValueChanged(uuid, func(ts *metrics.TaskStatus) {
				display.Update(uuid, ts)
			})
			<-pctx.Done()
		})

		display.Run()
	},
}
