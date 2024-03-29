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

package monitor

/*
Stats exported:

Monitor:
- Number of metrics posted: kubecc_metrics_posted_total (counter)
- Number of active listeners: kubecc_listener_count (gauge)


Scheduler:
- Number of completed remote tasks: kubecc_tasks_completed_total (counter)
- Number of failed remote tasks: kubecc_tasks_failed_total (counter)
- Number of scheduling requests: kubecc_scheduling_requests_total (counter)
- Number of agents: kubecc_agent_count (gauge)
- Number of consumer daemons: kubecc_cd_count (gauge)
- Agent Scheduling weight: kubecc_agent_weight (gauge)

Agent:
- Total tasks completed: kubecc_agent_tasks_total (counter)
- Max concurrent tasks: kubecc_agent_tasks_max (gauge)
- Current number of tasks: kubecc_agent_tasks_active (gauge)

Consumerd:
- Total local tasks completed: kubecc_cd_local_tasks_total (counter)
- Total remote requests sent: kubecc_cd_remote_req_total (counter)
- Number of connected consumers: kubecc_cd_consumer_count (gauge)
- Max concurrent tasks: kubecc_cd_tasks_max (gauge)
- Current number of local tasks: kubecc_cd_local_tasks_active (gauge)
- Current number of remote tasks: kubecc_cd_remote_tasks_active (gauge)

*/

import (
	"context"
	"net/http"
	"sync"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Monitor
var (
	metricsPostedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "metrics_posted_total",
		Help:      "Total number of metrics posted to the monitor",
	})
	listenerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "listener_count",
		Help:      "Current number of metric listeners",
	})
	providerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "provider_count",
		Help:      "Current number of metric providers",
	})
)

// Scheduler
var (
	tasksCompletedTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "tasks_completed_total",
		Help:      "Total number of remote tasks completed by agents",
	})
	tasksFailedTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "tasks_failed_total",
		Help:      "Total number of failed remote tasks",
	})
	schedulingRequestsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "scheduling_requests_total",
		Help:      "Total number of requests handled by the scheduler",
	})
	agentCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "agent_count",
		Help:      "Number of active agents",
	})
	cdCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_count",
		Help:      "Number of active consumer daemons",
	})
	agentWeight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "agent_weight",
		Help:      "Agent scheduling weight",
	}, []string{
		"agent",
	})
	cdRemoteReqTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_remote_req_total",
		Help:      "Total number of remote tasks requested",
	}, []string{
		"consumerd",
	})
	agentTasksTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "agent_tasks_total",
		Help:      "Total number of tasks completed by the agent",
	}, []string{
		"agent",
	})
)

// Agent
var (
	agentTasksMax = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "agent_tasks_max",
		Help:      "Maximum number of tasks the agent can run concurrently",
	}, []string{
		"agent",
	})
	agentTasksActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "agent_tasks_active",
		Help:      "Current number of tasks the agent is running",
	}, []string{
		"agent",
	})
	agentTasksQueued = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "agent_tasks_queued",
		Help:      "Current number of tasks in the agent's queue",
	}, []string{
		"agent",
	})
)

// Consumerd
var (
	cdLocalTasksTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_local_tasks_total",
		Help:      "Total number of local tasks completed",
	}, []string{
		"consumerd",
	})
	cdTasksMax = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_tasks_max",
		Help:      "Maximum number of concurrent local tasks",
	}, []string{
		"consumerd",
	})
	cdLocalTasksActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_local_tasks_active",
		Help:      "Current number of local tasks the consumerd is running",
	}, []string{
		"consumerd",
	})
	cdLocalTasksQueued = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_local_tasks_queued",
		Help:      "Current number of local tasks in queue",
	}, []string{
		"consumerd",
	})
	cdRemoteTasksActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "cd_remote_tasks_active",
		Help:      "Current number of remote tasks the consumerd is waiting on",
	}, []string{
		"consumerd",
	})
)

var (
	providerInfo = make(map[string]*types.WhoisResponse)
	infoMutex    = &sync.RWMutex{}
)

