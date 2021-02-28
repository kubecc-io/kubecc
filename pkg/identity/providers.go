package identity

import (
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
)

type componentMDP struct{}

var Component componentMDP

func (componentMDP) Key() meta.MetadataKey {
	return mdkeys.ComponentKey
}

func (componentMDP) Marshal(i interface{}) string {
	return types.Component_name[int32(i.(types.Component))]
}

func (componentMDP) Unmarshal(s string) interface{} {
	return types.Component(types.Component_value[s])
}

type uuidMDP struct{}

var UUID uuidMDP

func (uuidMDP) Key() meta.MetadataKey {
	return mdkeys.UUIDKey
}

func (uuidMDP) InitialValue(meta.Context) interface{} {
	return uuid.NewString()
}

func (uuidMDP) Marshal(i interface{}) string {
	return i.(string)
}

func (uuidMDP) Unmarshal(s string) interface{} {
	return s
}
