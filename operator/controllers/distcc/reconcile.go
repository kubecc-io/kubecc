package distcc

import (
	"context"
	"reflect"
	"time"

	kdcv1alpha1 "github.com/cobalt77/kube-distcc/operator/api/v1alpha1"
	"github.com/go-logr/logr"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *DistccReconciler) reconcileMgr(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking mgr pod")
	distcc := obj.(*kdcv1alpha1.Distcc)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "distcc-mgr",
		Namespace: "kdistcc-operator-system",
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", "distcc-mgr",
			"Namespace", "kdistcc-operator-system",
		).Info("Creating a new Deployment")
		ds := r.makeMgr(obj.(*kdcv1alpha1.Distcc))
		err := r.Create(ctx, ds)
		if err != nil && !errors.IsAlreadyExists(err) {
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
	if container.Image != distcc.Spec.MgrImage {
		log.Info("> Updating mgr image")
		container.Image = distcc.Spec.MgrImage
		err := r.Update(ctx, found)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *DistccReconciler) reconcileMgrService(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking mgr service")
	distcc := obj.(*kdcv1alpha1.Distcc)
	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "distcc-mgr",
		Namespace: "kdistcc-operator-system",
	}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", "distcc-mgr",
			"Namespace", "kdistcc-operator-system",
		).Info("Creating a new Service")
		ds := r.makeMgrService(distcc)
		err := r.Create(ctx, ds)
		if err != nil && !errors.IsAlreadyExists(err) {
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

func (r *DistccReconciler) reconcileAgentServices(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {

	log.Info("Checking agent services")
	/*
		- Get all distcc-agent pods
		- For each pod, ensure there is a matching service with the same name
	*/
	distcc := obj.(*kdcv1alpha1.Distcc)

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(distcc.Namespace),
		client.MatchingLabels(client.MatchingLabels{
			"app":                  "distcc-agent",
			"kdistcc.io/distcc_cr": distcc.Name,
		}),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods",
			"Name", distcc.Name,
			"Namespace", distcc.Namespace,
		)
		return ctrl.Result{}, err
	}

	needRequeue := false
	var svcCreateErr error

	for _, pod := range podList.Items {
		found := &v1.Service{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		}, found)
		if err != nil && errors.IsNotFound(err) {
			log.WithValues(
				"Name", pod.Name,
				"Namespace", pod.Namespace,
			).Info("Creating a new Service")
			ds := r.makeAgentService(distcc, &pod)
			err := r.Create(ctx, ds)
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "Failed to create Service")
				svcCreateErr = err
			}
			// Created successfully
			needRequeue = true
		} else if err != nil {
			log.Error(err, "Failed to get Service")
			needRequeue = true
			svcCreateErr = err
		}
	}
	return ctrl.Result{Requeue: needRequeue}, svcCreateErr
}

func (r *DistccReconciler) reconcileMgrIngress(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking mgr ingress")

	distcc := obj.(*kdcv1alpha1.Distcc)

	found := &v1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "distcc-mgr",
		Namespace: "kdistcc-operator-system",
	}, found)
	if err != nil {
		log.Info("> Not reconciling mgr ingress, mgr is not healthy. Retrying in 10s")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	ingressRoute := &traefikv1alpha1.IngressRoute{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      "distcc-mgr",
		Namespace: "kdistcc-operator-system",
	}, ingressRoute)
	if err != nil && errors.IsNotFound(err) {
		log.Info("> Creating mgr ingress route")
		route := r.makeMgrIngressRoute(distcc)
		err = r.Create(ctx, route)
		if err != nil {
			log.Error(err, "Failed to create ingress route")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ingress route")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DistccReconciler) reconcileAgentIngress(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {

	/*
		- Get all distcc-agent services
		- For each service, ensure there is a matching route
	*/

	distcc := obj.(*kdcv1alpha1.Distcc)

	// Get services

	svcList := &v1.ServiceList{}
	listOpts := []client.ListOption{
		client.InNamespace(distcc.Namespace),
		client.MatchingLabels(client.MatchingLabels{
			"kdistcc.io/distcc_cr": distcc.Name,
		}),
	}
	if err := r.List(ctx, svcList, listOpts...); err != nil {
		log.Error(err, "Failed to list services",
			"Name", distcc.Name,
			"Namespace", distcc.Namespace,
		)
		return ctrl.Result{}, err
	}

	// Get IngressRouteTCP

	route := &traefikv1alpha1.IngressRouteTCP{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "distcc-agent",
		Namespace: distcc.Namespace,
	}, route)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new IngressRouteTCP")
		rt := r.makeAgentIngressRoute(distcc, svcList)
		err := r.Create(ctx, rt)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create IngressRouteTCP")
			return ctrl.Result{}, err
		}
		// Created successfully
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get IngressRouteTCP",
			"Name", "distcc",
			"Namespace", distcc.Namespace,
		)
		return ctrl.Result{}, err
	}

	needsRequeue := false
	if routes := r.routesForServices(distcc, svcList); !reflect.DeepEqual(route.Spec.Routes, routes) {
		log.Info("> Updating routes to match services")
		route.Spec.Routes = routes
		err := r.Update(ctx, route)
		if err != nil {
			log.Error(err, "Failed to update routes")
			return ctrl.Result{}, err
		}
		// Updated successfully
		needsRequeue = true
	}
	if tls := r.tlsForServices(distcc, svcList); !reflect.DeepEqual(route.Spec.TLS, tls) {
		log.Info("> Updating TLS to match services")
		route.Spec.TLS = tls
		err := r.Update(ctx, route)
		if err != nil {
			log.Error(err, "Failed to update TLS")
			return ctrl.Result{}, err
		}
		// Updated successfully
		needsRequeue = true
	}

	return ctrl.Result{Requeue: needsRequeue}, nil
}

func (r *DistccReconciler) reconcileAgents(
	log logr.Logger,
	ctx context.Context,
	obj client.Object,
) (ctrl.Result, error) {
	log.Info("Checking agent pods")

	found := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      "distcc-agent",
		Namespace: obj.GetNamespace(),
	}, found)
	distcc := obj.(*kdcv1alpha1.Distcc)
	if err != nil && errors.IsNotFound(err) {
		log.WithValues(
			"Name", obj.GetName(),
			"Namespace", obj.GetNamespace(),
		).Info("Creating a new DaemonSet")
		ds := r.makeDaemonSet(distcc)
		err := r.Create(ctx, ds)
		if err != nil && !errors.IsAlreadyExists(err) {
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
		&distcc.Spec.Nodes.NodeAffinity) {
		log.Info("> Node affinity changed")
		found.Spec.Template.Spec.Affinity.NodeAffinity =
			&distcc.Spec.Nodes.NodeAffinity
		needsUpdate = true
	}

	container := &found.Spec.Template.Spec.Containers[0]
	if container.Image != distcc.Spec.AgentImage {
		log.Info("> Container image changed")
		container.Image = distcc.Spec.AgentImage
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
