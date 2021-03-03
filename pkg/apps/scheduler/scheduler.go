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

	agents         sync.Map // map[string]*Agent
	consumerds     sync.Map // map[string]*Consumerd
	ctx            context.Context
	lg             *zap.SugaredLogger
	completedTasks *atomic.Int64
	failedTasks    *atomic.Int64
	requestCount   *atomic.Int64
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
		completedTasks:   atomic.NewInt64(0),
		failedTasks:      atomic.NewInt64(0),
		requestCount:     atomic.NewInt64(0),
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
			// No agents available.
			// This could be because no agents are connected, or all agents have
			// a weight of 0, which would result in none being chosen.
			return nil, status.Error(codes.Unavailable, "No agents available")
		}
		agent := next.(*Agent)
		agentClient := agent.Client
		s.wLock.Unlock()
		response, err := agentClient.Compile(ctx, req, grpc.UseCompressor(gzip.Name))
		if status.Code(err) == codes.Unavailable {
			s.lg.Info("Agent rejected task, re-scheduling...")
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
	_, ok := s.agents.Load(a.UUID)
	return ok
}

func (s *Scheduler) ConsumerdIsConnected(c *Consumerd) bool {
	_, ok := s.consumerds.Load(c.UUID)
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
	agent.Client, err = s.agentDialer.Dial(ctx)
	if err != nil {
		return status.Error(codes.Internal,
			fmt.Sprintf("Error dialing agent: %s", err.Error()))
	}

	s.lg.With(
		zap.String("uuid", agent.UUID),
	).Info(types.Scheduler.Color().Add("Agent connected"))
	s.agents.Store(agent.UUID, agent)

	s.wLock.Lock()
	s.w.Add(agent, int(agent.Weight()))
	s.wLock.Unlock()

	go func() {
		<-agent.Context.Done()
		s.agents.Delete(agent.UUID)
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
	s.lg.With(
		zap.String("uuid", cd.UUID),
	).Info(types.Scheduler.Color().Add("Consumerd connected"))
	s.consumerds.Store(cd.UUID, cd)

	go func() {
		<-cd.Context.Done()
		s.consumerds.Delete(cd.UUID)
		s.lg.With(
			zap.String("uuid", cd.UUID),
		).Info(types.Scheduler.Color().Add("Consumerd disconnected"))
	}()
	return nil
}

func (s *Scheduler) reweightAll() {
	s.wLock.Lock()
	defer s.wLock.Unlock()
	s.w.RemoveAll()
	s.agents.Range(func(_, value interface{}) bool {
		a := value.(*Agent)
		a.RLock()
		defer a.RUnlock()
		s.w.Add(value, int(a.Weight()))
		return true
	})
}

func (s *Scheduler) SetQueueStatus(ctx context.Context, stat types.QueueStatus) error {
	uuid := meta.UUID(ctx)
	if agent, ok := s.agents.Load(uuid); ok {
		a := agent.(*Agent)
		a.Lock()
		a.QueueStatus = stat
		a.Unlock()
		s.reweightAll()
	}
	return nil
}

func (s *Scheduler) SetToolchains(ctx context.Context, tcs []*types.Toolchain) error {
	uuid := meta.UUID(ctx)
	if agent, ok := s.agents.Load(uuid); ok {
		a := agent.(*Agent)
		a.Lock()
		a.Toolchains = tcs
		a.Unlock()
	} else if consumerd, ok := s.consumerds.Load(uuid); ok {
		cd := consumerd.(*Consumerd)
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
		s.agents.Range(func(key, value interface{}) bool {
			agent := value.(*Agent)
			agent.RLock()
			defer agent.RUnlock()

			stats := agentStats{
				agentTasksTotal: &scmetrics.AgentTasksTotal{},
				agentWeight:     &scmetrics.AgentWeight{},
			}

			stats.agentWeight.UUID = agent.UUID
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
			return true
		})

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
		s.consumerds.Range(func(key, value interface{}) bool {
			cd := value.(*Consumerd)
			cd.RLock()
			defer cd.RUnlock()

			total := &scmetrics.CdTasksTotal{
				Total: cd.CompletedTasks.Load(),
			}
			total.Identifier.UUID = cd.UUID
			statsList = append(statsList, consumerdStats{
				cdRemoteTasksTotal: total,
			})
			return true
		})
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
