package cachesrv_test

import (
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"

	"github.com/cobalt77/kubecc/pkg/test"
)

var _ = Describe("Cache Server", func() {
	testEnv := test.NewDefaultEnvironment()
	Specify("setup", func() {
		testEnv.SpawnCache()
	})
})
