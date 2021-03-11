package toolchains_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestToolchains(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Toolchains Suite")
}
