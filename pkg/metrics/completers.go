package metrics

type UsageLimitsCompleter interface {
	CompleteUsageLimits(*UsageLimits)
}

type TaskStatusCompleter interface {
	CompleteTaskStatus(*TaskStatus)
}
