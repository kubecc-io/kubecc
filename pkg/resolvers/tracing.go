package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TracingResolver struct{}

func (r *TracingResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	// tracing := obj.(*v1alpha1.TracingSpec)

	/*
			v1.EnvVar{
			Name:  "JAEGER_ENDPOINT",
			Value: "http://simplest-collector.observability.svc.cluster.local:14268/api/traces",
		},
	*/
	return rec.DoNotRequeue()
}

func (r *TracingResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Tracing
}
