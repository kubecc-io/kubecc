package scheduler

import (
	"context"
	"fmt"
	"math"
	"sync"

	scmetrics "github.com/cobalt77/kubecc/pkg/apps/scheduler/metrics"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/smallnest/weighted"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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
	ctx            context.Context
	lg             *zap.SugaredLogger
	agents         map[string]*Agent
	consumerds     map[string]*Consumerd
	agentsMutex    *sync.RWMutex
	cdsMutex       *sync.RWMutex
	w              weighted.W
	wLock          *sync.Mutex
	completedTasks *atomic.Int64
	failedTasks    *atomic.Int64
	requestCount   *atomic.Int64
}

type SchedulerOptions struct {
}

type schedulerOption func(*SchedulerOptions)

func (o *SchedulerOptions) Apply(opts ...schedulerOption) {
	for _, op := range opts {
		op(o)
	}
}

func NewScheduler(ctx context.Context, opts ...schedulerOption) *Scheduler {
	options := SchedulerOptions{}
	options.Apply(opts...)

	return &Scheduler{
		SchedulerOptions: options,
		w:                &weighted.RRW{},
		wLock:            &sync.Mutex{},
		ctx:              ctx,
		lg:               meta.Log(ctx),
		completedTasks:   atomic.NewInt64(0),
		failedTasks:      atomic.NewInt64(0),
		requestCount:     atomic.NewInt64(0),
		agents:           make(map[string]*Agent),
		consumerds:       make(map[string]*Consumerd),
		agentsMutex:      &sync.RWMutex{},
		cdsMutex:         &sync.RWMutex{},
	}
}

func (s *Scheduler) Schedule(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	s.lg.Info("Scheduling")
	s.requestCount.Inc()
	for {
		s.wLock.Lock()
		next := s.w.Next()
		if next == nil {
			s.agentsMutex.RLock()
			numAgents := len(s.agents)
			s.agentsMutex.RUnlock()

			if numAgents > 0 {
				// All weights 0
				return nil, status.Error(codes.ResourceExhausted, "All agents busy")
			} else {
				return nil, status.Error(codes.Unavailable, "No agents available")
			}
		}
		agent := next.(*Agent)
		agentStream := agent.Stream
		s.wLock.Unlock()
		err := agentStream.Send(req)
		if status.Code(err) == codes.Unavailable {
			s.lg.With(
				zap.Error(err),
			).Info("Agent rejected task, re-scheduling...")
			continue
		}
		if err != nil {
			s.lg.With(zap.Error(err)).Error("Error from agent")
			s.failedTasks.Inc()
			return nil, err
		}
		agent.CompletedTasks.Inc()
		s.completedTasks.Inc()
		return response, nil
	}
}

func (s *Scheduler) AgentIsConnected(a *Agent) bool {
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()
	_, ok := s.agents[a.UUID]
	return ok
}

func (s *Scheduler) ConsumerdIsConnected(c *Consumerd) bool {
	s.cdsMutex.RLock()
	defer s.cdsMutex.RUnlock()
	_, ok := s.consumerds[c.UUID]
	return ok
}

func (s *Scheduler) AgentConnected(ctx context.Context) error {
	agent := &Agent{
		remoteInfo: remoteInfoFromContext(ctx),
		RWMutex:    &sync.RWMutex{},
	}
	if s.AgentIsConnected(agent) {
		return status.Error(codes.AlreadyExists, "Agent already connected")
	}
	var err error
	agent.Stream, err = s.agentDialer.Dial(ctx)
	if err != nil {
		return status.Error(codes.Internal,
			fmt.Sprintf("Error dialing agent: %s", err.Error()))
	}

	s.agentsMutex.Lock()
	defer s.agentsMutex.Unlock()

	s.lg.With(
		zap.String("uuid", agent.UUID),
	).Info(types.Scheduler.Color().Add("Agent connected"))
	s.agents[agent.UUID] = agent

	s.wLock.Lock()
	s.w.Add(agent, int(agent.Weight()))
	s.wLock.Unlock()

	go func() {
		<-agent.Context.Done()
		s.agentsMutex.Lock()
		defer s.agentsMutex.Unlock()
		delete(s.agents, agent.UUID)
		s.lg.With(
			zap.String("uuid", agent.UUID),
		).Info(types.Scheduler.Color().Add("Agent disconnected"))
	}()
	return nil
}

