package kubecc

import (
	"context"
	"reflect"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/internal/lll"
	ldv1alpha2 "github.com/linkerd/linkerd2/controller/gen/apis/serviceprofile/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *KubeccReconciler) reconcileScheduler(
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	lll.Info("Checking scheduler pod")
	kubecc := obj.(*v1alpha1.Kubecc)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-scheduler",
		Namespace: kubecc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		lll.With(
			"Name", "kubecc-scheduler",
			"Namespace", kubecc.Namespace,
		).Info("Creating scheduler Deployment")
		ds := r.makeScheduler(obj.(*v1alpha1.Kubecc))
		err := r.Create(ctx, ds)
		if err != nil {
			lll.Error(err, "Failed to create Deployment")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		lll.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	container := &found.Spec.Template.Spec.Containers[0]
	if container.Image != kubecc.Spec.SchedulerImage {
		lll.Info("> Updating scheduler image")
		container.Image = kubecc.Spec.SchedulerImage
		err := r.Update(ctx, found)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileSchedulerService(
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	lll.Info("Checking scheduler service")
	kubecc := obj.(*v1alpha1.Kubecc)
	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-scheduler",
		Namespace: kubecc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		lll.With(
			"Name", "kubecc-scheduler",
			"Namespace", kubecc.Namespace,
		).Info("Creating scheduler Service")
		ds := r.makeSchedulerService(kubecc)
		err := r.Create(ctx, ds)
		if err != nil {
			lll.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		lll.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileAgents(
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	lll.Info("Checking agent pods")

	found := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-agent",
		Namespace: obj.GetNamespace(),
	}, found)
	kubecc := obj.(*v1alpha1.Kubecc)
	if err != nil && errors.IsNotFound(err) {
		lll.With(
			"Name", obj.GetName(),
			"Namespace", obj.GetNamespace(),
		).Info("Creating agent DaemonSet")
		ds := r.makeDaemonSet(kubecc)
		err := r.Create(ctx, ds)
		if err != nil {
			lll.Error(err, "Failed to create DaemonSet")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		lll.Error(err, "Failed to get DaemonSet")
		return ctrl.Result{}, err
	}

	needsUpdate := false
	if !reflect.DeepEqual(
		found.Spec.Template.Spec.Affinity.NodeAffinity,
		&kubecc.Spec.Nodes.NodeAffinity) {
		lll.Info("> Node affinity changed")
		found.Spec.Template.Spec.Affinity.NodeAffinity =
			&kubecc.Spec.Nodes.NodeAffinity
		needsUpdate = true
	}

	container := &found.Spec.Template.Spec.Containers[0]
	if container.Image != kubecc.Spec.AgentImage {
		lll.Info("> Container image changed")
		container.Image = kubecc.Spec.AgentImage
		needsUpdate = true
	}
	if needsUpdate {
		lll.Info("Spec changes detected, updating DaemonSet")
		err = r.Update(ctx, found)
		if err != nil {
			lll.Error(err, "Failed to update DaemonSet")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileAgentService(
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	lll.Info("Checking agent service")
	kubecc := obj.(*v1alpha1.Kubecc)
	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-agent",
		Namespace: kubecc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		lll.With(
			"Name", "kubecc-agent",
			"Namespace", kubecc.Namespace,
		).Info("Creating agent Service")
		ds := r.makeAgentService(kubecc)
		err := r.Create(ctx, ds)
		if err != nil {
			lll.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		lll.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileConfigMaps(
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	lll.Info("Checking ConfigMaps")
	kubecc := obj.(*v1alpha1.Kubecc)

	found := &v1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "scheduler-config",
		Namespace: kubecc.Namespace,
	}, found)

	if err != nil && errors.IsNotFound(err) {
		lll.With(
			"Name", "scheduler-config",
			"Namespace", kubecc.Namespace,
		).Info("Creating scheduler ConfigMap")
		cfg := r.makeSchedulerConfigMap(kubecc)
		err := r.Create(ctx, cfg)
		if err != nil {
			lll.Error(err, "Failed to create ConfigMap")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		lll.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileServiceProfile(
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	lll.Info("Checking Linkerd ServiceProfile")
	kubecc := obj.(*v1alpha1.Kubecc)

	profiles := r.makeServiceProfiles(kubecc)
	requeue := false
	for _, p := range profiles {
		lg := lll.With("object", p)
		found := &ldv1alpha2.ServiceProfile{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      p.ObjectMeta.Name,
			Namespace: p.ObjectMeta.Namespace,
		}, found)
		if err != nil && errors.IsNotFound(err) {
			err := r.Create(ctx, p)
			if err != nil {
				lg.Error(err, "Failed to create")
				return ctrl.Result{}, err
			}
			requeue = true
			continue
		} else if err != nil {
			lg.Error(err, "Failed to get")
			return ctrl.Result{}, err
		}
		if !reflect.DeepEqual(found.Spec, p.Spec) {
			lg.Info("Updating modified object")
			found.Spec = p.Spec
			err := r.Update(ctx, found)
			if err != nil {
				lg.Error(err, "Failed to update")
			}
			requeue = true
		}
	}
	return ctrl.Result{Requeue: requeue}, nil
}
