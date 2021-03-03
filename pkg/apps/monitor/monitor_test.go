package monitor_test

import (
	"context"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/apps/monitor/test"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var _ = Describe("Monitor", func() {
	var listener *bufconn.Listener
	storeCreator := &test.TestStoreCreator{
		Stores: sync.Map{},
		Count:  atomic.NewInt32(0),
	}
	srvUuid := uuid.NewString()
	When("Creating a monitor server", func() {
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
			meta.WithProvider(identity.UUID, meta.WithValue(srvUuid)),
			meta.WithProvider(logkc.Logger),
			meta.WithProvider(tracing.Tracer),
		)

		It("should succeed", func() {
			mon := monitor.NewMonitorServer(ctx, storeCreator)
			listener = bufconn.Listen(1024 * 1024)
			srv := servers.NewServer(ctx)
			types.RegisterInternalMonitorServer(srv, mon)
			types.RegisterExternalMonitorServer(srv, mon)
			go func() {
				Expect(srv.Serve(listener)).NotTo(HaveOccurred())
			}()
		})
		It("should create a store", func() {
			Eventually(func() int32 {
				return storeCreator.Count.Load()
			}).Should(BeEquivalentTo(1))
		})
	})
	listenerEvents := map[string]chan interface{}{
		"providerAdded":   make(chan interface{}),
		"providerRemoved": make(chan interface{}),
		"testKey1Changed": make(chan interface{}),
		"testKey2Changed": make(chan interface{}),
		"testKey1Expired": make(chan interface{}),
		"testKey2Expired": make(chan interface{}),
	}

	When("A listener connects", func() {
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger),
			meta.WithProvider(tracing.Tracer),
		)

		It("should succeed", func() {
			cc, err := servers.Dial(ctx, uuid.NewString(), servers.With(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			client := types.NewExternalMonitorClient(cc)
			listener := metrics.NewListener(ctx, client)
			listener.OnProviderAdded(func(pctx context.Context, uuid string) {
				listenerEvents["providerAdded"] <- uuid
				listener.OnValueChanged(uuid, func(k1 *test.TestKey1) {
					listenerEvents["testKey1Changed"] <- k1.Counter
				}).OrExpired(func() metrics.RetryOptions {
					listenerEvents["testKey1Expired"] <- struct{}{}
					return metrics.NoRetry
				})
				listener.OnValueChanged(uuid, func(k2 *test.TestKey2) {
					listenerEvents["testKey2Changed"] <- k2.Value
				}).OrExpired(func() metrics.RetryOptions {
					listenerEvents["testKey2Expired"] <- struct{}{}
					return metrics.NoRetry
				})
				<-pctx.Done()
				listenerEvents["providerRemoved"] <- uuid
			})
		})
	})

	var provider metrics.Provider
	var providerCancel context.CancelFunc
	When("A provider connects", func() {
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger),
			meta.WithProvider(tracing.Tracer),
		)
		aaaa := meta.UUID(ctx)
		meta.Log(ctx).Info(aaaa)
		cctx, cancel := context.WithCancel(ctx)
		providerCancel = cancel
		It("should succeed", func() {
			cc, err := servers.Dial(cctx, uuid.NewString(), servers.With(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			client := types.NewInternalMonitorClient(cc)
			provider = metrics.NewMonitorProvider(cctx, client)
			Expect(provider).NotTo(BeNil())
		})
		It("should create a store", func() {
			Eventually(func() int32 {
				return storeCreator.Count.Load()
			}).Should(BeEquivalentTo(2))
		})
		It("should notify the listener", func() {
			Eventually(listenerEvents["providerAdded"]).Should(Receive(Equal(meta.UUID(ctx))))
			Expect(listenerEvents["providerRemoved"]).ShouldNot(Receive())
			// ensure the context is not cancelled and no duplicates occur
			Consistently(listenerEvents["providerAdded"]).ShouldNot(Receive())
			Consistently(listenerEvents["providerRemoved"]).ShouldNot(Receive())
		})
	})
	When("The provider updates a key", func() {
		It("should succeed", func() {
			provider.Post(&test.TestKey1{
				Counter: 1,
			})
		})
		It("should notify the listener", func() {
			Eventually(listenerEvents["testKey1Changed"]).Should(Receive(Equal(1)))
			Expect(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey1Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider updates a different key", func() {
		It("should succeed", func() {
			provider.Post(&test.TestKey2{
				Value: "test",
			})
		})
		It("should notify the other listener", func() {
			Eventually(listenerEvents["testKey2Changed"]).Should(Receive(Equal("test")))
			Expect(listenerEvents["testKey1Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider posts a key with the same value", func() {
		It("should succeed", func() {
			provider.Post(&test.TestKey2{
				Value: "test",
			})
		})
		It("should not notify the listener", func() {
			Consistently(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider exits", func() {
		It("should cancel its context", func() {
			providerCancel()
			Eventually(listenerEvents["providerRemoved"]).Should(Receive())
		})
		It("should expire the corresponding bucket", func() {
			Eventually(listenerEvents["testKey1Expired"]).Should(Receive())
			Eventually(listenerEvents["testKey2Expired"]).Should(Receive())
		})
	})
})
