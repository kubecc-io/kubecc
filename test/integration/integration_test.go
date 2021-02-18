// +build integration

package integration_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/test/integration"
	"github.com/opentracing/opentracing-go"
)

func TestIntegration(t *testing.T) {
	ctx := logkc.NewWithContext(context.Background(), types.TestComponent,
		logkc.WithName("-"))

	tracer, closer := tracing.Start(ctx, types.TestComponent)
	defer closer.Close()
	span, ctx := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "integration-test")
	defer span.Finish()

	lg := logkc.LogFromContext(ctx)

	numTasks := 200
	localJobs := 100
	taskPool := make(chan *types.RunRequest, numTasks)
	for i := 0; i < numTasks; i++ {
		taskPool <- &types.RunRequest{
			Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
			Args:     []string{"-duration", fmt.Sprintf("%dms", rand.Intn(5800)+1200)},
			UID:      1000,
			GID:      1000,
		}
	}

	tc := integration.NewTestController(ctx)
	tc.Start(integration.TestOptions{
		Clients: []*types.CpuConfig{
			{
				MaxRunningProcesses:    18,
				QueuePressureThreshold: 1.0,
				QueueRejectThreshold:   2.0,
			},
		},
		Agents: []*types.CpuConfig{
			{
				MaxRunningProcesses:    48,
				QueuePressureThreshold: 1.0,
				QueueRejectThreshold:   2.0,
			},
			{
				MaxRunningProcesses:    32,
				QueuePressureThreshold: 1.0,
				QueueRejectThreshold:   2.0,
			},
		},
	})
	defer tc.Teardown()

	wg := &sync.WaitGroup{}
	wg.Add(len(tc.Consumers) * localJobs)
	for _, c := range tc.Consumers {
		for i := 0; i < localJobs; i++ {
			go func(cd types.ConsumerdClient) {
				defer wg.Done()
				for {
					select {
					case task := <-taskPool:
						_, err := cd.Run(ctx, task)
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

	//tc.Wait()
}
