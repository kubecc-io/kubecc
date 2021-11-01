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

package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Optimizer is responsible for adjusting the preferred remote usage
// limits in real-time based on the state of the cluster.
//
// By default, the Optimizer will set the preferred remote usage limit
// to be the sum of all agents' usage limits.
//
// If a cache server is running and posts a cache usage percent, the limit
// controller will increase the remote usage limit by that percentage, to
// account for the fact that on average that percent of tasks will never
// make it to an agent and will therefore not affect actual remote usage.
//
// Each agent posts their cpu stats every second. These stats contain
// total cpu usage in nanoseconds and cgroup throttling info. Since we know how
// many tasks are running on any agent at any given time, we can use the
// average amount of tasks running between every interval and the cpu usage
// for that interval to solve an optimization problem of simulateneously
// maximizing cpu usage (or rather, minimizing unused cpu) and minimizing
// throttling. Too much throttling means the cgroup is overloaded but too
// little cpu usage means the agent is not being used to its full capacity.
type Optimizer struct {
	ctx    context.Context
	lg     *zap.SugaredLogger
	client types.MonitorClient
	broker *Broker
	usageC chan float64
}

func NewOptimizer(
	ctx context.Context,
	client types.MonitorClient,
	broker *Broker,
) *Optimizer {
	o := &Optimizer{
		ctx:    ctx,
		lg:     meta.Log(ctx),
		client: client,
		broker: broker,
		usageC: make(chan float64, 1),
	}
	o.usageC <- 1.0
	go o.run()
	return o
}

func (o *Optimizer) UsageLimitMultiplierChanged() <-chan float64 {
	return o.usageC
}

func (o *Optimizer) runCacheOptimizer(
	listener clients.MetricsListener,
	pctx context.Context,
	uuid string,
) {
	listener.OnValueChanged(uuid, func(c *metrics.CacheHits) {
		o.usageC <- 1.0 + c.CacheHitPercent
	})
	<-pctx.Done()
	o.lg.Info("Cache no longer available, resetting usage limits")
	o.usageC <- 1.0
}

type snapshot struct {
	wallTime    uint64
	tokenCount  int
	tokensInUse int
	stats       *metrics.CpuStats
}

func (s snapshot) UsageFactor(prev snapshot) float64 {
	quota := s.stats.CpuUsage.CfsQuota
	if s.stats.CpuUsage.CfsQuota < 0 {
		quota = s.stats.CpuUsage.CfsPeriod
	}
	Δt := float64(s.wallTime - prev.wallTime)
	Δu := float64(s.stats.CpuUsage.TotalUsage - prev.stats.CpuUsage.TotalUsage)
	max := (Δt / float64(s.stats.CpuUsage.CfsPeriod)) * float64(quota)
	return Δu / max
}

func (s snapshot) ThrottlingFactor(next snapshot) float64 {
	Δt := float64(next.wallTime - s.wallTime)
	return 1.0 - (float64(s.stats.ThrottlingData.ThrottledTime) / Δt)
}

func (s snapshot) TokenUsage() float64 {
	return float64(s.tokensInUse) / float64(s.tokenCount)
}

func (o *Optimizer) runAgentOptimizer(
	listener clients.MetricsListener,
	pctx context.Context,
	uuid string,
) {
	snapshots := []snapshot{}
	lock := &sync.Mutex{}
	agent, ok := o.broker.GetAgent(uuid)
	if !ok {
		o.lg.With(
			"uuid", uuid,
		).Error("Could not find agent")
		return
	}
	listener.OnValueChanged(uuid, func(s *metrics.CpuStats) {
		lock.Lock()
		defer lock.Unlock()
		available := len(agent.AvailableTokens)
		locked := len(agent.LockedTokens)
		snapshots = append(snapshots, snapshot{
			wallTime:    s.WallTime,
			tokenCount:  available + locked,
			tokensInUse: locked,
			stats:       proto.Clone(s).(*metrics.CpuStats),
		})
	})
	util.RunPeriodic(pctx, 8*time.Second, 0.5, false, func() { // every 8-12s
		lock.Lock()
		defer lock.Unlock()
		usage := 0.0
		throttling := 0.0
		tokens := 0.0
		for i, snapshot := range snapshots {
			if i == 0 {
				continue
			}
			usage += snapshot.UsageFactor(snapshots[i-1])
			throttling += snapshot.ThrottlingFactor(snapshots[i-1])
			tokens += snapshot.TokenUsage()
		}
		count := float64(len(snapshots) - 1)
		usage /= count
		throttling /= count
		tokens /= count
		// todo: math
		o.lg.With(
			types.ShortID(uuid),
			"usage", usage,
			"throttling", throttling,
			"tokens", tokens,
		).Info("Agent Optimization")
	})
	<-pctx.Done()
}

func (o *Optimizer) run() {
	lg := meta.Log(o.ctx)
	lg.Info("Starting optimizer")
	listener := clients.NewMetricsListener(o.ctx, o.client)
	listener.OnProviderAdded(func(c context.Context, s string) {
		whois, err := o.client.Whois(o.ctx, &types.WhoisRequest{
			UUID: s,
		})
		if err != nil {
			return
		}
		switch whois.Component {
		case types.Cache:
			o.runCacheOptimizer(listener, c, s)
		case types.Agent:
			// todo
			// o.runAgentOptimizer(listener, c, s)
		}
	})
}
