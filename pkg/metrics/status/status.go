package status

//go:generate msgp

const (
	Bucket = "status"
)

type QueueParamsCompleter interface {
	CompleteQueueParams(*QueueParams)
}

type TaskStatusCompleter interface {
	CompleteTaskStatus(*TaskStatus)
}

type QueueStatusCompleter interface {
	CompleteQueueStatus(*QueueStatus)
}

type QueueParams struct {
	ConcurrentProcessLimit  int32   `msg:"ConcurrentProcessLimit"`
	QueuePressureMultiplier float64 `msg:"queuePressureThreshold"`
	QueueRejectMultiplier   float64 `msg:"queueRejectThreshold"`
}

func (QueueParams) Key() string {
	return "QueueParams"
}

type TaskStatus struct {
	NumRunningProcesses int32 `msg:"numRunningProcesses"`
	NumQueuedProcesses  int32 `msg:"numQueuedProcesses"`
}

func (TaskStatus) Key() string {
	return "TaskStatus"
}

type QueueStatus struct {
	QueueStatus int32 `msg:"queueStatus"`
}

func (QueueStatus) Key() string {
	return "QueueStatus"
}
