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

type SchedulerResolver struct {
	rec.Resolver
}

const (
	schedulerDeploymentName = "kubecc-scheduler"
)

func (r *SchedulerResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	schedulerSpec := rc.Object.(v1alpha1.SchedulerSpec)
	deployment := &appsv1.Deployment{}
	res, err := rec.FindOrCreate(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      schedulerDeploymentName,
	}, deployment, rec.FromTemplate("scheduler_deployment.yaml", rc))
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}
	staticLabels := map[string]string{
		"app": "kubecc-scheduler",
	}

	res, err = rec.UpdateIfNeeded(rc, deployment,
		[]rec.Updater{
			rec.AffinityUpdater(schedulerSpec.NodeAffinity,
				&deployment.Spec.Template.Spec),
			rec.ResourceUpdater(schedulerSpec.Resources,
				&deployment.Spec.Template.Spec, 0),
			rec.ImageUpdater(schedulerSpec.Image,
				&deployment.Spec.Template.Spec, 0),
			rec.PullPolicyUpdater(v1.PullPolicy(schedulerSpec.ImagePullPolicy),
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
	return rec.DoNotRequeue()
}

func (r *SchedulerResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Scheduler
}
