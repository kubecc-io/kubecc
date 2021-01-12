package kubecc

import (
	"context"
	"reflect"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *KubeccReconciler) reconcileScheduler(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking scheduler pod")
	kubecc := obj.(*v1alpha1.Kubecc)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-scheduler",
		Namespace: kubecc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", "kubecc-scheduler",
			"Namespace", kubecc.Namespace,
		).Info("Creating scheduler Deployment")
		ds := r.makeScheduler(obj.(*v1alpha1.Kubecc))
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create Deployment")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	container := &found.Spec.Template.Spec.Containers[0]
	if container.Image != kubecc.Spec.SchedulerImage {
		log.Info("> Updating scheduler image")
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
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking scheduler service")
	kubecc := obj.(*v1alpha1.Kubecc)
	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-scheduler",
		Namespace: kubecc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", "kubecc-scheduler",
			"Namespace", kubecc.Namespace,
		).Info("Creating scheduler Service")
		ds := r.makeSchedulerService(kubecc)
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileAgents(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking agent pods")

	found := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-agent",
		Namespace: obj.GetNamespace(),
	}, found)
	kubecc := obj.(*v1alpha1.Kubecc)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", obj.GetName(),
			"Namespace", obj.GetNamespace(),
		).Info("Creating agent DaemonSet")
		ds := r.makeDaemonSet(kubecc)
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create DaemonSet")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get DaemonSet")
		return ctrl.Result{}, err
	}

	needsUpdate := false
	if !reflect.DeepEqual(
		found.Spec.Template.Spec.Affinity.NodeAffinity,
		&kubecc.Spec.Nodes.NodeAffinity) {
		log.Info("> Node affinity changed")
		found.Spec.Template.Spec.Affinity.NodeAffinity =
			&kubecc.Spec.Nodes.NodeAffinity
		needsUpdate = true
	}

	container := &found.Spec.Template.Spec.Containers[0]
	if container.Image != kubecc.Spec.AgentImage {
		log.Info("> Container image changed")
		container.Image = kubecc.Spec.AgentImage
		needsUpdate = true
	}
	if needsUpdate {
		log.Info("Spec changes detected, updating DaemonSet")
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update DaemonSet")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileAgentService(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking agent service")
	kubecc := obj.(*v1alpha1.Kubecc)
	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "kubecc-agent",
		Namespace: kubecc.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", "kubecc-agent",
			"Namespace", kubecc.Namespace,
		).Info("Creating agent Service")
		ds := r.makeAgentService(kubecc)
		err := r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *KubeccReconciler) reconcileConfigMaps(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking ConfigMaps")
	kubecc := obj.(*v1alpha1.Kubecc)

	found := &v1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "scheduler-config",
		Namespace: kubecc.Namespace,
	}, found)

	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", "scheduler-config",
			"Namespace", kubecc.Namespace,
		).Info("Creating scheduler ConfigMap")
		cfg := r.makeSchedulerConfigMap(kubecc)
		err := r.Create(ctx, cfg)
		if err != nil {
			log.Error(err, "Failed to create ConfigMap")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
