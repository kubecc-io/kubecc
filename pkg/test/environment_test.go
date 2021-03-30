/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package test_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
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
			env = test.NewEnvironmentWithLogLevel(zapcore.PanicLevel)
			env.SpawnMonitor()
			_, cancel1 = env.SpawnScheduler(test.WaitForReady())
			_, cancel2 = env.SpawnCache(test.WaitForReady())
			_, cancel3 = env.SpawnAgent(test.WaitForReady())
			_, cancel4 = env.SpawnConsumerd(test.WaitForReady())
			client = env.NewMonitorClient(ctx)
			Eventually(bucketCount).Should(Equal(6))
		})
	})
	When("adding additional components", func() {
		Specify("the number of components should update", func() {
			_, cancel5 = env.SpawnAgent(test.WithName("agent1"), test.WaitForReady())
			Eventually(bucketCount).Should(Equal(7))
			_, cancel6 = env.SpawnConsumerd(test.WithName("cd1"), test.WaitForReady())
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
