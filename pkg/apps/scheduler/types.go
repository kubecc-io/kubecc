package scheduler

import (
	"context"
	"sync"

	scmetrics "github.com/cobalt77/kubecc/pkg/apps/scheduler/metrics"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type remoteInfo struct {
	UUID           string
	Context        context.Context
	UsageLimits    *metrics.UsageLimits
	SystemInfo     *types.SystemInfo
	CompletedTasks *atomic.Int64
}

type Consumerd struct {
	remoteInfo
	*sync.RWMutex

	Toolchains *metrics.Toolchains
	Stream     types.Scheduler_StreamOutgoingTasksServer
}

type Agent struct {
	remoteInfo
	*sync.RWMutex

	Toolchains *metrics.Toolchains
	Stream     types.Scheduler_StreamIncomingTasksServer
}

func remoteInfoFromContext(ctx context.Context) remoteInfo {
	return remoteInfo{
		UUID:           meta.UUID(ctx),
		Context:        ctx,
		SystemInfo:     meta.SystemInfo(ctx),
		CompletedTasks: atomic.NewInt64(0),
	}
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
