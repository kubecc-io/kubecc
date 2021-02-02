package scheduler

import (
	"context"
	"reflect"
	"sync/atomic"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	lg        *zap.SugaredLogger
	scheduler atomic.Value
	monitor   *Monitor
}

func (s *schedulerServer) atomicScheduler() AgentScheduler {
	return s.scheduler.Load().(AgentScheduler)
}

func NewSchedulerServer(ctx context.Context) *schedulerServer {
	lg := lll.LogFromContext(ctx)

	mon := NewMonitor()
	AddRandDirectScheduler(mon)

	name := viper.GetString("scheduler")
	scheduler, ok := GetScheduler(name)
	if !ok {
		lg.With(zap.String("scheduler", name)).Fatal("No such scheduler")
	}
	srv := &schedulerServer{
		monitor: mon,
		lg:      lg,
	}
	srv.scheduler.Store(scheduler)
	viper.OnConfigChange(func(in fsnotify.Event) {
		lg.Info("Processing config reload")
		name := viper.GetString("scheduler")
		sch, ok := GetScheduler(name)
		if !ok {
			lg.Error("Error when reloading config")
			lg.With(zap.String("scheduler", name)).Fatal("No such scheduler")
		}
		if reflect.TypeOf(sch) != reflect.TypeOf(srv.scheduler) {
			lg.With(zap.String("scheduler", name)).Info("Switching scheduler")
			srv.scheduler.Store(sch)
		} else {
			lg.Info("No config changes found.")
		}
	})
	return srv
}

func (s *schedulerServer) AtCapacity(
	ctx context.Context,
	req *types.Empty,
) (*wrappers.BoolValue, error) {
	// this isnt used
	peer, ok := peer.FromContext(ctx)
	if ok {
		s.lg.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	return &wrappers.BoolValue{Value: false}, nil
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx, "schedule-compile")
	defer span.Finish()

	peer, ok := peer.FromContext(ctx)
	if ok {
		s.lg.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	return s.atomicScheduler().Schedule(
		lll.ContextWithLog(sctx, s.lg), req)
	// task, err := s.atomicScheduler().Schedule(ctx, req)
	// if err != nil {
	// 	lll.With(zap.Error(err)).Info("=> Schedule failed")
	// 	return nil, err
	// }
	// lll.Info("=> Schedule OK")
	// return s.monitor.Wait(task)
}

func (s *schedulerServer) Connect(
	srv types.Scheduler_ConnectServer,
) error {
	agent, err := NewAgentFromContext(srv.Context())
	if err != nil {
		s.lg.With(zap.Error(err)).Error("Error identifying agent using context")
		return nil
	}
	lg := s.lg.With("agent", agent)

	lg.Info("Agent connected")

	// add logic here maybe

	s.monitor.AgentConnected(agent)
	<-srv.Context().Done()

	lg.Info("Agent disconnected")
	return nil
}
