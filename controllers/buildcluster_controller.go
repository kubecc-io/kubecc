package controllers

import (
	"context"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/control"
	"github.com/cobalt77/kubecc/pkg/rec"
)

// BuildClusterReconciler reconciles a BuildCluster object
type BuildClusterReconciler struct {
	client.Client
	Log     *zap.SugaredLogger
	Scheme  *runtime.Scheme
	Binders map[types.NamespacedName]*control.Binder
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
	lg := r.Log.With("buildcluster", req.NamespacedName)

	cluster := &v1alpha1.BuildCluster{}
	res, err := rec.FindOrCreate(r.Client, ctx, nil, req.NamespacedName, cluster, rec.NeverCreate())
	if rec.Done(res, err) {
		return res, err
	}

	rec.Resolve(cluster)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BuildCluster{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Complete(r)
}
