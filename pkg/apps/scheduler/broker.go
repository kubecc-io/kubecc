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
	"io"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/onsi/ginkgo"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Broker struct {
	srvContext      context.Context
	lg              *zap.SugaredLogger
	completedTasks  *atomic.Int64
	failedTasks     *atomic.Int64
	requestCount    *atomic.Int64
	cacheHitCount   *atomic.Int64
	cacheMissCount  *atomic.Int64
	requestQueue    chan *types.CompileRequest
	responseQueue   chan *types.CompileResponse
	agents          map[string]*Agent
	consumerds      map[string]*Consumerd
	agentsMutex     *sync.RWMutex
	consumerdsMutex *sync.RWMutex
	router          *Router
	cacheClient     types.CacheClient
	hashSrv         *util.HashServer
	pendingRequests sync.Map // map[uuid string]*Consumerd
	tcWatcher       ToolchainWatcher
}

type BrokerOptions struct {
	cacheClient types.CacheClient
}

type BrokerOption func(*BrokerOptions)

func (o *BrokerOptions) Apply(opts ...BrokerOption) {
	for _, op := range opts {
		op(o)
	}
}

func CacheClient(client types.CacheClient) BrokerOption {
	return func(o *BrokerOptions) {
		o.cacheClient = client
	}
}

func NewBroker(
	ctx context.Context,
	tcw ToolchainWatcher,
	opts ...BrokerOption,
) *Broker {
	options := BrokerOptions{}
	options.Apply(opts...)

	b := &Broker{
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
		hashSrv:         util.NewHashServer(),
		tcWatcher:       tcw,
		cacheHitCount:   atomic.NewInt64(0),
		cacheMissCount:  atomic.NewInt64(0),
		cacheClient:     options.cacheClient,
	}

	routerOptions := []RouterOption{}
	if options.cacheClient != nil {
		routerOptions = append(routerOptions, WithHooks(b))
	} else {
		b.lg.Warn("Cache server not configured")
	}
	b.router = NewRouter(ctx, routerOptions...)

	go b.handleResponseQueue()
	return b
}

var ErrTokenImbalance = errors.New("Token imbalance")

func (b *Broker) handleAgentStream(
	stream types.Scheduler_StreamIncomingTasksServer,
	filterOutput <-chan *types.CompileRequest,
) {
	b.agentsMutex.RLock()
	uuid := meta.UUID(stream.Context())
	agent := b.agents[uuid]
	b.agentsMutex.RUnlock()
	availableTokens := make(chan struct{}, MaxTokens)
	lockedTokens := make(chan struct{}, MaxTokens)
	agent.Lock()
	for i := 0; i < int(agent.UsageLimits.ConcurrentProcessLimit); i++ {
		availableTokens <- struct{}{}
	}
	agent.Unlock()

	go func() {
		defer ginkgo.GinkgoRecover()
		b.lg.Debug("Handling agent stream (send)")
		defer b.lg.Debug("Agent stream done (send)")
		for {
			// Attempt to remove a token from the agent's token pool. This represents
			// exclusive access to a share of the agent's resources. The token will
			// be put back into the buffered channel once a response has been
			// received. If there are no tokens left, this will block until one
			// becomes available or the stream is done.
			select {
			case token := <-availableTokens:
				lockedTokens <- token
			case <-stream.Context().Done():
				return
			}
			req, ok := <-filterOutput
			if !ok {
				// Output closed
				return
			}
			err := stream.Send(req)
			if err != nil {
				if errors.Is(err, io.EOF) {
					b.lg.Debug(err)
				} else {
					b.lg.Error(err)
				}
				return
			}
		}
	}()
	go func() {
		defer ginkgo.GinkgoRecover()
		b.lg.Debug("Handling agent stream (recv)")
		defer b.lg.Debug("Agent stream done (recv)")

		for {
			resp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					b.lg.Debug(err)
				} else {
					b.lg.Error(err)
				}
				return
			}

			select {
			case token := <-lockedTokens:
				availableTokens <- token
			default:
				b.lg.With(
					types.ShortID(agent.UUID),
				).Error(ErrTokenImbalance)
			}
			agent.CompletedTasks.Inc()
			b.responseQueue <- resp
		}
	}()
}

