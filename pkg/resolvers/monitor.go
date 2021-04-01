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
	"github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/pkg/rec"
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
	ctrl.Log.Info("Resolving Monitor")
	monitorSpec := rc.Object.(v1alpha1.MonitorSpec)
	root := rc.RootObject.(*v1alpha1.BuildCluster)
	deployment := &appsv1.Deployment{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      monitorAppName,
	}, deployment,
		rec.WithCreator(rec.FromTemplate("objects/monitor_deployment.yaml")),
	)
	if rec.ShouldRequeue(res, err) {
		ctrl.Log.Info("> Creating Monitor Deployment")
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
			rec.LabelUpdater(monitorSpec.AdditionalLabels,
				&deployment.Spec.Template,
				staticLabels),
			rec.PodImageUpdater(root.Spec.Components.Image,
				&deployment.Spec.Template.Spec, 0),
			rec.PodPullPolicyUpdater(root.Spec.Components.ImagePullPolicy,
				&deployment.Spec.Template.Spec, 0),
		},
	)
	if rec.ShouldRequeue(res, err) {
		ctrl.Log.Info("> Updating Monitor Deployment")
		return rec.RequeueWith(res, err)
	}
	ctrl.Log.Info("✓ Monitor Deployment is up to date")

	svc := &v1.Service{}
	res, err = rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      monitorAppName,
	}, svc,
		rec.WithCreator(rec.FromTemplate("objects/monitor_service.yaml")),
		rec.RecreateIfChanged(),
	)
	if rec.ShouldRequeue(res, err) {
		ctrl.Log.Info("> Creating Monitor Service")
		return rec.RequeueWith(res, err)
	}
	ctrl.Log.Info("✓ Monitor Service already exists")

	return rec.DoNotRequeue()
}

func (r *MonitorResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components.Monitor
}
