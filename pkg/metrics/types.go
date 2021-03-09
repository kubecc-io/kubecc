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

type ContextMetric interface {
	Context() context.Context
}

type Provider interface {
	Post(metric KeyedMetric)
}

type Listener interface {
	OnValueChanged(bucket string, handler interface{}) ChangeListener
	OnProviderAdded(func(context.Context, string))
}

type ChangeListener interface {
	servers.StreamHandler
	OrExpired(handler func() RetryOptions)
}

type contextMetric struct {
	KeyedMetric
	ctx context.Context
}

func (cm *contextMetric) Context() context.Context {
	return cm.ctx
}

func WithContext(m KeyedMetric, ctx context.Context) KeyedMetric {
	return &contextMetric{
		KeyedMetric: m,
		ctx:         ctx,
	}
}

type deleter struct {
	msgp.Decodable
	msgp.Encodable
	key string
}

func (d deleter) Key() string {
	return d.key
}

func (deleter) Context() context.Context {
	return nil
}
