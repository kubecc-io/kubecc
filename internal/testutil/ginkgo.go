package testutil

import (
	"os"
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

// InGithubWorkflow returns true if the test is running inside github
// workflow CI, otherwise false. The GITHUB_WORKFLOW environment variable is
// set in .github/workflows/go.yml.
func InGithubWorkflow() bool {
	_, ok := os.LookupEnv("GITHUB_WORKFLOW")
	return ok
}

// SkipInGithubWorkflow will skip the current ginkgo test if running inside
// github workflow CI.
func SkipInGithubWorkflow() {
	if InGithubWorkflow() {
		ginkgo.Skip("Skipping test inside Github workflow")
		return
	}
}

func ExtendTimeoutsIfDebugging() {
	self, _ := os.Executable()
	if filepath.Base(self) == "debug.test" {
		gomega.SetDefaultEventuallyTimeout(1 * time.Hour)
	}
}
