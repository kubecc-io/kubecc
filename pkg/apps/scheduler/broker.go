package scheduler

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type worker struct {
	agent     *Agent
	taskQueue <-chan *types.CompileRequest
}

func (w *worker) stream() {
	for {
		select {
		case req := <-w.taskQueue:
			w.agent.Stream.Send(req)
		case <-w.agent.Context.Done():
			return
		}
	}
}

type Broker struct {
	srvContext      context.Context
	lg              *zap.SugaredLogger
	completedTasks  *atomic.Int64
	failedTasks     *atomic.Int64
	requestCount    *atomic.Int64
	requestQueue    chan *types.CompileRequest
	responseQueue   chan *types.CompileResponse
	agents          map[string]*Agent
	consumerds      map[string]*Consumerd
	agentsMutex     *sync.RWMutex
	consumerdsMutex *sync.RWMutex
	filter          *ToolchainFilter
	monClient       types.MonitorClient
	pendingRequests sync.Map // map[uuid string]Scheduler_StreamOutgoingTasksServer
}

func NewBroker(ctx context.Context, monClient types.MonitorClient) *Broker {
	return &Broker{
		srvContext:      ctx,
		lg:              meta.Log(ctx),
		completedTasks:  atomic.NewInt64(0),
		failedTasks:     atomic.NewInt64(0),
		requestCount:    atomic.NewInt64(0),
		requestQueue:    make(chan *types.CompileRequest),
		responseQueue:   make(chan *types.CompileResponse),
		agents:          make(map[string]*Agent),
		consumerds:      make(map[string]*Consumerd),
		agentsMutex:     &sync.RWMutex{},
		consumerdsMutex: &sync.RWMutex{},
		filter:          NewToolchainFilter(ctx),
		monClient:       monClient,
	}
}

func (b *Broker) watchToolchains(uuid string) chan *metrics.Toolchains {
	ch := make(chan *metrics.Toolchains)
	listener := clients.NewListener(b.srvContext, b.monClient)
	listener.OnProviderAdded(func(ctx context.Context, id string) {
		if id != uuid {
			return
		}
		listener.OnValueChanged(id, func(tc *metrics.Toolchains) {
			ch <- tc
		})
		<-ctx.Done()
	})
	return ch
}

func (b *Broker) handleAgentStream(
	srv types.Scheduler_StreamIncomingTasksServer,
	filterOutput <-chan interface{},
) {
	go func() {
		b.lg.Debug("Handling agent stream (send)")
		defer b.lg.Debug("Agent stream done (send)")
		for {
			select {
			case req, ok := <-filterOutput:
				if !ok {
					// Output closed
					return
				}
				err := srv.Send(req.(*types.CompileRequest))
				if err != nil {
					if errors.Is(err, io.EOF) {
						b.lg.Debug(err)
					} else {
						b.lg.Error(err)
					}
					return
				}
			}
		}
	}()
	go func() {
		b.lg.Debug("Handling agent stream (recv)")
		defer b.lg.Debug("Agent stream done (recv)")

		for {
			resp, err := srv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					b.lg.Debug(err)
				} else {
					b.lg.Error(err)
				}
				return
			}
			b.responseQueue <- resp
		}
	}()
}

func (b *Broker) handleConsumerdStream(
	srv types.Scheduler_StreamOutgoingTasksServer,
) {
	b.lg.Debug("Handling consumerd stream (recv)")
	defer b.lg.Debug("Consumerd stream done (recv)")

	for {
		req, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				b.lg.Debug(err)
			} else {
				b.lg.Error(err)
			}
			return
		}
		b.pendingRequests.Store(req.RequestID, srv)
		if err := b.filter.Send(req); err != nil {
			b.pendingRequests.Delete(req.RequestID)
			b.responseQueue <- &types.CompileResponse{
				RequestID:     req.RequestID,
				CompileResult: types.CompileResponse_InternalError,
				Data: &types.CompileResponse_Error{
					Error: err.Error(),
				},
			}
		}
	}
}

func (b *Broker) handleResponseQueue() {
	for {
		resp, open := <-b.responseQueue
		if !open {
			b.lg.Debug("Response queue closed")
			return
		}
		if stream, ok := b.pendingRequests.LoadAndDelete(resp.RequestID); ok {
			err := stream.(types.Scheduler_StreamOutgoingTasksServer).Send(resp)
			if err != nil {
				b.lg.With(
					zap.Error(err),
				).Error("Error sending response")
			}
		} else {
			b.lg.With(
				"id", resp.RequestID,
			).Error("Received response for which there was no pending request")
		}
	}
}

func (b *Broker) HandleIncomingTasksStream(
	stream types.Scheduler_StreamIncomingTasksServer,
) {
	b.agentsMutex.Lock()
	defer b.agentsMutex.Unlock()
	streamCtx := stream.Context()
	id := meta.UUID(streamCtx)
	tcChan := b.watchToolchains(id)

	b.lg.With(types.ShortID(id)).Info("Agent connected, waiting for toolchains")
	tcs := <-tcChan
	b.lg.With(types.ShortID(id)).Info("Toolchains received")

	agent := &Agent{
		remoteInfo: remoteInfoFromContext(streamCtx),
		RWMutex:    &sync.RWMutex{},
		Stream:     stream,
		Toolchains: tcs,
	}
	b.agents[agent.UUID] = agent

	b.agents[agent.UUID] = agent
	filterOutput := b.filter.AddReceiver(agent)
	b.handleAgentStream(stream, filterOutput)

	go func() {
		<-streamCtx.Done()

		b.agentsMutex.RLock()
		defer b.agentsMutex.RUnlock()
		delete(b.agents, agent.UUID)
	}()
}

func (b *Broker) HandleOutgoingTasksStream(
	stream types.Scheduler_StreamOutgoingTasksServer,
) {
	b.consumerdsMutex.Lock()
	defer b.consumerdsMutex.Unlock()
	streamCtx := stream.Context()
	id := meta.UUID(streamCtx)
	tcChan := b.watchToolchains(id)

	b.lg.With(types.ShortID(id)).Info("Consumerd connected, waiting for toolchains")
	tcs := <-tcChan
	b.lg.With(types.ShortID(id)).Info("Toolchains received")

	cd := &Consumerd{
		remoteInfo: remoteInfoFromContext(streamCtx),
		RWMutex:    &sync.RWMutex{},
		Stream:     stream,
		Toolchains: tcs,
	}
	b.consumerds[cd.UUID] = cd

	b.consumerds[cd.UUID] = cd
	b.filter.AddSender(cd)
	b.handleConsumerdStream(stream)

	go func() {
		select {
		case tcs := <-tcChan:
			b.filter.UpdateSenderToolchains(cd.UUID, tcs)
		case <-streamCtx.Done():
			return
		}
	}()

	go func() {
		<-streamCtx.Done()

		b.agentsMutex.RLock()
		defer b.agentsMutex.RUnlock()
		delete(b.agents, cd.UUID)
	}()
}
