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

package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/toolchains"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AgentServer struct {
	metrics.StatusController
	srvContext       context.Context
	lg               *zap.SugaredLogger
	tcStore          *toolchains.Store
	tcRunStore       *run.ToolchainRunnerStore
	metricsProvider  clients.MetricsProvider
	toolchainFinders []toolchains.FinderWithOptions
	toolchainRunners []run.StoreAddFunc
	schedulerClient  types.SchedulerClient
	monitorClient    types.MonitorClient
	usageLimits      *metrics.UsageLimits
	runningTasks     *atomic.Int32
	cfsQuota         int64
	cfsPeriod        int64
}

type AgentServerOptions struct {
	toolchainFinders []toolchains.FinderWithOptions
	toolchainRunners []run.StoreAddFunc
	schedulerClient  types.SchedulerClient
	monitorClient    types.MonitorClient
	usageLimits      *metrics.UsageLimits
}

type AgentServerOption func(*AgentServerOptions)

func (o *AgentServerOptions) Apply(opts ...AgentServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithToolchainFinders(args ...toolchains.FinderWithOptions) AgentServerOption {
	return func(o *AgentServerOptions) {
		o.toolchainFinders = args
	}
}

func WithToolchainRunners(args ...run.StoreAddFunc) AgentServerOption {
	return func(o *AgentServerOptions) {
		o.toolchainRunners = args
	}
}

func WithSchedulerClient(client types.SchedulerClient) AgentServerOption {
	return func(o *AgentServerOptions) {
		o.schedulerClient = client
	}
}

func WithMonitorClient(client types.MonitorClient) AgentServerOption {
	return func(o *AgentServerOptions) {
		o.monitorClient = client
	}
}

func WithUsageLimits(usageLimits *metrics.UsageLimits) AgentServerOption {
	return func(o *AgentServerOptions) {
		o.usageLimits = usageLimits
	}
}

func NewAgentServer(
	ctx context.Context,
	opts ...AgentServerOption,
) *AgentServer {
	options := AgentServerOptions{}
	options.Apply(opts...)

	runStore := run.NewToolchainRunnerStore()
	for _, add := range options.toolchainRunners {
		add(runStore)
	}

	if options.usageLimits.GetConcurrentProcessLimit() == -1 {
		options.usageLimits = &metrics.UsageLimits{
			ConcurrentProcessLimit: host.AutoConcurrentProcessLimit(),
		}
	}

	srv := &AgentServer{
		srvContext:       ctx,
		lg:               meta.Log(ctx),
		tcStore:          toolchains.Aggregate(ctx, options.toolchainFinders...),
		runningTasks:     atomic.NewInt32(0),
		tcRunStore:       runStore,
		toolchainFinders: options.toolchainFinders,
		toolchainRunners: options.toolchainRunners,
		monitorClient:    options.monitorClient,
		usageLimits:      options.usageLimits,
		schedulerClient:  options.schedulerClient,
		cfsQuota:         host.CfsQuota(),
		cfsPeriod:        host.CfsPeriod(),
	}
	srv.BeginInitialize(ctx)
	defer srv.EndInitialize()

	if options.monitorClient != nil {
		srv.metricsProvider = clients.NewMetricsProvider(ctx, options.monitorClient,
			clients.Buffered|clients.Discard,
			clients.StatusCtrl(&srv.StatusController))
	} else {
		srv.metricsProvider = clients.NewNoopMetricsProvider()
	}

	mgr := clients.NewStreamManager(ctx, srv, clients.WithStatusCtrl(
		&srv.StatusController,
		clients.Required,
	))
	go mgr.Run()

	return srv
}

func (s *AgentServer) postUsageLimits() {
	qp := &metrics.UsageLimits{
		ConcurrentProcessLimit: s.usageLimits.ConcurrentProcessLimit,
	}
	s.metricsProvider.Post(qp)
}

func (s *AgentServer) postTaskStatus() {
	ts := &metrics.TaskStatus{
		NumRunning: s.runningTasks.Load(),
	}
	s.metricsProvider.Post(ts)
}

func (s *AgentServer) postToolchains() {
	s.metricsProvider.Post(&metrics.Toolchains{
		Items: s.tcStore.ItemsList(),
	})
}

func (s *AgentServer) postCpuStats() {
	stats, err := host.CpuStats()
	if err != nil {
		s.lg.With(
			zap.Error(err),
		).Warn("Could not obtain CPU stats")
		return
	}
	s.metricsProvider.Post(&metrics.CpuStats{
		CpuUsage: &metrics.CpuUsage{
			TotalUsage: stats.CpuStats.CpuUsage.TotalUsage,
			CfsQuota:   s.cfsQuota,
			CfsPeriod:  s.cfsPeriod,
		},
		ThrottlingData: &metrics.ThrottlingData{
			Periods:          stats.CpuStats.ThrottlingData.Periods,
			ThrottledPeriods: stats.CpuStats.ThrottlingData.ThrottledPeriods,
			ThrottledTime:    stats.CpuStats.ThrottlingData.ThrottledTime,
		},
	})
}

func (s *AgentServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")

	util.RunPeriodic(s.srvContext, time.Second/6, 2.0, false,
		s.postTaskStatus)
	util.RunPeriodic(s.srvContext, 1*time.Second, -1, true,
		s.postCpuStats)
	util.RunPeriodic(s.srvContext, 5*time.Second, 0.5, true,
		s.postUsageLimits, s.postToolchains)
}

func (s *AgentServer) HandleStream(stream grpc.ClientStream) error {
	s.lg.Info("Streaming tasks from scheduler")
	defer s.lg.Warn("Task stream closed")
	for {
		compileRequest := &types.CompileRequest{}
		err := stream.RecvMsg(compileRequest)
		if err != nil {
			if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
				s.lg.Debug(err)
			} else {
				s.lg.Error(err)
			}
			return err
		}
		streamCtx := stream.Context()
		go func() {
			err := stream.SendMsg(s.compile(streamCtx, compileRequest))
			if err != nil {
				s.lg.With(
					zap.Error(err),
					zap.String("id", compileRequest.RequestID),
				).Warn("Task completed, but could not be returned")
			}
		}()
	}
}

