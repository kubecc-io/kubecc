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

package monitor_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test"
)

var _ = Describe("Monitor Server", func() {
	var monCtx context.Context
	var testEnv test.Environment
	Specify("setup", func() {
		testEnv = test.NewBufconnEnvironmentWithLogLevel(zapcore.ErrorLevel)
		monCtx, _ = test.SpawnMonitor(testEnv)
		test.SpawnScheduler(testEnv) // the scheduler posts a lot of metrics

	})

	It("should eventually become ready", func() {
		test.EventuallyHealthStatusShouldBeReady(monCtx, testEnv)
	})

	It("should post metrics", func() {
		// these metrics are posted evbery 5-7.5s

		Eventually(testEnv.MetricF(monCtx, &metrics.MetricsPostedTotal{}),
			9*time.Second, 100*time.Millisecond).
			Should(WithTransform(func(m *metrics.MetricsPostedTotal) int64 {
				return m.GetTotal()
			}, BeNumerically(">", 0)))

		Eventually(testEnv.MetricF(monCtx, &metrics.ListenerCount{}),
			9*time.Second, 100*time.Millisecond).
			Should(WithTransform(func(m *metrics.ListenerCount) int64 {
				return int64(m.GetCount())
			}, BeNumerically(">", 0)))

		Eventually(testEnv.MetricF(monCtx, &metrics.ProviderCount{}),
			1*time.Second, 20*time.Millisecond).
			Should(WithTransform(func(m *metrics.ProviderCount) int64 {
				return int64(m.GetCount())
			}, BeNumerically(">", 0)))
	})

	Specify("shutdown", func() {
		testEnv.Shutdown()
	})
})
