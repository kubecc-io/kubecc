package monitor_test

import (
	"testing"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var testLog *zap.SugaredLogger

func TestMonitor(t *testing.T) {
	testLog = logkc.New(types.TestComponent, logkc.WithWriter(GinkgoWriter))
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(3 * time.Second)
	// SetDefaultEventuallyTimeout(1 * time.Hour)
	SetDefaultEventuallyPollingInterval(50 * time.Millisecond)
	RunSpecs(t, "Monitor Suite")
}
