/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package meta

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kubecc-io/kubecc/pkg/meta/mdkeys"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type metaContextKeyType struct{}

var metaContextKey metaContextKeyType

type contextImpl struct {
	context.Context
	providers map[interface{}]Provider
	values    map[interface{}]interface{}
}

func (ci *contextImpl) Value(key interface{}) interface{} {
	if key == metaContextKey {
		return ci
	}
	if value, ok := ci.values[key]; ok {
		return value
	}
	return ci.Context.Value(key)
}

func (*contextImpl) String() string {
	return "meta.Context"
}

type InheritOptions struct {
	InheritFrom context.Context
	Providers   []Provider
}

type ImportOptions struct {
	Required []Provider
	Optional []Provider
	Inherit  *InheritOptions
}

func ImportFromIncoming(ctx context.Context, opts ImportOptions) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic("Not an incoming context")
	}
	incomingProviders := map[interface{}]Provider{}
	incomingValues := map[interface{}]interface{}{}

	addProvider := func(mp Provider) error {
		if values := md.Get(mp.Key().String()); len(values) > 0 {
			incomingProviders[mp.Key()] = mp
			var err error
			incomingValues[mp.Key()], err = mp.Unmarshal(values[0])
			if err != nil {
				return status.Error(codes.Internal,
					fmt.Sprintf("Failed to unmarshal required metadata for provider %s: %s",
						reflect.TypeOf(mp), err.Error()))
			}
			return nil
		}
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("Expected metadata provider %s was not found",
				reflect.TypeOf(mp)))
	}

	// Import required providers
	for _, mp := range opts.Required {
		if err := addProvider(mp); err != nil {
			return nil, err
		}
	}

	// Import optional providers
	for _, mp := range opts.Optional {
		// codes.Internal returned above if there is a programmer error
		// Note this doesn't check code != InvalidArgument, because
		// status.Code(err) returns codes.OK if err == nil
		if err := addProvider(mp); status.Code(err) == codes.Internal {
			return nil, err
		}
	}

	// Inherit providers (required) from another context
	if opts.Inherit != nil {
		metaCtx, ok := fromContext(opts.Inherit.InheritFrom)
		if !ok {
			panic("Inherited context must be a meta.Context")
		}
		for _, mp := range opts.Inherit.Providers {
			key := mp.Key()
			incomingProviders[key] = mp
			incomingValues[key] = metaCtx.values[key]
		}
	}

	return &contextImpl{
		Context:   ctx,
		providers: incomingProviders,
		values:    incomingValues,
	}, nil
}

func ExportToOutgoing(outgoing context.Context) context.Context {
	c, ok := fromContext(outgoing)
	if !ok {
		panic("Provided context is not a meta.Context")
	}
	kv := []string{}
	for _, mp := range c.MetadataProviders() {
		kv = append(kv,
			mp.Key().String(),
			mp.Marshal(c.Value(mp.Key())),
		)
	}
	return metadata.AppendToOutgoingContext(outgoing, kv...)
}

func (c *contextImpl) MetadataProviders() []Provider {
	providers := []Provider{}
	for _, v := range c.providers {
		providers = append(providers, v)
	}
	return providers
}

func fromContext(ctx context.Context) (*contextImpl, bool) {
	if mctx := ctx.Value(metaContextKey); mctx != nil {
		return mctx.(*contextImpl), true
	}
	return nil, false
}

type withProviderOptions struct {
	initializer func(context.Context) interface{}
}

type withProviderOption func(*withProviderOptions)

func (o *withProviderOptions) Apply(opts ...withProviderOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithValue(value interface{}) withProviderOption {
	return func(wpo *withProviderOptions) {
		wpo.initializer = func(context.Context) interface{} {
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

// NewContext creates a new context containing data from the given metadata
// providers.
func NewContext(providers ...providerInitInfo) context.Context {
	return NewContextWithParent(context.Background(), providers...)
}

// NewContextWithParent creates a new context parented to the given context,
// and containing data from metadata providers.
func NewContextWithParent(
	parentCtx context.Context,
	providers ...providerInitInfo,
) context.Context {
	providerMap := map[interface{}]Provider{}
	for _, mp := range providers {
		providerMap[mp.KeyProvider.Key()] = mp.KeyProvider
	}
	ctx := &contextImpl{
		Context:   context.Background(),
		providers: providerMap,
		values:    make(map[interface{}]interface{}),
	}
	for _, mp := range providers {
		ctx.values[mp.KeyProvider.Key()] = mp.initializer(ctx)
	}
	return context.WithValue(ctx, metaContextKey, ctx)
}

// CheckContext returns an error if the given context does not contain at least
// a component and UUID.
func CheckContext(ctx context.Context) error {
	if ctx.Value(mdkeys.ComponentKey) == nil || ctx.Value(mdkeys.UUIDKey) == nil {
		return status.Error(codes.InvalidArgument,
			"Context is not a meta.Context (are you using the correct interceptors?)")
	}
	return nil
}
