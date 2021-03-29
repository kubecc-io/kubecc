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

package clients

import (
	"context"
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/kubecc-io/kubecc/pkg/types"
)

// An AvailabilityListener allows asserting that a component is available
// (i.e. it is streaming metadata to the monitor) during a section of code.
// The default concrete implementation of this interface is the
// AvailabilityChecker. This interface is intended to be used with
// the function WatchAvailability.
//
// Example:
// avc := clients.NewAvailabilityChecker(clients.ComponentFilter(types.Cache))
// clients.WatchAvailability(testCtx, monClient, avc)
// avc.EnsureAvailable()
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

// ComponentFilter is an AvailabilityFilter which allows only a subset of
// all components to trigger the availability callback.
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

// EnsureAvailable blocks until the AvailabilityChecker reports that the
// component is available, then returns a context which will be canceled
// when the component becomes unavailable again.
func (rsm *AvailabilityChecker) EnsureAvailable() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		go func() {
			defer cancel()
			rsm.cond.L.Lock()
			for rsm.status == Available {
				rsm.cond.Wait()
			}
			rsm.cond.L.Unlock()
		}()
	}()

	rsm.cond.L.Lock()
	for rsm.status != Available {
		rsm.cond.Wait()
	}
	rsm.cond.L.Unlock()

	return ctx
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
	rsm.cond.L.Unlock()
	rsm.cond.Broadcast()

	<-ctx.Done()

	rsm.cond.L.Lock()
	rsm.status = Unavailable
	rsm.cond.L.Unlock()
	rsm.cond.Broadcast()
}

// WatchAvailability hooks an AvailabilityListener into a monitor listener
// which will call OnComponentAvailable when the component begins
// streaming metrics to the monitor.
func WatchAvailability(
	ctx context.Context,
	monClient types.MonitorClient,
	al AvailabilityListener,
) {
	listener := NewMetricsListener(ctx, monClient, WithLogEvents(0))
	listener.OnProviderAdded(func(ctx context.Context, uuid string) {
		info, err := monClient.Whois(ctx, &types.WhoisRequest{
			UUID: uuid,
		})
		if err != nil {
			return
		}
		al.OnComponentAvailable(ctx, info)
	})
}
