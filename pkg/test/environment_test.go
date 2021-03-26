package test_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
)

var _ = Describe("Test Environment", func() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	var cancel1, cancel2, cancel3, cancel4, cancel5, cancel6 context.CancelFunc
	var env *test.Environment
	var client types.MonitorClient
	bucketCount := func() int {
		b, err := client.GetBuckets(ctx, &types.Empty{})
		if err != nil {
			return 0
		}
		return len(b.Buckets)
	}
	When("spawning components", func() {
		It("should have the correct number of components", func() {
			cfg := test.DefaultConfig()
			cfg.Global.LogLevel = "panic"
			env = test.NewEnvironment(cfg)
			env.SpawnMonitor()
			_, cancel1 = env.SpawnScheduler()
			_, cancel2 = env.SpawnCache()
			_, cancel3 = env.SpawnAgent()
			_, cancel4 = env.SpawnConsumerd()
			client = env.NewMonitorClient(ctx)
			Eventually(bucketCount).Should(Equal(6))
		})
	})
	When("adding additional components", func() {
		Specify("the number of components should update", func() {
			_, cancel5 = env.SpawnAgent(test.WithName("agent1"))
			Eventually(bucketCount).Should(Equal(7))
			_, cancel6 = env.SpawnConsumerd(test.WithName("cd1"))
			Eventually(bucketCount).Should(Equal(8))
		})
	})
	When("stopping all components", func() {
		Specify("the number of components should update", func() {
			cancel1()
			Eventually(bucketCount).Should(Equal(7))
			cancel2()
			Eventually(bucketCount).Should(Equal(6))
			cancel3()
			Eventually(bucketCount).Should(Equal(5))
			cancel4()
			Eventually(bucketCount).Should(Equal(4))
			cancel5()
			Eventually(bucketCount).Should(Equal(3))
			cancel6()
			Eventually(bucketCount).Should(Equal(2))
		})
	})
})