func (s *AgentServer) TryConnect() (grpc.ClientStream, error) {
	tcs := s.tcStore.ItemsList()
	md := toolchains.CreateMetadata(&metrics.Toolchains{
		Items: tcs,
	})
	ctx := metadata.NewOutgoingContext(s.srvContext, md)
	return s.schedulerClient.StreamIncomingTasks(ctx)
}

func (s *AgentServer) Target() string {
	return "scheduler"
}

func (s *AgentServer) compile(
	ctx context.Context,
	req *types.CompileRequest,
) *types.CompileResponse {
	makeInternalErr := func(err string) *types.CompileResponse {
		return &types.CompileResponse{
			RequestID:     req.RequestID,
			CompileResult: types.CompileResponse_InternalError,
			Data: &types.CompileResponse_Error{
				Error: err,
			},
		}
	}

	s.lg.Debug("Handling compile request")
	if err := meta.CheckContext(ctx); err != nil {
		return makeInternalErr(err.Error())
	}

	s.runningTasks.Inc()
	defer s.runningTasks.Dec()
	span, sctx, err := servers.StartSpanFromServer(ctx, "compile")
	if err != nil {
		s.lg.Error(err)
	} else {
		defer span.Finish()
	}

	runner, err := s.tcRunStore.Get(req.GetToolchain().Kind)
	if err != nil {
		return makeInternalErr("No toolchain runner available")
	}

	tc, err := s.tcStore.TryMatch(req.GetToolchain())
	if err != nil {
		return makeInternalErr(err.Error())
	}

	// Swap remote toolchain with the local toolchain in case the executable
	// path is different locally
	req.Toolchain = tc
	resp, err := runner.RecvRemote().Process(run.PairContext{
		ServerContext: s.srvContext,
		ClientContext: sctx,
	}, req)
	if err != nil {
		return makeInternalErr(err.Error())
	}
	return resp.(*types.CompileResponse)
}
