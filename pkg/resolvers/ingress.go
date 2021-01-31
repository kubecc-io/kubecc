package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IngressResolver struct {
	rec.Resolver
}

func (r *IngressResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	// ingress := rc.Object.(v1alpha1.IngressSpec)
	// switch ingress.Kind {
	// case "IngressRoute":
	// 	route := &traefikv1alpha1.IngressRoute{}
	// 	res, err := rec.FindOrCreate(rc, types.NamespacedName{
	// 		Namespace: rc.RootObject.GetNamespace(),
	// 		Name:      schedulerDeploymentName,
	// 	}, route, rec.FromTemplate("ingressroute.yaml", rc))
	// 	if rec.ShouldRequeue(res, err) {
	// 		return rec.RequeueWith(res, err)
	// 	}
	// 	if ingress.
	// 	if len(route.Spec.TLS.Domains) == 0 {
	// 		route.Spec.TLS.Domains = []traefik.Domain{
	// 			{
	// 				Main: "",
	// 			},
	// 		}
	// 	}
	// 	res, err = rec.UpdateIfNeeded(rc, route,
	// 		[]rec.Update{
	// 			{
	// 				Desired: ingress.Host,
	// 				Actual:  route.Spec.TLS.Domains[0].Main,
	// 			},
	// 		})
	// 	if rec.ShouldRequeue(res, err) {
	// 		return rec.RequeueWith(res, err)
	// 	}
	// 	return rec.DoNotRequeue()
	// case "Ingress":
	// 	ingressObj := &v1.Ingress{}
	// 	res, err := rec.FindOrCreate(rc, types.NamespacedName{
	// 		Namespace: rc.RootObject.GetNamespace(),
	// 		Name:      schedulerDeploymentName,
	// 	}, ingressObj, rec.FromTemplate("ingress.yaml", rc))
	// default:
	// 	lll.Errorf("Unknown ingress kind: %s", ingress.Kind)
	// }
	return rec.DoNotRequeue()
}

func (r *IngressResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Ingress
}
