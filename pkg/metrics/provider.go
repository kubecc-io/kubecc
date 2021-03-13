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

type monitorProvider struct {
	ctx           context.Context
	lg            *zap.SugaredLogger
	monClient     types.MonitorClient
	postQueue     chan KeyedMetric
	queueStrategy QueueStrategy
}

type QueueStrategy int

const (
	Buffered QueueStrategy = 1 << iota
	Discard
	Block
)

func NewMonitorProvider(
	ctx context.Context,
	client types.MonitorClient,
	qs QueueStrategy,
) Provider {
	var postQueue chan KeyedMetric
	if (qs & Buffered) != 0 {
		postQueue = make(chan KeyedMetric, 1e6)
	} else {
		postQueue = make(chan KeyedMetric)
	}

	provider := &monitorProvider{
		ctx:       ctx,
		monClient: client,
		postQueue: postQueue,
		lg:        meta.Log(ctx),
	}

	mgr := servers.NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
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
			if mctx, ok := metric.(ContextMetric); ok {
				// The metric has a (presumably cancelable) context
				// If it is canceled, send a deleter to the server
				go func() {
					if mctx.Context() != nil {
						select {
						case <-mctx.Context().Done():
							p.Post(deleter{key: key.Name})
						case <-p.ctx.Done():
						}
					} else {
						<-p.ctx.Done()
					}
				}()
			}
			// Set the value to nil, which deletes the key, if metric is a deleter
			var value *types.Value = nil
			switch metric.(type) {
			case deleter:
			default:
				value = &types.Value{
					Data: util.EncodeMsgp(metric),
				}
			}
			err := stream.SendMsg(&types.Metric{
				Key:   key,
				Value: value,
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

func (p *monitorProvider) Target() string {
	return "monitor"
}

func (p *monitorProvider) Post(metric KeyedMetric) {
	if (p.queueStrategy & Discard) == 0 {
		// Block is the default
		p.postQueue <- metric
		return
	}
	// Discard logic below
	select {
	case p.postQueue <- metric:
		return
	default:
	}
	// If not buffered, do nothing
	if (p.queueStrategy & Buffered) == 0 {
		return
	}
	select {
	case p.postQueue <- metric:
	default:
		p.lg.Debug("Post queue filled, dropping some messages")
		for i := 0; i < cap(p.postQueue)/4; i++ {
			select {
			case <-p.postQueue:
			default:
				// Drain up to 25% of the oldest items, but the channel could be
				// being read from elsewhere, so prevent accidental blocking
				goto done
			}
		}
	done:
		p.postQueue <- metric
	}
}