func (b *Broker) handleConsumerdStream(
	srv types.Scheduler_StreamOutgoingTasksServer,
) {
	b.consumerdsMutex.RLock()
	uuid := meta.UUID(srv.Context())
	cd := b.consumerds[uuid]
	b.consumerdsMutex.RUnlock()

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
		b.requestCount.Inc()
		b.pendingRequests.Store(req.RequestID, cd)
		if err := b.router.Route(srv.Context(), req); err != nil {
			b.lg.With(
				zap.Error(err),
			).Error("Encountered an error while routing compile request")
			b.pendingRequests.Delete(req.RequestID)
			b.requestCount.Dec()
			b.responseQueue <- &types.CompileResponse{
				RequestID:     req.GetRequestID(),
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
		if value, ok := b.pendingRequests.LoadAndDelete(resp.RequestID); ok {
			b.lg.With(
				zap.String("request", resp.RequestID),
			).Debug("Sending response to consumerd")
			consumerd := value.(*Consumerd)
			consumerd.CompletedTasks.Inc()
			switch resp.CompileResult {
			case types.CompileResponse_Fail, types.CompileResponse_InternalError:
				b.failedTasks.Inc()
			case types.CompileResponse_Success:
				b.completedTasks.Inc()
			}
			err := consumerd.Stream.Send(resp)
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

func (b *Broker) NewAgentTaskStream(
	stream types.Scheduler_StreamIncomingTasksServer,
) {
	b.agentsMutex.Lock()
	streamCtx := stream.Context()
	id := meta.UUID(streamCtx)
	tcChan := b.tcWatcher.WatchToolchains(id)

	b.lg.With(types.ShortID(id)).Info("Agent connected, waiting for toolchains")
	tcs := <-tcChan
	b.lg.With(types.ShortID(id)).Info("Toolchains received")

	agent := &Agent{
		remoteInfo: remoteInfoFromContext(streamCtx),
		RWMutex:    &sync.RWMutex{},
		Stream:     stream,
		Toolchains: tcs,
	}
	agent.UsageLimits = &metrics.UsageLimits{
		ConcurrentProcessLimit: agent.SystemInfo.CpuThreads,
	}
	b.agents[agent.UUID] = agent
	b.agentsMutex.Unlock()

	filterOutput := b.router.AddReceiver(agent)
	b.handleAgentStream(stream, filterOutput)

	go func() {
		<-streamCtx.Done()

		b.agentsMutex.RLock()
		defer b.agentsMutex.RUnlock()
		delete(b.agents, agent.UUID)
	}()
}

func (b *Broker) NewConsumerdTaskStream(
	stream types.Scheduler_StreamOutgoingTasksServer,
) {
	b.consumerdsMutex.Lock()
	streamCtx := stream.Context()
	id := meta.UUID(streamCtx)
	tcChan := b.tcWatcher.WatchToolchains(id)

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
	b.consumerdsMutex.Unlock()

	b.router.AddSender(cd)
	b.handleConsumerdStream(stream)

	go func() {
		select {
		case tcs := <-tcChan:
			b.router.UpdateSenderToolchains(cd.UUID, tcs)
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

func (b *Broker) CalcAgentStats() <-chan []agentStats {
	stats := make(chan []agentStats)
	go func() {
		statsList := []agentStats{}
		b.agentsMutex.RLock()
		defer b.agentsMutex.RUnlock()

		for uuid, agent := range b.agents {
			agent.RLock()
			defer agent.RUnlock()

			stats := agentStats{
				agentCtx:        agent.Context,
				agentTasksTotal: &metrics.AgentTasksTotal{},
			}

			stats.agentTasksTotal.Total = agent.CompletedTasks.Load()
			stats.agentTasksTotal.UUID = uuid

			statsList = append(statsList, stats)
		}

		stats <- statsList
	}()
	return stats
}

func (b *Broker) CalcConsumerdStats() <-chan []consumerdStats {
	stats := make(chan []consumerdStats)
	go func() {
		statsList := []consumerdStats{}
		b.consumerdsMutex.RLock()
		defer b.consumerdsMutex.RUnlock()

		for uuid, cd := range b.consumerds {
			cd.RLock()
			defer cd.RUnlock()

			total := &metrics.ConsumerdTasksTotal{
				Total: cd.CompletedTasks.Load(),
			}
			total.UUID = uuid
			statsList = append(statsList, consumerdStats{
				consumerdCtx:       cd.Context,
				cdRemoteTasksTotal: total,
			})
		}

		stats <- statsList
	}()
	return stats
}

func (b *Broker) TaskStats() taskStats {
	return taskStats{
		completedTotal: &metrics.TasksCompletedTotal{
			Total: b.completedTasks.Load(),
		},
		failedTotal: &metrics.TasksFailedTotal{
			Total: b.failedTasks.Load(),
		},
		requestsTotal: &metrics.SchedulingRequestsTotal{
			Total: b.requestCount.Load(),
		},
	}
}

func (b *Broker) PreReceive(
	rt *route,
	req *types.CompileRequest,
) (action HookAction) {
	defer func(a *HookAction) {
		switch action {
		case ProcessRequestNormally:
			b.lg.Debug("Cache Miss")
			b.cacheMissCount.Inc()
		case RequestIntercepted:
			b.lg.Info("Cache Hit")
			b.cacheHitCount.Inc()
		}
	}(&action)

	if b.cacheClient == nil {
		action = ProcessRequestNormally
		return
	}
	reqHash := b.hashSrv.Hash(req)
	obj, err := b.cacheClient.Pull(b.srvContext, &types.PullRequest{
		Key: &types.CacheKey{
			Hash: reqHash,
		},
	})
	switch status.Code(err) {
	case codes.OK:
		b.responseQueue <- &types.CompileResponse{
			RequestID:     req.GetRequestID(),
			CompileResult: types.CompileResponse_Success,
			Data: &types.CompileResponse_CompiledSource{
				CompiledSource: obj.GetData(),
			},
		}
		action = RequestIntercepted
		return
	case codes.NotFound:
	default:
		b.lg.With(
			zap.Error(err),
		).Error("Error querying cache server")
	}
	action = ProcessRequestNormally
	return
}
