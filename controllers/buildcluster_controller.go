package controllers

import (
	"context"

	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	"github.com/cobalt77/kubecc/pkg/resolvers"
)

// BuildClusterReconciler reconciles a BuildCluster object
type BuildClusterReconciler struct {
	client.Client
	Log         *zap.SugaredLogger
	Scheme      *runtime.Scheme
	resolveTree *rec.ResolverTree
}

// +kubebuilder:rbac:groups=kubecc.io,resources=buildclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubecc.io,resources=buildclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubecc.io,resources=buildclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=traefik.containo.us,resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BuildClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cluster := &v1alpha1.BuildCluster{}
	res, err := rec.FindOrCreate(rec.ResolveContext{
		Context:    ctx,
		Client:     r.Client,
		RootObject: cluster,
		Object:     cluster,
	}, req.NamespacedName, cluster, rec.MustExist)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return r.resolveTree.Walk(ctx, cluster)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.resolveTree = rec.BuildRootResolver(r.Client, &rec.ResolverTree{
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
				},
			},
			{
				Resolver: &resolvers.IngressResolver{},
			},
			{
				Resolver: &resolvers.TracingResolver{},
			},
		},
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BuildCluster{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Owns(&traefikv1alpha1.IngressRoute{}).
		Complete(r)
}