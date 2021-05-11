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

package metrics

import (
	"fmt"

	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/zap/zapcore"
)

func (sc StatusConditions) FormatAll(msgs ...string) []string {
	formatted := make([]string, len(msgs))
	for i, msg := range msgs {
		formatted[i] = sc.Format(msg)
	}
	return formatted
}

func (sc StatusConditions) Format(msg string) string {
	switch sc {
	case StatusConditions_NoConditions:
		return msg
	case StatusConditions_Pending:
		return "[Pending] " + msg
	case StatusConditions_MissingOptionalComponent:
		return "[Missing Optional] " + msg
	case StatusConditions_MissingCriticalComponent:
		return "[Missing Critical] " + msg
	case StatusConditions_InvalidConfiguration:
		return "[Invalid Config] " + msg
	case StatusConditions_InternalError:
		return "[Internal Error] " + msg
	}
	return msg
}

func (a *TaskStatus) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("r/d/q", fmt.Sprintf("%d/%d/%d",
		a.NumRunning, a.NumDelegated, a.NumQueued))
	return nil
}

func (a *Toolchains) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt("items", len(a.GetItems()))
	return nil
}

func (a *UsageLimits) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt32("local", a.GetConcurrentProcessLimit())
	enc.AddInt32("remote", a.GetDelegatedTaskLimit())
	return nil
}

func (a *AgentInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("node", a.GetNode())
	enc.AddString("pod", a.GetPod())
	enc.AddString("ns", a.GetNamespace())
	return nil
}

func (a *CpuStats) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("time", a.GetWallTime())
	enc.AddObject("usage", a.GetCpuUsage())
	enc.AddObject("data", a.GetThrottlingData())
	return nil
}

func (a *CpuUsage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("q/p/u", fmt.Sprintf("%d/%d/%d",
		a.GetCfsQuota(), a.GetCfsPeriod(), a.GetTotalUsage()))
	return nil
}

func (a *ThrottlingData) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("np", a.GetPeriods())
	enc.AddUint64("tp", a.GetThrottledPeriods())
	enc.AddUint64("tt", a.GetThrottledTime())
	return nil
}

func (a *TasksCompletedTotal) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *TasksFailedTotal) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *SchedulingRequestsTotal) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *AgentCount) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("c", a.GetCount())
	return nil
}

func (a *ConsumerdCount) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("c", a.GetCount())
	return nil
}

func (a *Identifier) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", types.FormatShortID(a.GetUUID(), 6, types.ElideCenter))
	return nil
}

func (a *AgentTasksTotal) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *ConsumerdTasksTotal) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *PreferredUsageLimits) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("limit", a.GetConcurrentProcessLimit())
	return nil
}

func (a *MetricsPostedTotal) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *ListenerCount) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt32("c", a.GetCount())
	return nil
}

func (a *ProviderCount) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt32("c", a.GetCount())
	return nil
}

func (a *ProviderInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", types.FormatShortID(a.GetUUID(), 6, types.ElideCenter))
	enc.AddString("addr", a.GetAddress())
	enc.AddString("kind", a.GetComponent().Name())
	return nil
}

func (a *Providers) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt("items", len(a.GetItems()))
	return nil
}

func (a *BucketSpec) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", a.GetName())
	return nil
}

func (a *LocalTasksCompleted) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *DelegatedTasksCompleted) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("t", a.GetTotal())
	return nil
}

func (a *CacheUsage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("ct", a.GetObjectCount())
	enc.AddInt64("sz", a.GetTotalSize())
	enc.AddFloat64("pct", a.GetUsagePercent())
	return nil
}

func (a *CacheHits) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("h/m/%", fmt.Sprintf("%d/%d/%f",
		a.GetCacheHitsTotal(), a.GetCacheMissesTotal(), a.GetCacheHitPercent()))
	return nil
}

func (a *Health) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("status", a.GetStatus().String())
	enc.AddArray("msgs", types.NewStringSliceEncoder(a.GetMessages()))
	return nil
}
