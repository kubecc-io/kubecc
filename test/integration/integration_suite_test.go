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

package integration

import (
	"testing"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
)

var (
	testEnv *test.Environment
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.TestComponent, logkc.WithName("-")),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	testLog = meta.Log(testCtx)
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func makeTaskPool(numTasks int) chan *types.RunRequest {
	taskPool := make(chan *types.RunRequest, numTasks)
	for i := 0; i < numTasks; i++ {
		taskPool <- &types.RunRequest{
			Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
			Args:     []string{"-duration", "0ms"},
			UID:      1000,
			GID:      1000,
		}
	}
	return taskPool
}

func processTaskPool(jobs int, pool chan *types.RunRequest) {
	cdClient := testEnv.NewConsumerdClient(testCtx)
	remaining := atomic.NewInt32(int32(len(pool)))
	for i := 0; i < jobs; i++ {
		go func(cd types.ConsumerdClient) {
			defer GinkgoRecover()
			for {
				select {
				case task := <-pool:
					_, err := cd.Run(testCtx, task)
					if err != nil {
						panic(err)
					}
					testLog.Info(remaining.Dec())
				default:
					testLog.Info("Finished")
					return
				}
			}
		}(cdClient)
	}
	Eventually(remaining.Load, 500*time.Second, 100*time.Millisecond).
		Should(BeEquivalentTo(0))
}
