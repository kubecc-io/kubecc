package metrics

//go:generate msgp

type StorageProvider struct {
	Kind string `msg:"kind"`
}

func (StorageProvider) Key() string {
	return "StorageProvider"
}

type UsageInfo struct {
	ObjectCount  int64   `msg:"objectCount"`
	TotalSize    int64   `msg:"totalSize"`
	UsagePercent float64 `msg:"usagePercent"`
}

func (UsageInfo) Key() string {
	return "UsageInfo"
}

type CacheHits struct {
	CacheHitsTotal   int64   `msg:"cacheHitsTotal"`
	CacheMissesTotal int64   `msg:"cacheMissesTotal"`
	CacheHitPercent  float64 `msg:"cacheHitPercent"`
}

func (CacheHits) Key() string {
	return "CacheHits"
}

type PerformanceInfo struct {
	AveragePutTime int64 `msg:"averagePutTime"`
	AverageGetTime int64 `msg:"averageGetTime"`
}

func (PerformanceInfo) Key() string {
	return "PerformanceInfo"
}
