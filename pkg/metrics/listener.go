package metrics

import (
	"context"
	"io"
	"reflect"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/metrics/builtin"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Listener struct {
	ctx       context.Context
	monClient types.MonitorClient
	lg        *zap.SugaredLogger

	// todo: this might need a mutex but maybe not
	knownProviders map[string]context.CancelFunc
}

func NewListener(ctx context.Context) *Listener {
	lg := logkc.LogFromContext(ctx)
	monAddr := viper.GetString("monitorAddr")
	cc, err := servers.Dial(ctx, monAddr)
	if err != nil {
		lg.Error("Could not dial monitor, metrics will not be sent.")
	}
	client := types.NewMonitorClient(cc)
	listener := &Listener{
		ctx:       ctx,
		monClient: client,
		lg:        lg,
	}
	return listener
}

func (l *Listener) OnProviderAdded(handler func(pctx context.Context, uuid string)) {
	l.OnValueChanged(&types.Key{
		Bucket: builtin.Bucket,
		Name:   builtin.ProvidersKey,
	}, builtin.ProvidersValue, func(val interface{}) {
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
				defer delete(l.knownProviders, uuid)
				cancel()
			}
		}
	})
}

func (l *Listener) OnValueChanged(key *types.Key, expected msgp.Decodable, handler func(interface{})) {
	go func() {
		valueType := reflect.TypeOf(expected)
		for {
			stream, err := l.monClient.Watch(l.ctx, key, grpc.WaitForReady(true))
			if err != nil {
				l.lg.With(zap.Error(err)).Debug("Error watching key, retrying in 1 second...")
				time.Sleep(1)
				continue
			}
			for {
				untyped, err := stream.Recv()
				if err == io.EOF {
					break
				} else if err != nil {
					l.lg.With(zap.Error(err)).Debug("Error watching key, retrying in 1 second...")
					time.Sleep(1)
					break
				}
				typed := reflect.New(valueType).Interface().(msgp.Decodable)
				err = tools.DecodeMsgp(untyped.Data, typed)
				if err != nil {
					l.lg.With(zap.Error(err)).Error("Error decoding value")
					continue
				}
				handler(typed)
			}
		}
	}()
}
