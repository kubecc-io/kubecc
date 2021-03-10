package cc_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CC Suite")
}
