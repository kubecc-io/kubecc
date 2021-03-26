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
	"errors"
	"sync"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	mapset "github.com/deckarep/golang-set"
	md5simd "github.com/minio/md5-simd"
	"go.uber.org/atomic"
)

var (
	ErrNoAgents         = errors.New("No available agents can run this task")
	ErrStreamClosed     = errors.New("Task stream closed")
	ErrRequestRejected  = errors.New("The task has been rejected by the server")
	ErrInvalidToolchain = errors.New("Invalid or nil toolchain")
)

type request = *types.CompileRequest

type sender struct {
	cd *Consumerd
}

type receiver struct {
	agent          *Agent
	filteredOutput chan<- request
}

type route struct {
	tc         *types.Toolchain
	hash       string
	C          chan request
	rxRefCount *atomic.Int32
	txRefCount *atomic.Int32
	senders    mapset.Set
	receivers  mapset.Set
	cancel     context.CancelFunc
}

func (rt *route) CanSend() bool {
	return rt.rxRefCount.Load() > 0
}

// todo: unsure if we still need the ref counting here

func (rt *route) incRxRefCount() {
	rt.rxRefCount.Inc()
}

func (rt *route) decRxRefCount() {
	if rt.rxRefCount.Dec() <= 0 {
		if rt.txRefCount.Load() <= 0 {
			rt.cancel()
		}
	}
}

func (rt *route) incTxRefCount() {
	rt.txRefCount.Inc()
}

func (rt *route) decTxRefCount() {
	if rt.txRefCount.Dec() <= 0 {
		if rt.rxRefCount.Load() <= 0 {
			rt.cancel()
		}
	}
}

func (rt *route) attachSender(s *sender) {
	uuid := s.cd.UUID
	rt.senders.Add(uuid)
	rt.incTxRefCount()
	go func(uuid string) {
		defer rt.senders.Remove(uuid)
		defer rt.decTxRefCount()
		<-s.cd.Context.Done()
	}(uuid)
}

func (rt *route) attachReceiver(r *receiver) {
	uuid := r.agent.UUID
	rt.receivers.Add(uuid)
	rt.incRxRefCount()
	go func(uuid string) {
		defer rt.receivers.Remove(uuid)
		defer rt.decRxRefCount()
		for {
			select {
			case i, open := <-rt.C:
				if !open {
					// Channel closed
					return
				}
				r.filteredOutput <- i
			case <-r.agent.Context.Done():
				return
			}
		}
	}(uuid)
}

type HookAction int

const (
	ProcessRequestNormally HookAction = iota
	RejectRequest
	RequestIntercepted
)

type RouterHook interface {
	PreReceive(*route, request) HookAction
}

type RouterOptions struct {
	hooks []RouterHook
}

type RouterOption func(*RouterOptions)

