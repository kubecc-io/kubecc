package buildcluster

import (
	"fmt"

	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

func (r *Reconciler) configMap() ([]resources.Resource, error) {
	conf := config.KubeccSpec{
		Global: config.GlobalSpec{
			LogLevel: "info",
		},
		Agent: config.AgentSpec{
			UsageLimits: &config.UsageLimitsSpec{
				ConcurrentProcessLimit: -1,
			},
			SchedulerAddress: fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
			MonitorAddress:   fmt.Sprintf("kubecc-monitor.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
		},
		Scheduler: config.SchedulerSpec{
			MonitorAddress: fmt.Sprintf("kubecc-monitor.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
			CacheAddress:   fmt.Sprintf("kubecc-cache.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
			ListenAddress:  ":9090",
		},
		Monitor: config.MonitorSpec{
			ListenAddress: ":9090",
		},
		Cache: config.CacheSpec{
			MonitorAddress: fmt.Sprintf("kubecc-monitor.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
			ListenAddress:  ":9090",
		},
		Kcctl: config.KcctlSpec{
			MonitorAddress:   fmt.Sprintf("kubecc-monitor.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
			SchedulerAddress: fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090", r.buildCluster.Namespace),
			DisableTLS:       true,
		},
	}
	if vs := r.buildCluster.Spec.Components.Cache.VolatileStorage; vs != nil {
		conf.Cache.VolatileStorage = vs
	}
	if ls := r.buildCluster.Spec.Components.Cache.LocalStorage; ls != nil {
		conf.Cache.LocalStorage = ls
	}
	if rs := r.buildCluster.Spec.Components.Cache.RemoteStorage; rs != nil {
		conf.Cache.RemoteStorage = rs
	}
	configData, err := yaml.Marshal(conf)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc",
			Namespace: r.buildCluster.Namespace,
		},
		Data: map[string]string{
			"config.yaml": string(configData),
		},
	}
	ctrl.SetControllerReference(r.buildCluster, cm, r.client.Scheme())
	return []resources.Resource{
		resources.Present(cm),
	}, nil
}

func (r *Reconciler) monitor() ([]resources.Resource, error) {
	img, pullPolicy, err := r.kubeccImage()
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"app": "kubecc-monitor",
	}
	svc := genericService("kubecc-monitor", r.buildCluster.Namespace, labels)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-monitor",
			Namespace: r.buildCluster.Namespace,
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
					Containers: []corev1.Container{
						{
							Name:            "kubecc-monitor",
							Image:           img,
							ImagePullPolicy: pullPolicy,
							Resources:       r.buildCluster.Spec.Components.Monitor.Resources,
							Command:         []string{"/kubecc", "run", "monitor"},
							Ports: []corev1.ContainerPort{
								grpcPort(),
							},
							VolumeMounts: []corev1.VolumeMount{
								configVolumeMount(),
							},
							Env: downwardApiEnv(),
						},
					},
					Volumes: []corev1.Volume{
						configVolume(),
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(r.buildCluster, svc, r.client.Scheme())
	ctrl.SetControllerReference(r.buildCluster, deployment, r.client.Scheme())
	return []resources.Resource{
		resources.Created(svc),
		resources.Present(deployment),
	}, nil
}

func (r *Reconciler) scheduler() ([]resources.Resource, error) {
	img, pullPolicy, err := r.kubeccImage()
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"app": "kubecc-scheduler",
	}
	svc := genericService("kubecc-scheduler", r.buildCluster.Namespace, labels)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-scheduler",
			Namespace: r.buildCluster.Namespace,
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
					Containers: []corev1.Container{
						{
							Name:            "kubecc-scheduler",
							Image:           img,
							ImagePullPolicy: pullPolicy,
							Resources:       r.buildCluster.Spec.Components.Scheduler.Resources,
							Command:         []string{"/kubecc", "run", "scheduler"},
							Ports: []corev1.ContainerPort{
								grpcPort(),
							},
							VolumeMounts: []corev1.VolumeMount{
								configVolumeMount(),
							},
							Env: downwardApiEnv(),
						},
					},
					Volumes: []corev1.Volume{
						configVolume(),
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(r.buildCluster, svc, r.client.Scheme())
	ctrl.SetControllerReference(r.buildCluster, deployment, r.client.Scheme())
	return []resources.Resource{
		resources.Created(svc),
		resources.Present(deployment),
	}, nil
}

func (r *Reconciler) cacheSrv() ([]resources.Resource, error) {
	img, pullPolicy, err := r.kubeccImage()
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"app": "kubecc-cache",
	}
	svc := genericService("kubecc-cache", r.buildCluster.Namespace, labels)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-cache",
			Namespace: r.buildCluster.Namespace,
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
					Containers: []corev1.Container{
						{
							Name:            "kubecc-cache",
							Image:           img,
							ImagePullPolicy: pullPolicy,
							Resources:       r.buildCluster.Spec.Components.Cache.Resources,
							Command:         []string{"/kubecc", "run", "cache"},
							Ports: []corev1.ContainerPort{
								grpcPort(),
							},
							VolumeMounts: []corev1.VolumeMount{
								configVolumeMount(),
							},
							Env: downwardApiEnv(),
						},
					},
					Volumes: []corev1.Volume{
						configVolume(),
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(r.buildCluster, svc, r.client.Scheme())
	ctrl.SetControllerReference(r.buildCluster, deployment, r.client.Scheme())
	return []resources.Resource{
		resources.Created(svc),
		resources.Present(deployment),
	}, nil
}
