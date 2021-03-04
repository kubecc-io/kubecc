package metrics

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/tinylib/msgp/msgp"
)

type KeyedMetric interface {
	msgp.Decodable
	msgp.Encodable
	Key() string
}

type Provider interface {
	Post(metric KeyedMetric, contexts ...context.Context)
}

type Listener interface {
	OnValueChanged(bucket string, handler interface{}) ChangeListener
	OnProviderAdded(func(context.Context, string))
}

type ChangeListener interface {
	servers.StreamHandler
	OrExpired(handler func() RetryOptions)
}
