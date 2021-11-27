package servers_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	"go.uber.org/atomic"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type BenchmarkServer struct {
	test.UnimplementedBenchmarkServer
}

func (s *BenchmarkServer) Stream(stream test.Benchmark_StreamServer) error {
	for {
		select {
		case <-stream.Context().Done():
			return nil
		default:
		}
		payload, err := stream.Recv()
		if err != nil {
			return err
		}
		if err := stream.Send(payload); err != nil {
			return err
		}
	}
}

func doThroughputTest(experiment *gmeasure.Experiment, parallel int) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component,
			meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.ErrorLevel),
		))),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", "127.0.0.1:")
	Expect(err).To(BeNil())
	benchmarkServer := &BenchmarkServer{}
	test.RegisterBenchmarkServer(srv, benchmarkServer)
	go func() {
		defer GinkgoRecover()
		Expect(srv.Serve(listener)).To(Succeed())
	}()
	defer srv.Stop()
	clients := make([]test.BenchmarkClient, parallel)
	locks := make([]*sync.Mutex, parallel)
	streams := make([]test.Benchmark_StreamClient, parallel)
	for i := 0; i < parallel; i++ {
		cc, err := servers.Dial(ctx, listener.Addr().String())
		Expect(err).To(BeNil())
		clients[i] = test.NewBenchmarkClient(cc)
		locks[i] = &sync.Mutex{}
		streams[i], err = clients[i].Stream(ctx, grpc.WaitForReady(true))
		Expect(err).To(BeNil())
	}

	for _, payloadSize := range []int{1, 5, 10} {
		count := atomic.NewInt32(0)
		buf := make([]byte, 1024*1024*payloadSize)
		_, err = io.ReadFull(rand.Reader, buf)
		Expect(err).To(BeNil())

		payload := &test.Payload{
			Data: buf,
		}
		experiment.Sample(func(idx int) {
			defer GinkgoRecover()
			clientIdx := idx % parallel
			locks[clientIdx].Lock()
			defer locks[clientIdx].Unlock()
			stream := streams[clientIdx]
			err := stream.Send(payload)
			if err != nil {
				return
			}
			_, err = stream.Recv()
			if err != nil {
				return
			}
			count.Inc()
		}, gmeasure.SamplingConfig{
			Duration:    2 * time.Second,
			NumParallel: parallel,
		})
		experiment.RecordValue(
			fmt.Sprintf("Throughput (%dMB/Parallel=%d)", payloadSize, parallel),
			float64(count.Load()/2*int32(payloadSize)*2), gmeasure.Units("MB/s"))
	}
}

var _ = Describe("Benchmarking", func() {
	Specify("Benchmark throughput", func() {
		experiment := gmeasure.NewExperiment("Throughput")
		for x := 0; x < 5; x++ {
			wg := sync.WaitGroup{}
			for i := 1; i <= 2; i++ {
				wg.Add(1)
				go func(i int) {
					defer GinkgoRecover()
					defer wg.Done()
					doThroughputTest(experiment, i)
				}(i)
			}
			wg.Wait()
		}
		AddReportEntry(experiment.Name, experiment)
	})
})
