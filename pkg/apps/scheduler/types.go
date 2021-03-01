package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
)

type Agent struct {
	UUID    string
	Context context.Context
	Client  types.AgentClient

	UsageLimits *types.UsageLimits
	SystemInfo  *types.SystemInfo
	QueueStatus types.QueueStatus
	Toolchains  []*types.Toolchain
}

func AgentFromContext(ctx context.Context) *Agent {
	return &Agent{
		UUID:       meta.UUID(ctx),
		Context:    ctx,
		SystemInfo: meta.SystemInfo(ctx),
	}
}

func (a Agent) Weight() int32 {
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
