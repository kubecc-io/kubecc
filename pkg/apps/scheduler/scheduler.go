package scheduler

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/smallnest/weighted"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/peer"
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
	w     weighted.W
	wLock *sync.Mutex

	agents sync.Map // map[types.AgentID]*Agent
	ctx    context.Context
	lg     *zap.SugaredLogger
}

func NewScheduler(ctx context.Context) *Scheduler {
	return &Scheduler{
		w:     &weighted.RRW{},
		wLock: &sync.Mutex{},
		ctx:   ctx,
		lg:    logkc.LogFromContext(ctx),
	}
}

func (s *Scheduler) Schedule(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	s.lg.Info("Scheduling")
	for {
		s.wLock.Lock()
		agentClient := s.w.Next().(*Agent).Client
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

func (s *Scheduler) AgentIsConnected(id types.AgentID) bool {
	_, ok := s.agents.Load(id)
	return ok
}

func (s *Scheduler) AgentConnected(ctx context.Context) error {
	agent, err := AgentFromContext(ctx)
	if err != nil {
		return status.Error(codes.InvalidArgument,
			"Error identifying agent using context")
	}
	id, err := agent.Info.AgentID()
	if err != nil {
		return err
	}
	if s.AgentIsConnected(id) {
		return status.Error(codes.AlreadyExists, "Agent already connected")
	}

	peer, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Internal,
			"Error identifying agent peer")
	}
	cc, err := servers.Dial(context.Background(),
		fmt.Sprintf("%s:9090", peer.Addr.(*net.TCPAddr).IP.String()))
	if err != nil {
		if !ok {
			return status.Error(codes.Internal,
				"Error establishing connection to agent's server")
		}
	}
	agent.Client = types.NewAgentClient(cc)

	s.lg.With(zap.Object("agent", agent.Info)).Info("Agent connected")
	s.agents.Store(id, agent)

	cpuConfig, err := agent.Client.GetCpuConfig(
		context.Background(), &types.Empty{})
	if err != nil {
		return err
	}
	agent.CpuConfig = cpuConfig
	s.wLock.Lock()
	s.w.Add(agent, int(agent.Weight()))
	s.wLock.Unlock()

	go func() {
		<-agent.Context.Done()
		s.agents.Delete(id)
		s.lg.With(zap.Object("agent", agent.Info)).Info("Agent disconnected")
	}()
	return nil
}

func (s *Scheduler) reweightAll() {
	s.wLock.Lock()
	defer s.wLock.Unlock()
	s.w.RemoveAll()
	s.agents.Range(func(key, value interface{}) bool {
		s.w.Add(value, int(value.(*Agent).Weight()))
		return true
	})
}

func (s *Scheduler) GetAgentInfo(ctx context.Context) (*types.AgentInfo, error) {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := info.AgentID()
	if err != nil {
		return nil, err
	}
	if !s.AgentIsConnected(id) {
		return nil, status.Error(codes.FailedPrecondition,
			"Not connected, ensure a connection stream is active with Connect()")
	}
	return info, nil
}

func (s *Scheduler) SetQueueStatus(ctx context.Context, stat types.QueueStatus) error {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return err
	}
	id, err := info.AgentID()
	if err != nil {
		return err
	}
	if agent, ok := s.agents.Load(id); ok {
		agent.(*Agent).QueueStatus = stat
		s.reweightAll()
	}
	return nil
}

func (s *Scheduler) SetToolchains(ctx context.Context, tcs []*types.Toolchain) error {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return err
	}
	id, err := info.AgentID()
	if err != nil {
		return err
	}
	if agent, ok := s.agents.Load(id); ok {
		agent.(*Agent).Toolchains = tcs
	}
	return nil
}
