package run

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type Task struct {
	ctx    context.Context
	tc     *types.Toolchain
	runner Runner
	err    error
	doneCh chan struct{}
}

func (t *Task) Run() {
	if t == nil {
		return
	}
	tracer := tracing.TracerFromContext(t.ctx)
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		t.ctx, tracer, "task-run")
	defer span.Finish()
	defer close(t.doneCh)
	t.err = t.runner.Run(sctx, t.tc)
}

func (t *Task) Done() <-chan struct{} {
	return t.doneCh
}

func (t *Task) Error() error {
	return t.err
}

func NewTask(ctx context.Context, r Runner, tc *types.Toolchain) *Task {
	return &Task{
		doneCh: make(chan struct{}),
		ctx:    ctx,
		tc:     tc,
		runner: r,
	}
}
