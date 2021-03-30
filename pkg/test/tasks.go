package test

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
)

var (
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

type task struct {
	req            *types.RunRequest
	expectedOutput string
}

func MakeHashTaskPool(numTasks int) chan task {
	taskPool := make(chan task, numTasks)
	for i := 0; i < numTasks; i++ {
		idx := rand.Intn(len(hashInputs))
		input := hashInputs[idx]
		output := hashOutputs[idx]
		taskPool <- task{
			req: &types.RunRequest{
				Compiler: &types.RunRequest_Path{Path: TestToolchainExecutable},
				Args:     []string{"-hash", input},
				UID:      1000,
				GID:      1000,
			},
			expectedOutput: output,
		}
	}
	return taskPool
}

func MakeSleepTaskPool(numTasks int, genDuration ...func() string) chan task {
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
				Compiler: &types.RunRequest_Path{Path: TestToolchainExecutable},
				Args:     []string{"-sleep", gen()},
				UID:      1000,
				GID:      1000,
			},
		}
	}
	return taskPool
}

func ProcessTaskPool(testEnv *Environment, jobs int, pool chan task, duration time.Duration) {
	cdClient := testEnv.NewConsumerdClient(testEnv.Context())
	remaining := atomic.NewInt32(int32(len(pool)))
	for i := 0; i < jobs; i++ {
		go func(cd types.ConsumerdClient) {
			for {
				select {
				case task := <-pool:
					resp, err := cd.Run(testEnv.Context(), task.req)
					if err != nil {
						panic(err)
					}
					if resp.ReturnCode != 0 {
						panic(fmt.Sprintf("Expected return code to equal 0, was %d", resp.ReturnCode))
					}
					if base64.StdEncoding.EncodeToString(resp.Stdout) != task.expectedOutput {
						panic(fmt.Sprintf("Expected output to equal %s, was %s", task.expectedOutput, string(resp.Stdout)))
					}
					remaining.Dec()
				default:
					return
				}
			}
		}(cdClient)
	}
	Eventually(remaining.Load, duration, 50*time.Millisecond).
		Should(BeEquivalentTo(0))
}
