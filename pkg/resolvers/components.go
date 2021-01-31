package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ComponentsResolver struct {
	rec.Resolver
}

func (r *ComponentsResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	return rec.DoNotRequeue()
}

func (r *ComponentsResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components
}
