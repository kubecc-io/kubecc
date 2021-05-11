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
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

func status(h proto.Message) metrics.OverallStatus {
	if st, ok := h.(*metrics.Health); ok {
		return st.GetStatus()
	}
	return metrics.OverallStatus_UnknownStatus
}

var _ = Describe("Health Metrics", func() {
	var testEnv test.Environment

	var monitorCtx, schedulerCtx, cdCtx, agentCtx context.Context
	var monitorCancel context.CancelFunc
	var monitorHealth, schedulerHealth, cdHealth, agentHealth func() (proto.Message, error)
	fail := make(chan string)
	util.SetNotifyTimeoutHandler(func(info util.NotifyTimeoutInfo) {
		stack := make([]byte, 4096)
		runtime.Stack(stack, false)
		fail <- fmt.Sprintf("%s (%s)\n=> Original Caller: %s\n\n%s", info.Message, info.Name, info.Caller, stack)
	})
	go func() {
		msg := <-fail
		panic(msg)
	}()
	Specify("setup", func() {
		testEnv = test.NewLocalhostEnvironmentWithLogLevel(zapcore.DebugLevel)

		var ca context.CancelFunc
		monitorCtx, ca = test.SpawnMonitor(testEnv)
		monitorCancel = ca
		schedulerCtx, _ = test.SpawnScheduler(testEnv, test.WaitForReady())
		cdCtx, _ = test.SpawnConsumerd(testEnv, test.WaitForReady())
		agentCtx, _ = test.SpawnAgent(testEnv, test.WaitForReady())

		monitorHealth = testEnv.MetricF(monitorCtx, &metrics.Health{})
		schedulerHealth = testEnv.MetricF(schedulerCtx, &metrics.Health{})
		cdHealth = testEnv.MetricF(cdCtx, &metrics.Health{})
		agentHealth = testEnv.MetricF(agentCtx, &metrics.Health{})
	})
	Specify("components should set health to ready on startup", func() {
		Eventually(monitorHealth).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
		Eventually(schedulerHealth).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
		Eventually(cdHealth).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
		Eventually(agentHealth).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
	})
	When("the monitor is stopped", func() {
		Specify("health should be unavailable", func() {
			monitorCancel()
			Eventually(func() error {
				_, err := monitorHealth()
				return err
			}).Should(Not(BeNil()))
			Eventually(func() error {
				_, err := schedulerHealth()
				return err
			}).Should(Not(BeNil()))
			Eventually(func() error {
				_, err := cdHealth()
				return err
			}).Should(Not(BeNil()))
			Eventually(func() error {
				_, err := agentHealth()
				return err
			}).Should(Not(BeNil()))
		})
	})
	When("the monitor is restarted", func() {
		Specify("components should set health to ready", func() {
			var ca context.CancelFunc
			monitorCtx, ca = test.SpawnMonitor(testEnv, test.WaitForReady())
			monitorCancel = ca
			monitorHealth = testEnv.MetricF(monitorCtx, &metrics.Health{})

			Eventually(monitorHealth, 15*time.Second).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
			Eventually(schedulerHealth, 15*time.Second).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
			Eventually(cdHealth, 15*time.Second).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
			Eventually(agentHealth, 15*time.Second).Should(WithTransform(status, Equal(metrics.OverallStatus_Ready)))
		})
	})
})
