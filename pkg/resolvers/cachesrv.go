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
