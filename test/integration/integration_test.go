// +build integration

package integration_test

import (
	"testing"

	"github.com/cobalt77/kubecc/test/integration"
)

// todo: add a test toolchain to simulate agent tasks

func TestIntegration(t *testing.T) {
	tc := integration.TestController{}
	tc.Start(integration.TestOptions{
		NumClients: 2,
		NumAgents:  4,
	})
	tc.Wait()
}
