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
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/test/integration"
	"github.com/opentracing/opentracing-go"
)

func TestIntegration(t *testing.T) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.MetadataProvider, meta.WithValue(
			logkc.New(types.TestComponent, logkc.WithName("-")),
		)),
		meta.WithProvider(tracing.MetadataProvider),
	)
	lg := ctx.Log()

	tracer := ctx.Tracer()
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "integration-test")
	defer span.Finish()

	numTasks := 4000
	localJobs := 100
	taskPool := make(chan *types.RunRequest, numTasks)
	for i := 0; i < numTasks; i++ {
		taskPool <- &types.RunRequest{
			Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
			Args:     []string{"-duration", fmt.Sprintf("%dms", rand.Intn(6000)+2000)},
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
				ConcurrentProcessLimit:  32,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
			{
				ConcurrentProcessLimit:  16,
				QueuePressureMultiplier: 1.5,
				QueueRejectMultiplier:   2.0,
			},
		},
	}
	tc := integration.NewTestController(ctx)
	tc.Start(testOptions)

	cc, err := servers.Dial(ctx, "127.0.0.1:9960")
	if err != nil {
		panic(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(len(testOptions.Agents))
	extClient := types.NewExternalMonitorClient(cc)
	listener := metrics.NewListener(ctx, extClient)
	listener.OnProviderAdded(func(pctx context.Context, uuid string) {
		wg.Done()
		<-pctx.Done()
		wg.Add(1)
	})
	defer tc.Teardown()

	// Wait until all agents connect
	wg.Wait()

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
}
