package util

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kubecc-io/kubecc/internal/zapkc"
	"github.com/kubecc-io/kubecc/pkg/meta/mdkeys"
	"go.uber.org/zap"
)

var notifyTimeoutHandler = &atomic.Value{}

type NotifyTimeoutInfo struct {
	Name    string
	Message string
	Caller  string
}

func SetNotifyTimeoutHandler(handler func(NotifyTimeoutInfo)) {
	notifyTimeoutHandler.Store(handler)
}

func NotifyBlocking(ctx context.Context, timeout time.Duration, name string, f func()) {
	notify := NotifyBackground(ctx, timeout, name)
	defer notify.Done()
	f()
}

type waitNotifier struct {
	ca context.CancelFunc
}

func (w *waitNotifier) Done() {
	w.ca()
}

func NotifyBackground(ctx context.Context, timeout time.Duration, name string) *waitNotifier {
	ctx, ca := context.WithCancel(ctx)
	value := ctx.Value(mdkeys.LogKey)
	if value == nil {
		panic("No logger in context")
	}
	lg := value.(*zap.SugaredLogger)
	startTime := time.Now()
	_, file, line, ok := runtime.Caller(1)
	if strings.HasSuffix(file, "environment.go") {
		_, file, line, ok = runtime.Caller(2)
	}
	if !ok {
		panic("Could not identify caller")
	}
	handler := func(info NotifyTimeoutInfo) {
		lg.With(
			"operation", info.Name,
		).Warn(zapkc.Yellow.Add(info.Message))
	}
	if h := notifyTimeoutHandler.Load(); h != nil {
		handler = h.(func(NotifyTimeoutInfo))
	}
	RunPeriodic(ctx, timeout, -1, false, func() {
		handler(NotifyTimeoutInfo{
			Name: name,
			Message: fmt.Sprintf(
				"Waiting longer than expected (%s)",
				time.Since(startTime).Round(timeout).String()),
			Caller: fmt.Sprintf("%s:%d", file, line),
		})
	})
	return &waitNotifier{
		ca: ca,
	}
}

func CancelAny(ctxs ...context.Context) context.Context {
	if len(ctxs) == 0 {
		ctx, ca := context.WithCancel(context.Background())
		defer ca()
		return ctx
	}
	return cancelAny{
		ctxs: ctxs,
	}
}

// cancelAny is a combined set of contexts which will be canceled when
// any of the contexts are canceled.
type cancelAny struct {
	ctxs []context.Context
}

func (ca cancelAny) Deadline() (deadline time.Time, ok bool) {
	var times []time.Time
	for _, ctx := range ca.ctxs {
		if t, ok := ctx.Deadline(); ok {
			times = append(times, t)
		}
	}
	if len(times) == 0 {
		return time.Time{}, false
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	return times[0], true
}

func (ca cancelAny) Done() <-chan struct{} {
	c := make(chan struct{})
	go func() {
		cases := make([]reflect.SelectCase, len(ca.ctxs))
		for i, ctx := range ca.ctxs {
			cases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ctx.Done()),
			}
		}
		reflect.Select(cases)
		close(c)
	}()
	return c
}

func (pc cancelAny) Err() error {
	for _, ctx := range pc.ctxs {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (pc cancelAny) Value(key interface{}) interface{} {
	for _, ctx := range pc.ctxs {
		if val := ctx.Value(key); val != nil {
			return val
		}
	}
	return nil
}
