package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap/zapcore"
)

type Agent struct {
	zapcore.ObjectMarshaler

	Context context.Context
	Client  types.AgentClient

	UsageLimits *types.UsageLimits
	Info        *types.AgentInfo
	QueueStatus types.QueueStatus
	Toolchains  []*types.Toolchain
}

func AgentFromContext(ctx context.Context) (*Agent, error) {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &Agent{
		Info:    info,
		Context: ctx,
	}, nil
}

func (a Agent) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return enc.AddObject("info", a.Info)
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
