package metrics

import (
	"context"
	"errors"
	"io"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
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

type monitorProvider struct {
	ctx          context.Context
	lg           *zap.SugaredLogger
	monClient    types.InternalMonitorClient
	postQueue    chan KeyedMetric
	enableWaitRx chan bool
}

func (p *monitorProvider) HandleStream(stream grpc.ClientStream) error {
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
					Data: util.EncodeMsgp(metric),
				},
			})
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = stream.RecvMsg(nil)
				}
				p.lg.With(
					zap.Error(err),
					zap.String("key", key.Canonical()),
				).Error("Error posting metric")
				return err
			}
		case <-stream.Context().Done():
			return stream.RecvMsg(nil)
		case <-p.ctx.Done():
			return nil
		}
	}
}

func (p *monitorProvider) TryConnect() (grpc.ClientStream, error) {
	return p.monClient.Stream(p.ctx)
}

func (p *monitorProvider) OnConnected() {
	p.enableWaitRx <- false
}

func (p *monitorProvider) OnLostConnection() {
	p.enableWaitRx <- true
}

func (p *monitorProvider) Target() string {
	return "monitor"
}

func NewMonitorProvider(
	ctx context.Context,
	client types.InternalMonitorClient,
) Provider {
	provider := &monitorProvider{
		ctx:          ctx,
		monClient:    client,
		postQueue:    make(chan KeyedMetric, 10),
		enableWaitRx: make(chan bool),
		lg:           meta.Log(ctx),
	}

	go runWaitReceiver(provider.postQueue,
		provider.enableWaitRx)
	mgr := servers.NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
}

func (p *monitorProvider) Post(metric KeyedMetric) {
	p.postQueue <- metric
}
