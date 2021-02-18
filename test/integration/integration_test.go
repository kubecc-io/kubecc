// +build integration

package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/test/integration"
)

// todo: add a test toolchain to simulate agent tasks

func TestIntegration(t *testing.T) {
	tc := integration.NewTestController()
	tc.Start(integration.TestOptions{
		NumClients: 2,
		NumAgents:  4,
	})

	ctx := logkc.NewFromContext(context.Background(), types.TestComponent,
		logkc.WithName("-"))
	lg := logkc.LogFromContext(ctx)

	for i := 0; i < 50; i++ {
		for _, c := range tc.Consumers {
			go func(cd types.ConsumerdClient) {
				_, err := cd.Run(ctx, &types.RunRequest{
					Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
					Args:     strings.Split("-duration 1s", " "),
					UID:      1000,
					GID:      1000,
				})
				if err != nil {
					lg.Error(err)
				}
			}(c)
		}
	}

	tc.Wait()
}
