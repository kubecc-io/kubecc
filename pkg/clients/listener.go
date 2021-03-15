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
	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type monitorListener struct {
	ctx            context.Context
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
	listener := &monitorListener{
		ctx:            ctx,
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
				pctx, cancel := context.WithCancel(context.Background())
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
	if msg, ok := argValue.Interface().(proto.Message); !ok {
		msgReflect = msg
		panic("Handler argument does not implement proto.Message")
	}
	for {
		any, err := stream.Recv()
		if errors.Is(err, io.EOF) {
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

func handlerArgType(handler interface{}) (reflect.Type, reflect.Value) {
	funcType := reflect.TypeOf(handler)
	if funcType.NumIn() != 1 {
		panic("handler must be a function with one argument")
	}
	valuePtrType := funcType.In(0)
	valueType := valuePtrType.Elem()
	if !valuePtrType.Implements(reflect.TypeOf((*msgp.Decodable)(nil)).Elem()) {
		panic("argument must implement msgp.Decodable")
	}
	funcValue := reflect.ValueOf(handler)
	return valueType, funcValue
}

func (l *monitorListener) OnValueChanged(
	bucket string,
	handler interface{}, // func(type)
) metrics.ChangeListener {
	argType, funcValue := handlerArgType(handler)
	cl := &changeListener{
		ctx:       l.ctx,
		handler:   funcValue,
		argType:   argType,
		ehMutex:   &sync.Mutex{},
		monClient: l.monClient,
		key: &types.Key{
			Bucket: bucket,
			Name:   argType.Name(),
		},
	}
	mgr := servers.NewStreamManager(l.ctx, cl, l.streamOpts...)
	go mgr.Run()
	return cl
}
