package consumerd_test

import (
	"testing"

	"github.com/cobalt77/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testEnv *test.Environment

func TestConsumerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consumerd Suite")
}

var _ = BeforeSuite(func() {
	testEnv = test.NewDefaultEnvironment()
	testEnv.Start()
})

var _ = AfterSuite(func() {
	testEnv.Shutdown()
})
