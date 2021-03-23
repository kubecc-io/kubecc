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

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type AgentServer struct {
	srvContext       context.Context
	executor         run.Executor
	lg               *zap.SugaredLogger
	tcStore          *toolchains.Store
	tcRunStore       *run.ToolchainRunnerStore
	metricsProvider  metrics.Provider
	toolchainFinders []toolchains.FinderWithOptions
	toolchainRunners []run.StoreAddFunc
	schedulerClient  types.SchedulerClient
	monitorClient    types.MonitorClient
	usageLimits      *metrics.UsageLimits
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

	srv := &AgentServer{
		srvContext:       ctx,
		lg:               meta.Log(ctx),
		tcStore:          toolchains.Aggregate(ctx, options.toolchainFinders...),
		executor:         run.NewQueuedExecutor(run.WithUsageLimits(options.usageLimits)),
		tcRunStore:       runStore,
		toolchainFinders: options.toolchainFinders,
		toolchainRunners: options.toolchainRunners,
		monitorClient:    options.monitorClient,
		usageLimits:      options.usageLimits,
		schedulerClient:  options.schedulerClient,
	}

	if options.monitorClient != nil {
		srv.metricsProvider = clients.NewMonitorProvider(ctx, options.monitorClient,
			clients.Buffered|clients.Discard)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}

	mgr := servers.NewStreamManager(ctx, srv)
	go mgr.Run()

	return srv
}

func (s *AgentServer) postUsageLimits() {
	qp := &metrics.UsageLimits{}
	s.executor.CompleteUsageLimits(qp)
	s.metricsProvider.Post(qp)
}

func (s *AgentServer) postTaskStatus() {
	ts := &metrics.TaskStatus{}
	s.executor.CompleteTaskStatus(ts)
	s.metricsProvider.Post(ts)
}

func (s *AgentServer) postToolchains() {
	s.metricsProvider.Post(&metrics.Toolchains{
		Items: s.tcStore.ItemsList(),
	})
}

func (s *AgentServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postUsageLimits()
	s.postToolchains()

	fastTimer := util.NewJitteredTimer(time.Second/6, 2.0)
	go func() {
		for {
			<-fastTimer
			s.postTaskStatus()
		}
	}()

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.5)
	go func() {
		for {
			<-slowTimer
			s.postUsageLimits()
		}
	}()
}

func (s *AgentServer) SetUsageLimits(
	ctx context.Context,
	usageLimits *metrics.UsageLimits,
) (*types.Empty, error) {
	s.executor.(*run.QueuedExecutor).SetUsageLimits(usageLimits)
	s.usageLimits = usageLimits
	s.postUsageLimits()
	return &types.Empty{}, nil
}

func (s *AgentServer) HandleStream(stream grpc.ClientStream) error {
	s.lg.Info("Streaming tasks from scheduler")
	defer s.lg.Warn("Task stream closed")
	for {
		compileRequest := &types.CompileRequest{}
		err := stream.RecvMsg(compileRequest)
		if err != nil {
			if errors.Is(err, io.EOF) {
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
				).Error("Error sending response to scheduler")
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
	resp, err := runner.RecvRemote().Process(run.Contexts{
		ServerContext: s.srvContext,
		ClientContext: sctx,
	}, req)
	if err != nil {
		return makeInternalErr(err.Error())
	}
	return resp.(*types.CompileResponse)
}
