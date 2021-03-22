package clients

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type keyedBufferMonitorProvider struct {
	monitorProvider
	enableWaitRx chan bool
}

func runWaitReceiver(postQueue chan proto.Message, enableQueue <-chan bool) {
	latestMessages := map[string]proto.Message{}
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
				any, err := anypb.New(m)
				if err != nil {
					panic(err)
				}
				latestMessages[any.GetTypeUrl()] = m
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

func NewKeyedBufferMonitorProvider(
	ctx context.Context,
	client types.MonitorClient,
) metrics.Provider {
	provider := &keyedBufferMonitorProvider{
		monitorProvider: monitorProvider{
			ctx:       ctx,
			lg:        meta.Log(ctx),
			monClient: client,
			// Buffer enough for the WaitReceiver to keep up
			postQueue:     make(chan proto.Message, 10),
			queueStrategy: Block,
		},
		enableWaitRx: make(chan bool),
	}
	go runWaitReceiver(provider.postQueue, provider.enableWaitRx)
	mgr := servers.NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
}

func (p *keyedBufferMonitorProvider) OnConnected() {
	p.enableWaitRx <- false
}

func (p *keyedBufferMonitorProvider) OnLostConnection() {
	p.enableWaitRx <- true
}
