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

package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type agent struct {
	lock        sync.Mutex
	address     string
	uuid        string
	health      *metrics.Health
	taskStatus  *metrics.TaskStatus
	usageLimits *metrics.UsageLimits
}

func (a *agent) rowData() []string {
	a.lock.Lock()
	defer a.lock.Unlock()
	return []string{
		a.address,
		types.FormatShortID(a.uuid, 6, types.ElideCenter),
		a.health.Status.String(),
		fmt.Sprintf("%d/%d", a.taskStatus.NumRunning, a.usageLimits.GetConcurrentProcessLimit()),
	}
}

type consumerd struct {
	lock        sync.Mutex
	address     string
	uuid        string
	health      *metrics.Health
	taskStatus  *metrics.TaskStatus
	usageLimits *metrics.UsageLimits
}

func (c *consumerd) rowData() []string {
	c.lock.Lock()
	defer c.lock.Unlock()
	return []string{
		c.address,
		types.FormatShortID(c.uuid, 6, types.ElideCenter),
		c.health.Status.String(),
		fmt.Sprintf("%d/%d", c.taskStatus.NumRunning, c.usageLimits.GetConcurrentProcessLimit()),
		fmt.Sprintf("%d/%d", c.taskStatus.NumDelegated, c.usageLimits.DelegatedTaskLimit),
		fmt.Sprint(c.taskStatus.NumQueued),
	}
}

type agentDataSource struct {
	ctx    context.Context
	client types.MonitorClient
	agents sync.Map
}

type consumerdDataSource struct {
	ctx        context.Context
	client     types.MonitorClient
	consumerds sync.Map
}

type schedulerDataSource struct {
	ctx            context.Context
	client         types.MonitorClient
	lock           sync.Mutex
	tasksCompleted int64
	tasksFailed    int64
	requests       int64
}

type monitorDataSource struct {
	ctx           context.Context
	client        types.MonitorClient
	lock          sync.Mutex
	metricsPosted int64
	listeners     int32
	providers     int32
}

type cacheDataSource struct {
	ctx    context.Context
	client types.MonitorClient
	lock   sync.Mutex
	usage  *metrics.CacheUsage
	hits   *metrics.CacheHits
}

type routesDataSource struct {
	ctx    context.Context
	client types.SchedulerClient
}

func NewAgentDataSource(ctx context.Context, client types.MonitorClient) TableDataSource {
	return &agentDataSource{
		ctx:    ctx,
		client: client,
	}
}

func NewConsumerdDataSource(ctx context.Context, client types.MonitorClient) TableDataSource {
	return &consumerdDataSource{
		ctx:    ctx,
		client: client,
	}
}

func NewSchedulerDataSource(ctx context.Context, client types.MonitorClient) TableDataSource {
	return &schedulerDataSource{
		ctx:    ctx,
		client: client,
	}
}

func NewMonitorDataSource(ctx context.Context, client types.MonitorClient) TableDataSource {
	return &monitorDataSource{
		ctx:    ctx,
		client: client,
	}
}

func NewCacheDataSource(ctx context.Context, client types.MonitorClient) TableDataSource {
	return &cacheDataSource{
		ctx:    ctx,
		client: client,
	}
}

func NewRoutesDataSource(ctx context.Context, client types.SchedulerClient) TreeDataSource {
	return &routesDataSource{
		ctx:    ctx,
		client: client,
	}
}

func (a *agentDataSource) Title() string {
	return "Agents"
}

func (a *agentDataSource) Headers() []string {
	return []string{"Address", "ID", "Health", "Running"}
}

func healthStyle(h metrics.OverallStatus) termui.Style {
	switch h {
	case metrics.OverallStatus_UnknownStatus:
		return termui.NewStyle(termui.ColorClear)
	case metrics.OverallStatus_Ready:
		return termui.NewStyle(termui.ColorGreen)
	case metrics.OverallStatus_Initializing:
		return termui.NewStyle(termui.ColorClear)
	case metrics.OverallStatus_Degraded:
		return termui.NewStyle(termui.ColorYellow)
	case metrics.OverallStatus_Unavailable:
		return termui.NewStyle(termui.ColorRed)
	}
	return termui.StyleClear
}

