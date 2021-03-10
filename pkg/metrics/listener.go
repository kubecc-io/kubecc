package metrics

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"

	mmetrics "github.com/cobalt77/kubecc/pkg/apps/monitor/metrics"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type monitorListener struct {
	ctx       context.Context
	monClient types.ExternalMonitorClient
	lg        *zap.SugaredLogger

	knownProviders map[string]context.CancelFunc

	providersMutex *sync.Mutex
}

func NewListener(
	ctx context.Context,
	client types.ExternalMonitorClient,
) Listener {
	listener := &monitorListener{
		ctx:            ctx,
		lg:             meta.Log(ctx),
		monClient:      client,
		knownProviders: make(map[string]context.CancelFunc),
		providersMutex: &sync.Mutex{},
	}
	return listener
}

func (l *monitorListener) OnProviderAdded(handler func(context.Context, string)) {
	doUpdate := func(providers *mmetrics.Providers) {
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

	l.OnValueChanged(mmetrics.MetaBucket, func(providers *mmetrics.Providers) {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
		doUpdate(providers)
	}).OrExpired(func() RetryOptions {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
		doUpdate(&mmetrics.Providers{
			Items: make(map[string]mmetrics.ProviderInfo),
		})
		return Retry
	})
}

type RetryOptions uint32

const (
	NoRetry RetryOptions = iota
	Retry
)

type changeListener struct {
	ctx            context.Context
	expiredHandler func() RetryOptions
	handler        reflect.Value
	ehMutex        *sync.Mutex
	monClient      types.ExternalMonitorClient
	key            *types.Key
	argType        reflect.Type
}

func (cl *changeListener) HandleStream(stream grpc.ClientStream) error {
	lg := meta.Log(cl.ctx)
	argValue := reflect.New(cl.argType)
	if _, ok := argValue.Interface().(msgp.Decodable); !ok {
		panic("Handler argument is not msgp.Decodable")
	}
	decodable := argValue.Interface().(msgp.Decodable)

	for {
		rawData := &types.Value{}
		err := stream.RecvMsg(rawData)
		if errors.Is(err, io.EOF) {
			lg.Debug(err)
			return nil
		}
		switch status.Code(err) {
		case codes.OK:
			err = util.DecodeMsgp(rawData.Data, decodable)
			if err != nil {
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
) ChangeListener {
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
	mgr := servers.NewStreamManager(l.ctx, cl)
	go mgr.Run()
	return cl
}
