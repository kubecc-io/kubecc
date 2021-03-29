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

package scheduler

import (
	"context"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type ToolchainWatcher interface {
	WatchToolchains(uuid string) chan *metrics.Toolchains
}

func NewDefaultToolchainWatcher(
	ctx context.Context,
	client types.MonitorClient,
) ToolchainWatcher {
	return tcWatcher{
		Context: ctx,
		Client:  client,
	}
}

type tcWatcher struct {
	Context context.Context
	Client  types.MonitorClient
}

func (w tcWatcher) WatchToolchains(uuid string) chan *metrics.Toolchains {
	ch := make(chan *metrics.Toolchains)
	listener := clients.NewMetricsListener(w.Context, w.Client)
	listener.OnProviderAdded(func(ctx context.Context, id string) {
		if id != uuid {
			return
		}
		defer listener.Stop()
		listener.OnValueChanged(id, func(tc *metrics.Toolchains) {
			ch <- tc
		})
		<-ctx.Done()
	})
	return ch
}
