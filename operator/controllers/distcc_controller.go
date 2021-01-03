package controllers

import (
	"context"
	"reflect"
	"sync"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kdistccv1 "github.com/cobalt77/kube-distcc-operator/api/v1"
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
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

func (r *DistccReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("distcc", req.NamespacedName)
	log.Info("Starting reconcile")

	distcc := &kdistccv1.Distcc{}
	err := r.Get(ctx, req.NamespacedName, distcc)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Resource not found (may already be deleted)")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(3)

	var result1, result2, result3 ctrl.Result
	var err1, err2, err3 error
	go func() {
		defer wg.Done()
		result1, err1 = r.reconcileDaemonSet(log, ctx, distcc)
	}()
	go func() {
		defer wg.Done()
		result2, err2 = r.reconcileMgr(log, ctx, distcc)
	}()
	go func() {
		defer wg.Done()
		result3, err3 = r.reconcileService(log, ctx, distcc)
	}()
	wg.Wait()

	return ctrl.Result{
			Requeue: result1.Requeue || result2.Requeue || result3.Requeue,
		}, func() error {
			if err1 != nil {
				return err1
			}
			if err2 != nil {
				return err2
			}
			if err3 != nil {
				return err3
			}
			return nil
		}()
}

func (r *DistccReconciler) reconcileMgr(
	log logr.Logger,
	ctx context.Context,
	distcc *kdistccv1.Distcc,
) (ctrl.Result, error) {
	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      distcc.Name,
		Namespace: distcc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", distcc.Name,
			"Namespace", distcc.Namespace,
		).Info("Creating a new Deployment")
		ds := r.makeMgr(distcc)
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create Deployment")
			return ctrl.Result{}, err
		}
		// Deployment created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DistccReconciler) makeMgr(distcc *kdistccv1.Distcc) *appsv1.Deployment {
	labels := map[string]string{
		"app":       "distcc-mgr",
		"distcc_cr": distcc.Name,
	}
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-mgr",
			Namespace: distcc.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "distcc-mgr",
							Image:   "docker.io/alpine:latest",
							Command: []string{"tail", "-f", "/dev/null"},
							Ports: []v1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: 9090,
									Protocol:      v1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(distcc, dep, r.Scheme)
	return dep
}

func (r *DistccReconciler) reconcileService(
	log logr.Logger,
	ctx context.Context,
	distcc *kdistccv1.Distcc,
) (ctrl.Result, error) {
	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      distcc.Name,
		Namespace: distcc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", distcc.Name,
			"Namespace", distcc.Namespace,
		).Info("Creating a new Service")
		ds := r.makeService(distcc)
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		// Deployment created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DistccReconciler) makeService(distcc *kdistccv1.Distcc) *v1.Service {
	labels := map[string]string{
		"app":       "distcc-agent",
		"distcc_cr": distcc.Name,
	}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-agent",
			Namespace: distcc.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Type:     v1.ServiceTypeClusterIP,
			Ports: func() []v1.ServicePort {
				l := []v1.ServicePort{}
				for _, p := range distcc.Spec.Ports {
					l = append(l, v1.ServicePort{
						Name:     p.Name,
						Protocol: p.Protocol,
						Port:     p.ContainerPort,
					})
				}
				return l
			}(),
		},
	}
	ctrl.SetControllerReference(distcc, svc, r.Scheme)
	return svc
}

func (r *DistccReconciler) reconcileDaemonSet(
	log logr.Logger,
	ctx context.Context,
	distcc *kdistccv1.Distcc,
) (ctrl.Result, error) {
	found := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      distcc.Name,
		Namespace: distcc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", distcc.Name,
			"Namespace", distcc.Namespace,
		).Info("Creating a new DaemonSet")
		ds := r.makeDaemonSet(distcc)
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create DaemonSet")
			return ctrl.Result{}, err
		}
		// Deployment created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get DaemonSet")
		return ctrl.Result{}, err
	}

	needsUpdate := false
	if !reflect.DeepEqual(
		found.Spec.Template.Spec.Affinity.NodeAffinity,
		distcc.Spec.Nodes.NodeAffinity) {
		found.Spec.Template.Spec.Affinity.NodeAffinity =
			&distcc.Spec.Nodes.NodeAffinity
		needsUpdate = true
	}

	container := &found.Spec.Template.Spec.Containers[0]
	if container.Image != distcc.Spec.Image {
		container.Image = distcc.Spec.Image
		needsUpdate = true
	}
	if !reflect.DeepEqual(container.Command, distcc.Spec.Command) {
		container.Command = distcc.Spec.Command
		needsUpdate = true
	}
	if !reflect.DeepEqual(container.Ports, distcc.Spec.Ports) {
		container.Ports = distcc.Spec.Ports
		needsUpdate = true
	}
	if needsUpdate {
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update DaemonSet")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *DistccReconciler) makeDaemonSet(distcc *kdistccv1.Distcc) *appsv1.DaemonSet {
	labels := map[string]string{
		"app":       "distcc-agent",
		"distcc_cr": distcc.Name,
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-agent",
			Namespace: distcc.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						NodeAffinity: &distcc.Spec.Nodes.NodeAffinity,
					},
					Containers: []v1.Container{
						{
							Name:      "distcc-agent",
							Image:     distcc.Spec.Image,
							Command:   distcc.Spec.Command,
							Ports:     distcc.Spec.Ports,
							Resources: distcc.Spec.Nodes.Resources,
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(distcc, ds, r.Scheme)
	return ds
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistccReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kdistccv1.Distcc{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Complete(r)
}
