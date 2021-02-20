package builtin

import (
	"github.com/cobalt77/kubecc/pkg/types"
	_ "github.com/tinylib/msgp"
)

//go:generate msgp

const (
	ProvidersKey = "builtin.providers"
)

var (
	ProvidersValue *Providers = nil
)

type Providers struct {
	Items map[string]types.Component `msg:"items"`
}
