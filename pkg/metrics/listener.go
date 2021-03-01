package metrics

import (
	"context"
	"reflect"
	"sync"
	"time"

	mmeta "github.com/cobalt77/kubecc/pkg/metrics/meta"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Listener struct {
	ctx       context.Context
	monClient types.ExternalMonitorClient
	lg        *zap.SugaredLogger

	knownProviders map[string]context.CancelFunc
	providersMutex *sync.Mutex
}

func NewListener(ctx context.Context, client types.ExternalMonitorClient) *Listener {
	listener := &Listener{
		ctx:            ctx,
		monClient:      client,
		knownProviders: make(map[string]context.CancelFunc),
		providersMutex: &sync.Mutex{},
	}
	return listener
}

func (l *Listener) OnProviderAdded(handler func(pctx context.Context, uuid string)) {
	doUpdate := func(providers *mmeta.Providers) {
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

	l.OnValueChanged(mmeta.Bucket, func(providers *mmeta.Providers) {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
		doUpdate(providers)
	}).OrExpired(func() RetryOptions {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
		doUpdate(&mmeta.Providers{
			Items: map[string]int32{},
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
	expiredHandler func() RetryOptions
	ehMutex        *sync.Mutex
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

func (l *Listener) OnValueChanged(
	bucket string,
	handler interface{}, // func(type)
) *changeListener {
	valueType, funcValue := handlerArgType(handler)
	keyName := valueType.Name()
	cl := &changeListener{
		ehMutex: &sync.Mutex{},
	}
	go func() {
		for {
			stream, err := l.monClient.Listen(l.ctx, &types.Key{
				Bucket: bucket,
				Name:   keyName,
			}, grpc.WaitForReady(true))
			if err != nil {
				l.lg.With(
					zap.Error(err),
					zap.String("bucket", bucket),
					zap.String("key", keyName),
				).Warn("Error watching key, retrying in 1 second...")
				time.Sleep(1 * time.Second)
				continue
			}
			for {
				untyped, err := stream.Recv()
				switch status.Code(err) {
				case codes.OK:
					val := reflect.New(valueType)
					typed := val.Interface().(msgp.Decodable)
					err = tools.DecodeMsgp(untyped.Data, typed)
					if err != nil {
						l.lg.With(zap.Error(err)).Error("Error decoding value")
						continue
					}
					funcValue.Call([]reflect.Value{val})
				case codes.Aborted, codes.Unavailable:
					cl.ehMutex.Lock()
					if cl.expiredHandler != nil {
						retryOp := cl.expiredHandler()
						if retryOp == Retry {
							cl.ehMutex.Unlock()
							goto retry
						}
					}
					cl.ehMutex.Unlock()
					return
				case codes.InvalidArgument:
					l.lg.With(
						zap.Error(err),
						zap.String("bucket", bucket),
						zap.String("key", keyName),
					).Error("Error watching key")
					return
				default:
					l.lg.With(
						zap.Error(err),
						zap.String("bucket", bucket),
						zap.String("key", keyName),
					).Warn("Error watching key, retrying in 1 second...")
					time.Sleep(1 * time.Second)
					goto retry
				}
			}
		retry:
		}
	}()
	return cl
}
