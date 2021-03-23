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
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapResolver struct{}

const (
	configMapName = "kubecc"
)

func (r *ConfigMapResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	configMap := &v1.ConfigMap{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      configMapName,
	}, configMap,
		rec.WithCreator(rec.FromTemplate("objects/kubecc_configmap.yaml")),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return rec.DoNotRequeue()
}

func (r *ConfigMapResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components
}
