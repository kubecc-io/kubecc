package metrics

//go:generate msgp

type MetricsPostedTotal struct {
	Total int32 `msg:"total"`
}

func (MetricsPostedTotal) Key() string {
	return "MetricsPostedTotal"
}

type ListenerCount struct {
	Value int32 `msg:"value"`
}

func (ListenerCount) Key() string {
	return "ListenerCount"
}
