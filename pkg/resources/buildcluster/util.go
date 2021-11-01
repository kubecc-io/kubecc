package buildcluster

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func genericService(name, namespace string, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "grpc",
					Port:     9090,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

func downwardApiEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "KUBECC_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "KUBECC_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "KUBECC_NODE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func grpcPort() corev1.ContainerPort {
	return corev1.ContainerPort{
		Name:          "grpc",
		ContainerPort: 9090,
		Protocol:      corev1.ProtocolTCP,
	}
}

func configVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "config",
		MountPath: "/etc/kubecc",
	}
}

func configVolume() corev1.Volume {
	return corev1.Volume{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "kubecc",
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "config.yaml",
						Path: "config.yaml",
					},
				},
			},
		},
	}
}

func (r *Reconciler) kubeccImage() (string, corev1.PullPolicy, error) {
	if r.buildCluster.Spec.Image != "" {
		return r.buildCluster.Spec.Image, r.buildCluster.Spec.ImagePullPolicy, nil
	}
	if r.buildCluster.Status.DefaultImageName != "" {
		return r.buildCluster.Status.DefaultImageName, r.buildCluster.Spec.ImagePullPolicy, nil
	}
	return "", corev1.PullPolicy(""), errors.New("failed to get default image name, retrying")
}

func agentAntiAffinity() *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubecc-role": "agent",
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
	}
}
func controlPlaneAntiAffinity() *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubecc-role": "control-plane",
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
	}
}
