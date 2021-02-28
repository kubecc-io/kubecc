package meta

import (
	"fmt"

	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
)

type MetadataKey interface {
	fmt.Stringer
}

type InitProvider interface {
	Provider
	InitialValue(Context) interface{}
}

type Provider interface {
	Key() MetadataKey
	Marshal(interface{}) string
	Unmarshal(string) interface{}
}

type ValueAccessors interface {
	ComponentAccessor
	UUIDAccessor
	LogAccessor
	TracingAccessor
}

type ComponentAccessor interface {
	Component() types.Component
}

type UUIDAccessor interface {
	UUID() string
}

type LogAccessor interface {
	Log() *zap.SugaredLogger
}

type TracingAccessor interface {
	Tracer() opentracing.Tracer
}
