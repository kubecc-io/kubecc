package run_test

import (
	"github.com/cobalt77/kubecc/internal/logkc"
	. "github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Run", func() {
	_ = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	// lg := meta.Log(ctx)

	When("something", func() {
		It("should do something", func() {
			exec := run.NewQueuedExecutor(run.WithUsageLimits(&metrics.UsageLimits{
				ConcurrentProcessLimit:  1,
				QueuePressureMultiplier: 1,
				QueueRejectMultiplier:   1,
			}))
			exec.Exec(&SleepTask{Duration: 1})
		})
	})
})