func (o *RouterOptions) Apply(opts ...RouterOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithHooks(hooks ...RouterHook) RouterOption {
	return func(o *RouterOptions) {
		o.hooks = hooks
	}
}

type Router struct {
	ctx            context.Context
	senders        map[string]*sender   // key = uuid
	receivers      map[string]*receiver // key = uuid
	routes         map[string]*route    // key = toolchain hash
	routesMutex    *sync.RWMutex
	sendersMutex   *sync.RWMutex
	receiversMutex *sync.RWMutex
	hooks          []RouterHook
}

func NewRouter(ctx context.Context, opts ...RouterOption) *Router {
	options := RouterOptions{
		hooks: []RouterHook{},
	}
	options.Apply(opts...)

	return &Router{
		ctx:            ctx,
		senders:        make(map[string]*sender),
		receivers:      make(map[string]*receiver),
		routes:         make(map[string]*route),
		routesMutex:    &sync.RWMutex{},
		sendersMutex:   &sync.RWMutex{},
		receiversMutex: &sync.RWMutex{},
		hooks:          options.hooks,
	}
}

func tcHash(tc *types.Toolchain) string {
	hasher := md5simd.StdlibHasher()
	defer hasher.Close()
	tc.Hash(hasher)
	sum := hasher.Sum(nil)

	return string(sum)
}

func (r *Router) newRoute(tc *types.Toolchain, hash string) *route {
	ctx, cancel := context.WithCancel(r.ctx)
	rt := &route{
		tc:         tc,
		hash:       hash,
		C:          make(chan request),
		rxRefCount: atomic.NewInt32(0),
		txRefCount: atomic.NewInt32(0),
		senders:    mapset.NewSet(),
		receivers:  mapset.NewSet(),
		cancel:     cancel,
	}
	go func() {
		<-ctx.Done()
		// Ref count hit 0, clean up the channel to avoid a resource leak
		r.routesMutex.Lock()
		defer r.routesMutex.Unlock()
		close(rt.C)
		delete(r.routes, hash)
	}()
	return rt
}

func (r *Router) routeForToolchain(tc *types.Toolchain) *route {
	r.routesMutex.Lock()
	defer r.routesMutex.Unlock()
	hash := tcHash(tc)
	var rt *route
	if c, ok := r.routes[hash]; !ok || c == nil {
		r.routes[hash] = r.newRoute(tc, hash)
		rt = r.routes[hash]
	} else {
		rt = c
	}
	return rt
}

func (r *Router) AddSender(cd *Consumerd) {
	r.sendersMutex.Lock()
	defer r.sendersMutex.Unlock()
	sender := &sender{
		cd: cd,
	}
	r.senders[cd.UUID] = sender
	for _, tc := range cd.Toolchains.GetItems() {
		rt := r.routeForToolchain(tc)
		rt.attachSender(sender)
	}
}

func (r *Router) AddReceiver(agent *Agent) <-chan request {
	r.receiversMutex.Lock()
	defer r.receiversMutex.Unlock()
	output := make(chan request)
	receiver := &receiver{
		agent:          agent,
		filteredOutput: output,
	}
	r.receivers[agent.UUID] = receiver
	for _, tc := range agent.Toolchains.GetItems() {
		rt := r.routeForToolchain(tc)
		rt.attachReceiver(receiver)
	}
	return output
}

func (r *Router) UpdateSenderToolchains(
	uuid string,
	tcs *metrics.Toolchains,
) {
	r.sendersMutex.Lock()
	defer r.sendersMutex.Unlock()
	sender, ok := r.senders[uuid]
	if !ok {
		return
	}
	oldToolchains := sender.cd.Toolchains
	newToolchains := tcs

	for _, oldTc := range oldToolchains.GetItems() {
		stillExists := false
		for _, newTc := range newToolchains.GetItems() {
			if oldTc.EquivalentTo(newTc) {
				stillExists = true
				break
			}
		}
		if !stillExists {
			r.routeForToolchain(oldTc).cancel()
		}
	}
	for _, newTc := range newToolchains.GetItems() {
		isNew := true
		for _, oldTc := range oldToolchains.GetItems() {
			if newTc.EquivalentTo(oldTc) {
				isNew = false
				break
			}
		}
		if isNew {
			defer func() {
				r.routeForToolchain(newTc).attachSender(sender)
			}()
		}
	}

	sender.cd.Toolchains = newToolchains
}

func (r *Router) Route(ctx context.Context, req request) error {
	tc := req.GetToolchain()
	if tc == nil {
		return ErrInvalidToolchain
	}
	rt := r.routeForToolchain(tc)
	for _, hook := range r.hooks {
		switch hook.PreReceive(rt, req) {
		case ProcessRequestNormally:
		case RejectRequest:
			return ErrRequestRejected
		case RequestIntercepted:
			return nil
		}
	}
	if rt.rxRefCount.Load() == 0 {
		return ErrNoAgents
	}
	select {
	case rt.C <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func stringSlice(interfaces []interface{}) []string {
	s := make([]string, len(interfaces))
	for i, v := range interfaces {
		s[i] = v.(string)
	}
	return s
}

func (r *Router) GetRoutes() *types.RouteList {
	list := &types.RouteList{
		Routes: []*types.Route{},
	}
	r.routesMutex.RLock()
	defer r.routesMutex.RUnlock()
	for _, v := range r.routes {
		list.Routes = append(list.Routes, &types.Route{
			Toolchain:  v.tc,
			Consumerds: stringSlice(v.receivers.ToSlice()),
			Agents:     stringSlice(v.senders.ToSlice()),
		})
	}
	return list
}
