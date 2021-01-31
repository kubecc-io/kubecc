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

	"github.com/cobalt77/kubecc/api/v1alpha1"
)

var _ = Describe("BuildCluster Controller", func() {
	const (
		Name      = "test-buildcluster"
		Namespace = "kubecc-test"
		timeout   = 20 * time.Second
		duration  = 20 * time.Second
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
		It("Should succeed", func() {
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
						Agent: v1alpha1.AgentSpec{
							NodeAffinity:    nodeAffinity,
							Image:           "gcr.io/kubecc/agent:latest",
							ImagePullPolicy: "Always",
							Resources:       resources,
						},
						Scheduler: v1alpha1.SchedulerSpec{
							Resources:       resources,
							Image:           "gcr.io/kubecc/scheduler:latest",
							ImagePullPolicy: "Always",
						},
					},
				},
			}
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
			cluster.Spec.Components.Agent.NodeAffinity = affinity
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
			cluster.Spec.Components.Agent.Resources = resources
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.Containers[0].Resources,
					resources)
			}, timeout, interval).Should(BeTrue())

			By("Updating container image")
			image := "gcr.io/kubecc/test:doesntexist"
			cluster.Spec.Components.Agent.Image = image
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.Containers[0].Image,
					image)
			}, timeout, interval).Should(BeTrue())

			By("Updating labels")
			labels := map[string]string{
				"testing":  "true",
				"testing2": "yes",
			}
			cluster.Spec.Components.Agent.AdditionalLabels = labels
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
			cluster.Spec.Components.Agent.ImagePullPolicy = policy
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				ds := getDaemonSet()
				return reflect.DeepEqual(
					ds.Spec.Template.Spec.Containers[0].ImagePullPolicy,
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
			image := "gcr.io/kubecc/test:doesntexist"
			cluster.Spec.Components.Scheduler.Image = image
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
			cluster.Spec.Components.Scheduler.ImagePullPolicy = policy
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
