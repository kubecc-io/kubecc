package distcc

import (
	"context"

	"github.com/go-logr/logr"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/cobalt77/kube-distcc/operator/controllers/tools"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kdcv1alpha1 "github.com/cobalt77/kube-distcc/operator/api/v1alpha1"
)

// DistccReconciler reconciles a Distcc object
type DistccReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kdistcc.io,resources=distccs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kdistcc.io,resources=distccs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kdistcc.io,resources=distccs/finalizers,verbs=update
// +kubebuilder:rbac:groups=traefik.containo.us,resources=ingressroutetcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traefik.containo.us,resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

func (r *DistccReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, recErr error) {
	log := r.Log.WithValues("distcc", req.NamespacedName)
	log.Info("Starting reconcile")

	distcc := &kdcv1alpha1.Distcc{}
	err := r.Get(ctx, req.NamespacedName, distcc)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Resource not found (may already be deleted)")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	result, recErr = tools.ReconcileAndAggregate(
		log, ctx, distcc,
		r.reconcileAgents,
		r.reconcileMgr,
		r.reconcileAgentServices,
		r.reconcileMgrService,
		r.reconcileAgentIngress,
		r.reconcileMgrIngress,
	)
	if result.Requeue && result.RequeueAfter == 0 {
		log.Info("=> Requeueing...")
	} else if result.Requeue && result.RequeueAfter > 0 {
		log.Info("=> Requeueing after %s", result.RequeueAfter.String())
	} else if recErr != nil {
		log.Info("=> Requeueing due to an error")
	} else {
		log.Info("=> All resources healthy")
	}
	return
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistccReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kdcv1alpha1.Distcc{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Owns(&traefikv1alpha1.IngressRoute{}).
		Owns(&traefikv1alpha1.IngressRouteTCP{}).
		Complete(r)
}
