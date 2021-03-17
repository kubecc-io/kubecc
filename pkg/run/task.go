package run

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
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

func (t *Task) Context() context.Context {
	return t.ctx
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

func Begin(sctx context.Context, r Runner, tc *types.Toolchain) *Task {
	tracer := meta.Tracer(sctx)
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		sctx, tracer, "task-wait")
	return &Task{
		doneCh: make(chan struct{}),
		tracer: tracer,
		ctx:    sctx,
		tc:     tc,
		runner: r,
		span:   span,
	}
}

func (t *Task) Restart() *Task {
	span := t.tracer.StartSpan("task-restart",
		opentracing.FollowsFrom(t.span.Context()))
	sctx := opentracing.ContextWithSpan(t.ctx, span)
	return &Task{
		doneCh: make(chan struct{}),
		tracer: t.tracer,
		ctx:    sctx,
		tc:     t.tc,
		runner: t.runner,
		span:   span,
	}
}
