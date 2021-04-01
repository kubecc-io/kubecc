/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package controllers

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kubecc-io/kubecc/api/v1alpha1"
)

var _ = Describe("BuildCluster Controller", func() {
	const (
		Name      = "test-buildcluster"
		Namespace = "kubecc-test"
		timeout   = 10 * time.Second
		interval  = 500 * time.Millisecond
	)
	var (
		nodeAffinity = &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "allow-kubecc",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		}
		resources = v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
			},
		}
	)

	Context("When creating a BuildCluster", func() {
		ctx := context.Background()
		cluster := &v1alpha1.BuildCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kubecc.io/v1alpha1",
				Kind:       "BuildCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: Namespace,
			},
			Spec: v1alpha1.BuildClusterSpec{
				Components: v1alpha1.ComponentsSpec{
					Image:           "kubecc/kubecc:latest",
					ImagePullPolicy: v1.PullIfNotPresent,
					Agents: v1alpha1.AgentSpec{
						NodeAffinity:    nodeAffinity,
						Resources:       resources,
						Image:           "kubecc/environment:latest",
						ImagePullPolicy: v1.PullIfNotPresent,
					},
					// Scheduler: v1alpha1.SchedulerSpec{
					// 	Resources:       resources,
					// 	ImagePullPolicy: "Always",
					// },
					// Monitor: v1alpha1.MonitorSpec{
					// 	Resources:       resources,
					// 	ImagePullPolicy: "Always",
					// },
					// Cache: v1alpha1.CacheSpec{
					// 	Resources:       resources,
					// 	ImagePullPolicy: "Always",
					// },
				},
			},
		}
		It("Should succeed", func() {
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			Eventually(func() bool {
				cluster := &v1alpha1.BuildCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      Name,
					Namespace: Namespace,
				}, cluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(cluster).ShouldNot(BeNil())
		})
		It("Should create a scheduler deployment", func() {
			Eventually(func() bool {
				scheduler := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc-scheduler",
					Namespace: Namespace,
				}, scheduler)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("Should create an agent DaemonSet", func() {
			Eventually(func() bool {
				agents := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc-agent",
					Namespace: Namespace,
				}, agents)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("Should create a monitor deployment", func() {
			Eventually(func() bool {
				monitor := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc-monitor",
					Namespace: Namespace,
				}, monitor)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("Should create a cache server deployment", func() {
			Eventually(func() bool {
				cachesrv := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc-cache",
					Namespace: Namespace,
				}, cachesrv)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("should apply the default image", func() {
			// scheduler
			scheduler := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "kubecc-scheduler",
				Namespace: Namespace,
			}, scheduler)
			Expect(err).NotTo(HaveOccurred())
			Expect(scheduler.Spec.Template.Spec.Containers[0].Image ==
				cluster.Spec.Components.Image)

			// monitor
			monitor := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "kubecc-monitor",
				Namespace: Namespace,
			}, monitor)
			Expect(err).NotTo(HaveOccurred())
			Expect(monitor.Spec.Template.Spec.Containers[0].Image ==
				cluster.Spec.Components.Image)

			// cache
			cachesrv := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "kubecc-cache",
				Namespace: Namespace,
			}, cachesrv)
			Expect(err).NotTo(HaveOccurred())
			Expect(monitor.Spec.Template.Spec.Containers[0].Image ==
				cluster.Spec.Components.Image)
		})
		It("Should create a configmap", func() {
			Eventually(func() bool {
				agents := &v1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc",
					Namespace: Namespace,
				}, agents)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		It("Should resolve agent CRD updates", func() {
			cluster := &v1alpha1.BuildCluster{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      Name,
				Namespace: Namespace,
			}, cluster)
			Expect(err).NotTo(HaveOccurred())
			getDaemonSet := func() *appsv1.DaemonSet {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc-agent",
					Namespace: Namespace,
				}, ds)
				Expect(err).NotTo(HaveOccurred())
				return ds
			}

			By("Updating node affinity")
			affinity := &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "test",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"test"},
								},
							},
						},
					},
				},
			}
			cluster.Spec.Components.Agents.NodeAffinity = affinity
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.Affinity.NodeAffinity,
					affinity)
			}, timeout, interval).Should(BeTrue())

			By("Updating resources")
			resources := v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("400m"),
				},
			}
			cluster.Spec.Components.Agents.Resources = resources
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.Containers[0].Resources,
					resources)
			}, timeout, interval).Should(BeTrue())

			By("Updating container image")
			image := "kubecc/test:doesntexist"
			cluster.Spec.Components.Image = image
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.InitContainers[0].Image,
					image)
			}, timeout, interval).Should(BeTrue())

			By("Updating labels")
			labels := map[string]string{
				"testing":  "true",
				"testing2": "yes",
			}
			cluster.Spec.Components.Agents.AdditionalLabels = labels
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			labels["app"] = "kubecc-agent"
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Labels,
					labels)
			}, timeout, interval).Should(BeTrue())

			By("Updating imagePullPolicy")
			policy := v1.PullNever
			cluster.Spec.Components.ImagePullPolicy = policy
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.InitContainers[0].ImagePullPolicy,
					policy)
			}, timeout, interval).Should(BeTrue())
		})

		It("Should resolve scheduler CRD updates", func() {
			cluster := &v1alpha1.BuildCluster{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      Name,
				Namespace: Namespace,
			}, cluster)
			Expect(err).NotTo(HaveOccurred())
			getDeployment := func() *appsv1.Deployment {
				d := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "kubecc-scheduler",
					Namespace: Namespace,
				}, d)
				Expect(err).NotTo(HaveOccurred())
				return d
			}

			By("Updating node affinity")
			affinity := &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "test",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"test"},
								},
							},
						},
					},
				},
			}
			cluster.Spec.Components.Scheduler.NodeAffinity = affinity
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				d := getDeployment()
				return d.Spec.Template.Spec.Affinity != nil &&
					reflect.DeepEqual(
						d.Spec.Template.Spec.Affinity.NodeAffinity,
						affinity)
			}, timeout, interval).Should(BeTrue())

			By("Updating resources")
			resources := v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("400m"),
				},
			}
			cluster.Spec.Components.Scheduler.Resources = resources
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				d := getDeployment()
				return reflect.DeepEqual(
					d.Spec.Template.Spec.Containers[0].Resources,
					resources)
			}, timeout, interval).Should(BeTrue())

			By("Updating container image")
			image := "kubecc/test:doesntexist"
			cluster.Spec.Components.Image = image
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				d := getDeployment()
				return reflect.DeepEqual(
					d.Spec.Template.Spec.Containers[0].Image,
					image)
			}, timeout, interval).Should(BeTrue())

			By("Updating labels")
			labels := map[string]string{
				"testing":  "true",
				"testing2": "yes",
			}
			cluster.Spec.Components.Scheduler.AdditionalLabels = labels
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			labels["app"] = "kubecc-scheduler"
			Eventually(func() bool {
				d := getDeployment()
				return reflect.DeepEqual(
					d.Spec.Template.Labels,
					labels)
			}, timeout, interval).Should(BeTrue())

			By("Updating imagePullPolicy")
			policy := v1.PullNever
			cluster.Spec.Components.ImagePullPolicy = policy
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				d := getDeployment()
				return reflect.DeepEqual(
					d.Spec.Template.Spec.Containers[0].ImagePullPolicy,
					policy)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
