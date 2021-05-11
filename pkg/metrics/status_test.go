package metrics_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	types "github.com/kubecc-io/kubecc/pkg/types"
)

type test struct {
	metrics.StatusController
}

func status(h *metrics.Health) metrics.OverallStatus {
	return h.Status
}

var _ = Describe("Status", func() {
	test := &test{}

	var updateStream chan *metrics.Health
	testCtx := meta.NewContext(
		meta.WithProvider(identity.Component,
			meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.ErrorLevel),
		))),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)

	When("initializing", func() {
		It("should set health to Pending on BeginInitialize", func() {
			test.BeginInitialize(testCtx)
			updateStream = test.StreamHealthUpdates()
			Eventually(updateStream).Should(Receive(
				WithTransform(status, Equal(metrics.OverallStatus_Initializing))))
		})
		It("should set health to Ready on EndInitialize", func() {
			test.EndInitialize()
			Eventually(updateStream).Should(Receive(
				WithTransform(status, Equal(metrics.OverallStatus_Ready))))
		})
	})
	cond1Ctx, cond1Cancel := context.WithCancel(context.Background())
	When("applying the missing optional condition", func() {
		It("should set health to Degraded", func() {
			test.ApplyCondition(cond1Ctx, metrics.StatusConditions_MissingOptionalComponent)
			Eventually(updateStream).Should(Receive(
				WithTransform(status, Equal(metrics.OverallStatus_Degraded))))
		})
		It("should clear the condition when its context is canceled", func() {
			cond1Cancel()
			Eventually(updateStream).Should(Receive(
				WithTransform(status, Equal(metrics.OverallStatus_Ready))))
		})
	})
	cond2Ctx, cond2Cancel := context.WithCancel(context.Background())
	When("applying the missing critical condition", func() {
		It("should set health to Unavailable", func() {
			test.ApplyCondition(cond2Ctx, metrics.StatusConditions_MissingCriticalComponent)
			Eventually(updateStream).Should(Receive(
				WithTransform(status, Equal(metrics.OverallStatus_Unavailable))))
		})
		It("should clear the condition when its context is canceled", func() {
			cond2Cancel()
			Eventually(updateStream).Should(Receive(
				WithTransform(status, Equal(metrics.OverallStatus_Ready))))
		})
	})
	type testCase struct {
		conds  []metrics.StatusConditions
		states []metrics.OverallStatus
	}
	When("applying multiple conditions", func() {
		tests := []testCase{
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Degraded,
					metrics.OverallStatus_Degraded,
					metrics.OverallStatus_Degraded,
					metrics.OverallStatus_Degraded,
				},
			},
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingCriticalComponent,
					metrics.StatusConditions_MissingCriticalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Degraded,
					metrics.OverallStatus_Degraded,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
				},
			},
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_MissingCriticalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
				},
			},
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_MissingCriticalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingCriticalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
				},
			},
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingCriticalComponent,
					metrics.StatusConditions_MissingOptionalComponent,
					metrics.StatusConditions_MissingCriticalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Degraded,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
				},
			},
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_MissingCriticalComponent,
					metrics.StatusConditions_InvalidConfiguration,
					metrics.StatusConditions_InternalError,
					metrics.StatusConditions_MissingOptionalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Unavailable,
				},
			},
			{
				conds: []metrics.StatusConditions{
					metrics.StatusConditions_InvalidConfiguration,
					metrics.StatusConditions_Pending,
					metrics.StatusConditions_InternalError,
					metrics.StatusConditions_MissingOptionalComponent,
				},
				states: []metrics.OverallStatus{
					metrics.OverallStatus_Unavailable,
					metrics.OverallStatus_Initializing,
					metrics.OverallStatus_Initializing,
					metrics.OverallStatus_Initializing,
				},
			},
		}
		Context("it should keep track of each condition", func() {
			for i, t := range tests {
				conds := t.conds
				states := t.states
				Specify(fmt.Sprintf("Test %d", i+1), func() {
					contexts := make([]context.Context, len(conds))
					cancels := make([]context.CancelFunc, len(conds))
					for ii := range conds {
						c, ca := context.WithCancel(context.Background())
						contexts[ii] = c
						cancels[ii] = ca
					}
					for ii := range conds {
						ctx := contexts[ii]
						cond := conds[ii]
						state := states[ii]
						test.ApplyCondition(ctx, cond)
						Eventually(updateStream).Should(Receive(
							WithTransform(status, Equal(state))))
					}
					for ii := len(conds) - 1; ii >= 0; ii-- {
						ca := cancels[ii]
						ca()
						expected := metrics.OverallStatus_Ready
						if ii > 0 {
							expected = states[ii-1]
						}
						Eventually(updateStream).Should(Receive(
							WithTransform(status, Equal(expected))))
					}
				})
			}
		})
	})
})
