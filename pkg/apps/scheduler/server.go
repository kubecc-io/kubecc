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
	"sync"
	"time"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type schedulerServer struct {
	types.UnimplementedSchedulerServer
	metrics.StatusController

	monClient   types.MonitorClient
	cacheClient types.CacheClient

	srvContext      context.Context
	lg              *zap.SugaredLogger
	metricsProvider clients.MetricsProvider
	broker          *Broker

	agentCount     *atomic.Int64
	consumerdCount *atomic.Int64

	condNoAgentsCancel     context.CancelFunc
	condNoCdsCancel        context.CancelFunc
	condNoAgentsCancelLock sync.Mutex
	condNoCdsCancelLock    sync.Mutex
}

type SchedulerServerOptions struct {
	monClient   types.MonitorClient
	cacheClient types.CacheClient
}

type SchedulerServerOption func(*SchedulerServerOptions)

func (o *SchedulerServerOptions) Apply(opts ...SchedulerServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithMonitorClient(monClient types.MonitorClient) SchedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.monClient = monClient
	}
}

func WithCacheClient(cacheClient types.CacheClient) SchedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.cacheClient = cacheClient
	}
}

func NewSchedulerServer(
	ctx context.Context,
	opts ...SchedulerServerOption,
) *schedulerServer {
	options := SchedulerServerOptions{}
	options.Apply(opts...)

	srv := &schedulerServer{
		srvContext:     ctx,
		lg:             meta.Log(ctx),
		monClient:      options.monClient,
		cacheClient:    options.cacheClient,
		agentCount:     atomic.NewInt64(0),
		consumerdCount: atomic.NewInt64(0),
		broker: NewBroker(ctx,
			NewDefaultToolchainWatcher(ctx, options.monClient),
			CacheClient(options.cacheClient),
			MonitorClient(options.monClient),
		),
	}
	srv.BeginInitialize()
	srv.applyNoAgentsCond()
	srv.applyNoCdsCond()
	defer srv.EndInitialize()

	if options.monClient != nil {
		srv.metricsProvider = clients.NewMetricsProvider(
			ctx, options.monClient, clients.Discard,
			clients.StatusCtrl(&srv.StatusController))
	} else {
		srv.metricsProvider = clients.NewNoopMetricsProvider()
	}
	return srv
}

func (s *schedulerServer) applyNoAgentsCond() {
	s.lg.Debug("Applying status condition [no agents]")
	ctx, cancel := context.WithCancel(context.Background())
	s.ApplyCondition(ctx, metrics.StatusConditions_MissingOptionalComponent,
		"No agents connected")
	s.condNoAgentsCancelLock.Lock()
	s.condNoAgentsCancel = cancel
	s.condNoAgentsCancelLock.Unlock()
}

func (s *schedulerServer) applyNoCdsCond() {
	s.lg.Debug("Applying status condition [no consumerds]")
	ctx, cancel := context.WithCancel(context.Background())
	s.ApplyCondition(ctx, metrics.StatusConditions_MissingOptionalComponent,
		"No consumerds connected")
	s.condNoCdsCancelLock.Lock()
	s.condNoCdsCancel = cancel
	s.condNoCdsCancelLock.Unlock()
}

// agent <-> scheduler
func (s *schedulerServer) StreamIncomingTasks(
	srv types.Scheduler_StreamIncomingTasksServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	err := s.broker.NewAgentTaskStream(srv)
	if err != nil {
		s.lg.With(zap.Error(err)).Error("Agent error")
	}
	if s.agentCount.Inc() == 1 {
		s.lg.Debug("Removing status condition [no agents]")
		s.condNoAgentsCancelLock.Lock()
		s.condNoAgentsCancel()
		s.condNoAgentsCancelLock.Unlock()
	}
	defer func() {
		if s.agentCount.Dec() == 0 {
			s.applyNoAgentsCond()
		}
	}()

	s.postPreferredUsageLimits()
	defer s.postPreferredUsageLimits()

	select {
	case <-srv.Context().Done():
	case <-s.srvContext.Done():
	}

	return nil
}

// consumerd <-> scheduler
func (s *schedulerServer) StreamOutgoingTasks(
	srv types.Scheduler_StreamOutgoingTasksServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	err := s.broker.NewConsumerdTaskStream(srv)
	if err != nil {
		s.lg.With(zap.Error(err)).Error("Consumerd error")
	}
	if s.consumerdCount.Inc() == 1 {
		s.lg.Debug("Removing status condition [no consumerds]")
		s.condNoCdsCancelLock.Lock()
		s.condNoCdsCancel()
		s.condNoCdsCancelLock.Unlock()
	}
	s.metricsProvider.Post(&metrics.ConsumerdCount{
		Count: s.consumerdCount.Load(),
	})
	defer func() {
		if s.consumerdCount.Dec() == 0 {
			s.applyNoCdsCond()
		}
		s.metricsProvider.Post(&metrics.ConsumerdCount{
			Count: s.consumerdCount.Load(),
		})
	}()

	select {
	case <-srv.Context().Done():
	case <-s.srvContext.Done():
	}

	return nil
}

func (s *schedulerServer) postCounts() {
	s.metricsProvider.Post(&metrics.AgentCount{
		Count: s.agentCount.Load(),
	})
	s.metricsProvider.Post(&metrics.ConsumerdCount{
		Count: s.consumerdCount.Load(),
	})
}

func (s *schedulerServer) postTotals() {
	stats := s.broker.TaskStats()
	s.metricsProvider.Post(stats.completedTotal)
	s.metricsProvider.Post(stats.failedTotal)
	s.metricsProvider.Post(stats.requestsTotal)
}

func (s *schedulerServer) postAgentStats() {
	for _, stat := range <-s.broker.CalcAgentStats() {
		s.metricsProvider.PostContext(stat.agentTasksTotal, stat.agentCtx)
	}
}

func (s *schedulerServer) postConsumerdStats() {
	for _, stat := range <-s.broker.CalcConsumerdStats() {
		s.metricsProvider.PostContext(stat.cdRemoteTasksTotal, stat.consumerdCtx)
	}
}

func (s *schedulerServer) postPreferredUsageLimits() {
	s.metricsProvider.Post(&metrics.PreferredUsageLimits{
		ConcurrentProcessLimit: s.broker.calcPreferredUsageLimits(),
	})
}

func (s *schedulerServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postPreferredUsageLimits()

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.5) // 5-7.5 sec
	go func() {
		for {
			<-slowTimer
			s.postCounts()
			s.postTotals()
			s.postAgentStats()
			s.postConsumerdStats()
		}
	}()
}

func (s *schedulerServer) GetRoutes(
	ctx context.Context,
	_ *types.Empty,
) (*types.RouteList, error) {
	return s.broker.router.GetRoutes(), nil
}
