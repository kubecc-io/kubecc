package metrics

import (
	"context"
	"time"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Provider struct {
	ctx       context.Context
	monClient types.InternalMonitorClient
	lg        *zap.SugaredLogger
	postQueue chan KeyedMetric
}

func (p *Provider) start() {
	for {
		stream, err := p.monClient.Stream(p.ctx, grpc.WaitForReady(true))
		if err != nil {
			p.lg.With(zap.Error(err)).Warn("Could not connect to monitor, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		for {
			select {
			case metric := <-p.postQueue:
				key := &types.Key{
					Bucket: meta.UUID(p.ctx),
					Name:   metric.Key(),
				}
				p.lg.With(
					types.ShortID(key.ShortID()),
				).Debug("Posting metric")
				err := stream.Send(&types.Metric{
					Key: key,
					Value: &types.Value{
						Data: tools.EncodeMsgp(metric),
					},
				})
				if err != nil {
					p.lg.With(
						zap.Error(err),
						zap.String("key", key.Canonical()),
					).Error("Error posting metric")
				}
			case <-stream.Context().Done():
				// if errors.Is(err, io.EOF) {
				// 	p.lg.With(zap.Error(err)).Warn("Connection lost, retrying in 5 seconds...")
				// } else {
				p.lg.With(zap.Error(err)).Error("Connection failed, retrying in 5 seconds...")
				// }
				time.Sleep(5 * time.Second)
				goto reconnect
			case <-p.ctx.Done():
				err := stream.CloseSend()
				if err != nil {
					p.lg.With(zap.Error(err)).Error("Error closing metrics stream")
				}
				return
			}
		}
	reconnect:
	}
}

func NewProvider(
	ctx context.Context,
	client types.InternalMonitorClient,
) *Provider {
	provider := &Provider{
		ctx:       ctx,
		monClient: client,
		lg:        meta.Log(ctx),
		postQueue: make(chan KeyedMetric, 10),
	}

	go provider.start()
	return provider
}

func (p *Provider) Post(metric KeyedMetric) {
	p.postQueue <- metric
}
