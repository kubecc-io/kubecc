package metrics

//go:generate msgp

type TasksCompletedTotal struct {
	Total int64 `msg:"total"`
}

func (TasksCompletedTotal) Key() string {
	return "TasksCompletedTotal"
}

type TasksFailedTotal struct {
	Total int64 `msg:"total"`
}

func (TasksFailedTotal) Key() string {
	return "TasksFailedTotal"
}

type SchedulingRequestsTotal struct {
	Total int64 `msg:"total"`
}

func (SchedulingRequestsTotal) Key() string {
	return "SchedulingRequestsTotal"
}

type AgentCount struct {
	Count int32 `msg:"count"`
}

func (AgentCount) Key() string {
	return "AgentCount"
}

type CdCount struct {
	Count int32 `msg:"count"`
}

func (CdCount) Key() string {
	return "CdCount"
}

type Identifier struct {
	UUID string `msg:"uuid"`
}

type AgentWeight struct {
	Identifier
	Value float64 `msg:"agentWeight"`
}

func (AgentWeight) Key() string {
	return "AgentWeight"
}

type AgentTasksTotal struct {
	Identifier
	Total int64 `msg:"total"`
}

func (AgentTasksTotal) Key() string {
	return "AgentTasksTotal"
}

type CdTasksTotal struct {
	Identifier
	Total int64 `msg:"total"`
}

func (CdTasksTotal) Key() string {
	return "CdTasksTotal"
}
