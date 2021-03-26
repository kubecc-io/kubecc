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
	"errors"
	"sync"

	"github.com/cobalt77/kubecc/pkg/types"
)

var ErrStreamNotReady = errors.New("Stream is not ready yet")

type CompileRequestClient struct {
	ctx     context.Context
	stream  types.Scheduler_StreamOutgoingTasksClient
	pending sync.Map // map[string]chan response
	queue   chan request

	streamLock   *sync.Mutex
	streamActive *sync.Cond
}

func NewCompileRequestClient(
	ctx context.Context,
	stream types.Scheduler_StreamOutgoingTasksClient,
) *CompileRequestClient {
	lock := &sync.Mutex{}
	c := &CompileRequestClient{
		ctx:          ctx,
		stream:       stream,
		queue:        make(chan request),
		streamLock:   lock,
		streamActive: sync.NewCond(lock),
	}
	go c.recvWorker()
	return c
}

// LoadNewStream replaces an existing (or nil) stream with a new one. This
// is used in remote PackagedTasks to allow switching a task to be remote
// or local while it is in queue if the remote state changes. If this was
// not used, a SplitTask would never be able to run remote if the remote
// came online after the task was posted to the queue but before being run.
func (rc *CompileRequestClient) LoadNewStream(
	stream types.Scheduler_StreamOutgoingTasksClient,
) {
	rc.streamLock.Lock()
	rc.stream = stream
	rc.streamActive.Signal()
	rc.streamLock.Unlock()
}

type request struct {
	C       chan response
	Request *types.CompileRequest
}

type response struct {
	Value *types.CompileResponse
	Err   error
}

func (rc *CompileRequestClient) Compile(
	request *types.CompileRequest,
) (interface{}, error) {
	rc.streamLock.Lock()
	if rc.stream == nil {
		rc.streamLock.Unlock()
		return nil, ErrStreamNotReady
	}

	wait := make(chan response)
	defer close(wait)
	err := rc.stream.Send(request)
	rc.streamLock.Unlock()
	if err != nil {
		return nil, err
	}
	rc.pending.Store(request.GetRequestID(), wait)
	select {
	case resp := <-wait:
		return resp.Value, resp.Err
	case <-rc.ctx.Done():
		return nil, rc.ctx.Err()
	}
}

func (rc *CompileRequestClient) recvWorker() {
	for {
		rc.streamLock.Lock()
		for rc.stream == nil {
			rc.streamActive.Wait()
		}
		rc.streamLock.Unlock()

		for {
			resp, err := rc.stream.Recv()
			if err != nil {
				rc.pending.Range(func(key, value interface{}) bool {
					defer rc.pending.Delete(key)
					value.(chan response) <- response{
						Value: nil,
						Err:   err,
					}
					return true
				})
				break
			}
			if ch, ok := rc.pending.LoadAndDelete(resp.GetRequestID()); ok {
				ch.(chan response) <- response{Value: resp}
			}
		}
		rc.streamLock.Lock()
		rc.stream = nil
		rc.streamLock.Unlock()
	}
}
