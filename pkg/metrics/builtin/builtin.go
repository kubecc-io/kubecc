package builtin

//go:generate msgp

const (
	Bucket       = "builtin"
	ProvidersKey = "providers"
)

var (
	ProvidersValue *Providers
)

type Providers struct {
	Items map[string]int32 `msg:"items"`
}
