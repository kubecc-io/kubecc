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
	"os/exec"
	"strings"
	"time"

	"github.com/kubecc-io/kubecc/pkg/agent"
	"github.com/kubecc-io/kubecc/pkg/cc"
	ccctrl "github.com/kubecc-io/kubecc/pkg/cc/controller"
	"github.com/kubecc-io/kubecc/pkg/consumerd"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/toolchains"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
)

var _ = FDescribe("Debug Prefix Map Test", func() {
	var testEnv test.Environment

	Specify("setup", func() {
		testEnv = test.NewLocalhostEnvironmentWithLogLevel(zapcore.DebugLevel)

		test.SpawnMonitor(testEnv, test.WaitForReady())
		test.SpawnScheduler(testEnv, test.WaitForReady())
	})

	Specify("Starting consumerd", func() {
		test.SpawnConsumerd(testEnv,
			test.WaitForReady(),
			test.WithConsumerdOptions(
				consumerd.WithQueueOptions(
					consumerd.WithLocalUsageManager(consumerd.FixedUsageLimits(0)),
				),
				consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
					Finder: cc.CCFinder{},
				}),
				consumerd.WithToolchainRunners(ccctrl.AddToStore),
			),
		)
	})

	Specify("1 agent, no cache", func() {
		test.SpawnAgent(testEnv,
			test.WaitForReady(),
			test.WithAgentOptions(
				agent.WithToolchainFinders(toolchains.FinderWithOptions{
					Finder: cc.CCFinder{},
				}),
				agent.WithToolchainRunners(ccctrl.AddToStore),
			),
		)
	})

	It("should compile", func() {
		ctx := testEnv.Context()
		cdClient := test.NewConsumerdClient(testEnv, ctx)
		time.Sleep(time.Second)
		resp, err := cdClient.Run(ctx, &types.RunRequest{
			Compiler: &types.RunRequest_Path{
				Path: "/usr/bin/gcc",
			},
			Args: []string{"-g", "-c", "-o", "testdata/fast_inverse_sqrt/fast_inverse_sqrt.o", "testdata/fast_inverse_sqrt/fast_inverse_sqrt.c"},
			UID:  1000,
			GID:  1000,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ReturnCode).To(BeEquivalentTo(0))

		// Check dwarf info to make sure the prefix map applied correctly
		cmd := exec.Command("/bin/sh", "-c",
			`/usr/bin/readelf -W --debug-dump=decodedline testdata/fast_inverse_sqrt/fast_inverse_sqrt.o --dwarf-depth=1 | tail -n +5 | awk '{print $1}'`)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		lines := strings.Split(string(output), "\n")
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if line == "" {
				continue
			}
			filtered = append(filtered, line)
		}
		Expect(filtered).To(HaveLen(22))
		for _, line := range filtered {
			Expect(line).To(Equal("testdata/fast_inverse_sqrt/fast_inverse_sqrt.c"))
		}
	})
})
