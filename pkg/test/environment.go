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

package test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/imdario/mergo"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/apps/agent"
	"github.com/kubecc-io/kubecc/pkg/apps/cachesrv"
	"github.com/kubecc-io/kubecc/pkg/apps/consumerd"
	"github.com/kubecc-io/kubecc/pkg/apps/monitor"
	"github.com/kubecc-io/kubecc/pkg/apps/scheduler"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/storage"
	"github.com/kubecc-io/kubecc/pkg/toolchains"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util/ctxutil"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type SpawnOptions struct {
	config           config.KubeccSpec
	agentOptions     []agent.AgentServerOption
	consumerdOptions []consumerd.ConsumerdServerOption
	schedulerOptions []scheduler.SchedulerServerOption
	cacheOptions     []cachesrv.CacheServerOption
	name             string
	uuid             string
	waitForReady     bool
}

type SpawnOption func(*SpawnOptions)

func (o *SpawnOptions) Apply(opts ...SpawnOption) {
	for _, op := range opts {
		op(o)
	}
}

// WithConfig is used to override specific config values when spawning a component.
// Any values provided here will be merged with the defaults.
func WithConfig(cfg interface{}) SpawnOption {
	return func(o *SpawnOptions) {
		switch conf := cfg.(type) {
		case config.KubeccSpec:
			o.config = conf
		case config.AgentSpec:
			o.config = config.KubeccSpec{Agent: conf}
		case config.ConsumerdSpec:
			o.config = config.KubeccSpec{Consumerd: conf}
		case config.SchedulerSpec:
			o.config = config.KubeccSpec{Scheduler: conf}
		case config.CacheSpec:
			o.config = config.KubeccSpec{Cache: conf}
		case config.MonitorSpec:
			o.config = config.KubeccSpec{Monitor: conf}
		}
	}
}

// WithName provides a component name. Each component has a default name of
// "default" which can be used to dial that component when creating the
// in-memory connection.
func WithName(name string) SpawnOption {
	return func(o *SpawnOptions) {
		o.name = name
	}
}

func WaitForReady() SpawnOption {
	return func(o *SpawnOptions) {
		o.waitForReady = true
	}
}

func WithAgentOptions(opts ...agent.AgentServerOption) SpawnOption {
	return func(o *SpawnOptions) {
		o.agentOptions = opts
	}
}

func WithConsumerdOptions(opts ...consumerd.ConsumerdServerOption) SpawnOption {
	return func(o *SpawnOptions) {
		o.consumerdOptions = opts
	}
}

func WithSchedulerOptions(opts ...scheduler.SchedulerServerOption) SpawnOption {
	return func(o *SpawnOptions) {
		o.schedulerOptions = opts
	}
}

func WithCacheOptions(opts ...cachesrv.CacheServerOption) SpawnOption {
	return func(o *SpawnOptions) {
		o.cacheOptions = opts
	}
}

func WithUUID(uuid string) SpawnOption {
	return func(o *SpawnOptions) {
		o.uuid = uuid
	}
}

type Environment interface {
	// Context returns the top level environment context.
	Context() context.Context

	// DefaultConfig returns a reasonable set of default values for testing.
	DefaultConfig() config.KubeccSpec

	// Dial creates a grpc client connection to the component named by the component
	// type and an optional name which defaults to "default" if not provided.
	Dial(ctx context.Context, c types.Component, name ...string) *grpc.ClientConn

	// Shutdown terminates all components running in the environment by canceling
	// the top-level parent context.
	Shutdown()

	// WaitForReady blocks until the component with the given UUID posts a
	// Ready status. Requires a running monitor.
	WaitForReady(uuid string, timeout ...time.Duration)

	// MetricF returns a function that when called will populate out with the
	// current values of the metric of the matching type.
	MetricF(srvCtx context.Context, out proto.Message) func() (proto.Message, error)

	// Serve starts a server component. This function should not block.
	Serve(ctx context.Context, server interface{}, name string)
}

type Spawner interface {
	// SpawnAgent adds a new agent to the environment. If the returned context
	// is canceled, the agent will be stopped.
	SpawnAgent(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc)

	// SpawnScheduler adds a new scheduler to the environment. If the returned context
	// is canceled, the scheduler will be stopped.
	SpawnScheduler(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc)

	// SpawnConsumerd adds a new consumerd to the environment. If the returned context
	// is canceled, the consumerd will be stopped.
	SpawnConsumerd(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc)

	// SpawnMonitor adds a new monitor to the environment. If the returned context
	// is canceled, the monitor will be stopped.
	SpawnMonitor(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc)

	// SpawnCache adds a new cache server  to the environment. If the returned context
	// is canceled, the cache server will be stopped.
	SpawnCache(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc)
}

// these counts are only used to give the agents and consumerds unique names
var (
	agentCount     = atomic.NewInt32(0)
	consumerdCount = atomic.NewInt32(0)
)

func makeOptions(e Environment, opts ...SpawnOption) (SpawnOptions, *config.KubeccSpec) {
	so := SpawnOptions{
		config: e.DefaultConfig(),
		name:   "default",
		uuid:   uuid.NewString(),
	}
	so.Apply(opts...)
	cfg := e.DefaultConfig()
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}
	return so, &cfg
}

