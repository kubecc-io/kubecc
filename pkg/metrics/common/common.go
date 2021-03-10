package common

//go:generate msgp

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
	ConcurrentProcessLimit  int32   `msg:"concurrentProcessLimit"`
	QueuePressureMultiplier float64 `msg:"queuePressureMultiplier"`
	QueueRejectMultiplier   float64 `msg:"queueRejectMultiplier"`
}

func (QueueParams) Key() string {
	return "QueueParams"
}

type TaskStatus struct {
	NumRunning   int32 `msg:"numRunning"`
	NumQueued    int32 `msg:"numQueued"`
	NumDelegated int32 `msg:"numDelegated"`
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

type Alive struct {
}

func (Alive) Key() string {
	return "Alive"
}
