package kubecc

import (
	"fmt"

	v1alpha1 "github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/cluster"
	ldv1alpha2 "github.com/linkerd/linkerd2/controller/gen/apis/serviceprofile/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *KubeccReconciler) makeScheduler(kubecc *v1alpha1.Kubecc) *appsv1.Deployment {
	labels := map[string]string{
		"app":                 "kubecc-scheduler",
		"kubecc.io/kubecc_cr": kubecc.Name,
	}
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-scheduler",
			Namespace: kubecc.Namespace,
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
					Annotations: map[string]string{
						"linkerd.io/inject": "enabled",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "kubecc-scheduler",
							Image:           kubecc.Spec.SchedulerImage,
							ImagePullPolicy: v1.PullAlways,
							Env: append(cluster.MakeDownwardApi(),
								v1.EnvVar{
									Name:  "JAEGER_ENDPOINT",
									Value: "http://simplest-collector.observability.svc.cluster.local:14268/api/traces",
								},
							),
							Ports: []v1.ContainerPort{
								{

									Name:          "grpc",
									ContainerPort: 9090,
									Protocol:      v1.ProtocolTCP,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/config",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "config",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "scheduler-config",
									},
									Items: []v1.KeyToPath{
										{
											Key:  "scheduler.yaml",
											Path: "scheduler.yaml",
										},
									},
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

func (r *KubeccReconciler) makeSchedulerService(kubecc *v1alpha1.Kubecc) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-scheduler",
			Namespace: kubecc.Namespace,
			Annotations: map[string]string{
				"linkerd.io/inject": "enabled",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":                 "kubecc-scheduler",
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

func (r *KubeccReconciler) makeAgentService(kubecc *v1alpha1.Kubecc) *v1.Service {
	labels := map[string]string{
		"app":                 "kubecc-agent",
		"kubecc.io/kubecc_cr": kubecc.Name,
	}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-agent",
			Namespace: kubecc.Namespace,
			Labels: map[string]string{
				"kubecc.io/kubecc_cr": kubecc.Name,
			},
			Annotations: map[string]string{
				"linkerd.io/inject": "enabled",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Type:     v1.ServiceTypeClusterIP,
			//	ClusterIP: v1.ClusterIPNone,
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

func (r *KubeccReconciler) makeDaemonSet(kubecc *v1alpha1.Kubecc) *appsv1.DaemonSet {
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
					Annotations: map[string]string{
						"linkerd.io/inject": "enabled",
					},
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
							Env: append(cluster.MakeDownwardApi(),
								v1.EnvVar{
									Name:  "JAEGER_ENDPOINT",
									Value: "http://simplest-collector.observability.svc.cluster.local:14268/api/traces",
								},
							),
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

var schedulerDefaultConfig string = `
scheduler: roundRobinDns
`

func (r *KubeccReconciler) makeSchedulerConfigMap(kubecc *v1alpha1.Kubecc) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduler-config",
			Namespace: kubecc.Namespace,
		},
		Data: map[string]string{
			"scheduler.yaml": schedulerDefaultConfig,
		},
	}
}

func (r *KubeccReconciler) makeServiceProfiles(kubecc *v1alpha1.Kubecc) []*ldv1alpha2.ServiceProfile {
	spAgent := &ldv1alpha2.ServiceProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kubecc-agent.%s.svc.cluster.local", kubecc.Namespace),
			Namespace: kubecc.Namespace,
		},
		Spec: ldv1alpha2.ServiceProfileSpec{
			Routes: []*ldv1alpha2.RouteSpec{
				{
					Name: "POST /Agent/Compile",
					Condition: &ldv1alpha2.RequestMatch{
						Method:    "POST",
						PathRegex: "/Agent/Compile",
					},
					IsRetryable: true,
				},
				{
					Name: "POST /Scheduler/Compile",
					Condition: &ldv1alpha2.RequestMatch{
						Method:    "POST",
						PathRegex: "/Scheduler/Compile",
					},
					IsRetryable: true,
				},
			},
		},
	}
	spScheduler := &ldv1alpha2.ServiceProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local", kubecc.Namespace),
			Namespace: kubecc.Namespace,
		},
		Spec: ldv1alpha2.ServiceProfileSpec{
			Routes: []*ldv1alpha2.RouteSpec{
				{
					Name: "POST /Agent/Compile",
					Condition: &ldv1alpha2.RequestMatch{
						Method:    "POST",
						PathRegex: "/Agent/Compile",
					},
					IsRetryable: true,
				},
				{
					Name: "POST /Scheduler/Compile",
					Condition: &ldv1alpha2.RequestMatch{
						Method:    "POST",
						PathRegex: "/Scheduler/Compile",
					},
					IsRetryable: true,
				},
			},
		},
	}
	ctrl.SetControllerReference(kubecc, spAgent, r.Scheme)
	ctrl.SetControllerReference(kubecc, spScheduler, r.Scheme)
	return []*ldv1alpha2.ServiceProfile{spAgent, spScheduler}
}
