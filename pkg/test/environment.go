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
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testctrl "github.com/cobalt77/kubecc/internal/testutil/controller"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/apps/cachesrv"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/storage"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/imdario/mergo"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// Environment is an in-process simulated kubecc cluster environment used
// in tests.
type Environment struct {
	defaultConfig  config.KubeccSpec
	envContext     context.Context
	envCancel      context.CancelFunc
	listeners      map[types.Component]map[string]*bufconn.Listener
	server         *grpc.Server
	agentCount     *atomic.Int32
	consumerdCount *atomic.Int32
}

var (
	bufferSize = 1024 * 1024
)

type SpawnOptions struct {
	config           config.KubeccSpec
	agentOptions     []agent.AgentServerOption
	consumerdOptions []consumerd.ConsumerdServerOption
	schedulerOptions []scheduler.SchedulerServerOption
	cacheOptions     []cachesrv.CacheServerOption
	name             string
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

func (e *Environment) serve(ctx context.Context, server interface{}, name string) {
	srv := servers.NewServer(ctx)
	component := meta.Component(ctx)
	if _, ok := e.listeners[component][name]; !ok {
		e.listeners[component][name] = bufconn.Listen(bufferSize)
	}
	switch s := server.(type) {
	case types.ConsumerdServer:
		types.RegisterConsumerdServer(srv, s)
	case types.SchedulerServer:
		types.RegisterSchedulerServer(srv, s)
	case types.MonitorServer:
		types.RegisterMonitorServer(srv, s)
	case types.CacheServer:
		types.RegisterCacheServer(srv, s)
	}
	go func() {
		defer delete(e.listeners[component], name)
		err := srv.Serve(e.listeners[component][name])
		if err != nil {
			meta.Log(ctx).Error(err)
		}
	}()
}

// SpawnAgent adds a new agent to the environment. If the returned context
// is canceled, the agent will be stopped.
func (e *Environment) SpawnAgent(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
		name:   "default",
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string('a'+e.agentCount.Load())),
				logkc.WithLogLevel(cfg.Agent.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	e.agentCount.Inc()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		e.agentCount.Dec()
	}()

	options := []agent.AgentServerOption{
		agent.WithUsageLimits(&metrics.UsageLimits{
			ConcurrentProcessLimit:  int32(cfg.Agent.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: cfg.Agent.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   cfg.Agent.UsageLimits.QueueRejectMultiplier,
		}),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(testctrl.AddToStore),
		agent.WithMonitorClient(e.NewMonitorClient(ctx)),
		agent.WithSchedulerClient(e.NewSchedulerClient(ctx)),
	}
	options = append(options, so.agentOptions...)

	agentSrv := agent.NewAgentServer(ctx, options...)
	go agentSrv.StartMetricsProvider()

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

// SpawnScheduler adds a new scheduler to the environment. If the returned context
// is canceled, the scheduler will be stopped.
func (e *Environment) SpawnScheduler(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
		name:   "default",
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Scheduler,
				logkc.WithName("a"),
				logkc.WithLogLevel(cfg.Scheduler.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	options := []scheduler.SchedulerServerOption{
		scheduler.WithMonitorClient(e.NewMonitorClient(ctx)),
		scheduler.WithCacheClient(e.NewCacheClient(ctx)),
	}
	options = append(options, so.schedulerOptions...)

	sc := scheduler.NewSchedulerServer(ctx, options...)
	go sc.StartMetricsProvider()
	e.serve(ctx, sc, so.name)

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

// SpawnConsumerd adds a new consumerd to the environment. If the returned context
// is canceled, the consumerd will be stopped.
func (e *Environment) SpawnConsumerd(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
		name:   "default",
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumerd,
				logkc.WithName(string('a'+e.consumerdCount.Load())),
				logkc.WithLogLevel(cfg.Consumerd.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	e.consumerdCount.Inc()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		e.consumerdCount.Dec()
	}()

	options := []consumerd.ConsumerdServerOption{
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithToolchainRunners(testctrl.AddToStore),
		consumerd.WithMonitorClient(e.NewMonitorClient(ctx)),
		consumerd.WithSchedulerClient(e.NewSchedulerClient(ctx)),
		consumerd.WithQueueOptions(
			consumerd.WithLocalUsageManager(consumerd.AutoUsageLimits()),
			consumerd.WithRemoteUsageManager(clients.NewRemoteUsageManager(ctx, e.NewMonitorClient(ctx))),
		),
	}
	options = append(options, so.consumerdOptions...)

	cd := consumerd.NewConsumerdServer(ctx, options...)

	go cd.StartMetricsProvider()
	e.serve(ctx, cd, so.name)

	if so.waitForReady {
		e.WaitForReady(meta.UUID(ctx))
	}
	return ctx, cancel
}

// SpawnMonitor adds a new monitor to the environment. If the returned context
// is canceled, the monitor will be stopped.
func (e *Environment) SpawnMonitor(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
		name:   "default",
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Monitor,
				logkc.WithName("a"),
				logkc.WithLogLevel(cfg.Monitor.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	mon := monitor.NewMonitorServer(ctx, cfg.Monitor, monitor.InMemoryStoreCreator)
	e.serve(ctx, mon, so.name)

	return ctx, cancel
}

// SpawnCache adds a new cache server  to the environment. If the returned context
// is canceled, the cache server will be stopped.
func (e *Environment) SpawnCache(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
		name:   "default",
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Cache)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Cache,
				logkc.WithName("a"),
				logkc.WithLogLevel(cfg.Cache.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	options := []cachesrv.CacheServerOption{
		cachesrv.WithMonitorClient(e.NewMonitorClient(ctx)),
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
	e.serve(ctx, cacheSrv, so.name)

	return ctx, cancel
}

// DefaultConfig returns a reasonable set of default values for testing.
func DefaultConfig() config.KubeccSpec {
	return config.KubeccSpec{
		Global: config.GlobalSpec{
			LogLevel: "info",
		},
		Agent: config.AgentSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit:  32,
				QueuePressureMultiplier: 1.0,
				QueueRejectMultiplier:   2.0,
			},
		},
		Cache: config.CacheSpec{
			LocalStorage: &config.LocalStorageSpec{
				Limits: config.StorageLimitsSpec{
					Memory: "1Gi",
				},
			},
		},
		Monitor: config.MonitorSpec{
			ServePrometheusMetrics: false,
		},
		Consumerd: config.ConsumerdSpec{
			DisableTLS: true,
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit: 20,
			},
		},
	}
}

// NewEnvironment creates a new environment with no components added. Use the
// SpawnX methods to add components to the environment.
func NewEnvironment(cfg config.KubeccSpec) *Environment {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	config.ApplyGlobals(&cfg)

	env := &Environment{
		defaultConfig:  cfg,
		envContext:     ctx,
		envCancel:      cancel,
		listeners:      make(map[types.Component]map[string]*bufconn.Listener),
		server:         servers.NewServer(ctx),
		agentCount:     atomic.NewInt32(0),
		consumerdCount: atomic.NewInt32(0),
	}
	for _, component := range []types.Component{
		types.Scheduler,
		types.Consumerd,
		types.Monitor,
		types.Cache,
	} {
		env.listeners[component] = make(map[string]*bufconn.Listener)
	}
	return env
}

// NewDefaultEnvironment creates a new environment with the default configuration.
func NewDefaultEnvironment() *Environment {
	return NewEnvironment(DefaultConfig())
}

// Dial creates a grpc client connection to the component named by the component
// type and an optional name which defaults to "default" if not provided.
func (e *Environment) Dial(ctx context.Context, c types.Component, name ...string) *grpc.ClientConn {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	if _, ok := e.listeners[c][srvName]; !ok {
		e.listeners[c][srvName] = bufconn.Listen(bufferSize)
	}
	cc, err := servers.Dial(ctx, "bufconn", servers.WithDialOpts(
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
				return e.listeners[c][srvName].Dial()
			}),
	))
	if err != nil {
		panic(err)
	}
	return cc
}

// NewMonitorClient creates a new MonitorClient to the component with the given
// name, which defaults to "default" if not provided.
func (e *Environment) NewMonitorClient(ctx context.Context, name ...string) types.MonitorClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewMonitorClient(e.Dial(ctx, types.Monitor, srvName))
}

