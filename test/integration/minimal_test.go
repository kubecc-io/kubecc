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
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Integration test", func() {
	localJobs := 100
	taskPool := makeTaskPool(200)
	Specify("Starting components", func() {
		cfg := test.DefaultConfig()
		cfg.Global.LogLevel = "debug"
		testEnv = test.NewEnvironment(cfg)

		testEnv.SpawnMonitor()
		testEnv.SpawnScheduler()

		testEnv.SpawnAgent(test.WithConfig(config.AgentSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit: 50,
			},
		}))

		testEnv.SpawnConsumerd(test.WithConfig(config.ConsumerdSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit: 50,
			},
		}))
	})

	Measure("Run test", func(b Benchmarker) {
		testutil.SkipInGithubWorkflow()
		b.Time("process tasks", func() {
			processTaskPool(localJobs, taskPool)
		})
	}, 1)
})
