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

	mapset "github.com/deckarep/golang-set"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

type monitorMetricsProvider struct {
	ctx               context.Context
	lg                *zap.SugaredLogger
	monClient         types.MonitorClient
	postQueue         chan proto.Message
	queueStrategy     QueueStrategy
	metricCtxMap      map[string]context.CancelFunc
	metricCtxMapMutex *sync.Mutex
}

type QueueStrategy int

const (
	Buffered QueueStrategy = 1 << iota
	Discard
	Block
)

type MetricsProviderOptions struct {
	statusCtrl *metrics.StatusController
}

type MetricsProviderOption func(*MetricsProviderOptions)

func (o *MetricsProviderOptions) Apply(opts ...MetricsProviderOption) {
	for _, op := range opts {
		op(o)
	}
}

func StatusCtrl(ctrl *metrics.StatusController) MetricsProviderOption {
	return func(o *MetricsProviderOptions) {
		o.statusCtrl = ctrl
	}
}

func NewMetricsProvider(
	ctx context.Context,
	client types.MonitorClient,
	qs QueueStrategy,
	opts ...MetricsProviderOption,
) MetricsProvider {
	options := MetricsProviderOptions{}
	options.Apply(opts...)

	var postQueue chan proto.Message
	if (qs & Buffered) != 0 {
		postQueue = make(chan proto.Message, 1e6)
	} else {
		postQueue = make(chan proto.Message)
	}

	provider := &monitorMetricsProvider{
		ctx:               ctx,
		monClient:         client,
		postQueue:         postQueue,
		lg:                meta.Log(ctx),
		metricCtxMap:      make(map[string]context.CancelFunc),
		metricCtxMapMutex: &sync.Mutex{},
	}

	if options.statusCtrl != nil {
		go provider.watchStatus(options.statusCtrl)
	}

	mgr := NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
}

func (p *monitorMetricsProvider) HandleStream(stream grpc.ClientStream) error {
	for {
		select {
		case metric := <-p.postQueue:
			any, err := anypb.New(metric)
			if err != nil {
				return err
			}
			key := &types.Key{
				Bucket: meta.UUID(p.ctx),
				Name:   any.GetTypeUrl(),
			}
			p.lg.With(
				types.ShortID(key.ShortID()),
			).Debug("Posting metric")
			err = stream.SendMsg(&types.Metric{
				Key:   key,
				Value: any,
			})
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = stream.RecvMsg(nil)
				}
				p.lg.With(
					zap.Error(err),
					zap.String("key", key.Canonical()),
				).Error("Error posting metric")
				return err
			}
		case <-stream.Context().Done():
			return stream.RecvMsg(nil)
		case <-p.ctx.Done():
			return nil
		}
	}
}

func (p *monitorMetricsProvider) TryConnect() (grpc.ClientStream, error) {
	return p.monClient.Stream(p.ctx)
}

func (p *monitorMetricsProvider) Target() string {
	return "monitor"
}

func (p *monitorMetricsProvider) PostContext(metric proto.Message, ctx context.Context) {
	p.Post(metric)
	p.metricCtxMapMutex.Lock()
	defer p.metricCtxMapMutex.Unlock()
	any, err := anypb.New(metric)
	if err != nil {
		panic(err)
	}
	key := any.GetTypeUrl()
	if cancel, ok := p.metricCtxMap[key]; ok {
		cancel()
	}
	localCtx, cancel := context.WithCancel(ctx)
	p.metricCtxMap[key] = cancel
	// When the context is done, send a deleter to the server
	go func() {
		defer func() {
			p.metricCtxMapMutex.Lock()
			defer p.metricCtxMapMutex.Unlock()
			delete(p.metricCtxMap, key)
		}()
		select {
		case <-ctx.Done():
			p.Post(&metrics.Deleter{
				Key: key,
			})
		case <-localCtx.Done(): // the map context
			return
		case <-p.ctx.Done():
		}
	}()
}

func (p *monitorMetricsProvider) Post(metric proto.Message) {
	if (p.queueStrategy & Discard) == 0 {
		// Block is the default
		p.postQueue <- metric
		return
	}
	// Discard logic below
	select {
	case p.postQueue <- metric:
		return
	default:
	}
	// If not buffered, do nothing
	if (p.queueStrategy & Buffered) == 0 {
		return
	}
	select {
	case p.postQueue <- metric:
	default:
		p.lg.Debug("Post queue filled, dropping some messages")
		for i := 0; i < cap(p.postQueue)/4; i++ {
			select {
			case <-p.postQueue:
			default:
				// Drain up to 25% of the oldest items, but the channel could be
				// being read from elsewhere, so prevent accidental blocking
				goto done
			}
		}
	done:
		p.postQueue <- metric
	}
}

func (p *monitorMetricsProvider) watchStatus(ctrl *metrics.StatusController) {
	stream := ctrl.StreamHealthUpdates()
	for {
		h := <-stream
		p.Post(h)
	}
}

type monitorListener struct {
	ctx            context.Context
	cancel         context.CancelFunc
	monClient      types.MonitorClient
	lg             *zap.SugaredLogger
	streamOpts     []StreamManagerOption
	knownProviders map[string]context.CancelFunc
	providersMutex *sync.Mutex
}

func NewMetricsListener(
	ctx context.Context,
	client types.MonitorClient,
	streamOpts ...StreamManagerOption,
) MetricsListener {
	ctx, cancel := context.WithCancel(ctx)
	listener := &monitorListener{
		ctx:            ctx,
		cancel:         cancel,
		lg:             meta.Log(ctx),
		monClient:      client,
		knownProviders: make(map[string]context.CancelFunc),
		providersMutex: &sync.Mutex{},
		streamOpts: append([]StreamManagerOption{
			WithLogEvents(LogConnectionFailed),
		}, streamOpts...),
	}
	return listener
}

func (l *monitorListener) OnProviderAdded(handler func(context.Context, string)) {
	doUpdate := func(providers *metrics.Providers) {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()

		known := mapset.NewSet()
		for uuid := range l.knownProviders {
			known.Add(uuid)
		}
		updated := mapset.NewSet()
		for uuid := range providers.GetItems() {
			updated.Add(uuid)
		}
		removed := known.Difference(updated)
		added := updated.Difference(known)

		removed.Each(func(i interface{}) (stop bool) {
			uuid := i.(string)
			cancel := l.knownProviders[uuid]
			delete(l.knownProviders, uuid)
			defer cancel()
			return
		})
		added.Each(func(i interface{}) (stop bool) {
			uuid := i.(string)
			pctx, cancel := context.WithCancel(l.ctx)
			l.knownProviders[uuid] = cancel
			defer func() {
				go handler(pctx, uuid)
			}()
			return
		})
	}

	l.OnValueChanged(MetaBucket, func(providers *metrics.Providers) {
		doUpdate(providers)
	}).OrExpired(func() RetryOptions {
		doUpdate(&metrics.Providers{})
		return Retry
	})
}

type changeListener struct {
	ctx            context.Context
	expiredHandler func() RetryOptions
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
				if retryOp == Retry {
					cl.ehMutex.Unlock()
					return err
				}
			}
			cl.ehMutex.Unlock()
			return nil
		case codes.Canceled:
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

func (c *changeListener) OrExpired(handler func() RetryOptions) {
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
) ChangeListener {
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
	mgr := NewStreamManager(l.ctx, cl, l.streamOpts...)
	go mgr.Run()
	return cl
}

func (l *monitorListener) Stop() {
	l.cancel()
}