func SpawnAgent(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so, cfg := makeOptions(e, opts...)
	parentCtx, cancel := ctxutil.WithCancel(e.Context())

	ctx := meta.NewContextWithParent(parentCtx,
		meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
		meta.WithProvider(identity.UUID, meta.WithValue(so.uuid)),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string('a'+agentCount.Load())),
				logkc.WithLogLevel(cfg.Agent.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	agentCount.Inc()
	go func() {
		<-ctx.Done()
		agentCount.Dec()
	}()

	options := []agent.AgentServerOption{
		agent.WithUsageLimits(&metrics.UsageLimits{
			ConcurrentProcessLimit: int32(cfg.Agent.UsageLimits.GetConcurrentProcessLimit()),
		}),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(AddToStore),
		agent.WithMonitorClient(NewMonitorClient(e, ctx)),
		agent.WithSchedulerClient(NewSchedulerClient(e, ctx)),
	}
	options = append(options, so.agentOptions...)

	agentSrv := agent.NewAgentServer(ctx, options...)
	go agentSrv.StartMetricsProvider()

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

func SpawnScheduler(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so, cfg := makeOptions(e, opts...)
	parentCtx, cancel := ctxutil.WithCancel(e.Context())

	ctx := meta.NewContextWithParent(parentCtx,
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID, meta.WithValue(so.uuid)),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Scheduler,
				logkc.WithName("a"),
				logkc.WithLogLevel(cfg.Scheduler.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)

	options := []scheduler.SchedulerServerOption{
		scheduler.WithMonitorClient(NewMonitorClient(e, ctx)),
		scheduler.WithCacheClient(NewCacheClient(e, ctx)),
	}
	options = append(options, so.schedulerOptions...)

	sc := scheduler.NewSchedulerServer(ctx, options...)
	go sc.StartMetricsProvider()
	e.Serve(ctx, sc, so.name)

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

func SpawnConsumerd(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so, cfg := makeOptions(e, opts...)
	parentCtx, cancel := ctxutil.WithCancel(e.Context())

	ctx := meta.NewContextWithParent(parentCtx,
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID, meta.WithValue(so.uuid)),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumerd,
				logkc.WithName(string('a'+consumerdCount.Load())),
				logkc.WithLogLevel(cfg.Consumerd.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	consumerdCount.Inc()

	options := []consumerd.ConsumerdServerOption{
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: TestToolchainFinder{},
		}),
		consumerd.WithToolchainRunners(AddToStore),
		consumerd.WithMonitorClient(NewMonitorClient(e, ctx)),
		consumerd.WithSchedulerClient(NewSchedulerClient(e, ctx)),
		consumerd.WithQueueOptions(
			consumerd.WithLocalUsageManager(consumerd.AutoUsageLimits()),
			consumerd.WithRemoteUsageManager(clients.NewRemoteUsageManager(ctx, NewMonitorClient(e, ctx))),
		),
	}
	options = append(options, so.consumerdOptions...)

	cd := consumerd.NewConsumerdServer(ctx, options...)

	go cd.StartMetricsProvider()
	e.Serve(ctx, cd, so.name)

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

func SpawnMonitor(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so, cfg := makeOptions(e, opts...)
	parentCtx, cancel := ctxutil.WithCancel(e.Context())

	ctx := meta.NewContextWithParent(parentCtx,
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID, meta.WithValue(so.uuid)),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Monitor,
				logkc.WithName("a"),
				logkc.WithLogLevel(cfg.Monitor.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)

	mon := monitor.NewMonitorServer(ctx, cfg.Monitor, monitor.InMemoryStoreCreator)
	e.Serve(ctx, mon, so.name)

	return ctx, cancel
}

func SpawnCache(e Environment, opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so, cfg := makeOptions(e, opts...)
	parentCtx, cancel := ctxutil.WithCancel(e.Context())

	ctx := meta.NewContextWithParent(parentCtx,
		meta.WithProvider(identity.Component, meta.WithValue(types.Cache)),
		meta.WithProvider(identity.UUID, meta.WithValue(so.uuid)),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Cache,
				logkc.WithName("a"),
				logkc.WithLogLevel(cfg.Cache.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)

	options := []cachesrv.CacheServerOption{
		cachesrv.WithMonitorClient(NewMonitorClient(e, ctx)),
	}

	providers := []storage.StorageProvider{}
	if cfg.Cache.LocalStorage != nil {
		providers = append(providers,
			storage.NewVolatileStorageProvider(ctx, *cfg.Cache.LocalStorage))
	}
	if cfg.Cache.RemoteStorage != nil {
		providers = append(providers,
			storage.NewS3StorageProvider(ctx, *cfg.Cache.RemoteStorage))
	}
	options = append(options, cachesrv.WithStorageProvider(
		storage.NewChainStorageProvider(ctx, providers...),
	))
	options = append(options, so.cacheOptions...)

	cacheSrv := cachesrv.NewCacheServer(ctx, cfg.Cache, options...)

	go cacheSrv.StartMetricsProvider()
	e.Serve(ctx, cacheSrv, so.name)

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

func NewMonitorClient(e Environment, ctx context.Context, name ...string) types.MonitorClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewMonitorClient(e.Dial(ctx, types.Monitor, srvName))
}

func NewSchedulerClient(e Environment, ctx context.Context, name ...string) types.SchedulerClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewSchedulerClient(e.Dial(ctx, types.Scheduler, srvName))
}

func NewCacheClient(e Environment, ctx context.Context, name ...string) types.CacheClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewCacheClient(e.Dial(ctx, types.Cache, srvName))
}

func NewConsumerdClient(e Environment, ctx context.Context, name ...string) types.ConsumerdClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewConsumerdClient(e.Dial(ctx, types.Consumerd, srvName))
}
