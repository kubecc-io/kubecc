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

package controllers

import (
	"context"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/rec"
	"github.com/kubecc-io/kubecc/pkg/resolvers"
)

// BuildClusterReconciler reconciles a BuildCluster object.
type BuildClusterReconciler struct {
	client.Client
	Context     context.Context
	Log         *zap.SugaredLogger
	Scheme      *runtime.Scheme
	resolveTree *rec.ResolverTree
}

// +kubebuilder:rbac:groups=kubecc.io,resources=buildclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubecc.io,resources=buildclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubecc.io,resources=buildclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BuildClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cluster := &v1alpha1.BuildCluster{}
	res, err := rec.Find(rec.ResolveContext{
		Context:    ctx,
		Log:        meta.Log(r.Context),
		Client:     r.Client,
		RootObject: cluster,
		Object:     cluster,
	}, req.NamespacedName, cluster,
		rec.WithCreator(rec.MustExist))
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return r.resolveTree.Walk(ctx, cluster)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.resolveTree = rec.BuildRootResolver(r.Context, r.Client, &rec.ResolverTree{
		Nodes: []*rec.ResolverTree{
			{
				Resolver: &resolvers.ComponentsResolver{},
				Nodes: []*rec.ResolverTree{
					{
						Resolver: &resolvers.AgentResolver{},
					},
					{
						Resolver: &resolvers.SchedulerResolver{},
					},
					{
						Resolver: &resolvers.MonitorResolver{},
					},
					{
						Resolver: &resolvers.CacheSrvResolver{},
					},
				},
			},
			{
				Resolver: &resolvers.TracingResolver{},
			},
			{
				Resolver: &resolvers.ConfigMapResolver{},
			},
		},
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BuildCluster{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Complete(r)
}
