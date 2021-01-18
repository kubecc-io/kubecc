package run

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/opentracing/opentracing-go"
)

type Task struct {
	ctx      context.Context
	compiler string
	info     *cc.ArgsInfo
	runner   Runner
	err      error
	doneCh   chan struct{}
}

func (t *Task) Run() {
	span, _ := opentracing.StartSpanFromContext(t.ctx, "task-run")
	defer span.Finish()
	defer close(t.doneCh)
	t.err = t.runner.Run(t.compiler, t.info)
}

func (t *Task) Done() <-chan struct{} {
	return t.doneCh
}

func (t *Task) Error() error {
	return t.err
}

func NewTask(ctx context.Context, r Runner, compiler string, info *cc.ArgsInfo) *Task {
	return &Task{
		doneCh:   make(chan struct{}),
		ctx:      ctx,
		compiler: compiler,
		info:     info,
		runner:   r,
	}
}
