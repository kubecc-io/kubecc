package e2e

import (
	. "github.com/kralicky/kmatch"
	"github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

var _ = Describe("E2E", func() {
	It("should install into a new cluster", func() {
		errs := util.ForEachStagingResource(clientConfig,
			func(dr dynamic.ResourceInterface, obj *unstructured.Unstructured) error {
				_, err := dr.Create(testCtx, obj, v1.CreateOptions{})
				return err
			})
		Expect(errs).To(BeEmpty())
		buildCluster := &v1alpha1.BuildCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
			Spec: v1alpha1.BuildClusterSpec{
				Components: v1alpha1.ComponentsSpec{
					Image:           "kubecc/kubecc:testing",
					ImagePullPolicy: corev1.PullAlways,
					Agents: v1alpha1.AgentSpec{
						Image:           "kubecc/environment:latest",
						ImagePullPolicy: corev1.PullAlways,
					},
					Cache: v1alpha1.CacheSpec{
						RemoteStorage: &config.RemoteStorageSpec{
							Endpoint:  infra.S3Info.URL,
							AccessKey: infra.S3Info.AccessKey,
							SecretKey: infra.S3Info.SecretKey,
							TLS:       false,
							Bucket:    infra.S3Info.Bucket,
						},
					},
				},
			},
		}
		err := k8sClient.Create(testCtx, buildCluster)
		Expect(err).NotTo(HaveOccurred())
		Eventually(Object(buildCluster)).Should(Exist())
	})
})
