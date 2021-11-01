package buildcluster

import (
	"github.com/kubecc-io/kubecc/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) toolbox() ([]resources.Resource, error) {
	img, pullPolicy, err := r.kubeccImage()
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"app":         "kubecc-toolbox",
		"kubecc-role": "control-plane",
	}
	toolbox := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-toolbox",
			Namespace: r.buildCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					PriorityClassName: "kubecc-low-priority",
					Affinity: &corev1.Affinity{
						PodAntiAffinity: agentAntiAffinity(),
					},
					InitContainers: []corev1.Container{
						{
							Name:            "copy-binaries",
							Image:           img,
							ImagePullPolicy: pullPolicy,
							Command: []string{
								"/bin/sh",
								"-c",
								"cp /kubecc /tmp/kubecc-bin",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kubecc-binary",
									MountPath: "/tmp/kubecc-bin",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "kubecc-toolbox",
							Image:   "alpine:latest",
							Command: []string{"sleep"},
							Args:    []string{"infinity"},
							VolumeMounts: []corev1.VolumeMount{
								configVolumeMount(),
								{
									Name:      "kubecc-binary",
									MountPath: "/usr/bin/kubecc",
									SubPath:   "kubecc",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						configVolume(),
						{
							Name: "kubecc-binary",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(r.buildCluster, toolbox, r.client.Scheme())
	if r.buildCluster.Spec.DeployToolbox {
		return []resources.Resource{
			resources.Present(toolbox),
		}, nil
	}
	return []resources.Resource{
		resources.Absent(toolbox),
	}, nil
}
