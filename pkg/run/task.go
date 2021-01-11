package run

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cc"
)

type Task struct {
	ctx    context.Context
	info   *cc.ArgsInfo
	runner Runner
	err    error
	doneCh chan struct{}
}

func (t *Task) Run() {
	defer close(t.doneCh)
	t.err = t.runner.Run(t.info)
}

func (t *Task) Done() <-chan struct{} {
	return t.doneCh
}

func (t *Task) Error() error {
	return t.err
}

func NewTask(ctx context.Context, r Runner, info *cc.ArgsInfo) *Task {
	return &Task{
		doneCh: make(chan struct{}),
		ctx:    ctx,
		info:   info,
		runner: r,
	}
}
