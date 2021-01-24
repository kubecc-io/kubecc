package resolvers

import (
	appsv1 "k8s.io/api/apps/v1"

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
	agentDaemonSetName = "kubecc-agent"
)

func (r *AgentResolver) Resolve(
	rc rec.ResolveContext,
	obj interface{},
) (ctrl.Result, error) {
	daemonSet := &appsv1.DaemonSet{}
	res, err := rec.FindOrCreate(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      agentDaemonSetName,
	}, daemonSet, rec.FromTemplate("agent_daemonset.yaml", rc))
	if rec.ShouldRequeue(res, err) {
		return rec.Requeue(res, err)
	}
	return rec.DoNotRequeue()
}

func (r *AgentResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Agent
}