func serveMetricsEndpoint(ctx context.Context, address string) {
	lg := meta.Log(ctx)
	http.Handle("/metrics", promhttp.Handler())
	lg.With(
		zap.String("addr", address+"/metrics"),
	).Info("Serving Prometheus metrics")
	err := http.ListenAndServe(address, nil)
	lg.With(
		zap.Error(err),
		zap.String("address", address),
	).Fatal("Failed to serve metrics")
}

func servePrometheusMetrics(
	srvContext context.Context,
	client types.MonitorClient,
) {
	go serveMetricsEndpoint(srvContext, ":2112")
	lg := meta.Log(srvContext)
	listener := clients.NewMetricsListener(srvContext, client,
		clients.WithLogEvents(clients.LogNone),
	)
	listener.OnProviderAdded(func(ctx context.Context, uuid string) {
		info, err := client.Whois(srvContext, &types.WhoisRequest{UUID: uuid})
		if err != nil {
			lg.With(
				zap.String("uuid", uuid),
			).Error(err)
		}

		switch info.Component {
		case types.Agent:
			infoMutex.Lock()
			providerInfo[uuid] = info
			infoMutex.Unlock()
			watchAgentKeys(listener, info)
		case types.Scheduler:
			watchSchedulerKeys(listener, info)
		case types.Consumerd:
			infoMutex.Lock()
			providerInfo[uuid] = info
			infoMutex.Unlock()
			watchConsumerdKeys(listener, info)
		}
		<-ctx.Done()

		infoMutex.Lock()
		defer infoMutex.Unlock()
		delete(providerInfo, uuid)
	})
}

func watchAgentKeys(
	listener clients.MetricsListener,
	info *types.WhoisResponse,
) {
	labels := prometheus.Labels{
		"agent": info.Address,
	}
	listener.OnValueChanged(info.UUID, func(value *metrics.TaskStatus) {
		agentTasksActive.With(labels).Set(float64(value.NumRunning))
		agentTasksActive.With(labels).Set(float64(value.NumRunning))
		agentTasksQueued.With(labels).Set(float64(value.NumQueued))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.UsageLimits) {
		agentTasksMax.With(labels).Set(float64(value.GetConcurrentProcessLimit()))
	})
}

func watchSchedulerKeys(
	listener clients.MetricsListener,
	info *types.WhoisResponse,
) {
	listener.OnValueChanged(info.UUID, func(value *metrics.AgentCount) {
		agentCount.Set(float64(value.Count))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.AgentTasksTotal) {
		infoMutex.RLock()
		defer infoMutex.RUnlock()
		if info, ok := providerInfo[value.UUID]; ok {
			agentTasksTotal.WithLabelValues(info.Address).
				Set(float64(value.Total))
		}
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.ConsumerdCount) {
		cdCount.Set(float64(value.Count))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.ConsumerdTasksTotal) {
		infoMutex.RLock()
		defer infoMutex.RUnlock()
		if info, ok := providerInfo[value.UUID]; ok {
			cdRemoteReqTotal.WithLabelValues(info.Address).
				Set(float64(value.Total))
		}
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.SchedulingRequestsTotal) {
		schedulingRequestsTotal.Set(float64(value.Total))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.TasksCompletedTotal) {
		tasksCompletedTotal.Set(float64(value.Total))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.TasksFailedTotal) {
		tasksFailedTotal.Set(float64(value.Total))
	})
}

func watchConsumerdKeys(
	listener clients.MetricsListener,
	info *types.WhoisResponse,
) {
	labels := prometheus.Labels{
		"consumerd": info.Address,
	}
	listener.OnValueChanged(info.UUID, func(value *metrics.TaskStatus) {
		cdLocalTasksActive.With(labels).Set(float64(value.NumRunning))
		cdRemoteTasksActive.With(labels).Set(float64(value.NumDelegated))
		cdLocalTasksQueued.With(labels).Set(float64(value.NumQueued))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.UsageLimits) {
		cdTasksMax.With(labels).Set(float64(value.GetConcurrentProcessLimit()))
	})
	listener.OnValueChanged(info.UUID, func(value *metrics.LocalTasksCompleted) {
		cdLocalTasksTotal.With(labels).Set(float64(value.Total))
	})
}
