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

	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type remoteInfo struct {
	UUID           string
	Context        context.Context
	UsageLimits    *metrics.UsageLimits
	SystemInfo     *types.SystemInfo
	CompletedTasks *atomic.Int64
}

type Consumerd struct {
	remoteInfo
	*sync.RWMutex

	Toolchains *metrics.Toolchains
	Stream     types.Scheduler_StreamOutgoingTasksServer
}

// MaxTokens is an arbitrary upper limit on the number of concurrent tasks
// an agent can run.
const MaxTokens = 1000

type Agent struct {
	remoteInfo
	*sync.RWMutex

	Toolchains *metrics.Toolchains
	Stream     types.Scheduler_StreamIncomingTasksServer
}

func remoteInfoFromContext(ctx context.Context) remoteInfo {
	return remoteInfo{
		UUID:           meta.UUID(ctx),
		Context:        ctx,
		SystemInfo:     meta.SystemInfo(ctx),
		CompletedTasks: atomic.NewInt64(0),
	}
}

type agentStats struct {
	agentCtx        context.Context
	agentTasksTotal *metrics.AgentTasksTotal
}

type consumerdStats struct {
	consumerdCtx       context.Context
	cdRemoteTasksTotal *metrics.ConsumerdTasksTotal
}

type taskStats struct {
	completedTotal *metrics.TasksCompletedTotal
	failedTotal    *metrics.TasksFailedTotal
	requestsTotal  *metrics.SchedulingRequestsTotal
}
