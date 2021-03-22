package consumerd_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	testctrl "github.com/cobalt77/kubecc/internal/testutil/controller"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
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
	plotStatsLog()
})

var (
	testEnv *test.Environment
	queue   *consumerd.SplitQueue
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.WarnLevel)))),
		meta.WithProvider(tracing.Tracer),
	)
	testToolchainRunner = &testctrl.TestToolchainCtrlLocal{}
)

func makeTaskPool(numTasks int) chan *consumerd.SplitTask {
	taskPool := make(chan *consumerd.SplitTask, numTasks)
	for i := 0; i < numTasks; i++ {
		contexts := run.Contexts{
			ServerContext: testCtx,
			ClientContext: testCtx,
		}
		request := &types.RunRequest{
			Args: []string{"--duration", fmt.Sprintf("%dms", rand.Intn(4)+1)},
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

func makeInfiniteTaskPool() (chan *consumerd.SplitTask, context.CancelFunc) {
	taskPool := make(chan *consumerd.SplitTask, 100)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		contexts := run.Contexts{
			ServerContext: testCtx,
			ClientContext: testCtx,
		}
		request := &types.RunRequest{}
		for {
			if ctx.Err() != nil {
				break
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
	}()
	return taskPool, cancel
}

var collectionPeriod = 25 * time.Millisecond

type filter struct {
	kind consumerd.EntryKind
}

func plotStatsLog() {
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
