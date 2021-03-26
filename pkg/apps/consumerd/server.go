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

package consumerd

import (
	"context"
	"io/fs"
	"time"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

type consumerdServer struct {
	types.UnimplementedConsumerdServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	tcRunStore          *run.ToolchainRunnerStore
	tcStore             *toolchains.Store
	storeUpdateCh       chan struct{}
	schedulerClient     types.SchedulerClient
	monitorClient       types.MonitorClient
	metricsProvider     metrics.Provider
	executor            run.Executor
	numConsumers        *atomic.Int32
	localTasksCompleted *atomic.Int64
	requestClient       *clients.CompileRequestClient
	streamMgr           *servers.StreamManager
}

type ConsumerdServerOptions struct {
	toolchainFinders []toolchains.FinderWithOptions
	toolchainRunners []run.StoreAddFunc
	schedulerClient  types.SchedulerClient
	monitorClient    types.MonitorClient
	usageLimits      *metrics.UsageLimits
}

type ConsumerdServerOption func(*ConsumerdServerOptions)

func (o *ConsumerdServerOptions) Apply(opts ...ConsumerdServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithToolchainFinders(args ...toolchains.FinderWithOptions) ConsumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.toolchainFinders = args
	}
}

func WithToolchainRunners(args ...run.StoreAddFunc) ConsumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.toolchainRunners = args
	}
}

func WithSchedulerClient(
	client types.SchedulerClient,
) ConsumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.schedulerClient = client
	}
}

// Note this accepts an MonitorClient even though consumerd runs
// outside the cluster.
func WithMonitorClient(
	client types.MonitorClient,
) ConsumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.monitorClient = client
	}
}

func WithUsageLimits(cpuConfig *metrics.UsageLimits) ConsumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.usageLimits = cpuConfig
	}
}

func NewConsumerdServer(
	ctx context.Context,
	opts ...ConsumerdServerOption,
) *consumerdServer {
	options := ConsumerdServerOptions{}
	options.Apply(opts...)

	runStore := run.NewToolchainRunnerStore()
	for _, add := range options.toolchainRunners {
		add(runStore)
	}
	srv := &consumerdServer{
		srvContext:          ctx,
		lg:                  meta.Log(ctx),
		tcStore:             toolchains.Aggregate(ctx, options.toolchainFinders...),
		tcRunStore:          runStore,
		storeUpdateCh:       make(chan struct{}, 1),
		numConsumers:        atomic.NewInt32(0),
		localTasksCompleted: atomic.NewInt64(0),
		executor:            NewSplitQueue(ctx, options.monitorClient),
		schedulerClient:     options.schedulerClient,
		monitorClient:       options.monitorClient,
		requestClient:       clients.NewCompileRequestClient(ctx, nil),
	}
	srv.streamMgr = servers.NewStreamManager(srv.srvContext, srv)

	if options.monitorClient != nil {
		srv.metricsProvider = clients.NewMonitorProvider(ctx, options.monitorClient,
			clients.Buffered|clients.Discard)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}

	go srv.runRequestClient()
	go srv.streamMgr.Run()
	return srv
}

func (c *consumerdServer) runRequestClient() {
	av := clients.NewAvailabilityChecker(clients.ComponentFilter(types.Scheduler))
	clients.WatchAvailability(c.srvContext, c.monitorClient, av)

	for {
		ctx := av.EnsureAvailable()
		c.lg.Debug("Remote is now available: Trying to connect immediately")
		// Try to connect to the scheduler immediately in case we are in a backoff
		c.streamMgr.TryImmediately()
		<-ctx.Done()
	}
}

func (c *consumerdServer) applyToolchainToReq(req *types.RunRequest) error {
	path := req.GetPath()
	if path == "" {
		return status.Error(codes.InvalidArgument, "No compiler path given")
	}
	tc, err := c.tcStore.Find(path)
	defer func(r *types.RunRequest) {
		r.Compiler = &types.RunRequest_Toolchain{
			Toolchain: tc,
		}
	}(req)
	sendUpdate := func() {
		select {
		case c.storeUpdateCh <- struct{}{}:
		default:
		}
	}

	if err != nil {
		// Add a new toolchain
		c.lg.Info("Consumer sent unknown toolchain; attempting to add it")
		tc, err = c.tcStore.Add(path, toolchains.ExecQuerier{})
		if err != nil {
			c.lg.With(zap.Error(err)).Error("Could not add toolchain")
			return status.Error(codes.InvalidArgument,
				errors.WithMessage(err, "Could not add toolchain").Error())
		}
		defer sendUpdate()
		c.lg.With("compiler", tc.Executable).Info("New toolchain added")
		return nil
	}

	// err and updated are independent
	err, updated := c.tcStore.UpdateIfNeeded(tc)
	if updated {
		defer sendUpdate()
	}
	if err != nil {
		// The toolchain was updated and is no longer valid
		c.lg.With(
			"compiler", tc.Executable,
			zap.Error(err),
		).Error("Error when updating toolchain")
		if errors.As(err, &fs.PathError{}) {
			return status.Error(codes.NotFound,
				errors.WithMessage(err, "Compiler no longer exists").Error())
		}
		return status.Error(codes.InvalidArgument,
			errors.WithMessage(err, "Toolchain is no longer valid").Error())
	}
	return nil
}

