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
	var testEnv *test.Environment
	Specify("setup", func() {
		testEnv = test.NewEnvironmentWithLogLevel(zapcore.ErrorLevel)
		monCtx, _ = testEnv.SpawnMonitor()
		testEnv.SpawnScheduler() // the scheduler posts a lot of metrics

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
