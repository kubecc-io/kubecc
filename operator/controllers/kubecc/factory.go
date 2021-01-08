package kubecc

import (
	kccv1alpha1 "github.com/cobalt77/kube-cc/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *KubeccReconciler) makeMgr(kubecc *kccv1alpha1.Kubecc) *appsv1.Deployment {
	labels := map[string]string{
		"app":                 "kubecc-mgr",
		"kubecc.io/kubecc_cr": kubecc.Name,
	}
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-mgr",
			Namespace: "kubecc-operator-system", // todo
			Labels: map[string]string{
				"kubecc.io/kubecc_cr": kubecc.Name,
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
							Name:            "kubecc-mgr",
							Image:           kubecc.Spec.MgrImage,
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
	ctrl.SetControllerReference(kubecc, dep, r.Scheme)
	return dep
}

func (r *KubeccReconciler) makeMgrService(kubecc *kccv1alpha1.Kubecc) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-mgr",
			Namespace: "kubecc-operator-system",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":                 "kubecc-mgr",
				"kubecc.io/kubecc_cr": kubecc.Name,
			},
			Type: v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
				{
					Name:     "grpc",
					Port:     9090,
					Protocol: v1.ProtocolTCP,
				},
			},
		},
	}
	ctrl.SetControllerReference(kubecc, svc, r.Scheme)
	return svc
}

// func (r *kubeccReconciler) makeAgentService(kubecc *kdcv1alpha1.kubecc, pod *v1.Pod) *v1.Service {
// 	labels := map[string]string{
// 		"app":                  "kubecc-agent",
// 		"kkubecc.io/kubecc_cr": kubecc.Name,
// 	}
// 	svc := &v1.Service{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      pod.Name,
// 			Namespace: pod.Namespace,
// 			Labels: map[string]string{
// 				"kkubecc.io/kubecc_cr": kubecc.Name,
// 			},
// 		},
// 		Spec: v1.ServiceSpec{
// 			Selector: labels,
// 			Type:     v1.ServiceTypeClusterIP,
// 			Ports: []v1.ServicePort{
// 				{
// 					Name:     "grpc",
// 					Port:     9090,
// 					Protocol: v1.ProtocolTCP,
// 				},
// 			},
// 		},
// 	}
// 	ctrl.SetControllerReference(kubecc, svc, r.Scheme)
// 	return svc
// }

func (r *KubeccReconciler) makeDaemonSet(kubecc *kccv1alpha1.Kubecc) *appsv1.DaemonSet {
	labels := map[string]string{
		"app":                 "kubecc-agent",
		"kubecc.io/kubecc_cr": kubecc.Name,
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-agent",
			Namespace: kubecc.Namespace,
			Labels: map[string]string{
				"kubecc.io/kubecc_cr": kubecc.Name,
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
						NodeAffinity: &kubecc.Spec.Nodes.NodeAffinity,
					},
					Containers: []v1.Container{
						{
							Name:            "kubecc-agent",
							Image:           kubecc.Spec.AgentImage,
							ImagePullPolicy: v1.PullAlways,
							Resources:       kubecc.Spec.Nodes.Resources,
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
	ctrl.SetControllerReference(kubecc, ds, r.Scheme)
	return ds
}