func (s *consumerdServer) postUsageLimits() {
	qp := &metrics.UsageLimits{}
	s.executor.CompleteUsageLimits(qp)
	s.metricsProvider.Post(qp)
}

func (s *consumerdServer) postTaskStatus() {
	ts := &metrics.TaskStatus{}
	s.executor.CompleteTaskStatus(ts)
	s.metricsProvider.Post(ts)
}

func (s *consumerdServer) postTotals() {
	s.metricsProvider.Post(&metrics.LocalTasksCompleted{
		Total: s.localTasksCompleted.Load(),
	})
}

func (s *consumerdServer) postToolchains() {
	s.metricsProvider.Post(&metrics.Toolchains{
		Items: s.tcStore.ItemsList(),
	})
}

func (s *consumerdServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postUsageLimits()
	s.postToolchains()

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.25)
	go func() {
		for {
			<-slowTimer
			s.postUsageLimits()
			s.postTotals()
		}
	}()

	fastTimer := util.NewJitteredTimer(time.Second/6, 2.0)
	go func() {
		for {
			<-fastTimer
			s.postTaskStatus()
		}
	}()

	go func() {
		for {
			<-s.storeUpdateCh
			s.postToolchains()
		}
	}()
}

func (c *consumerdServer) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	if err := meta.CheckContext(ctx); err != nil {
		return nil, err
	}

	c.lg.Debug("Running request")
	err := c.applyToolchainToReq(req)
	if err != nil {
		return nil, err
	}
	// todo
	// rootContext := tracing.ContextWithTracer(ctx, tracer)
	// for _, env := range req.Env {
	// 	spl := strings.Split(env, "=")
	// 	if len(spl) == 2 && spl[0] == "KUBECC_MAKE_PID" {
	// 		pid, err := strconv.Atoi(spl[1])
	// 		if err != nil {
	// 			c.lg.Debug("Invalid value for KUBECC_MAKE_PID, should be a number")
	// 			break
	// 		}
	// 		rootContext = tracing.PIDSpanContext(tracer, pid)
	// 	}
	// }

	span, sctx, err := servers.StartSpanFromServer(ctx, "run")
	if err != nil {
		c.lg.Error(err)
	} else {
		ctx = sctx
		defer span.Finish()
	}

	if req.UID == 0 || req.GID == 0 {
		return nil, status.Error(codes.InvalidArgument,
			"UID or GID cannot be 0")
	}

	runner, err := c.tcRunStore.Get(req.GetToolchain().Kind)
	if err != nil {
		return nil, status.Error(codes.Unavailable,
			"No toolchain runner available")
	}

	ap := runner.NewArgParser(c.srvContext, req.Args)
	ap.Parse()

	ctxs := run.Contexts{
		ServerContext: c.srvContext,
		ClientContext: ctx,
	}

	st := &SplitTask{
		Local:  run.PackageRequest(runner.RunLocal(ap), ctxs, req),
		Remote: run.PackageRequest(runner.SendRemote(ap, c.requestClient), ctxs, req),
	}

	// Exec blocks only while the task is queued
	if err := c.executor.Exec(st); err != nil {
		panic(err)
	}

	// Wait will block until either the local or remote task completes
	resp, err := st.Wait()

	if err != nil {
		return nil, err
	}
	return resp.(*types.RunResponse), nil
}

func (c *consumerdServer) HandleStream(stream grpc.ClientStream) error {
	c.requestClient.LoadNewStream(
		stream.(types.Scheduler_StreamOutgoingTasksClient))
	select {
	case <-c.srvContext.Done():
		return c.srvContext.Err()
	case <-stream.Context().Done():
		return stream.Context().Err()
	}
}

func (c *consumerdServer) TryConnect() (grpc.ClientStream, error) {
	return c.schedulerClient.StreamOutgoingTasks(
		c.srvContext, grpc.UseCompressor(gzip.Name))
}

func (c *consumerdServer) Target() string {
	return "scheduler"
}