// NewSchedulerClient creates a new SchedulerClient to the component with the given
// name, which defaults to "default" if not provided.
func (e *Environment) NewSchedulerClient(ctx context.Context, name ...string) types.SchedulerClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewSchedulerClient(e.Dial(ctx, types.Scheduler, srvName))
}

// NewCacheClient creates a new CacheClient to the component with the given
// name, which defaults to "default" if not provided.
func (e *Environment) NewCacheClient(ctx context.Context, name ...string) types.CacheClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewCacheClient(e.Dial(ctx, types.Cache, srvName))
}

// NewConsumerdClient creates a new ConsumerdClient to the component with the given
// name, which defaults to "default" if not provided.
func (e *Environment) NewConsumerdClient(ctx context.Context, name ...string) types.ConsumerdClient {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}
	return types.NewConsumerdClient(e.Dial(ctx, types.Consumerd, srvName))
}

// Shutdown terminates all components running in the environment by canceling
// the top-level parent context.
func (e *Environment) Shutdown() {
	e.envCancel()
}

func (e *Environment) WaitForReady(uuid string) {
	client := e.NewMonitorClient(e.envContext)
	listener := clients.NewMetricsListener(e.envContext, client)
	defer listener.Stop()
	done := make(chan struct{})
	listener.OnProviderAdded(func(c context.Context, s string) {
		if s != uuid {
			return
		}
		listener.OnValueChanged(uuid, func(h *metrics.Health) {
			if h.GetStatus() != metrics.OverallStatus_Initializing {
				close(done)
			}
		})
	})
	<-done
}
