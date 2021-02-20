package monitor_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMonitor(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(3 * time.Second)
	SetDefaultEventuallyPollingInterval(50 * time.Millisecond)
	RunSpecs(t, "Monitor Suite")
}
