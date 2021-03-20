package toolchain_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestToolchain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Toolchain Suite")
}
