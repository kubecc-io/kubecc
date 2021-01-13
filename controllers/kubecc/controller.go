package kubecc

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/cobalt77/kubecc/pkg/tools"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/internal/lll"
)

// KubeccReconciler reconciles a Kubecc object
type KubeccReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubecc.io,resources=kubeccs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubecc.io,resources=kubeccs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubecc.io,resources=kubeccs/finalizers,verbs=update
// +kubebuilder:rbac:groups=linkerd.io,resources=serviceprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

func (r *KubeccReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, recErr error) {
	lg := lll.With("kubecc", req.NamespacedName)
	lg.Info("Starting reconcile")

	kubecc := &v1alpha1.Kubecc{}
	err := r.Get(ctx, req.NamespacedName, kubecc)
	if err != nil {
		if errors.IsNotFound(err) {
			lg.Info("Resource not found (may already be deleted)")
			return ctrl.Result{}, nil
		}
		lg.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	result, recErr = tools.ReconcileAndAggregate(
		ctx, kubecc,
		r.reconcileAgents,
		r.reconcileAgentService,
		r.reconcileScheduler,
		r.reconcileSchedulerService,
		r.reconcileConfigMaps,
	//	r.reconcileServiceProfile,
	)
	if result.Requeue && result.RequeueAfter == 0 {
		lg.Info("=> Requeueing...")
	} else if result.Requeue && result.RequeueAfter > 0 {
		lg.Info("=> Requeueing after %s", result.RequeueAfter.String())
	} else if recErr != nil {
		lg.Info("=> Requeueing due to an error")
	} else {
		lg.Info("=> All resources healthy")
	}
	return
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeccReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Kubecc{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		//	Owns(&ldv1alpha2.ServiceProfile{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Complete(r)
}
