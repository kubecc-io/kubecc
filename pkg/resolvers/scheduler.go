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

type SchedulerResolver struct{}

const (
	schedulerAppName = "kubecc-scheduler"
)

func (r *SchedulerResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	schedulerSpec := rc.Object.(v1alpha1.SchedulerSpec)
	deployment := &appsv1.Deployment{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      schedulerAppName,
	}, deployment,
		rec.WithCreator(rec.FromTemplate("scheduler_deployment.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}
	staticLabels := map[string]string{
		"app": schedulerAppName,
	}

	res, err = rec.UpdateIfNeeded(rc, deployment,
		[]rec.Updater{
			rec.AffinityUpdater(schedulerSpec.NodeAffinity,
				&deployment.Spec.Template.Spec),
			rec.ResourceUpdater(schedulerSpec.Resources,
				&deployment.Spec.Template.Spec, 0),
			rec.ImageUpdater(schedulerSpec.Image,
				&deployment.Spec.Template.Spec, 0),
			rec.PullPolicyUpdater(schedulerSpec.ImagePullPolicy,
				&deployment.Spec.Template.Spec, 0),
			rec.LabelUpdater(schedulerSpec.AdditionalLabels,
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
		Name:      schedulerAppName,
	}, svc,
		rec.WithCreator(rec.FromTemplate("scheduler_service.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return rec.DoNotRequeue()
}

func (r *SchedulerResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Scheduler
}
