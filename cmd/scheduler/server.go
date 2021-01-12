package main

import (
	"context"
	"reflect"
	"sync/atomic"

	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	scheduler atomic.Value
	monitor   *Monitor
}

func (s *schedulerServer) atomicScheduler() AgentScheduler {
	return s.scheduler.Load().(AgentScheduler)
}

func NewSchedulerServer() *schedulerServer {
	mon := NewMonitor()
	AddRandDirectScheduler(mon)

	name := viper.GetString("scheduler")
	scheduler, ok := GetScheduler(name)
	if !ok {
		log.With(zap.String("scheduler", name)).Fatal("No such scheduler")
	}
	srv := &schedulerServer{
		monitor: mon,
	}
	srv.scheduler.Store(scheduler)
	viper.OnConfigChange(func(in fsnotify.Event) {
		log.Info("Processing config reload")
		name := viper.GetString("scheduler")
		sch, ok := GetScheduler(name)
		if !ok {
			log.Error("Error when reloading config")
			log.With(zap.String("scheduler", name)).Fatal("No such scheduler")
		}
		if reflect.TypeOf(sch) != reflect.TypeOf(srv.scheduler) {
			log.With(zap.String("scheduler", name)).Info("Switching scheduler")
			srv.scheduler.Store(sch)
		} else {
			log.Info("No config changes found.")
		}
	})
	return srv
}

func (s *schedulerServer) AtCapacity(
	ctx context.Context,
	req *types.Empty,
) (*wrappers.BoolValue, error) {
	peer, ok := peer.FromContext(ctx)
	if ok {
		log.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	return &wrappers.BoolValue{Value: false}, nil
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if ok {
		log.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	task, err := s.atomicScheduler().Schedule(ctx, req)
	if err != nil {
		log.With(zap.Error(err)).Info("=> Schedule failed")
		return nil, err
	}
	log.Info("=> Schedule OK")
	return s.monitor.Wait(task)
}

func (s *schedulerServer) Connect(
	srv types.Scheduler_ConnectServer,
) error {
	agent, err := NewAgentFromContext(srv.Context())
	if err != nil {
		log.With(zap.Error(err)).Error("Error identifying agent using context")
		return nil
	}
	lg := log.With("agent", agent)

	lg.Info("Agent connected")

	s.monitor.AgentConnected(agent)
	<-srv.Context().Done()

	lg.Info("Agent disconnected")
	return nil
}
