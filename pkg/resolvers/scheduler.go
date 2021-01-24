package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	"github.com/cobalt77/kubecc/pkg/templates"
	appsv1 "k8s.io/api/apps/v1"
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
	obj interface{},
) (ctrl.Result, error) {
	deployment := &appsv1.Deployment{}
	res, err := rec.FindOrCreate(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      schedulerDeploymentName,
	}, deployment, rec.FromTemplate("scheduler_deployment.yaml", rc))
	if rec.ShouldRequeue(res, err) {
		return rec.Requeue(res, err)
	}
	return rec.DoNotRequeue()
}

func (r *SchedulerResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Scheduler
}

func (r *AgentResolver) makeDefaultDeployment(
	rc rec.ResolveContext,
) (client.Object, error) {
	agent := rc.Object.(*v1alpha1.AgentSpec)
	tmplData, err := templates.Load("scheduler_deployment", agent)
	if err != nil {
		return nil, err
	}
	ds := &appsv1.DaemonSet{}
	err = templates.Unmarshal(tmplData, rc.Client.Scheme(), ds)
	if err != nil {
		return nil, err
	}
	ds.ObjectMeta.Namespace = rc.RootObject.GetNamespace()
	return ds, nil
}
