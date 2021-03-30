package test

import (
	"context"

	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/onsi/gomega"
)

func EventuallyHealthStatusShouldBeReady(ctx context.Context, env *Environment) {
	gomega.Eventually(env.MetricF(ctx, &metrics.Health{})).Should(EqualProto(
		&metrics.Health{
			Status: metrics.OverallStatus_Ready,
		},
	))
}
