/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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
	return ctx
}
