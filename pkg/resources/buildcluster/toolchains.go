package buildcluster

import (
	"errors"
	"fmt"

	v1alpha1 "github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) toolchain(toolchain v1alpha1.ToolchainSpec) ([]resources.Resource, error) {
	var toolchainName string
	if toolchain.Name == nil {
		toolchainName = fmt.Sprintf("kubecc-agent-%s-%s", toolchain.Kind, toolchain.Version)
	} else {
		toolchainName = *toolchain.Name
	}
	labels := map[string]string{
		"app":       "kubecc-agent",
		"toolchain": toolchainName,
	}

	var compilerImage string
	switch toolchain.Kind {
	case "gcc":
		if toolchain.CustomImage == nil {
			compilerImage = fmt.Sprintf("docker.io/gcc:%s", toolchain.Version)
		} else {
			compilerImage = *toolchain.CustomImage
		}
	case "custom":
		if toolchain.CustomImage == nil {
			return nil, errors.New("toolchain kind is custom, but customImage is not set")
		}
		compilerImage = fmt.Sprintf("%s:%s", *toolchain.CustomImage, toolchain.Version)
	default:
		return nil, errors.New("toolchain kind is not supported")
	}

	img, pullPolicy, err := r.kubeccImage()
	if err != nil {
		return nil, err
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      toolchainName,
			Namespace: r.buildCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: r.buildCluster.Spec.Components.Agent.NodeAffinity,
					},
					Containers: []corev1.Container{
						{
							Name:  "compiler",
							Image: compilerImage,
							Command: []string{
								"/usr/bin/kubecc",
								"run",
								"agent",
							},
							Env: downwardApiEnv(),
							Ports: []corev1.ContainerPort{
								grpcPort(),
							},
							VolumeMounts: []corev1.VolumeMount{
								configVolumeMount(),
								{
									Name:      "kubecc-binary",
									MountPath: "/usr/bin/kubecc",
									SubPath:   "kubecc",
								},
							},
							Resources: r.buildCluster.Spec.Components.Agent.Resources,
						},
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
	ctrl.SetControllerReference(r.buildCluster, ds, r.client.Scheme())
	return []resources.Resource{
		resources.Present(ds),
	}, nil
}
