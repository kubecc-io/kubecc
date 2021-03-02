package metrics

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func runWaitReceiver(postQueue chan KeyedMetric, enableQueue <-chan bool) {
	latestMessages := map[string]KeyedMetric{}
	for {
		if e := <-enableQueue; !e {
			// Already disabled
			continue
		}
		for {
			select {
			case m, ok := <-postQueue:
				if !ok {
					// Post queue closed
					return
				}
				latestMessages[m.Key()] = m
			case e := <-enableQueue:
				if e {
					// Already enabled
					continue
				}
				// Send the latest queued message for each key
				for k, v := range latestMessages {
					postQueue <- v
					delete(latestMessages, k)
				}
				goto restart
			}
		}
	restart:
	}
}

type Provider struct {
	ctx          context.Context
	monClient    types.InternalMonitorClient
	lg           *zap.SugaredLogger
	postQueue    chan KeyedMetric
	enableWaitRx chan bool
}

func (p *Provider) HandleStream(stream grpc.ClientStream) error {
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
			err := stream.SendMsg(&types.Metric{
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
				return err
			}
		case <-stream.Context().Done():
			return nil
		case <-p.ctx.Done():
			return nil
		}
	}
}

func (p *Provider) TryConnect() (grpc.ClientStream, error) {
	return p.monClient.Stream(p.ctx)
}

func (p *Provider) OnConnected() {
	p.enableWaitRx <- false
}

func (p *Provider) OnLostConnection() {
	p.enableWaitRx <- true
}

func (p *Provider) Target() string {
	return "monitor"
}

func NewProvider(
	ctx context.Context,
	client types.InternalMonitorClient,
) *Provider {
	provider := &Provider{
		ctx:          ctx,
		monClient:    client,
		lg:           meta.Log(ctx),
		postQueue:    make(chan KeyedMetric, 10),
		enableWaitRx: make(chan bool),
	}

	go runWaitReceiver(provider.postQueue, provider.enableWaitRx)
	mgr := servers.NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
}

func (p *Provider) Post(metric KeyedMetric) {
	p.postQueue <- metric
}
