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

var collectionPeriod = 50 * time.Millisecond

var start = time.Now()
var statsTicker = time.NewTicker(collectionPeriod)

// func collectStats() {
// 	times := ring.New(15)
// 	local := ring.New(15)
// 	remote := ring.New(15)
// 	for range statsTicker.C {
// 		timestamp := time.Since(start)
// 		localCompleted := localExec.completedTotal.Load()
// 		remoteCompleted := remoteExec.completedTotal.Load()

// 		times.Value = float64(timestamp.Milliseconds())
// 		local.Value = float64(localCompleted)
// 		remote.Value = float64(remoteCompleted)

// 		localValues := []float64{}
// 		remoteValues := []float64{}
// 		timeValues := []float64{}
// 		local.Do(func(i interface{}) {
// 			if i == nil {
// 				return
// 			}
// 			localValues = append(localValues, i.(float64))
// 		})
// 		remote.Do(func(i interface{}) {
// 			if i == nil {
// 				return
// 			}
// 			remoteValues = append(remoteValues, i.(float64))
// 		})
// 		times.Do(func(i interface{}) {
// 			if i == nil {
// 				return
// 			}
// 			timeValues = append(timeValues, i.(float64))
// 		})
// 		if len(timeValues) >= 2 {
// 			_, localBeta := stat.LinearRegression(timeValues, localValues, nil, false)
// 			_, remoteBeta := stat.LinearRegression(timeValues, remoteValues, nil, false)

// 			localExec.stats = append(localExec.stats, plotter.XY{
// 				X: times.Value.(float64),
// 				Y: math.Max(localValues[len(localValues)-1]-localValues[len(localValues)-2], 0),
// 			})
// 			remoteExec.stats = append(remoteExec.stats, plotter.XY{
// 				X: times.Value.(float64),
// 				Y: remoteValues[len(remoteValues)-1] - remoteValues[len(remoteValues)-2],
// 			})
// 			localExec.stats2 = append(localExec.stats2, plotter.XY{
// 				X: times.Value.(float64),
// 				Y: localBeta,
// 			})
// 			remoteExec.stats2 = append(remoteExec.stats2, plotter.XY{
// 				X: times.Value.(float64),
// 				Y: remoteBeta,
// 			})
// 		}
// 		times = times.Next()
// 		local = local.Next()
// 		remote = remote.Next()
// 	}
// }

type trend int

const (
	increasing trend = iota
	decreasing
	steady
)

// func (x *testExecutor) Slope() float64 {
// 	indexes := []float64{}
// 	values := []float64{}
// 	x.stats.Do(func(i interface{}) {
// 		if i == nil {
// 			return
// 		}
// 		indexes = append(indexes, float64(len(indexes)))
// 		values = append(values, i.(float64))
// 	})
// 	_, slope := stat.LinearRegression(indexes, values, nil, false)
// 	if math.IsNaN(slope) {
// 		return 0
// 	}
// 	return slope
// }

type filter struct {
	loc  consumerd.SplitTaskLocation
	kind consumerd.EntryKind
}

func plotStatsLog() {
	p := plot.New()
	p.Title.Text = "Local/Remote Executor Usage"
	p.X.Label.Text = "Time since test start (ms)"
	p.Y.Label.Text = fmt.Sprintf("Tasks completed per period (%s)", collectionPeriod.String())

	entries := queue.Telemetry().Entries()
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
			loc:  consumerd.Local,
			kind: consumerd.CompletedTasks,
		},
		{
			loc:  consumerd.Remote,
			kind: consumerd.CompletedTasks,
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
				return e.Loc == f.loc && e.Kind == f.kind
			})
		}(i, f)
	}

	wg.Wait()

	wg.Add(5)
	go func() {
		wg.Done()
		xys[0] = filtered[0].ToXYs()
	}()
	go func() {
		wg.Done()
		xys[1] = filtered[1].ToXYs()
	}()
	go func() {
		wg.Done()
		xys[2] = filtered[2].ToXYs()
	}()
	go func() {
		wg.Done()
		xys[3] = filtered[3].Deltas().ToXYs()
	}()
	go func() {
		wg.Done()
		xys[4] = filtered[4].Deltas().ToXYs()
	}()
	wg.Wait()

	if err := plotutil.AddLinePoints(p,
		"Delegated", xys[0],
		"Queued", xys[1],
		"Running", xys[2],
		"Local/Completed", xys[3],
		"Remote/Completed", xys[4],
	); err != nil {
		panic(err)
	}
	if err := p.Save(16*vg.Inch, 8*vg.Inch, "stats.svg"); err != nil {
		panic(err)
	}

	// p2 := plot.New()
	// p2.Title.Text = "Local/Remote Executor Rate of Change"
	// p2.X.Label.Text = "Time since test start (ms)"
	// p2.Y.Label.Text = "dy/dt"

	// if err := plotutil.AddLinePoints(p2,
	// 	"Local", localExec.stats2,
	// 	"Remote", remoteExec.stats2,
	// ); err != nil {
	// 	panic(err)
	// }
	// if err := p2.Save(16*vg.Inch, 8*vg.Inch, "stats2.svg"); err != nil {
	// 	panic(err)
	// }

}