func (a *agentDataSource) Data() (<-chan [][]string, <-chan map[int]termui.Style) {
	dataCh := make(chan [][]string)
	styleCh := make(chan map[int]termui.Style)
	listener := clients.NewMetricsListener(a.ctx, a.client)
	doUpdate := func() {
		rows := [][]string{}
		style := map[int]termui.Style{}
		a.agents.Range(func(key, value interface{}) bool {
			agent := value.(*agent)
			rows = append(rows, agent.rowData())
			style[len(rows)] = healthStyle(agent.health.Status)
			return true
		})
		sort.Slice(rows, func(i, j int) bool {
			return rows[i][1] < rows[j][1]
		})
		dataCh <- rows
		styleCh <- style
	}
	listener.OnProviderAdded(func(c context.Context, s string) {
		whois, err := a.client.Whois(a.ctx, &types.WhoisRequest{
			UUID: s,
		})
		if err != nil {
			return
		}
		if whois.Component != types.Agent {
			return
		}
		a.agents.Store(s, &agent{
			address:     whois.Address,
			uuid:        s,
			health:      &metrics.Health{},
			taskStatus:  &metrics.TaskStatus{},
			usageLimits: &metrics.UsageLimits{},
		})
		updateHealth := func(h *metrics.Health) {
			v, ok := a.agents.Load(s)
			if !ok {
				return
			}
			agent := v.(*agent)
			agent.lock.Lock()
			agent.health = h
			agent.lock.Unlock()
			doUpdate()
		}
		listener.OnValueChanged(s, updateHealth).
			OrExpired(func() clients.RetryOptions {
				updateHealth(&metrics.Health{
					Status: metrics.OverallStatus_UnknownStatus,
				})
				return clients.NoRetry
			})
		updateTaskStatus := func(status *metrics.TaskStatus) {
			v, ok := a.agents.Load(s)
			if !ok {
				return
			}
			agent := v.(*agent)
			agent.lock.Lock()
			agent.taskStatus = status
			agent.lock.Unlock()
			doUpdate()
		}
		listener.OnValueChanged(s, updateTaskStatus).
			OrExpired(func() clients.RetryOptions {
				updateTaskStatus(&metrics.TaskStatus{})
				return clients.NoRetry
			})
		updateUsageLimits := func(limits *metrics.UsageLimits) {
			v, ok := a.agents.Load(s)
			if !ok {
				return
			}
			agent := v.(*agent)
			agent.lock.Lock()
			agent.usageLimits = limits
			agent.lock.Unlock()
			doUpdate()
		}
		listener.OnValueChanged(s, updateUsageLimits).
			OrExpired(func() clients.RetryOptions {
				updateUsageLimits(&metrics.UsageLimits{})
				return clients.NoRetry
			})
		<-c.Done()
		_, ok := a.agents.LoadAndDelete(s)
		if !ok {
			panic("deleted nonexistent agent")
		}
		doUpdate()
	})
	return dataCh, styleCh
}

func (c *consumerdDataSource) Title() string {
	return "Consumer Daemons"
}

func (c *consumerdDataSource) Headers() []string {
	return []string{"Address", "ID", "Health", "Local", "Remote", "Queued"}
}

func (a *consumerdDataSource) Data() (<-chan [][]string, <-chan map[int]termui.Style) {
	dataCh := make(chan [][]string)
	styleCh := make(chan map[int]termui.Style)
	listener := clients.NewMetricsListener(a.ctx, a.client)
	doUpdate := func() {
		rows := [][]string{}
		style := map[int]termui.Style{}
		a.consumerds.Range(func(key, value interface{}) bool {
			cd := value.(*consumerd)
			rows = append(rows, cd.rowData())
			style[len(rows)] = healthStyle(cd.health.Status)
			return true
		})
		sort.Slice(rows, func(i, j int) bool {
			return rows[i][1] < rows[j][1]
		})
		dataCh <- rows
		styleCh <- style
	}
	listener.OnProviderAdded(func(c context.Context, s string) {
		whois, err := a.client.Whois(a.ctx, &types.WhoisRequest{
			UUID: s,
		})
		if err != nil {
			return
		}
		if whois.Component != types.Consumerd {
			return
		}
		a.consumerds.Store(s, &consumerd{
			address:     whois.Address,
			uuid:        s,
			health:      &metrics.Health{},
			taskStatus:  &metrics.TaskStatus{},
			usageLimits: &metrics.UsageLimits{},
		})
		updateHealth := func(h *metrics.Health) {
			v, ok := a.consumerds.Load(s)
			if !ok {
				return
			}
			cd := v.(*consumerd)
			cd.lock.Lock()
			cd.health = h
			cd.lock.Unlock()
			doUpdate()
		}
		listener.OnValueChanged(s, updateHealth).
			OrExpired(func() clients.RetryOptions {
				updateHealth(&metrics.Health{
					Status: metrics.OverallStatus_UnknownStatus,
				})
				return clients.NoRetry
			})
		updateTaskStatus := func(status *metrics.TaskStatus) {
			v, ok := a.consumerds.Load(s)
			if !ok {
				return
			}
			cd := v.(*consumerd)
			cd.lock.Lock()
			cd.taskStatus = status
			cd.lock.Unlock()
			doUpdate()
		}
		listener.OnValueChanged(s, updateTaskStatus).
			OrExpired(func() clients.RetryOptions {
				updateTaskStatus(&metrics.TaskStatus{})
				return clients.NoRetry
			})
		updateUsageLimits := func(limits *metrics.UsageLimits) {
			v, ok := a.consumerds.Load(s)
			if !ok {
				return
			}
			cd := v.(*consumerd)
			cd.lock.Lock()
			cd.usageLimits = limits
			cd.lock.Unlock()
			doUpdate()
		}
		listener.OnValueChanged(s, updateUsageLimits).
			OrExpired(func() clients.RetryOptions {
				updateUsageLimits(&metrics.UsageLimits{})
				return clients.NoRetry
			})
		<-c.Done()
		_, ok := a.consumerds.LoadAndDelete(s)
		if !ok {
			panic("deleted nonexistent consumerd")
		}
		doUpdate()
	})
	return dataCh, styleCh
}

