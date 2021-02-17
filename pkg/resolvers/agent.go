package resolvers

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AgentResolver struct {
	rec.Resolver
}

const (
	agentAppName = "kubecc-agent"
)

func (r *AgentResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	agentSpec := rc.Object.(v1alpha1.AgentSpec)
	daemonSet := &appsv1.DaemonSet{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      agentAppName,
	}, daemonSet,
		rec.WithCreator(rec.FromTemplate("agent_daemonset.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}
	staticLabels := map[string]string{
		"app": agentAppName,
	}

	res, err = rec.UpdateIfNeeded(rc, daemonSet,
		[]rec.Updater{
			rec.AffinityUpdater(agentSpec.NodeAffinity,
				&daemonSet.Spec.Template.Spec),
			rec.ResourceUpdater(agentSpec.Resources,
				&daemonSet.Spec.Template.Spec, 0),
			rec.ImageUpdater(agentSpec.Image,
				&daemonSet.Spec.Template.Spec, 0),
			rec.PullPolicyUpdater(agentSpec.ImagePullPolicy,
				&daemonSet.Spec.Template.Spec, 0),
			rec.LabelUpdater(agentSpec.AdditionalLabels,
				&daemonSet.Spec.Template,
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
		Name:      agentAppName,
	}, svc,
		rec.WithCreator(rec.FromTemplate("agent_service.yaml")),
		rec.RecreateIfChanged(),
	)

	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return rec.DoNotRequeue()
}

func (r *AgentResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Agent
}
