package meta

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Context interface {
	context.Context
	ImportFromIncoming(context.Context, ImportOptions) error
	ExportToOutgoing() context.Context
	MetadataProviders() []Provider
}

type withProviderOptions struct {
	initializer func(Context) interface{}
}

type withProviderOption func(*withProviderOptions)

func (o *withProviderOptions) Apply(opts ...withProviderOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithValue(value interface{}) withProviderOption {
	return func(wpo *withProviderOptions) {
		wpo.initializer = func(Context) interface{} {
			return value
		}
	}
}

type providerInitInfo struct {
	KeyProvider Provider
	withProviderOptions
}

func WithProvider(kp Provider, opts ...withProviderOption) providerInitInfo {
	options := withProviderOptions{}
	options.Apply(opts...)

	if options.initializer == nil {
		if p, ok := kp.(InitProvider); ok {
			options.initializer = p.InitialValue
		} else {
			panic(fmt.Sprintf("Metadata provider %s requires an initial value",
				reflect.TypeOf(kp)))
		}
	}
	return providerInitInfo{
		KeyProvider:         kp,
		withProviderOptions: options,
	}
}

func NewContext(providers ...providerInitInfo) Context {
	providerMap := map[interface{}]Provider{}
	for _, mp := range providers {
		providerMap[mp.KeyProvider.Key()] = mp.KeyProvider
	}
	ctx := &contextImpl{
		providers: providerMap,
		values:    make(map[interface{}]interface{}),
	}
	for _, mp := range providers {
		ctx.values[mp.KeyProvider.Key()] = mp.initializer(ctx)
	}
	return ctx
}

func CheckContext(ctx context.Context) error {
	if ctx.Value(mdkeys.ComponentKey) == nil || ctx.Value(mdkeys.UUIDKey) == nil {
		return status.Error(codes.InvalidArgument,
			"Context is not a meta.Context (are you using the correct interceptors?)")
	}
	return nil
}
