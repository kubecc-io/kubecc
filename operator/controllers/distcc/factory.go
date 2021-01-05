package distcc

import (
	"fmt"

	kdistccv1 "github.com/cobalt77/kube-distcc/operator/api/v1"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	"github.com/traefik/traefik/v2/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *DistccReconciler) makeMgr(distcc *kdistccv1.Distcc) *appsv1.Deployment {
	labels := map[string]string{
		"app":                  "distcc-mgr",
		"kdistcc.io/distcc_cr": distcc.Name,
	}
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-mgr",
			Namespace: "kdistcc-operator-system", // todo
			Labels: map[string]string{
				"kdistcc.io/distcc_cr": distcc.Name,
			},
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
							Name:            "distcc-mgr",
							Image:           distcc.Spec.MgrImage,
							ImagePullPolicy: v1.PullAlways,
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

func (r *DistccReconciler) makeMgrService(distcc *kdistccv1.Distcc) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-mgr",
			Namespace: "kdistcc-operator-system",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":                  "distcc-mgr",
				"kdistcc.io/distcc_cr": distcc.Name,
			},
			Type: v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
				{
					Name:     "grpc",
					Protocol: v1.ProtocolTCP,
					Port:     9090,
				},
			},
		},
	}
	ctrl.SetControllerReference(distcc, svc, r.Scheme)
	return svc
}

func (r *DistccReconciler) makeAgentService(distcc *kdistccv1.Distcc, pod *v1.Pod) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels: map[string]string{
				"kdistcc.io/distcc_cr": distcc.Name,
			},
		},
		Spec: v1.ServiceSpec{
			ExternalName: fmt.Sprintf("%s.%s",
				pod.Name,
				distcc.Spec.Hostname),
			Type: v1.ServiceTypeExternalName,
		},
	}
	ctrl.SetControllerReference(distcc, svc, r.Scheme)
	return svc
}

func (r *DistccReconciler) routesForServices(
	distcc *kdistccv1.Distcc,
	services *v1.ServiceList,
) (list []traefikv1alpha1.RouteTCP) {
	list = make([]traefikv1alpha1.RouteTCP, len(services.Items))
	for i, svc := range services.Items {
		list[i] = traefikv1alpha1.RouteTCP{
			Match: fmt.Sprintf("HostSNI(`%s`)", svc.Spec.ExternalName),
			Services: []traefikv1alpha1.ServiceTCP{
				{
					Name:      svc.Name,
					Namespace: svc.Namespace,
					Port:      3632,
				},
			},
		}
	}
	return
}

func (r *DistccReconciler) tlsForServices(
	distcc *kdistccv1.Distcc,
	services *v1.ServiceList,
) *traefikv1alpha1.TLSTCP {
	return &traefikv1alpha1.TLSTCP{
		SecretName: "kdistcc-tls",
		Domains: func() (list []types.Domain) {
			list = make([]types.Domain, len(services.Items))
			for i, svc := range services.Items {
				list[i] = types.Domain{
					Main: fmt.Sprintf("%s", svc.Spec.ExternalName),
				}
			}
			return
		}(),
	}
}

func (r *DistccReconciler) makeMgrIngressRoute(
	distcc *kdistccv1.Distcc,
) *traefikv1alpha1.IngressRoute {
	rt := &traefikv1alpha1.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-mgr",
			Namespace: "kdistcc-operator-system",
		},
		Spec: traefikv1alpha1.IngressRouteSpec{
			EntryPoints: []string{"websecure"},
			Routes: []traefikv1alpha1.Route{
				{
					Kind:  "Rule",
					Match: fmt.Sprintf("Host(`%s`)", distcc.Spec.Hostname),
					Services: []traefikv1alpha1.Service{
						{
							LoadBalancerSpec: traefikv1alpha1.LoadBalancerSpec{
								Name:      "distcc-mgr",
								Namespace: "kdistcc-operator-system",
								Kind:      "Service",
								Port:      9090,
							},
						},
					},
				},
			},
			TLS: &traefikv1alpha1.TLS{
				SecretName: "kdistcc-tls",
				Domains: []types.Domain{
					{
						Main: distcc.Spec.Hostname,
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(distcc, rt, r.Scheme)
	return rt
}

func (r *DistccReconciler) makeAgentIngressRoute(
	distcc *kdistccv1.Distcc,
	services *v1.ServiceList,
) *traefikv1alpha1.IngressRouteTCP {
	rt := &traefikv1alpha1.IngressRouteTCP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-agent",
			Namespace: distcc.Namespace,
		},
		Spec: traefikv1alpha1.IngressRouteTCPSpec{
			EntryPoints: []string{"websecure"},
			Routes:      r.routesForServices(distcc, services),
			TLS:         r.tlsForServices(distcc, services),
		},
	}

	ctrl.SetControllerReference(distcc, rt, r.Scheme)
	return rt
}

func (r *DistccReconciler) makeDaemonSet(distcc *kdistccv1.Distcc) *appsv1.DaemonSet {
	labels := map[string]string{
		"app":                  "distcc-agent",
		"kdistcc.io/distcc_cr": distcc.Name,
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "distcc-agent",
			Namespace: distcc.Namespace,
			Labels: map[string]string{
				"kdistcc.io/distcc_cr": distcc.Name,
			},
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
							Name:            "distcc-agent",
							Image:           distcc.Spec.AgentImage,
							ImagePullPolicy: v1.PullAlways,
							Resources:       distcc.Spec.Nodes.Resources,
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(distcc, ds, r.Scheme)
	return ds
}
