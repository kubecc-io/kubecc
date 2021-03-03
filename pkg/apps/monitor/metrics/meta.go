package metrics

//go:generate msgp

const (
	MetaBucket = "meta"
)

type Providers struct {
	Items map[string]int32 `msg:"items"`
}

func (Providers) Key() string {
	return "Providers"
}

type StoreContents struct {
	Buckets []BucketSpec `json:"buckets" msg:"buckets"`
}

type BucketSpec struct {
	Name string            `json:"name" msg:"name"`
	Data map[string][]byte `json:"data" msg:"data"`
}

func (StoreContents) Key() string {
	return "StoreContents"
}
