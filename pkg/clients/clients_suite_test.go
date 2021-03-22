package clients_test

import (
	"testing"

	"github.com/cobalt77/kubecc/internal/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestClients(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clients Suite")
	testutil.ExtendTimeoutsIfDebugging()
}
