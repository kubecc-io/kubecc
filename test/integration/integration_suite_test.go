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
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/internal/testutil"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
	"go.uber.org/zap/zapcore"
)

var (
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.TestComponent,
				logkc.WithName("-"),
				logkc.WithLogLevel(zapcore.InfoLevel),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	testLog     = meta.Log(testCtx)
	hashInputs  []string
	hashOutputs []string
)

func init() {
	for i := 0; i < 100; i++ {
		str := uuid.NewString()
		hashInputs = append(hashInputs, str)
		h := md5.New()
		h.Write([]byte(str))
		sum := h.Sum(nil)
		hashOutputs = append(hashOutputs, base64.StdEncoding.EncodeToString(sum))
	}
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

type task struct {
	req            *types.RunRequest
	expectedOutput string
}

func makeHashTaskPool(numTasks int) chan task {
	taskPool := make(chan task, numTasks)
	for i := 0; i < numTasks; i++ {
		idx := rand.Intn(len(hashInputs))
		input := hashInputs[idx]
		output := hashOutputs[idx]
		taskPool <- task{
			req: &types.RunRequest{
				Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
				Args:     []string{"-hash", input},
				UID:      1000,
				GID:      1000,
			},
			expectedOutput: output,
		}
	}
	return taskPool
}

func makeSleepTaskPool(numTasks int, genDuration ...func() string) chan task {
	gen := func() string {
		return "0s"
	}
	if len(genDuration) == 1 {
		gen = genDuration[0]
	}
	taskPool := make(chan task, numTasks)
	for i := 0; i < numTasks; i++ {
		taskPool <- task{
			req: &types.RunRequest{
				Compiler: &types.RunRequest_Path{Path: testutil.TestToolchainExecutable},
				Args:     []string{"-sleep", gen()},
				UID:      1000,
				GID:      1000,
			},
		}
	}
	return taskPool
}

func processTaskPool(testEnv *test.Environment, jobs int, pool chan task, duration time.Duration) {
	cdClient := testEnv.NewConsumerdClient(testCtx)
	remaining := atomic.NewInt32(int32(len(pool)))
	for i := 0; i < jobs; i++ {
		go func(cd types.ConsumerdClient) {
			defer GinkgoRecover()
			for {
				select {
				case task := <-pool:
					resp, err := cd.Run(testCtx, task.req)
					if err != nil {
						panic(err)
					}
					if resp.ReturnCode != 0 {
						panic(fmt.Sprintf("Expected return code to equal 0, was %d", resp.ReturnCode))
					}
					if base64.StdEncoding.EncodeToString(resp.Stdout) != task.expectedOutput {
						panic(fmt.Sprintf("Expected output to equal %s, was %s", task.expectedOutput, string(resp.Stdout)))
					}
					testLog.Debug(remaining.Dec())
				default:
					testLog.Debug("Finished")
					return
				}
			}
		}(cdClient)
	}
	Eventually(remaining.Load, duration, 50*time.Millisecond).
		Should(BeEquivalentTo(0))
}
