package run

import (
	"context"

	"github.com/opentracing/opentracing-go"
)

type Task struct {
	ctx      context.Context
	compiler string
	runner   Runner
	err      error
	doneCh   chan struct{}
}

func (t *Task) Run() {
	span, _ := opentracing.StartSpanFromContext(t.ctx, "task-run")
	defer span.Finish()
	defer close(t.doneCh)
	t.err = t.runner.Run(t.compiler)
}

func (t *Task) Done() <-chan struct{} {
	return t.doneCh
}

func (t *Task) Error() error {
	return t.err
}

func NewTask(ctx context.Context, r Runner, compiler string) *Task {
	return &Task{
		doneCh:   make(chan struct{}),
		ctx:      ctx,
		compiler: compiler,
		runner:   r,
	}
}
