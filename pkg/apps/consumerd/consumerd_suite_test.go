package consumerd_test

import (
	"container/ring"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	testctrl "github.com/cobalt77/kubecc/internal/testutil/controller"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
	"go.uber.org/zap/zapcore"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func TestConsumerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consumerd Suite")
}

var _ = AfterSuite(func() {
	plotStatsLog()
})

var (
	testEnv *test.Environment
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	outLog = logkc.New(types.TestComponent,
		logkc.WithLogLevel(zapcore.DebugLevel),
		logkc.WithOutputPaths([]string{"./test.log"}),
	)
	testToolchainRunner = &testctrl.TestToolchainCtrlLocal{}
	localExec           = newTestExecutor()
	remoteExec          = newTestExecutor()
)

func makeTaskPool(numTasks int) chan *consumerd.SplitTask {
	taskPool := make(chan *consumerd.SplitTask, numTasks)
	for i := 0; i < numTasks; i++ {
		contexts := run.Contexts{
			ServerContext: testCtx,
			ClientContext: testCtx,
		}
		request := &types.RunRequest{
			Args: []string{"--duration", "1ms"},
		}

		taskPool <- &consumerd.SplitTask{
			Local: run.PackageRequest(
				testToolchainRunner.RunLocal(
					testToolchainRunner.NewArgParser(testCtx, []string{})),
				contexts,
				localExec,
				request,
			),
			Remote: run.PackageRequest(
				testToolchainRunner.SendRemote(
					testToolchainRunner.NewArgParser(testCtx, []string{}),
					nil, // Testing CompileRequestClient is out of scope for this test
				),
				contexts,
				remoteExec,
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
					localExec,
					request,
				),
				Remote: run.PackageRequest(
					testToolchainRunner.SendRemote(
						testToolchainRunner.NewArgParser(testCtx, []string{}),
						nil, // Testing CompileRequestClient is out of scope for this test
					),
					contexts,
					remoteExec,
					request,
				),
			}
		}
	}()
	return taskPool, cancel
}

var collectionPeriod = 500 * time.Millisecond

type testExecutor struct {
	numTasks       *atomic.Int32
	completed      *atomic.Int32
	completedTotal *atomic.Int32 // same as completed, but doesn't get reset

	pauseLock *sync.Mutex
	paused    bool
	pause     *sync.Cond
	stats     *ring.Ring
	statsLog  plotter.XYs
}

type testExecutorStats struct {
	period    time.Duration
	completed int64
}

func (x *testExecutor) Pause() {
	x.pause.L.Lock()
	x.paused = true
	x.pause.L.Unlock()
}

func (x *testExecutor) Resume() {
	x.pause.L.Lock()
	x.paused = true
	x.pause.L.Unlock()
	x.pause.Signal()
}

func (x *testExecutor) Exec(task run.Task) error {
	x.numTasks.Inc()
	defer x.numTasks.Dec()

	x.pause.L.Lock()
	for x.paused {
		x.pause.Wait()
	}
	x.pause.L.Unlock()

	err := <-run.RunAsync(task)

	x.pause.L.Lock()
	for x.paused {
		x.pause.Wait()
	}
	x.pause.L.Unlock()

	x.completed.Inc()
	x.completedTotal.Inc()
	return err
}

func newTestExecutor() *testExecutor {
	lock := &sync.Mutex{}
	x := &testExecutor{
		numTasks:       atomic.NewInt32(0),
		completed:      atomic.NewInt32(0),
		completedTotal: atomic.NewInt32(0),
		pauseLock:      &sync.Mutex{},
		pause:          sync.NewCond(lock),
		stats:          ring.New(10),
	}

	go x.collectStats()
	return x
}

func (x *testExecutor) CompleteUsageLimits(*metrics.UsageLimits) {

}

func (x *testExecutor) CompleteTaskStatus(s *metrics.TaskStatus) {
	s.NumDelegated = x.numTasks.Load()
}

func (x *testExecutor) ExecAsync(task run.Task) <-chan error {
	panic("not implemented")
}

var startTime = time.Now()

func (x *testExecutor) collectStats() {
	lastCompleted := x.completedTotal.Load()
	for range time.Tick(collectionPeriod) {
		completed := x.completedTotal.Load()
		value := float64(completed - lastCompleted)
		x.stats.Value = value
		x.stats = x.stats.Next()
		lastCompleted = completed
		x.statsLog = append(x.statsLog, plotter.XY{
			X: float64(time.Since(startTime).Milliseconds()),
			Y: value,
		})
	}
}

type trend int

const (
	increasing trend = iota
	decreasing
	steady
)

func (x *testExecutor) Slope() float64 {
	indexes := []float64{}
	values := []float64{}
	x.stats.Do(func(i interface{}) {
		if i == nil {
			return
		}
		indexes = append(indexes, float64(len(indexes)))
		values = append(values, i.(float64))
	})
	_, slope := stat.LinearRegression(indexes, values, nil, true)
	return slope
}

func plotStatsLog() {
	p := plot.New()
	p.Title.Text = "Local/Remote Executor Usage"
	p.X.Label.Text = "Time since test start (ms)"
	p.Y.Label.Text = fmt.Sprintf("Tasks completed per period (%s)", collectionPeriod.String())
	if err := plotutil.AddLinePoints(p,
		"Local", localExec.statsLog,
		"Remote", remoteExec.statsLog,
	); err != nil {
		panic(err)
	}
	if err := p.Save(16*vg.Inch, 8*vg.Inch, "stats.png"); err != nil {
		panic(err)
	}
}
