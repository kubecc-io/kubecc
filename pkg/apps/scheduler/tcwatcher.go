package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
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
	listener := clients.NewListener(w.Context, w.Client)
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
