package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CacheSrvResolver struct{}

const (
	cacheSrvAppName = "kubecc-cache"
)

func (r *CacheSrvResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	cacheSpec := rc.Object.(v1alpha1.CacheSpec)
	deployment := &appsv1.Deployment{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      cacheSrvAppName,
	}, deployment,
		rec.WithCreator(rec.FromTemplate("objects/cachesrv_deployment.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}
	staticLabels := map[string]string{
		"app": cacheSrvAppName,
	}

	res, err = rec.UpdateIfNeeded(rc, deployment,
		[]rec.Updater{
			rec.AffinityUpdater(cacheSpec.NodeAffinity,
				&deployment.Spec.Template.Spec),
			rec.ResourceUpdater(cacheSpec.Resources,
				&deployment.Spec.Template.Spec, 0),
			rec.ImageUpdater(cacheSpec.Image,
				&deployment.Spec.Template.Spec, 0),
			rec.PullPolicyUpdater(cacheSpec.ImagePullPolicy,
				&deployment.Spec.Template.Spec, 0),
			rec.LabelUpdater(cacheSpec.AdditionalLabels,
				&deployment.Spec.Template,
				staticLabels,
			),
		},
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	svc := &v1.Service{}
	res, err = rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      cacheSrvAppName,
	}, svc,
		rec.WithCreator(rec.FromTemplate("objects/cachesrv_service.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return rec.DoNotRequeue()
}

func (r *CacheSrvResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Cache
}
