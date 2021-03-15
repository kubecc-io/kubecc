package metrics

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type noopProvider struct{}

func NewNoopProvider() Provider {
	return &noopProvider{}
}

func (noopProvider) Post(proto.Message) {}

func (noopProvider) PostContext(proto.Message, context.Context) {}

type noopListener struct{}

func NewNoopListener() Listener {
	return &noopListener{}
}

func (noopListener) OnValueChanged(string, interface{}) ChangeListener {
	return noopChangeListener{}
}

func (noopListener) OnProviderAdded(func(context.Context, string)) {}

type noopChangeListener struct{}

func (noopChangeListener) TryConnect() (grpc.ClientStream, error) {
	return nil, nil
}

func (noopChangeListener) HandleStream(grpc.ClientStream) error {
	return nil
}

func (noopChangeListener) Target() string {
	return "<unknown>"
}

func (noopChangeListener) OrExpired(func() RetryOptions) {}
