package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IngressResolver struct {
	rec.Resolver
}

func (r *IngressResolver) Resolve(
	rc rec.ResolveContext,
	obj interface{},
) (ctrl.Result, error) {
	ingress := obj.(*v1alpha1.IngressSpec)
	switch ingress.Kind {
	case "IngressRoute":
		route := &traefikv1alpha1.IngressRoute{}
		res, err := rec.FindOrCreate(rc, types.NamespacedName{
			Namespace: rc.RootObject.GetNamespace(),
			Name:      schedulerDeploymentName,
		}, route, rec.FromTemplate("ingressroute.yaml", rc))
	}
}

func (r *IngressResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Ingress
}
