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

type MonitorResolver struct{}

const (
	monitorAppName = "kubecc-monitor"
)

func (r *MonitorResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	monitorSpec := rc.Object.(v1alpha1.MonitorSpec)
	deployment := &appsv1.Deployment{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      monitorAppName,
	}, deployment,
		rec.WithCreator(rec.FromTemplate("monitor_deployment.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}
	staticLabels := map[string]string{
		"app": monitorAppName,
	}

	res, err = rec.UpdateIfNeeded(rc, deployment,
		[]rec.Updater{
			rec.AffinityUpdater(monitorSpec.NodeAffinity,
				&deployment.Spec.Template.Spec),
			rec.ResourceUpdater(monitorSpec.Resources,
				&deployment.Spec.Template.Spec, 0),
			rec.ImageUpdater(monitorSpec.Image,
				&deployment.Spec.Template.Spec, 0),
			rec.PullPolicyUpdater(monitorSpec.ImagePullPolicy,
				&deployment.Spec.Template.Spec, 0),
			rec.LabelUpdater(monitorSpec.AdditionalLabels,
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
		Name:      monitorAppName,
	}, svc,
		rec.WithCreator(rec.FromTemplate("monitor_service.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return rec.DoNotRequeue()
}

func (r *MonitorResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Monitor
}
