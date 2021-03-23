/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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

type AgentResolver struct{}

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
		rec.WithCreator(rec.FromTemplate("objects/agent_daemonset.yaml")),
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
		rec.WithCreator(rec.FromTemplate("objects/agent_service.yaml")),
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