func (c *schedulerDataSource) Title() string {
	return "Scheduler"
}

func (c *schedulerDataSource) Headers() []string {
	return []string{"Metric", "Value"}
}

func (a *schedulerDataSource) Data() (<-chan [][]string, <-chan map[int]termui.Style) {
	dataCh := make(chan [][]string)
	listener := clients.NewMetricsListener(a.ctx, a.client)

	doUpdate := func() {
		a.lock.Lock()
		rows := [][]string{
			{"Completed", fmt.Sprint(a.tasksCompleted)},
			{"Failed", fmt.Sprint(a.tasksFailed)},
			{"Requests", fmt.Sprint(a.requests)},
		}
		a.lock.Unlock()
		dataCh <- rows
	}
	listener.OnProviderAdded(func(c context.Context, s string) {
		whois, err := a.client.Whois(a.ctx, &types.WhoisRequest{
			UUID: s,
		})
		if err != nil {
			return
		}
		if whois.Component != types.Scheduler {
			return
		}
		listener.OnValueChanged(s, func(m *metrics.TasksCompletedTotal) {
			a.lock.Lock()
			a.tasksCompleted = m.GetTotal()
			a.lock.Unlock()
			doUpdate()
		})
		listener.OnValueChanged(s, func(m *metrics.TasksFailedTotal) {
			a.lock.Lock()
			a.tasksFailed = m.GetTotal()
			a.lock.Unlock()
			doUpdate()
		})
		listener.OnValueChanged(s, func(m *metrics.SchedulingRequestsTotal) {
			a.lock.Lock()
			a.requests = m.GetTotal()
			a.lock.Unlock()
			doUpdate()
		})
		<-c.Done()
		a.lock.Lock()
		a.tasksCompleted = 0
		a.tasksFailed = 0
		a.requests = 0
		a.lock.Unlock()
		dataCh <- [][]string{}
	})
	return dataCh, make(chan map[int]termui.Style)
}

func (c *monitorDataSource) Title() string {
	return "Monitor"
}

func (c *monitorDataSource) Headers() []string {
	return []string{"Metric", "Value"}
}

func (a *monitorDataSource) Data() (<-chan [][]string, <-chan map[int]termui.Style) {
	ch := make(chan [][]string)
	listener := clients.NewMetricsListener(a.ctx, a.client)

	doUpdate := func() {
		a.lock.Lock()
		rows := [][]string{
			{"Metrics Posted", fmt.Sprint(a.metricsPosted)},
			{"Listeners", fmt.Sprint(a.listeners)},
			{"Providers", fmt.Sprint(a.providers)},
		}
		a.lock.Unlock()
		ch <- rows
	}
	listener.OnProviderAdded(func(c context.Context, s string) {
		whois, err := a.client.Whois(a.ctx, &types.WhoisRequest{
			UUID: s,
		})
		if err != nil {
			return
		}
		if whois.Component != types.Monitor {
			return
		}
		listener.OnValueChanged(s, func(m *metrics.MetricsPostedTotal) {
			a.lock.Lock()
			a.metricsPosted = m.GetTotal()
			a.lock.Unlock()
			doUpdate()
		})
		listener.OnValueChanged(s, func(m *metrics.ListenerCount) {
			a.lock.Lock()
			a.listeners = m.GetCount()
			a.lock.Unlock()
			doUpdate()
		})
		listener.OnValueChanged(s, func(m *metrics.ProviderCount) {
			a.lock.Lock()
			a.providers = m.GetCount()
			a.lock.Unlock()
			doUpdate()
		})
		<-c.Done()
		a.lock.Lock()
		a.metricsPosted = 0
		a.listeners = 0
		a.providers = 0
		a.lock.Unlock()
		ch <- [][]string{}
	})
	return ch, make(chan map[int]termui.Style)
}

