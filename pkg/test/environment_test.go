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

var _ = Describe("Test Bufconn Environment", func() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.ErrorLevel),
		))),
		meta.WithProvider(tracing.Tracer),
	)
	var cancel1, cancel2, cancel3, cancel4, cancel5, cancel6 context.CancelFunc
	var env test.Environment
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
			env = test.NewBufconnEnvironmentWithLogLevel(zapcore.PanicLevel)
			test.SpawnMonitor(env)
			_, cancel1 = test.SpawnScheduler(env, test.WaitForReady())
			_, cancel2 = test.SpawnCache(env, test.WaitForReady())
			_, cancel3 = test.SpawnAgent(env, test.WaitForReady())
			_, cancel4 = test.SpawnConsumerd(env, test.WaitForReady())
			client = test.NewMonitorClient(env, ctx)
			Eventually(bucketCount).Should(Equal(6))
		})
	})
	When("adding additional components", func() {
		Specify("the number of components should update", func() {
			_, cancel5 = test.SpawnAgent(env, test.WithName("agent1"), test.WaitForReady())
			Eventually(bucketCount).Should(Equal(7))
			_, cancel6 = test.SpawnConsumerd(env, test.WithName("cd1"), test.WaitForReady())
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

var _ = Describe("Test Localhost Environment", func() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.ErrorLevel),
		))),
		meta.WithProvider(tracing.Tracer),
	)
	var cancel1, cancel2, cancel3, cancel4, cancel5, cancel6 context.CancelFunc
	var env test.Environment
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
			env = test.NewLocalhostEnvironmentWithLogLevel(zapcore.PanicLevel)
			test.SpawnMonitor(env)
			_, cancel1 = test.SpawnScheduler(env, test.WaitForReady())
			_, cancel2 = test.SpawnCache(env, test.WaitForReady())
			_, cancel3 = test.SpawnAgent(env, test.WaitForReady())
			_, cancel4 = test.SpawnConsumerd(env, test.WaitForReady())
			client = test.NewMonitorClient(env, ctx)
			Eventually(bucketCount).Should(Equal(6))
		})
	})
	When("adding additional components", func() {
		Specify("the number of components should update", func() {
			_, cancel5 = test.SpawnAgent(env, test.WithName("agent1"), test.WaitForReady())
			Eventually(bucketCount).Should(Equal(7))
			_, cancel6 = test.SpawnConsumerd(env, test.WithName("cd1"), test.WaitForReady())
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
