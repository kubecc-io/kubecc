package clients

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type monitorProvider struct {
	ctx               context.Context
	lg                *zap.SugaredLogger
	monClient         types.MonitorClient
	postQueue         chan proto.Message
	queueStrategy     QueueStrategy
	metricCtxMap      map[string]context.CancelFunc
	metricCtxMapMutex *sync.Mutex
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
) metrics.Provider {
	var postQueue chan proto.Message
	if (qs & Buffered) != 0 {
		postQueue = make(chan proto.Message, 1e6)
	} else {
		postQueue = make(chan proto.Message)
	}

	provider := &monitorProvider{
		ctx:               ctx,
		monClient:         client,
		postQueue:         postQueue,
		lg:                meta.Log(ctx),
		metricCtxMap:      make(map[string]context.CancelFunc),
		metricCtxMapMutex: &sync.Mutex{},
	}

	mgr := servers.NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
}

func (p *monitorProvider) HandleStream(stream grpc.ClientStream) error {
	for {
		select {
		case metric := <-p.postQueue:
			any, err := anypb.New(metric)
			key := &types.Key{
				Bucket: meta.UUID(p.ctx),
				Name:   any.GetTypeUrl(),
			}
			p.lg.With(
				types.ShortID(key.ShortID()),
			).Debug("Posting metric")
			err = stream.SendMsg(&types.Metric{
				Key:   key,
				Value: any,
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

func (p *monitorProvider) PostContext(metric proto.Message, ctx context.Context) {
	p.Post(metric)
	p.metricCtxMapMutex.Lock()
	defer p.metricCtxMapMutex.Unlock()
	any, err := anypb.New(metric)
	if err != nil {
		panic(err)
	}
	key := any.GetTypeUrl()
	if cancel, ok := p.metricCtxMap[key]; ok {
		cancel()
	}
	localCtx, cancel := context.WithCancel(ctx)
	p.metricCtxMap[key] = cancel
	// When the context is done, send a deleter to the server
	go func() {
		defer func() {
			p.metricCtxMapMutex.Lock()
			defer p.metricCtxMapMutex.Unlock()
			delete(p.metricCtxMap, key)
		}()
		select {
		case <-ctx.Done():
			p.Post(&metrics.Deleter{
				Key: key,
			})
		case <-localCtx.Done(): // the map context
			return
		case <-p.ctx.Done():
		}
	}()
}

func (p *monitorProvider) Post(metric proto.Message) {
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
