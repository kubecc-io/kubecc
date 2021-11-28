package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kralicky/kmatch"
	"github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.TestComponent,
				logkc.WithName("-"),
				logkc.WithLogLevel(zapcore.InfoLevel),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	testLog        = meta.Log(testCtx)
	infra          *TestInfra
	k8sClient      client.Client
	clientConfig   *rest.Config
	kubeconfigPath string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}

func TestE2e(t *testing.T) {
	if test.InGithubWorkflow() {
		t.Skip("Skipping e2e tests in GitHub workflow")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

var _ = BeforeSuite(func() {
	var err error
	infra, err = SetupE2EInfra(testCtx)
	Expect(err).NotTo(HaveOccurred())
	err = clientcmd.WriteToFile(*infra.Kubeconfig, "e2e-kubeconfig.yaml")
	Expect(err).NotTo(HaveOccurred())
	kubeconfigPath, err = filepath.Abs("e2e-kubeconfig.yaml")
	Expect(err).NotTo(HaveOccurred())
	cmd := clientcmd.NewDefaultClientConfig(*infra.Kubeconfig, nil)
	clientConfig, err = cmd.ClientConfig()
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(clientConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	kmatch.SetDefaultObjectClient(k8sClient)
})

var _ = AfterSuite(func() {
	os.Remove("e2e-kubeconfig.yaml")
	err := CleanupE2EInfra(testCtx, infra)
	Expect(err).NotTo(HaveOccurred())
})
