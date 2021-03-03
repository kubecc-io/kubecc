package metrics

//go:generate msgp

type LocalTasksCompleted struct {
	Total int64 `msg:"total"`
}

func (LocalTasksCompleted) Key() string {
	return "LocalTasksCompleted"
}
