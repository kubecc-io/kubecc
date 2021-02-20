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

	tracer opentracing.Tracer
	span   opentracing.Span
}

func (t *Task) Run() {
	if t == nil {
		return
	}
	t.span.Finish()
	span := t.tracer.StartSpan("task-run",
		opentracing.FollowsFrom(t.span.Context()))
	sctx := opentracing.ContextWithSpan(t.ctx, span)
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

func Begin(ctx context.Context, r Runner, tc *types.Toolchain) *Task {
	tracer := tracing.TracerFromContext(ctx)
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "task-wait")
	return &Task{
		doneCh: make(chan struct{}),
		tracer: tracer,
		ctx:    sctx,
		tc:     tc,
		runner: r,
		span:   span,
	}
}
