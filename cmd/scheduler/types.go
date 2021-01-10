package main

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
)

type Agent struct {
	Info        *types.AgentInfo
	Context     context.Context
	ActiveTasks map[context.Context]context.CancelFunc
}

type CompileTask struct {
	Context context.Context

	Status <-chan *types.CompileStatus
	Error  <-chan error
	Cancel context.CancelFunc
}

func NewAgentFromContext(ctx context.Context) (*Agent, error) {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return &Agent{
		Info:        info,
		Context:     ctx,
		ActiveTasks: make(map[context.Context]context.CancelFunc),
	}, nil
}
