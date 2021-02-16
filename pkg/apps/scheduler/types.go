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

	CpuConfig   *types.CpuConfig
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
	enc.AddObject("info", a.Info)
	return nil
}

func (a Agent) Weight() int32 {
	switch a.QueueStatus {
	case types.Available, types.Queueing:
		return a.CpuConfig.GetMaxRunningProcesses()
	case types.QueuePressure:
		return a.CpuConfig.GetMaxRunningProcesses() / 2
	case types.QueueFull:
		return 0
	}
	return 0
}
