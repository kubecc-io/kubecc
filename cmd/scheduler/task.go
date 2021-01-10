package main

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/types"
)

type HandlerFunc func(*types.CompileStatus)

type CompileTask struct {
	Context context.Context
	Status  <-chan *types.CompileStatus
	Error   <-chan error
	Cancel  context.CancelFunc
}

func NewCompileTask(
	stream types.Agent_CompileClient,
	cancel context.CancelFunc,
) *CompileTask {
	return &CompileTask{
		Status:  make(chan *types.CompileStatus),
		Error:   make(chan error),
		Cancel:  cancel,
		Context: stream.Context(),
	}
}
