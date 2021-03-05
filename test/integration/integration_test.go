// +build integration

package integration_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/test/integration"
	"github.com/opentracing/opentracing-go"
)

func TestIntegration(t *testing.T) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.TestComponent, logkc.WithName("-")),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	tracer := meta.Tracer(ctx)
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "integration-test")
	defer span.Finish()

	numTasks := 4000
	localJobs := 500
	taskPool := make(chan *types.RunRequest, numTasks)
	for i := 0; i < numTasks; i++ {
		taskPool <- &types.RunRequest{
			Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
			Args:     []string{"-duration", fmt.Sprintf("%dms", rand.Intn(100)*80)},
			UID:      1000,
			GID:      1000,
		}
	}

	testOptions := integration.TestOptions{
		Clients: []*types.UsageLimits{
			{
				ConcurrentProcessLimit:  18,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		},
		Agents: []*types.UsageLimits{
			{
				ConcurrentProcessLimit:  48,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
			{
				ConcurrentProcessLimit:  48,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
			{
				ConcurrentProcessLimit:  48,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		},
	}
	tc := integration.NewTestController(sctx)
	tc.Start(testOptions)
	defer tc.Teardown()

	wg := sync.WaitGroup{}
	wg.Add(len(tc.Consumers) * localJobs)

	for _, c := range tc.Consumers {
		for i := 0; i < localJobs; i++ {
			go func(cd types.ConsumerdClient) {
				defer wg.Done()
				for {
					select {
					case task := <-taskPool:
						_, err := cd.Run(sctx, task)
						if err != nil {
							panic(err)
						}
					default:
						lg.Info("Finished")
						return
					}
				}
			}(c)
		}
	}

	wg.Wait()
}
