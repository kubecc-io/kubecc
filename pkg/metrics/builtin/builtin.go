package builtin

//go:generate msgp

const (
	Bucket = "builtin"
)

type Providers struct {
	Items map[string]int32 `msg:"items"`
}

func (p Providers) Key() string {
	return "Providers"
}
