package distcc

import (
	"fmt"

	kdistccv1 "github.com/cobalt77/kube-distcc-operator/api/v1"
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
			Namespace: distcc.Namespace,
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

func (r *DistccReconciler) makeService(distcc *kdistccv1.Distcc, pod *v1.Pod) *v1.Service {
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

func (r *DistccReconciler) makeIngressRoute(
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
