package metrics

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/servers"
	"google.golang.org/protobuf/proto"
)

const MetaBucket = "meta"

type RetryOptions uint32

const (
	NoRetry RetryOptions = iota
	Retry
)

type ContextMetric interface {
	Context() context.Context
}

type Provider interface {
	Post(metric proto.Message)
	PostContext(metric proto.Message, ctx context.Context)
}

type Listener interface {
	OnValueChanged(bucket string, handler interface{}) ChangeListener
	OnProviderAdded(func(context.Context, string))
	Stop()
}

type ChangeListener interface {
	servers.StreamHandler
	OrExpired(handler func() RetryOptions)
}
