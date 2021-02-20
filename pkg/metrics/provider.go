package metrics

import (
	"context"
	"io"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
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

func NewProvider(ctx context.Context, id *types.Identity, cc *grpc.ClientConn) *Provider {
	ctx = types.OutgoingContextWithIdentity(ctx, id)
	lg := logkc.LogFromContext(ctx)
	client := types.NewMonitorClient(cc)
	go func() {
		for {
			stream, err := client.Connect(ctx, &types.Empty{}, grpc.WaitForReady(true))
			if err != nil {
				lg.With(zap.Error(err)).Warn("Could not connect to monitor, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			}
			select {
			case err := <-tools.StreamClosed(stream):
				if err == io.EOF {
					lg.With(zap.Error(err)).Warn("Connection lost, retrying in 5 seconds...")
				} else {
					lg.With(zap.Error(err)).Error("Connection failed, retrying in 5 seconds...")
				}
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
	_, err := p.monClient.Post(p.ctx, &types.Metric{
		Key: key,
		Value: &types.Value{
			Data: tools.EncodeMsgp(value),
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
