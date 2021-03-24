package cachesrv_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCachesrv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cachesrv Suite")
}
