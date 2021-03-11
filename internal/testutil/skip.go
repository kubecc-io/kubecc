package testutil

import (
	"os"

	"github.com/onsi/ginkgo"
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
