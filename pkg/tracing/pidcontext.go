package tracing

import (
	"context"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/opentracing/opentracing-go"
)

var (
	contexts = new(sync.Map)
)

func WaitForPid(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	for {
		err = proc.Signal(syscall.Signal(0))
		if err != nil {
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func PIDSpanContext(tracer opentracing.Tracer, pid int) context.Context {
	if ctx, ok := contexts.Load(pid); ok {
		return ctx.(context.Context)
	}
	span, ctx := opentracing.StartSpanFromContextWithTracer(
		context.Background(), tracer, "make")
	contexts.Store(pid, ctx)
	go func() {
		WaitForPid(pid)
		contexts.Delete(pid)
		span.Finish()
	}()
	return ContextWithTracer(ctx, tracer)
}