func (s *Scheduler) ConsumerdConnected(ctx context.Context) error {
	cd := &Consumerd{
		remoteInfo: remoteInfoFromContext(ctx),
		RWMutex:    &sync.RWMutex{},
	}
	if s.ConsumerdIsConnected(cd) {
		return status.Error(codes.AlreadyExists, "Consumerd already connected")
	}

	s.cdsMutex.Lock()
	defer s.cdsMutex.Unlock()

	s.lg.With(
		zap.String("uuid", cd.UUID),
	).Info(types.Scheduler.Color().Add("Consumerd connected"))
	s.consumerds[cd.UUID] = cd

	go func() {
		<-cd.Context.Done()
		s.cdsMutex.Lock()
		defer s.cdsMutex.Unlock()

		delete(s.consumerds, cd.UUID)
		s.lg.With(
			zap.String("uuid", cd.UUID),
		).Info(types.Scheduler.Color().Add("Consumerd disconnected"))
	}()
	return nil
}

func (s *Scheduler) reweightAll() {
	s.wLock.Lock()
	defer s.wLock.Unlock()
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()

	s.w.RemoveAll()
	for _, agent := range s.agents {
		agent.RLock()
		s.w.Add(agent, int(agent.Weight()))
		agent.RUnlock()
	}
}

func (s *Scheduler) SetQueueStatus(ctx context.Context, stat types.QueueStatus) error {
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()

	uuid := meta.UUID(ctx)
	if agent, ok := s.agents[uuid]; ok {
		agent.Lock()
		agent.QueueStatus = stat
		agent.Unlock()

		s.reweightAll()
	}
	return nil
}

func (s *Scheduler) SetToolchains(ctx context.Context, tcs []*types.Toolchain) error {
	s.agentsMutex.RLock()
	defer s.agentsMutex.RUnlock()
	s.cdsMutex.RLock()
	defer s.cdsMutex.RUnlock()

	uuid := meta.UUID(ctx)
	if agent, ok := s.agents[uuid]; ok {
		agent.Lock()
		agent.Toolchains = tcs
		agent.Unlock()
	} else if cd, ok := s.consumerds[uuid]; ok {
		cd.Lock()
		cd.Toolchains = tcs
		cd.Unlock()
	}
	return nil
}

var float64Epsilon = 1e-6

func (s *Scheduler) CalcAgentStats() <-chan []agentStats {
	stats := make(chan []agentStats)
	go func() {
		var min, max float64
		statsList := []agentStats{}
		s.agentsMutex.RLock()
		defer s.agentsMutex.RUnlock()

		for uuid, agent := range s.agents {
			agent.RLock()
			defer agent.RUnlock()

			stats := agentStats{
				agentCtx:        agent.Context,
				agentTasksTotal: &scmetrics.AgentTasksTotal{},
				agentWeight:     &scmetrics.AgentWeight{},
			}

			stats.agentWeight.UUID = uuid
			stats.agentTasksTotal.Total = agent.CompletedTasks.Load()
			stats.agentTasksTotal.Identifier = stats.agentWeight.Identifier

			w := float64(agent.Weight())
			switch {
			case len(statsList) == 0:
				min = w
				max = w
			case w > max:
				w = max
			case w < min:
				w = min
			}

			// Set the non-normalized weight here, adjust below
			stats.agentWeight.Value = w
			statsList = append(statsList, stats)
		}

		// Normalize weights
		for i, stat := range statsList {
			if math.Abs(max-min) <= float64Epsilon {
				// If max == min, set each weight to 1, they are all equal
				statsList[i].agentWeight.Value = 1.0
			} else {
				statsList[i].agentWeight.Value =
					(stat.agentWeight.Value - min) / (max - min)
			}
		}

		stats <- statsList
	}()
	return stats
}

func (s *Scheduler) CalcConsumerdStats() <-chan []consumerdStats {
	stats := make(chan []consumerdStats)
	go func() {
		statsList := []consumerdStats{}
		s.cdsMutex.RLock()
		defer s.cdsMutex.RUnlock()

		for uuid, cd := range s.consumerds {
			cd.RLock()
			defer cd.RUnlock()

			total := &scmetrics.CdTasksTotal{
				Total: cd.CompletedTasks.Load(),
			}
			total.Identifier.UUID = uuid
			statsList = append(statsList, consumerdStats{
				consumerdCtx:       cd.Context,
				cdRemoteTasksTotal: total,
			})
		}

		stats <- statsList
	}()
	return stats
}

func (s *Scheduler) TaskStats() taskStats {
	return taskStats{
		completedTotal: &scmetrics.TasksCompletedTotal{
			Total: s.completedTasks.Load(),
		},
		failedTotal: &scmetrics.TasksFailedTotal{
			Total: s.failedTasks.Load(),
		},
		requestsTotal: &scmetrics.SchedulingRequestsTotal{
			Total: s.requestCount.Load(),
		},
	}
}
