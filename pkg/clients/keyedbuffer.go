/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package clients

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type keyedBufferMonitorProvider struct {
	monitorMetricsProvider
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
) MetricsProvider {
	provider := &keyedBufferMonitorProvider{
		monitorMetricsProvider: monitorMetricsProvider{
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
	mgr := NewStreamManager(ctx, provider)
	go mgr.Run()
	return provider
}

func (p *keyedBufferMonitorProvider) OnConnected() {
	p.enableWaitRx <- false
}

func (p *keyedBufferMonitorProvider) OnLostConnection() {
	p.enableWaitRx <- true
}
