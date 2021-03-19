package integration_test

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Integration test", func() {

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.TestComponent, logkc.WithName("-")),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	numTasks := 400
	localJobs := 50
	taskPool := make(chan *types.RunRequest, numTasks)
	for i := 0; i < numTasks; i++ {
		taskPool <- &types.RunRequest{
			Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
			Args:     []string{"-duration", fmt.Sprintf("%dms", rand.Intn(100)*80)},
			UID:      1000,
			GID:      1000,
		}
	}

	Specify("Starting components", func() {
		testEnv = test.NewDefaultEnvironment()

		testEnv.SpawnMonitor()
		testEnv.SpawnCache()
		testEnv.SpawnScheduler()

		testEnv.SpawnAgent(test.WithConfig(config.AgentSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit:  24,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		}))

		testEnv.SpawnAgent(test.WithConfig(config.AgentSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit:  32,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		}))

		testEnv.SpawnAgent(test.WithConfig(config.AgentSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit:  16,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		}))

		testEnv.SpawnConsumerd(test.WithConfig(config.ConsumerdSpec{
			ListenAddress: "1111",
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit:  18,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		}))
	})

	Measure("Run test", func(b Benchmarker) {
		cd := testEnv.NewConsumerdClient(ctx)
		wg := sync.WaitGroup{}
		wg.Add(localJobs)

		for i := 0; i < localJobs; i++ {
			go func(cd types.ConsumerdClient) {
				defer wg.Done()
				for {
					select {
					case task := <-taskPool:
						b.Time("Run task", func() {
							_, err := cd.Run(ctx, task)
							if err != nil {
								panic(err)
							}
						})
					default:
						lg.Info("Finished")
						return
					}
				}
			}(cd)
		}
		wg.Wait()
	}, 1)
})