func (c *cacheDataSource) Title() string {
	return "Cache"
}

func (c *cacheDataSource) Headers() []string {
	return []string{"Metric", "Value"}
}

func (a *cacheDataSource) Data() (<-chan [][]string, <-chan map[int]termui.Style) {
	ch := make(chan [][]string)
	listener := clients.NewMetricsListener(a.ctx, a.client)
	a.hits = &metrics.CacheHits{}
	a.usage = &metrics.CacheUsage{}
	doUpdate := func() {
		a.lock.Lock()
		rows := [][]string{
			{"Objects", fmt.Sprint(a.usage.ObjectCount)},
			{"Usage", fmt.Sprint(a.usage.UsagePercent)},
			{"Cache Hits", fmt.Sprint(a.hits.CacheHitsTotal)},
			{"Cache Misses", fmt.Sprint(a.hits.CacheMissesTotal)},
			{"Hit %", fmt.Sprint(a.hits.CacheHitPercent)},
		}
		a.lock.Unlock()
		ch <- rows
	}
	listener.OnProviderAdded(func(c context.Context, s string) {
		whois, err := a.client.Whois(a.ctx, &types.WhoisRequest{
			UUID: s,
		})
		if err != nil {
			return
		}
		if whois.Component != types.Cache {
			return
		}
		listener.OnValueChanged(s, func(m *metrics.CacheUsage) {
			a.lock.Lock()
			a.usage = m
			a.lock.Unlock()
			doUpdate()
		})
		listener.OnValueChanged(s, func(m *metrics.CacheHits) {
			a.lock.Lock()
			a.hits = m
			a.lock.Unlock()
			doUpdate()
		})
		<-c.Done()
		a.lock.Lock()
		a.hits = &metrics.CacheHits{}
		a.usage = &metrics.CacheUsage{}
		a.lock.Unlock()
		ch <- [][]string{}
	})
	return ch, make(chan map[int]termui.Style)
}

func (c *routesDataSource) Title() string {
	return "Routes"
}

func stringer(s string) fmt.Stringer {
	b := strings.Builder{}
	b.WriteString(s)
	return &b
}

func friendlyName(tc *types.Toolchain) string {
	switch tc.Kind {
	case types.Clang:
		return fmt.Sprintf("Clang %s (%s)", tc.Version, tc.TargetArch)
	case types.Gnu:
		switch tc.Lang {
		case types.C:
			return fmt.Sprintf("GCC %s (%s)", tc.Version, tc.TargetArch)
		case types.CXX:
			return fmt.Sprintf("G++ %s (%s)", tc.Version, tc.TargetArch)
		}
	case types.Sleep:
		return fmt.Sprintf("Sleep %s (%s)", tc.Version, tc.TargetArch)
	}
	return "Unknown"
}

type sortableNodes []*widgets.TreeNode

func (s sortableNodes) Len() int {
	return len(s)
}
func (s sortableNodes) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortableNodes) Less(i, j int) bool {
	return s[i].Value.String() < s[j].Value.String()
}

func (a *routesDataSource) Data() <-chan []*widgets.TreeNode {
	ch := make(chan []*widgets.TreeNode)

	ticker := time.NewTicker(3 * time.Second)
	go func() {
		time.Sleep(1 * time.Second)
		for {
			routes, err := a.client.GetRoutes(a.ctx, &types.Empty{})
			if err != nil {
				ch <- []*widgets.TreeNode{}
				continue
			}
			nodes := []*widgets.TreeNode{}
			for _, route := range routes.GetRoutes() {
				node := &widgets.TreeNode{
					Expanded: true,
					Value:    stringer(friendlyName(route.Toolchain)),
				}
				node.Nodes = []*widgets.TreeNode{
					{
						Value:    stringer(fmt.Sprintf("Agents (%d)", len(route.Agents))),
						Expanded: true,
					},
					{
						Value:    stringer(fmt.Sprintf("Consumerds (%d)", len(route.Consumerds))),
						Expanded: true,
					},
				}
				sort.Strings(route.Agents)
				sort.Strings(route.Consumerds)
				for _, agent := range route.Agents {
					node.Nodes[0].Nodes = append(node.Nodes[0].Nodes, &widgets.TreeNode{
						Value: stringer(agent),
					})
				}
				for _, cd := range route.Consumerds {
					node.Nodes[1].Nodes = append(node.Nodes[1].Nodes, &widgets.TreeNode{
						Value: stringer(cd),
					})
				}
				nodes = append(nodes, node)
				sort.Sort(sortableNodes(nodes))
			}
			ch <- nodes
			<-ticker.C
		}
	}()
	return ch
}
