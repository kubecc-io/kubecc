package monitor_test

import (
	"bytes"
	"context"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/apps/monitor/test"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/builtin"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opentracing/opentracing-go"
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
	srvIdentity := types.NewIdentity(types.TestComponent)
	When("Creating a monitor server", func() {
		ctx := logkc.NewWithContext(context.Background(), types.TestComponent)
		ctx = tracing.ContextWithTracer(ctx, opentracing.NoopTracer{})
		ctx = types.ContextWithIdentity(ctx, srvIdentity)

		It("should succeed", func() {
			mon := monitor.NewMonitorServer(ctx, storeCreator)
			listener = bufconn.Listen(1024 * 1024)
			srv := servers.NewServer(ctx)
			types.RegisterMonitorServer(srv, mon)
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
		ctx := logkc.NewWithContext(context.Background(), types.CLI)
		ctx = tracing.ContextWithTracer(ctx, opentracing.NoopTracer{})
		id := types.NewIdentity(types.CLI)
		listenerCtx := types.OutgoingContextWithIdentity(ctx, id)

		It("should succeed", func() {
			cc, err := servers.Dial(listenerCtx, uuid.NewString(), servers.With(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())

			listener := metrics.NewListener(listenerCtx, cc)
			listener.OnProviderAdded(func(pctx context.Context, uuid string) {
				listenerEvents["providerAdded"] <- uuid
				listener.OnValueChanged(uuid, func(k1 *test.TestKey1) {
					listenerEvents["testKey1Changed"] <- k1.Counter
				}).OrExpired(func() {
					listenerEvents["testKey1Expired"] <- struct{}{}
				})
				listener.OnValueChanged(uuid, func(k2 *test.TestKey2) {
					listenerEvents["testKey2Changed"] <- k2.Value
				}).OrExpired(func() {
					listenerEvents["testKey2Expired"] <- struct{}{}
				})
				<-pctx.Done()
				listenerEvents["providerRemoved"] <- uuid
			})
		})
	})

	var provider *metrics.Provider
	var providerCancel context.CancelFunc
	providerId := types.NewIdentity(types.Agent)
	When("A provider connects", func() {
		ctx := logkc.NewWithContext(context.Background(), types.Agent)
		ctx = tracing.ContextWithTracer(ctx, opentracing.NoopTracer{})

		providerCtx, cancel := context.WithCancel(
			types.OutgoingContextWithIdentity(ctx, providerId))
		providerCancel = cancel
		It("should succeed", func() {
			cc, err := servers.Dial(providerCtx, uuid.NewString(), servers.With(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			provider = metrics.NewProvider(providerCtx, providerId, cc)
			Expect(provider).NotTo(BeNil())
		})
		It("should create a store", func() {
			Eventually(func() int32 {
				return storeCreator.Count.Load()
			}).Should(BeEquivalentTo(2))
		})
		It("should store the provider", func() {
			Eventually(func() bool {
				istore, ok := storeCreator.Stores.Load(srvIdentity.UUID)
				if !ok {
					return false
				}
				store, ok := istore.(monitor.KeyValueStore)
				if !ok {
					return false
				}
				providers := &builtin.Providers{
					Items: map[string]int32{
						providerId.UUID: int32(providerId.Component),
					},
				}
				expected := tools.EncodeMsgp(providers)
				actual, ok := store.Get(builtin.Providers{}.Key())
				if !ok {
					return false
				}
				return bytes.Equal(actual, expected)
			}).Should(BeTrue())
		})

		It("should notify the listener", func() {
			Eventually(listenerEvents["providerAdded"]).Should(Receive(Equal(providerId.UUID)))
			Expect(listenerEvents["providerRemoved"]).ShouldNot(Receive())
			// ensure the context is not cancelled and no duplicates occur
			Consistently(listenerEvents["providerAdded"]).ShouldNot(Receive())
			Consistently(listenerEvents["providerRemoved"]).ShouldNot(Receive())
		})
	})
	When("The provider updates a key", func() {
		It("should succeed", func() {
			ok := provider.Post(&types.Key{
				Bucket: providerId.UUID,
				Name:   test.TestKey1{}.Key(),
			}, &test.TestKey1{
				Counter: 1,
			})
			Expect(ok).To(BeTrue())
		})
		It("should notify the listener", func() {
			Eventually(listenerEvents["testKey1Changed"]).Should(Receive(Equal(1)))
			Expect(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey1Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider updates a different key", func() {
		It("should succeed", func() {
			ok := provider.Post(&types.Key{
				Bucket: providerId.UUID,
				Name:   test.TestKey2{}.Key(),
			}, &test.TestKey2{
				Value: "test",
			})
			Expect(ok).To(BeTrue())
		})
		It("should notify the other listener", func() {
			Eventually(listenerEvents["testKey2Changed"]).Should(Receive(Equal("test")))
			Expect(listenerEvents["testKey1Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider posts a key with the same value", func() {
		It("should succeed", func() {
			ok := provider.Post(&types.Key{
				Bucket: providerId.UUID,
				Name:   test.TestKey2{}.Key(),
			}, &test.TestKey2{
				Value: "test",
			})
			Expect(ok).To(BeTrue())
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
