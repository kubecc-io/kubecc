package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
	"go.uber.org/zap/zapcore"
)

type Agent struct {
	zapcore.ObjectMarshaler

	Info        *types.AgentInfo
	Context     context.Context
	ActiveTasks atomic.Int32
}

func (a *Agent) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddObject("info", a.Info)
	enc.AddInt32("activeTasks", a.ActiveTasks.Load())
	return nil
}

func NewAgentFromContext(ctx context.Context) (*Agent, error) {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &Agent{
		Info:        info,
		Context:     ctx,
		ActiveTasks: atomic.Int32{},
	}, nil
}
