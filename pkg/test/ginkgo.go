/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"google.golang.org/protobuf/proto"
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

type ProtoEqualMatcher struct {
	Expected proto.Message
}

func (matcher *ProtoEqualMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil && matcher.Expected == nil {
		return false, fmt.Errorf("Both actual and expected must not be nil.")
	}

	return proto.Equal(matcher.Expected, actual.(proto.Message)), nil
}

func (matcher *ProtoEqualMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be equivalent to", matcher.Expected)
}

func (matcher *ProtoEqualMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to be equivalent to", matcher.Expected)
}

func EqualProto(expected proto.Message) types.GomegaMatcher {
	return &ProtoEqualMatcher{
		Expected: expected,
	}
}
