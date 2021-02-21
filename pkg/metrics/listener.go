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
	l.OnValueChanged(&types.Key{
		Bucket: builtin.Bucket,
		Name:   builtin.ProvidersKey,
	}, builtin.ProvidersValue, func(val interface{}) {
		l.providersMutex.Lock()
		defer l.providersMutex.Unlock()

		providers := val.(*builtin.Providers)
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
	key *types.Key,
	expected msgp.Decodable,
	handler func(interface{}),
) *changeListener {
	cl := &changeListener{}
	go func() {
		valueType := reflect.TypeOf(expected).Elem()
		for {
			stream, err := l.monClient.Watch(l.ctx, key, grpc.WaitForReady(true))
			if err != nil {
				l.lg.With(zap.Error(err)).Warn("Error watching key, retrying in 1 second...")
				time.Sleep(1 * time.Second)
				continue
			}
			for {
				untyped, err := stream.Recv()
				switch status.Code(err) {
				case codes.OK:
					typed := reflect.New(valueType).Interface().(msgp.Decodable)
					err = tools.DecodeMsgp(untyped.Data, typed)
					if err != nil {
						l.lg.With(zap.Error(err)).Error("Error decoding value")
						continue
					}
					handler(typed)
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
