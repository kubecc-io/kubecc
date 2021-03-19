package clients

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	mapset "github.com/deckarep/golang-set"
)

type AvailabilityListener interface {
	OnComponentAvailable(context.Context, *types.WhoisResponse)
}

type RemoteStatus int

const (
	Unavailable RemoteStatus = iota
	Available
)

type AvailabilityChecker struct {
	status RemoteStatus
	filter AvailabilityFilter
	cond   *sync.Cond
}

type AvailabilityFilter = func(*types.WhoisResponse) bool

func ComponentFilter(c ...types.Component) AvailabilityFilter {
	set := mapset.NewSet()
	for _, item := range c {
		set.Add(item)
	}
	return func(info *types.WhoisResponse) bool {
		return set.Contains(info.Component)
	}
}

func NewAvailabilityChecker(filter AvailabilityFilter) *AvailabilityChecker {
	rsm := &AvailabilityChecker{
		status: Unavailable,
		filter: filter,
		cond:   sync.NewCond(&sync.Mutex{}),
	}
	return rsm
}

func (rsm *AvailabilityChecker) EnsureAvailable() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		go func() {
			rsm.cond.L.Lock()
			defer rsm.cond.L.Unlock()

			for {
				if rsm.status != Available {
					cancel()
					return
				}
				rsm.cond.Wait()
			}
		}()
	}()

	rsm.cond.L.Lock()
	defer rsm.cond.L.Unlock()

	for {
		if rsm.status == Available {
			return ctx
		}
		rsm.cond.Wait()
	}
}

func (rsm *AvailabilityChecker) OnComponentAvailable(
	ctx context.Context,
	info *types.WhoisResponse,
) {
	if !rsm.filter(info) {
		return
	}

	rsm.cond.L.Lock()
	rsm.status = Available
	rsm.cond.Broadcast()
	rsm.cond.L.Unlock()

	<-ctx.Done()

	rsm.cond.L.Lock()
	rsm.status = Unavailable
	rsm.cond.Broadcast()
	rsm.cond.L.Unlock()
}

func WatchAvailability(
	ctx context.Context,
	component types.Component,
	monClient types.MonitorClient,
	al AvailabilityListener,
) {
	listener := NewListener(ctx, monClient, servers.WithLogEvents(0))
	listener.OnProviderAdded(func(ctx context.Context, uuid string) {
		info, err := monClient.Whois(ctx, &types.WhoisRequest{
			UUID: uuid,
		})
		if err != nil {
			return
		}
		if info.Component == component {
			al.OnComponentAvailable(ctx, info)
		}
	})
}
