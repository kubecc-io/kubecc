package metrics

import (
	"bytes"
	"context"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Provider struct {
	ctx       context.Context
	monClient types.MonitorClient
	id        *types.Identity
	lg        *zap.SugaredLogger
}

func NewProvider(ctx context.Context, id *types.Identity) *Provider {
	ctx = types.ContextWithIdentity(ctx, id)
	lg := logkc.LogFromContext(ctx)
	monAddr := viper.GetString("monitorAddr")
	cc, err := servers.Dial(ctx, monAddr)
	if err != nil {
		lg.Error("Could not dial monitor, metrics will not be sent.")
	}
	client := types.NewMonitorClient(cc)
	go func() {
		for {
			stream, err := client.Connect(ctx, &types.Empty{}, grpc.WaitForReady(true))
			if err != nil {
				lg.Debug("Could not connect to monitor, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			}
			select {
			case <-tools.StreamClosed(stream):
				lg.Debug("Connection lost, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			case <-ctx.Done():
				return
			}
		}
	}()
	return &Provider{
		ctx:       ctx,
		monClient: client,
		id:        id,
		lg:        lg,
	}
}

func (p *Provider) Post(key *types.Key, value msgp.Encodable) bool {
	buf := new(bytes.Buffer)
	err := value.EncodeMsg(msgp.NewWriter(buf))
	if err != nil {
		p.lg.With(
			zap.Error(err),
			zap.String("key", key.Canonical()),
		).Fatal("Encoding error")
	}
	_, err = p.monClient.Post(p.ctx, &types.Metric{
		Key: key,
		Value: &types.Value{
			Data: buf.Bytes(),
		},
	})
	if err != nil {
		p.lg.With(
			zap.Error(err),
			zap.String("key", key.Canonical()),
		).Error("Error posting metric")
		return false
	}
	return true
}
