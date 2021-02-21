package meta

//go:generate msgp

const (
	Bucket = "meta"
)

type Providers struct {
	Items map[string]int32 `msg:"items"`
}

func (Providers) Key() string {
	return "Providers"
}

type Alive struct{}

func (Alive) Key() string {
	return "Alive"
}
