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

package consumerd_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/consumerd"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func TestConsumerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consumerd Suite")
}

var _ = BeforeSuite(func() {
	// go collectStats()
})

var _ = AfterSuite(func() {
	testEnv.Shutdown()
})

var (
	testEnv test.Environment
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.ErrorLevel)))),
		meta.WithProvider(tracing.Tracer),
	)
	testToolchainRunner = &test.TestToolchainCtrlLocal{}
)

func makeTaskPool(numTasks int) chan *consumerd.SplitTask {
	taskPool := make(chan *consumerd.SplitTask, numTasks)
	for i := 0; i < numTasks; i++ {
		contexts := run.PairContext{
			ServerContext: testCtx,
			ClientContext: testCtx,
		}
		request := &types.RunRequest{
			Args: []string{"-sleep", fmt.Sprintf("%dms", rand.Intn(10)+1)},
		}

		taskPool <- &consumerd.SplitTask{
			Local: run.PackageRequest(
				testToolchainRunner.RunLocal(
					testToolchainRunner.NewArgParser(testCtx, []string{})),
				contexts,
				request,
			),
			Remote: run.PackageRequest(
				testToolchainRunner.SendRemote(
					testToolchainRunner.NewArgParser(testCtx, []string{}),
					nil, // Testing CompileRequestClient is out of scope for this test
				),
				contexts,
				request,
			),
		}
	}
	return taskPool
}

type infiniteTaskPool struct {
	C      chan *consumerd.SplitTask
	ctx    context.Context
	cancel context.CancelFunc
	args   []string
}

func (i *infiniteTaskPool) Run() {
	contexts := run.PairContext{
		ServerContext: testCtx,
		ClientContext: testCtx,
	}
	request := &types.RunRequest{
		Args: i.args,
	}
	for {
		select {
		case <-i.ctx.Done():
			close(i.C)
			return
		case i.C <- &consumerd.SplitTask{
			Local: run.PackageRequest(
				testToolchainRunner.RunLocal(
					testToolchainRunner.NewArgParser(testCtx, []string{})),
				contexts,
				request,
			),
			Remote: run.PackageRequest(
				testToolchainRunner.SendRemote(
					testToolchainRunner.NewArgParser(testCtx, []string{}),
					nil, // Testing CompileRequestClient is out of scope for this test
				),
				contexts,
				request,
			),
		}:
		}
	}
}

func (i *infiniteTaskPool) Cancel() {
	// This will cause the channel to be closed
	i.cancel()
}

// makeInfiniteTaskPool creates a new task pool that will always have tasks
// available until it is canceled with the Cancel method. It can be paused
// and resumed without closing its channel.
func makeInfiniteTaskPool(args []string) *infiniteTaskPool {
	taskPool := make(chan *consumerd.SplitTask)
	ctx, cancel := context.WithCancel(context.Background())
	pool := &infiniteTaskPool{
		args:   args,
		ctx:    ctx,
		cancel: cancel,
		C:      taskPool,
	}
	go pool.Run()
	return pool
}

var collectionPeriod = 10 * time.Millisecond

type filter struct {
	kind consumerd.EntryKind
}

func plotStatsLog(queue *consumerd.SplitQueue) {
	p := plot.New()
	p.Title.Text = "Local/Remote Executor Usage"
	p.X.Label.Text = "Timestamp (ms)"

	entries := queue.Telemetry().Entries()

	// normalize time scale
	startTime := entries[0].X
	for i, v := range entries {
		entries[i].X = time.Unix(0, v.X.UnixNano()-startTime.UnixNano())
	}

	filters := []filter{
		{
			kind: consumerd.DelegatedTasks,
		},
		{
			kind: consumerd.QueuedTasks,
		},
		{
			kind: consumerd.RunningTasks,
		},
		{
			kind: consumerd.CompletedTasksLocal,
		},
		{
			kind: consumerd.CompletedTasksRemote,
		},
	}
	filtered := make([]consumerd.Entries, len(filters))
	xys := make([]plotter.XYs, len(filters))
	wg := sync.WaitGroup{}
	wg.Add(len(filters))

	for i, f := range filters {
		go func(i int, f filter) {
			defer wg.Done()
			filtered[i] = entries.Filter(func(e consumerd.Entry) bool {
				return e.Kind == f.kind
			})
		}(i, f)
	}

	wg.Wait()

	wg.Add(5)
	go func() {
		defer wg.Done()
		xys[0] = filtered[0].ToXYs()
	}()
	go func() {
		defer wg.Done()
		xys[1] = filtered[1].ToXYs()
	}()
	go func() {
		defer wg.Done()
		xys[2] = filtered[2].ToXYs()
	}()
	go func() {
		defer wg.Done()
		xys[3] = filtered[3].Deltas().EWMA(collectionPeriod * 10).ToXYs()
	}()
	go func() {
		defer wg.Done()
		xys[4] = filtered[4].Deltas().EWMA(collectionPeriod * 10).ToXYs()
	}()
	wg.Wait()
	p.Legend.Left = true
	p.Legend.Top = true
	if err := plotutil.AddLinePoints(p,
		"Delegated", xys[0],
		"Queued (scaled 0-10)", xys[1],
		"Running", xys[2],
		"Local/Completed EWMA", xys[3],
		"Remote/Completed EWMA", xys[4],
	); err != nil {
		panic(err)
	}
	if err := p.Save(16*vg.Inch, 8*vg.Inch, "stats.svg"); err != nil {
		panic(err)
	}
}

type testRemoteUsageMgr struct {
	c    chan int64
	done chan struct{}
}

func (m *testRemoteUsageMgr) Manage(r run.Resizer) {
	for {
		r.Resize(<-m.c)
		m.done <- struct{}{}
	}
}

func (m *testRemoteUsageMgr) Resize(sz int64) {
	m.c <- sz
	<-m.done
}
