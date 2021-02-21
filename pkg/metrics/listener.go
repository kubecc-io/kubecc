package metrics

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/metrics/builtin"
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
	monClient types.MonitorClient
	lg        *zap.SugaredLogger

	knownProviders map[string]context.CancelFunc
	providersMutex *sync.Mutex
}

func NewListener(ctx context.Context, cc *grpc.ClientConn) *Listener {
	lg := logkc.LogFromContext(ctx)
	client := types.NewMonitorClient(cc)
	listener := &Listener{
		ctx:            ctx,
		monClient:      client,
		lg:             lg,
		knownProviders: make(map[string]context.CancelFunc),
		providersMutex: &sync.Mutex{},
	}
	return listener
}

func (l *Listener) OnProviderAdded(handler func(pctx context.Context, uuid string)) {
	l.OnValueChanged(builtin.Bucket, func(providers *builtin.Providers) {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()
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
	})
}

type changeListener struct {
	expiredHandler func()
}

func (c *changeListener) OrExpired(handler func()) {
	c.expiredHandler = handler
}

func (l *Listener) OnValueChanged(
	bucket string,
	handler interface{}, // func(type)
) *changeListener {
	funcType := reflect.TypeOf(handler)
	if funcType.NumIn() != 1 {
		panic("handler must be a function with one argument")
	}
	valuePtrType := funcType.In(0)
	valueType := valuePtrType.Elem()
	if !valuePtrType.Implements(reflect.TypeOf((*msgp.Decodable)(nil)).Elem()) {
		panic("argument must implement msgp.Decodable")
	}
	keyName := valueType.Name()
	funcValue := reflect.ValueOf(handler)

	cl := &changeListener{}
	go func() {
		for {
			stream, err := l.monClient.Watch(l.ctx, &types.Key{
				Bucket: bucket,
				Name:   keyName,
			}, grpc.WaitForReady(true))
			if err != nil {
				l.lg.With(zap.Error(err)).Warn("Error watching key, retrying in 1 second...")
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
				case codes.Aborted:
					if cl.expiredHandler != nil {
						cl.expiredHandler()
					}
					return
				default:
					l.lg.With(zap.Error(err)).Warn("Error watching key, retrying in 1 second...")
					time.Sleep(1 * time.Second)
					goto retry
				}
			}
		retry:
		}
	}()
	return cl
}
