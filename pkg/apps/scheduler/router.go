package scheduler

import (
	"context"
	"errors"
	"sync"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
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
	hash       string
	C          chan request
	rxRefCount *atomic.Int32
	txRefCount *atomic.Int32
	cancel     context.CancelFunc
}

func (c *route) CanSend() bool {
	return c.rxRefCount.Load() > 0
}

func (c *route) incRxRefCount() {
	c.rxRefCount.Inc()
}

func (c *route) decRxRefCount() {
	if c.rxRefCount.Dec() <= 0 {
		if c.txRefCount.Load() <= 0 {
			c.cancel()
		}
	}
}

func (c *route) incTxRefCount() {
	c.txRefCount.Inc()
}

func (c *route) decTxRefCount() {
	if c.txRefCount.Dec() <= 0 {
		if c.rxRefCount.Load() <= 0 {
			c.cancel()
		}
	}
}

func (c *route) attachSender(s *sender) {
	c.incTxRefCount()
	defer c.decTxRefCount()
	<-s.cd.Context.Done()
}

func (c *route) attachReceiver(r *receiver) {
	c.incRxRefCount()
	defer c.decRxRefCount()

	for {
		select {
		case i, open := <-c.C:
			if !open {
				// Channel closed
				return
			}
			r.filteredOutput <- i
		case <-r.agent.Context.Done():
			return
		}
	}
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

func NewRouter(ctx context.Context) *Router {
	return &Router{
		ctx:            ctx,
		senders:        make(map[string]*sender),
		receivers:      make(map[string]*receiver),
		routes:         make(map[string]*route),
		routesMutex:    &sync.RWMutex{},
		sendersMutex:   &sync.RWMutex{},
		receiversMutex: &sync.RWMutex{},
	}
}

func tcHash(tc *types.Toolchain) string {
	hasher := md5simd.StdlibHasher()
	defer hasher.Close()
	tc.Hash(hasher)
	sum := hasher.Sum(nil)

	return string(sum)
}

func (f *Router) newRoute(hash string) *route {
	ctx, cancel := context.WithCancel(f.ctx)
	rt := &route{
		hash:       hash,
		C:          make(chan request),
		rxRefCount: atomic.NewInt32(0),
		txRefCount: atomic.NewInt32(0),
		cancel:     cancel,
	}
	go func() {
		<-ctx.Done()
		// Ref count hit 0, clean up the channel to avoid a resource leak
		f.routesMutex.Lock()
		defer f.routesMutex.Unlock()
		close(rt.C)
		delete(f.routes, hash)
	}()
	return rt
}

func (f *Router) routeForToolchain(tc *types.Toolchain) *route {
	f.routesMutex.Lock()
	defer f.routesMutex.Unlock()
	hash := tcHash(tc)
	var rt *route
	if c, ok := f.routes[hash]; !ok || c == nil {
		f.routes[hash] = f.newRoute(hash)
		rt = f.routes[hash]
	} else {
		rt = c
	}
	return rt
}

func (f *Router) AddSender(cd *Consumerd) {
	f.sendersMutex.Lock()
	defer f.sendersMutex.Unlock()
	sender := &sender{
		cd: cd,
	}
	f.senders[cd.UUID] = sender
	for _, tc := range cd.Toolchains.GetItems() {
		rt := f.routeForToolchain(tc)
		go rt.attachSender(sender)
	}
}

func (f *Router) AddReceiver(agent *Agent) <-chan request {
	f.receiversMutex.Lock()
	defer f.receiversMutex.Unlock()
	output := make(chan request)
	receiver := &receiver{
		agent:          agent,
		filteredOutput: output,
	}
	f.receivers[agent.UUID] = receiver
	for _, tc := range agent.Toolchains.GetItems() {
		rt := f.routeForToolchain(tc)
		go rt.attachReceiver(receiver)
	}
	return output
}

func (f *Router) UpdateSenderToolchains(
	uuid string,
	tcs *metrics.Toolchains,
) {
	f.sendersMutex.Lock()
	defer f.sendersMutex.Unlock()
	sender, ok := f.senders[uuid]
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
			f.routeForToolchain(oldTc).cancel()
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
				f.routeForToolchain(newTc).attachSender(sender)
			}()
		}
	}

	sender.cd.Toolchains = newToolchains
}

func (f *Router) Send(ctx context.Context, req request) error {
	tc := req.GetToolchain()
	if tc == nil {
		return ErrInvalidToolchain
	}
	rt := f.routeForToolchain(tc)
	for _, hook := range f.hooks {
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
		return context.Canceled
	default:
		return ErrStreamClosed
	}
}
