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

	cdmetrics "github.com/cobalt77/kubecc/pkg/apps/consumerd/metrics"
	scmetrics "github.com/cobalt77/kubecc/pkg/apps/scheduler/metrics"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Monitor
var (
	metricsPostedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_metrics_posted_total",
	})
	listenerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_listener_count",
	})
)

// Scheduler
var (
	tasksCompletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_tasks_completed_total",
		Help:      "Total number of remote tasks completed by agents",
	})
	tasksFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_tasks_failed_total",
		Help:      "Total number of failed remote tasks",
	})
	schedulingRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_scheduling_requests_total",
		Help:      "Total number of requests handled by the scheduler",
	})
	agentCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_agent_count",
		Help:      "Number of active agents",
	})
	cdCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_count",
		Help:      "Number of active consumer daemons",
	})
	agentWeight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_agent_weight",
		Help:      "Agent scheduling weight",
	})
	cdRemoteReqTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_remote_req_total",
		Help:      "Total number of remote tasks requested",
	})
	agentTasksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_agent_tasks_total",
		Help:      "Total number of tasks completed by the agent",
	})
)

// Agent
var (
	agentTasksMax = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_agent_tasks_max",
		Help:      "Maximum number of tasks the agent can run concurrently",
	})
	agentTasksActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_agent_tasks_active",
		Help:      "Current number of tasks the agent is running",
	})
	agentTasksQueued = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_agent_tasks_queued",
		Help:      "Current number of tasks in the agent's queue",
	})
)

// Consumerd
var (
	cdLocalTasksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_local_tasks_total",
		Help:      "Total number of local tasks completed",
	})
	cdTasksMax = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_tasks_max",
		Help:      "Maximum number of concurrent local tasks",
	})
	cdLocalTasksActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_local_tasks_active",
		Help:      "Current number of local tasks the consumerd is running",
	})
	cdLocalTasksQueued = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_local_tasks_queued",
		Help:      "Current number of local tasks in queue",
	})
	cdRemoteTasksActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubecc",
		Name:      "kubecc_cd_remote_tasks_active",
		Help:      "Current number of remote tasks the consumerd is waiting on",
	})
)

func serveMetricsEndpoint(ctx context.Context, address string) {
	lg := meta.Log(ctx)
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(address, nil)
	lg.With(
		zap.Error(err),
		zap.String("address", address),
	).Fatal("Failed to serve metrics")
}

func servePrometheusMetrics(ctx context.Context, client types.ExternalMonitorClient) {
	go serveMetricsEndpoint(ctx, ":2112")
	listener := metrics.NewListener(ctx, client)
	listener.OnProviderAdded(func(ctx context.Context, uuid string) {
		// Wait until the provider posts the Alive message containing its
		// component type, then subscribe accordingly
		componentCh := make(chan types.Component)
		listener.OnValueChanged(uuid, func(a *common.Alive) {
			componentCh <- types.Component(a.Component)
		})
		var component types.Component
		select {
		case component = <-componentCh:
		case <-ctx.Done():
			return
		}

		switch component {
		case types.Agent:
			watchAgentKeys(listener, ctx, uuid)
		case types.Scheduler:
			watchSchedulerKeys(listener, ctx, uuid)
		case types.Consumerd:
			watchConsumerdKeys(listener, ctx, uuid)
		}
	})
}

func watchAgentKeys(l metrics.Listener, ctx context.Context, uuid string) {
	l.OnValueChanged(uuid, func(ts *common.TaskStatus) {

	})
	l.OnValueChanged(uuid, func(ts *common.QueueStatus) {

	})
	l.OnValueChanged(uuid, func(ts *common.QueueParams) {

	})
}

func watchSchedulerKeys(l metrics.Listener, ctx context.Context, uuid string) {
	l.OnValueChanged(uuid, func(ts *scmetrics.AgentCount) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.AgentTasksTotal) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.AgentWeight) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.CdCount) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.CdTasksTotal) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.SchedulingRequestsTotal) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.TasksCompletedTotal) {

	})
	l.OnValueChanged(uuid, func(ts *scmetrics.TasksFailedTotal) {

	})
}

func watchConsumerdKeys(l metrics.Listener, ctx context.Context, uuid string) {
	l.OnValueChanged(uuid, func(ts *common.TaskStatus) {

	})
	l.OnValueChanged(uuid, func(ts *common.QueueStatus) {

	})
	l.OnValueChanged(uuid, func(ts *common.QueueParams) {

	})
	l.OnValueChanged(uuid, func(ts *cdmetrics.LocalTasksCompleted) {

	})
}
