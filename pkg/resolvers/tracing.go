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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TracingResolver struct{}

func (r *TracingResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	// tracing := obj.(*v1alpha1.TracingSpec)

	/*
			v1.EnvVar{
			Name:  "JAEGER_ENDPOINT",
			Value: "http://simplest-collector.observability.svc.cluster.local:14268/api/traces",
		},
	*/
	return rec.DoNotRequeue()
}

func (r *TracingResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Tracing
}
