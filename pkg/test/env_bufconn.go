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
	sync "sync"
	"time"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/internal/zapkc"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// BufconnEnvironment is an in-process simulated kubecc cluster environment used
// in tests.
type BufconnEnvironment struct {
	defaultConfig config.KubeccSpec
	envContext    context.Context
	envCancel     context.CancelFunc
	listeners     map[types.Component]map[string]*bufconn.Listener
	listenersMu   *sync.Mutex
	server        *grpc.Server
}

var (
	_          Environment = (*BufconnEnvironment)(nil)
	bufferSize             = 1024 * 1024
)

func (e *BufconnEnvironment) Serve(ctx context.Context, server interface{}, name string) {
	srv := servers.NewServer(ctx)
	component := meta.Component(ctx)
	e.listenersMu.Lock()
	if _, ok := e.listeners[component][name]; !ok {
		e.listeners[component][name] = bufconn.Listen(bufferSize)
	}
	e.listenersMu.Unlock()
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
		go func() {
			<-ctx.Done()
			e.Log().With(
				"component", component.Name(),
				"name", name,
			).Info(zapkc.Red.Add("Server shutting down"))
			srv.Stop() // DO NOT USE GRACEFULSTOP
		}()
		e.listenersMu.Lock()
		listener := e.listeners[component][name]
		e.listenersMu.Unlock()
		err := srv.Serve(listener)
		if err != nil {
			meta.Log(ctx).Error(err)
		}
		e.listenersMu.Lock()
		delete(e.listeners[component], name)
		e.listenersMu.Unlock()
	}()
}

func defaultBufconnConfig() config.KubeccSpec {
	return config.KubeccSpec{
		Global: config.GlobalSpec{
			LogLevel: "warn",
		},
		Agent: config.AgentSpec{
			UsageLimits: &config.UsageLimitsSpec{
				ConcurrentProcessLimit: 32,
			},
		},
		Cache: config.CacheSpec{
			VolatileStorage: &config.VolatileStorageSpec{
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
			UsageLimits: &config.UsageLimitsSpec{
				ConcurrentProcessLimit: 20,
			},
		},
	}
}

func (e *BufconnEnvironment) DefaultConfig() config.KubeccSpec {
	return defaultBufconnConfig()
}

func NewBufconnEnvironment(cfg config.KubeccSpec) Environment {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.InfoLevel),
		))),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	config.ApplyGlobals(&cfg)

	env := &BufconnEnvironment{
		defaultConfig: cfg,
		envContext:    ctx,
		envCancel:     cancel,
		listeners:     make(map[types.Component]map[string]*bufconn.Listener),
		listenersMu:   &sync.Mutex{},
		server:        servers.NewServer(ctx),
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

func NewDefaultBufconnEnvironment() Environment {
	return NewBufconnEnvironment(defaultBufconnConfig())
}

func NewBufconnEnvironmentWithLogLevel(lv zapcore.Level) Environment {
	cfg := defaultBufconnConfig()
	cfg.Global.LogLevel = config.LogLevelString(lv.String())
	return NewBufconnEnvironment(cfg)
}

func (e *BufconnEnvironment) Dial(ctx context.Context, c types.Component, name ...string) *grpc.ClientConn {
	srvName := "default"
	if len(name) > 0 {
		srvName = name[0]
	}

	cc, err := servers.Dial(ctx, "bufconn", servers.WithDialOpts(
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
				e.listenersMu.Lock()
				if _, ok := e.listeners[c][srvName]; !ok {
					e.listeners[c][srvName] = bufconn.Listen(bufferSize)
				}
				listener := e.listeners[c][srvName]
				e.listenersMu.Unlock()
				conn, err := listener.Dial()
				if err != nil && err.Error() == "closed" {
					e.listenersMu.Lock()
					e.listeners[c][srvName] = bufconn.Listen(bufferSize)
					listener := e.listeners[c][srvName]
					e.listenersMu.Unlock()
					return listener.Dial()
				}
				return conn, err
			}),
	))
	if err != nil {
		panic(err)
	}
	return cc
}

func (e *BufconnEnvironment) Context() context.Context {
	return e.envContext
}

func (e *BufconnEnvironment) Log() *zap.SugaredLogger {
	return meta.Log(e.envContext)
}

func (e *BufconnEnvironment) Shutdown() {
	e.envCancel()
}

func (e *BufconnEnvironment) WaitForReady(uuid string, timeout ...time.Duration) {
	ctx, ca := context.WithCancel(e.envContext)
	client := NewMonitorClient(e, ctx)
	listener := clients.NewMetricsListener(ctx, client)
	done := make(chan struct{})
	listener.OnProviderAdded(func(c context.Context, s string) {
		if s != uuid {
			return
		}
		listener.OnValueChanged(uuid, func(h *metrics.Health) {
			if h.GetStatus() != metrics.OverallStatus_Initializing {
				select {
				case done <- struct{}{}:
				default:
				}
			}
		})
	})
	if len(timeout) == 0 {
		timeout = []time.Duration{time.Second * 10}
	}
	select {
	case <-time.After(timeout[0]):
		e.Log().Fatal("WaitForReady timed out")
	case <-done:
	}

	ca()
}

func (e *BufconnEnvironment) MetricF(srvCtx context.Context, out proto.Message) func() (proto.Message, error) {
	any, err := anypb.New(out)
	if err != nil {
		panic(err)
	}
	clone, err := any.UnmarshalNew()
	if err != nil {
		panic(err)
	}
	client := NewMonitorClient(e, e.envContext)
	return func() (proto.Message, error) {
		notify := util.NotifyBackground(srvCtx, 5*time.Second, "MetricF: "+any.GetTypeUrl())
		defer notify.Done()

		if err := srvCtx.Err(); err != nil {
			return nil, err
		}
		ctx, ca := context.WithTimeout(srvCtx, 1*time.Second)
		defer ca()
		metric, err := client.GetMetric(ctx, &types.Key{
			Bucket: meta.UUID(srvCtx),
			Name:   any.TypeUrl,
		})
		if err != nil {
			return nil, err
		}
		err = metric.Value.UnmarshalTo(clone)
		return proto.Clone(clone), err
	}
}
