package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
)

type ToolchainWatcher struct {
	Context context.Context
	Client  types.MonitorClient
}

func (b ToolchainWatcher) WatchToolchains(uuid string) chan *metrics.Toolchains {
	ch := make(chan *metrics.Toolchains)
	listener := clients.NewListener(b.Context, b.Client)
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
