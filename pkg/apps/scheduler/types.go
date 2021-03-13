package scheduler

import (
	"context"
	"sync"

	scmetrics "github.com/cobalt77/kubecc/pkg/apps/scheduler/metrics"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type remoteInfo struct {
	UUID           string
	Context        context.Context
	UsageLimits    *types.UsageLimits
	SystemInfo     *types.SystemInfo
	Toolchains     []*types.Toolchain
	CompletedTasks *atomic.Int64
}

type Consumerd struct {
	remoteInfo
	*sync.RWMutex
}

type Agent struct {
	remoteInfo
	*sync.RWMutex

	Client      types.Scheduler_StreamTasksServer
	QueueStatus types.QueueStatus
}

func remoteInfoFromContext(ctx context.Context) remoteInfo {
	return remoteInfo{
		UUID:           meta.UUID(ctx),
		Context:        ctx,
		SystemInfo:     meta.SystemInfo(ctx),
		CompletedTasks: atomic.NewInt64(0),
	}
}

func (a Agent) Weight() int32 {
	if a.UsageLimits == nil {
		// Use a default value of the number of cpu threads
		// until the agent posts its own usage limits
		return a.SystemInfo.CpuThreads
	}
	switch a.QueueStatus {
	case types.Available, types.Queueing:
		return a.UsageLimits.GetConcurrentProcessLimit()
	case types.QueuePressure:
		return a.UsageLimits.GetConcurrentProcessLimit() / 2
	case types.QueueFull:
		return 0
	}
	return 0
}

type agentStats struct {
	agentCtx        context.Context
	agentTasksTotal *scmetrics.AgentTasksTotal
	agentWeight     *scmetrics.AgentWeight
}

type consumerdStats struct {
	consumerdCtx       context.Context
	cdRemoteTasksTotal *scmetrics.CdTasksTotal
}

type taskStats struct {
	completedTotal *scmetrics.TasksCompletedTotal
	failedTotal    *scmetrics.TasksFailedTotal
	requestsTotal  *scmetrics.SchedulingRequestsTotal
}
