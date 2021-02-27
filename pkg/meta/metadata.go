package meta

import (
	"context"
	"fmt"

	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type MetadataKey interface {
	fmt.Stringer
}

type Provider interface {
	Key() MetadataKey
	ExportToOutgoing(context.Context)
	ImportFromIncoming(context.Context, metadata.MD)
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
