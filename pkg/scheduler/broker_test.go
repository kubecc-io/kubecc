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

package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test/mock_types"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gmeasure"
	"go.uber.org/zap/zapcore"
)

var (
	ctrl  *gomock.Controller
	locks sync.Map
)

func makeCtx(c types.Component) context.Context {
	return meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(c)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(c,
			logkc.WithLogLevel(zapcore.WarnLevel),
		))),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
}

// check if a channel is closed by attempting to close it and recovering
// if close() panicked, indicating the channel was already closed.
// This permanently locks a mutex associated with the channel so that
// closing it will not trigger deferred close statements within the test,
// leading to unintended double-close panics.
func checkStatus(c chan struct{}) (str string) {
	str = "✗"
	defer func() {
		if err := recover(); err != nil {
			str = "✓"
			return
		}
	}()
	lock, _ := locks.LoadOrStore(c, &sync.Mutex{})
	lock.(*sync.Mutex).Lock() // does not unlock
	close(c)
	return
}

// closes the given channel in accordance with the logic in checkStatus
func safeClose(c chan struct{}) {
	lock, _ := locks.LoadOrStore(c, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	close(c)
	lock.(*sync.Mutex).Unlock()
}

var _ = Describe("Broker", func() {
	Specify("stream event order", func() {
		experiment := gmeasure.NewExperiment("stream event order")
		for loop := 0; loop < 100; loop++ {
			ctrl = gomock.NewController(GinkgoT())

			ctx := makeCtx(types.Scheduler)

			incomingCtx1 := makeCtx(types.Agent)
			incomingCtx2 := makeCtx(types.Agent)

			outgoingCtx1 := makeCtx(types.Consumerd)
			outgoingCtx2 := makeCtx(types.Consumerd)

			tcw := mockTcWatcher{
				C: map[string]chan *metrics.Toolchains{
					meta.UUID(incomingCtx1): make(chan *metrics.Toolchains),
					meta.UUID(incomingCtx2): make(chan *metrics.Toolchains),
					meta.UUID(outgoingCtx1): make(chan *metrics.Toolchains),
					meta.UUID(outgoingCtx2): make(chan *metrics.Toolchains),
				},
			}

			broker := NewBroker(ctx, tcw)

			incoming1 := mock_types.NewMockScheduler_StreamIncomingTasksServer(ctrl)
			incoming2 := mock_types.NewMockScheduler_StreamIncomingTasksServer(ctrl)

			outgoing1 := mock_types.NewMockScheduler_StreamOutgoingTasksServer(ctrl)
			outgoing2 := mock_types.NewMockScheduler_StreamOutgoingTasksServer(ctrl)

			incoming1.EXPECT().Context().Return(incomingCtx1).AnyTimes()
			incoming2.EXPECT().Context().Return(incomingCtx2).AnyTimes()
			outgoing1.EXPECT().Context().Return(outgoingCtx1).AnyTimes()
			outgoing2.EXPECT().Context().Return(outgoingCtx2).AnyTimes()

			/*
				Order of operations
				a. Scheduler (outgoing1) calls Recv() and receives sample_req1
				b. Scheduler (incoming1) calls Send(sample_req1)
				c. Scheduler (incoming1) calls Recv() and receives sample_resp1
				d. Scheduler (outgoing1) calls Send(sample_resp1)
			*/

			// onceXY channels ensure Recv functions block the second time they are called
			once1a := make(chan struct{}, 1)
			once1b := make(chan struct{}, 1)
			once2a := make(chan struct{}, 1)
			once2b := make(chan struct{}, 1)
			once1a <- struct{}{}
			once1b <- struct{}{}
			once2a <- struct{}{}
			once2b <- struct{}{}

			// sequence of 4 channels allow mock calls to block until conditions are met
			seq1 := make([]chan struct{}, 4)
			seq2 := make([]chan struct{}, 4)

			for i := 0; i < 4; i++ {
				seq1[i] = make(chan struct{})
				seq2[i] = make(chan struct{})
			}

			// set 1
			outgoing1.EXPECT().Recv().DoAndReturn(func() (*types.CompileRequest, error) {
				<-once1a // block the second time
				defer safeClose(seq1[0])
				return sample_req1, nil
			}).MinTimes(1).MaxTimes(2)

			incoming1.EXPECT().Send(gomock.Eq(sample_req1)).DoAndReturn(func(r *types.CompileRequest) error {
				defer safeClose(seq1[1])
				<-seq1[0] // wait for previous
				return nil
			}).Times(1)

			incoming1.EXPECT().Recv().DoAndReturn(func() (*types.CompileResponse, error) {
				<-once1b // block the second time
				defer safeClose(seq1[2])
				<-seq1[1] // wait for previous
				return sample_resp1, nil
			}).MinTimes(1).MaxTimes(2)

			outgoing1.EXPECT().Send(gomock.Eq(sample_resp1)).DoAndReturn(func(r *types.CompileResponse) error {
				defer safeClose(seq1[3])
				<-seq1[2] // wait for previous
				return nil
			}).Times(1)

			// set 2
			outgoing2.EXPECT().Recv().DoAndReturn(func() (*types.CompileRequest, error) {
				<-once2a // block the second time
				defer safeClose(seq2[0])
				return sample_req2, nil
			}).MinTimes(1).MaxTimes(2)

			incoming2.EXPECT().Send(gomock.Eq(sample_req2)).DoAndReturn(func(r *types.CompileRequest) error {
				defer safeClose(seq2[1])
				<-seq2[0] // wait for previous
				return nil
			}).Times(1)

			incoming2.EXPECT().Recv().DoAndReturn(func() (*types.CompileResponse, error) {
				<-once2b // block the second time
				defer safeClose(seq2[2])
				<-seq2[1] // wait for previous
				return sample_resp2, nil
			}).MinTimes(1).MaxTimes(2)

			outgoing2.EXPECT().Send(gomock.Eq(sample_resp2)).DoAndReturn(func(r *types.CompileResponse) error {
				defer safeClose(seq2[3])
				<-seq2[2] // wait for previous
				return nil
			}).Times(1)

			// Add agents first, they need to be available to process the requests.
			// The broker will not wait for agents to become available if it receives
			// a request it cannot satisfy immediately.

			start := time.Now()

			// Add agent 1
			go func() {
				tcw.C[meta.UUID(incoming1.Context())] <- testAgent1.Toolchains
			}()
			broker.NewAgentTaskStream(incoming1)

			// Add agent 2
			go func() {
				tcw.C[meta.UUID(incoming2.Context())] <- testAgent2.Toolchains
			}()
			broker.NewAgentTaskStream(incoming2)

			// Add consumerds (order here is not important)
			go broker.NewConsumerdTaskStream(outgoing1)
			go broker.NewConsumerdTaskStream(outgoing2)
			go func() {
				tcw.C[meta.UUID(outgoing1.Context())] <- testCd1.Toolchains
			}()
			go func() {
				tcw.C[meta.UUID(outgoing2.Context())] <- testCd2.Toolchains
			}()

			// wait until the final channel of each sequence closes
			wg := &sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-seq1[3]
			}()
			go func() {
				defer wg.Done()
				<-seq2[3]
			}()
			completed := make(chan struct{})
			go func() {
				wg.Wait()
				close(completed)
			}()
			// wait until completed or timed out
			select {
			case <-completed:
				experiment.RecordDuration(
					"scenario duration",
					time.Since(start),
					gmeasure.Precision(time.Microsecond),
				)
			case <-time.After(2 * time.Second):
				Fail(fmt.Sprintf(`-- Timed out --

Event status:

Sequence 1:
[%s] Scheduler (outgoing1) calls Recv() and receives sample_req1
[%s] Scheduler (incoming1) calls Send(sample_req1)
[%s] Scheduler (incoming1) calls Recv() and receives sample_resp1
[%s] Scheduler (outgoing1) calls Send(sample_resp1)
Sequence 2:
[%s] Scheduler (outgoing2) calls Recv() and receives sample_req2
[%s] Scheduler (incoming2) calls Send(sample_req2)
[%s] Scheduler (incoming2) calls Recv() and receives sample_resp2
[%s] Scheduler (outgoing2) calls Send(sample_resp2)`,
					checkStatus(seq1[0]),
					checkStatus(seq1[1]),
					checkStatus(seq1[2]),
					checkStatus(seq1[3]),
					checkStatus(seq2[0]),
					checkStatus(seq2[1]),
					checkStatus(seq2[2]),
					checkStatus(seq2[3])))
			}
			ctrl.Finish() // will print an error if calls didn't match
		}
		AddReportEntry(experiment.Name, experiment)
	})
})
