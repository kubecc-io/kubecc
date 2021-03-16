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
	ErrNoAgents        = errors.New("No available agents can run this task")
	ErrStreamClosed    = errors.New("Task stream closed")
	ErrRequestRejected = errors.New("The task has been rejected by the server")
)

type sender struct {
	cd              *Consumerd
	unfilteredInput <-chan interface{}
}

type receiver struct {
	agent          *Agent
	filteredOutput chan<- interface{}
}

type taskChannel struct {
	hash       string
	C          chan interface{}
	rxRefCount *atomic.Int32
	txRefCount *atomic.Int32
	channelCtx context.Context
	cancel     context.CancelFunc
}

func (c *taskChannel) CanSend() bool {
	return c.rxRefCount.Load() > 0
}

func (c *taskChannel) incRxRefCount() {
	c.rxRefCount.Inc()
}

func (c *taskChannel) decRxRefCount() {
	if c.rxRefCount.Dec() <= 0 {
		if c.txRefCount.Load() <= 0 {
			c.cancel()
		}
	}
}

func (c *taskChannel) incTxRefCount() {
	c.txRefCount.Inc()
}

func (c *taskChannel) decTxRefCount() {
	if c.txRefCount.Dec() <= 0 {
		if c.rxRefCount.Load() <= 0 {
			c.cancel()
		}
	}
}

func (c *taskChannel) AttachSender(s *sender) {
	c.incTxRefCount()
	defer c.decTxRefCount()

	for {
		select {
		case i := <-s.unfilteredInput:
			select {
			case c.C <- i:
			default:
				// Channel closed
				return
			}
		case <-s.cd.Context.Done():
			return
		}
	}
}

func (c *taskChannel) AttachReceiver(r *receiver) {
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

type FilterHook interface {
	PreReceive(*taskChannel, *types.CompileRequest) HookAction
}

type ToolchainFilter struct {
	ctx            context.Context
	senders        map[string]*sender      // key = uuid
	receivers      map[string]*receiver    // key = uuid
	channels       map[string]*taskChannel // key = toolchain hash
	channelsMutex  *sync.RWMutex
	sendersMutex   *sync.RWMutex
	receiversMutex *sync.RWMutex
	hooks          []FilterHook
}

func NewToolchainFilter(ctx context.Context) *ToolchainFilter {
	return &ToolchainFilter{
		ctx:            ctx,
		senders:        make(map[string]*sender),
		receivers:      make(map[string]*receiver),
		channels:       make(map[string]*taskChannel),
		channelsMutex:  &sync.RWMutex{},
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

func (f *ToolchainFilter) newTaskChannel(hash string) *taskChannel {
	ctx, cancel := context.WithCancel(f.ctx)
	taskCh := &taskChannel{
		hash:       hash,
		C:          make(chan interface{}),
		rxRefCount: atomic.NewInt32(0),
		txRefCount: atomic.NewInt32(0),
		cancel:     cancel,
	}
	go func() {
		<-ctx.Done()
		// Ref count hit 0, clean up the channel to avoid a resource leak
		f.channelsMutex.Lock()
		defer f.channelsMutex.Unlock()
		close(taskCh.C)
		delete(f.channels, hash)
	}()
	return taskCh
}

func (f *ToolchainFilter) taskChannelForToolchain(tc *types.Toolchain) *taskChannel {
	f.channelsMutex.Lock()
	defer f.channelsMutex.Unlock()
	hash := tcHash(tc)
	var taskCh *taskChannel
	if c, ok := f.channels[hash]; ok {
		taskCh = c
	} else {
		f.channels[hash] = f.newTaskChannel(hash)
	}
	return taskCh
}

func (f *ToolchainFilter) AddSender(cd *Consumerd) {
	f.sendersMutex.Lock()
	defer f.sendersMutex.Unlock()
	input := make(chan interface{})
	sender := &sender{
		cd:              cd,
		unfilteredInput: input,
	}
	f.senders[cd.UUID] = sender
	for _, tc := range cd.Toolchains.GetItems() {
		taskCh := f.taskChannelForToolchain(tc)
		go taskCh.AttachSender(sender)
	}
}

func (f *ToolchainFilter) AddReceiver(agent *Agent) <-chan interface{} {
	f.receiversMutex.Lock()
	defer f.receiversMutex.Unlock()
	output := make(chan interface{})
	receiver := &receiver{
		agent:          agent,
		filteredOutput: output,
	}
	f.receivers[agent.UUID] = receiver
	for _, tc := range agent.Toolchains.GetItems() {
		taskCh := f.taskChannelForToolchain(tc)
		go taskCh.AttachReceiver(receiver)
	}
	return output
}

func (f *ToolchainFilter) UpdateSenderToolchains(
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
			f.taskChannelForToolchain(oldTc).cancel()
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
				f.taskChannelForToolchain(newTc).AttachSender(sender)
			}()
		}
	}

	sender.cd.Toolchains = newToolchains
}

func (f *ToolchainFilter) Send(ctx context.Context, req *types.CompileRequest) error {
	taskCh := f.taskChannelForToolchain(req.GetToolchain())
	for _, hook := range f.hooks {
		switch hook.PreReceive(taskCh, req) {
		case ProcessRequestNormally:
		case RejectRequest:
			return ErrRequestRejected
		case RequestIntercepted:
			return nil
		}
	}
	if taskCh.rxRefCount.Load() == 0 {
		return ErrNoAgents
	}
	select {
	case taskCh.C <- req:
		return nil
	case <-ctx.Done():
		return context.Canceled
	default:
		return ErrStreamClosed
	}
}
