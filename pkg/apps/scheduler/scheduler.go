package scheduler

import (
	"context"
	"fmt"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/smallnest/weighted"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

/*
Assumptions:
- Each invocation of the compiler takes a single input and produces a single output
- Each process will consume up to 100% of a single cpu thread
- Agents run in containers belonging to their own kernel cgroup with a limited
	CFS quota.
- Compiling locally is always faster than preprocessing locally + compiling remotely.

Notes:
Agents and consumers are persistently connected to the scheduler. The scheduler
knows which jobs are running at all times and on which agents, and it knows
which jobs are being run locally.

While building, the ultimate goal is to reach 100% cpu usage on the consumer,
and 100% usage on agents (relative to their cgroup).

The maximum number of concurrent processes is determined by:
(cfs_quota)/(cfs_period)*(multiple) where multiple is a configurable constant.
*/

type Scheduler struct {
	SchedulerOptions
	w     weighted.W
	wLock *sync.Mutex

	agents sync.Map // map[string]*Agent
	ctx    context.Context
	lg     *zap.SugaredLogger
}

type SchedulerOptions struct {
	agentDialer AgentDialer
}

type schedulerOption func(*SchedulerOptions)

func (o *SchedulerOptions) Apply(opts ...schedulerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithAgentDialer(d AgentDialer) schedulerOption {
	return func(o *SchedulerOptions) {
		o.agentDialer = d
	}
}

func NewScheduler(ctx context.Context, opts ...schedulerOption) *Scheduler {
	options := SchedulerOptions{
		agentDialer: &tcpDialer{},
	}
	options.Apply(opts...)

	return &Scheduler{
		SchedulerOptions: options,
		w:                &weighted.RRW{},
		wLock:            &sync.Mutex{},
		ctx:              ctx,
		lg:               meta.Log(ctx),
	}
}

func (s *Scheduler) Schedule(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	s.lg.Info("Scheduling")
	for {
		s.wLock.Lock()
		next := s.w.Next()
		if next == nil {
			// No agents available.
			// This could be because no agents are connected, or all agents have
			// a weight of 0, which would result in none being chosen.
			return nil, status.Error(codes.Unavailable, "No agents available")
		}
		agentClient := next.(*Agent).Client
		s.wLock.Unlock()
		response, err := agentClient.Compile(ctx, req, grpc.UseCompressor(gzip.Name))
		if status.Code(err) == codes.Unavailable {
			s.lg.Info("Agent rejected task, re-scheduling...")
			continue
		}
		if err != nil {
			s.lg.With(zap.Error(err)).Error("Error from agent")
			return nil, err
		}
		return response, nil
	}
}

func (s *Scheduler) AgentIsConnected(a *Agent) bool {
	_, ok := s.agents.Load(a.UUID)
	return ok
}

func (s *Scheduler) AgentConnected(ctx context.Context) error {
	agent := AgentFromContext(ctx)
	if s.AgentIsConnected(agent) {
		return status.Error(codes.AlreadyExists, "Agent already connected")
	}
	var err error
	agent.Client, err = s.agentDialer.Dial(ctx)
	if err != nil {
		return status.Error(codes.Internal,
			fmt.Sprintf("Error dialing agent: %s", err.Error()))
	}

	s.lg.With(
		zap.String("agent", agent.UUID),
	).Info(types.Scheduler.Color().Add("Agent connected"))
	s.agents.Store(agent.UUID, agent)

	s.wLock.Lock()
	s.w.Add(agent, int(agent.Weight()))
	s.wLock.Unlock()

	go func() {
		<-agent.Context.Done()
		s.agents.Delete(agent.UUID)
		s.lg.With(
			zap.String("agent", agent.UUID),
		).Info(types.Scheduler.Color().Add("Agent disconnected"))
	}()
	return nil
}

func (s *Scheduler) reweightAll() {
	s.wLock.Lock()
	defer s.wLock.Unlock()
	s.w.RemoveAll()
	s.agents.Range(func(_, value interface{}) bool {
		s.w.Add(value, int(value.(*Agent).Weight()))
		return true
	})
}

func (s *Scheduler) SetQueueStatus(ctx context.Context, stat types.QueueStatus) error {
	uuid := meta.UUID(ctx)
	if agent, ok := s.agents.Load(uuid); ok {
		agent.(*Agent).QueueStatus = stat
		s.reweightAll()
	}
	return nil
}

func (s *Scheduler) SetToolchains(ctx context.Context, tcs []*types.Toolchain) error {
	uuid := meta.UUID(ctx)
	if agent, ok := s.agents.Load(uuid); ok {
		agent.(*Agent).Toolchains = tcs
	}
	return nil
}
