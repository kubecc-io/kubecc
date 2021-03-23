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

package clients

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

type monitorListener struct {
	ctx            context.Context
	cancel         context.CancelFunc
	monClient      types.MonitorClient
	lg             *zap.SugaredLogger
	streamOpts     []servers.StreamManagerOption
	knownProviders map[string]context.CancelFunc
	providersMutex *sync.Mutex
}

func NewListener(
	ctx context.Context,
	client types.MonitorClient,
	streamOpts ...servers.StreamManagerOption,
) metrics.Listener {
	ctx, cancel := context.WithCancel(ctx)
	listener := &monitorListener{
		ctx:            ctx,
		cancel:         cancel,
		lg:             meta.Log(ctx),
		monClient:      client,
		knownProviders: make(map[string]context.CancelFunc),
		providersMutex: &sync.Mutex{},
		streamOpts:     streamOpts,
	}
	return listener
}

func (l *monitorListener) OnProviderAdded(handler func(context.Context, string)) {
	doUpdate := func(providers *metrics.Providers) {
		for uuid := range providers.Items {
			if _, ok := l.knownProviders[uuid]; !ok {
				pctx, cancel := context.WithCancel(l.ctx)
				l.knownProviders[uuid] = cancel
				go handler(pctx, uuid)
			}
		}
		for uuid, cancel := range l.knownProviders {
			if _, ok := providers.Items[uuid]; !ok {
				// this is called before the mutex is unlocked, defers are LIFO
				defer delete(l.knownProviders, uuid)
				cancel()
			}
		}
	}

	l.OnValueChanged(metrics.MetaBucket, func(providers *metrics.Providers) {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
		doUpdate(providers)
	}).OrExpired(func() metrics.RetryOptions {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
		doUpdate(&metrics.Providers{})
		return metrics.Retry
	})
}

type changeListener struct {
	ctx            context.Context
	expiredHandler func() metrics.RetryOptions
	handler        reflect.Value
	ehMutex        *sync.Mutex
	monClient      types.MonitorClient
	key            *types.Key
	argType        reflect.Type
}

func (cl *changeListener) HandleStream(clientStream grpc.ClientStream) error {
	stream := clientStream.(types.Monitor_ListenClient)
	lg := meta.Log(cl.ctx)
	argValue := reflect.New(cl.argType)
	var msgReflect protoreflect.ProtoMessage
	if msg, ok := argValue.Interface().(proto.Message); ok {
		msgReflect = msg
	} else {
		panic("Handler argument does not implement proto.Message")
	}
	for {
		any, err := stream.Recv()
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
			lg.Debug(err)
			return nil
		}
		switch status.Code(err) {
		case codes.OK:
			if err := any.UnmarshalTo(msgReflect); err != nil {
				lg.With(zap.Error(err)).Error("Error decoding value")
				return err
			}
			cl.handler.Call([]reflect.Value{argValue})
		case codes.Aborted, codes.Unavailable:
			cl.ehMutex.Lock()
			if cl.expiredHandler != nil {
				retryOp := cl.expiredHandler()
				if retryOp == metrics.Retry {
					cl.ehMutex.Unlock()
					return err
				}
			}
			cl.ehMutex.Unlock()
			return nil
		default:
			lg.With(
				zap.Error(err),
				zap.String("bucket", cl.key.Bucket),
				zap.String("key", cl.key.Name),
			).Warn("Error watching key, retrying")
			return err
		}
	}
}

func (s *changeListener) TryConnect() (grpc.ClientStream, error) {
	return s.monClient.Listen(s.ctx, s.key)
}

func (s *changeListener) Target() string {
	return "monitor"
}

func (c *changeListener) OrExpired(handler func() metrics.RetryOptions) {
	c.ehMutex.Lock()
	defer c.ehMutex.Unlock()
	c.expiredHandler = handler
}

func handlerArgType(handler interface{}) (reflect.Type, reflect.Value, string) {
	funcType := reflect.TypeOf(handler)
	if funcType.NumIn() != 1 {
		panic("handler must be a function with one argument")
	}
	valueType := funcType.In(0).Elem()
	proto, ok := reflect.New(valueType).Interface().(proto.Message)
	if !ok {
		panic("argument must implement proto.Message")
	}
	any, err := anypb.New(proto)
	if err != nil {
		panic(err)
	}
	funcValue := reflect.ValueOf(handler)
	return valueType, funcValue, any.GetTypeUrl()
}

func (l *monitorListener) OnValueChanged(
	bucket string,
	handler interface{}, // func(type)
) metrics.ChangeListener {
	argType, funcValue, typeUrl := handlerArgType(handler)
	cl := &changeListener{
		ctx:       l.ctx,
		handler:   funcValue,
		argType:   argType,
		ehMutex:   &sync.Mutex{},
		monClient: l.monClient,
		key: &types.Key{
			Bucket: bucket,
			Name:   typeUrl,
		},
	}
	mgr := servers.NewStreamManager(l.ctx, cl, l.streamOpts...)
	go mgr.Run()
	return cl
}

func (l *monitorListener) Stop() {
	l.cancel()
}
