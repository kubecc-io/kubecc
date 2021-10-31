package e2e

import (
	"fmt"

	. "github.com/kralicky/kmatch"
	"github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/pkg/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("E2E", func() {
	It("should install into a new cluster", func() {
		// TODO: fix
		buildCluster := &v1alpha1.BuildCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
			Spec: v1alpha1.BuildClusterSpec{
				Components: v1alpha1.ComponentsSpec{
					Cache: v1alpha1.CacheSpec{
						RemoteStorage: &config.RemoteStorageSpec{
							Endpoint:  infra.S3Info.URL,
							AccessKey: infra.S3Info.AccessKey,
							SecretKey: infra.S3Info.SecretKey,
							TLS:       false,
							Bucket:    infra.S3Info.CacheBucket,
						},
					},
				},
			},
		}
		err := k8sClient.Create(testCtx, buildCluster)
		Expect(err).NotTo(HaveOccurred())
		Eventually(Object(buildCluster)).Should(Exist())
	})
	Specify("setting up SSH connection to client node", func() {
		privateKey, err := ssh.ParsePrivateKey(infra.PrivateKey)
		Expect(err).NotTo(HaveOccurred())

		conf := ssh.ClientConfig{
			User:            "ubuntu",
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(privateKey)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		client, err := ssh.Dial("tcp", infra.ClientIP+":22", &conf)
		Expect(err).NotTo(HaveOccurred())

		test, err := client.NewSession()

		Expect(err).NotTo(HaveOccurred())
		o, err := test.Output("kubecc")
		Expect(err).NotTo(HaveOccurred())
		fmt.Println(string(o))
	})
})
