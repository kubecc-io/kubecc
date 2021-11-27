package servers_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Servers Suite")
}
