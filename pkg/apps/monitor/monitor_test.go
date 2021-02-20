package monitor_test

import (
	"bytes"
	"context"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
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

type testStoreCreator struct {
	count  *atomic.Int32
	stores sync.Map // map[string]monitor.KeyValueStore
}

func (c *testStoreCreator) NewStore(ctx context.Context) monitor.KeyValueStore {
	id, ok := types.IdentityFromContext(ctx)
	if !ok {
		idIncoming, err := types.IdentityFromIncomingContext(ctx)
		if err != nil {
			panic(err)
		}
		id = idIncoming
	}
	store := monitor.InMemoryStoreCreator.NewStore(ctx)
	c.stores.Store(id.UUID, store)
	c.count.Inc()
	return store
}

var _ = Describe("Monitor", func() {
	var listener *bufconn.Listener
	storeCreator := &testStoreCreator{
		stores: sync.Map{},
		count:  atomic.NewInt32(0),
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
			go srv.Serve(listener)
		})
		It("should create a store", func() {
			Eventually(func() int32 {
				return storeCreator.count.Load()
			}).Should(BeEquivalentTo(1))
		})
	})

	When("A provider connects", func() {
		ctx := logkc.NewWithContext(context.Background(), types.Agent)
		ctx = tracing.ContextWithTracer(ctx, opentracing.NoopTracer{})

		var provider *metrics.Provider
		id := types.NewIdentity(types.Agent)
		providerCtx := types.OutgoingContextWithIdentity(ctx, id)
		It("should succeed", func() {
			cc, err := servers.Dial(providerCtx, uuid.NewString(), servers.With(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			provider = metrics.NewProvider(providerCtx, id, cc)
			Expect(provider).NotTo(BeNil())
		})
		It("should create a store", func() {
			Eventually(func() int32 {
				return storeCreator.count.Load()
			}).Should(BeEquivalentTo(2))
		})
		It("should store the provider", func() {
			Eventually(func() bool {
				istore, ok := storeCreator.stores.Load(srvIdentity.UUID)
				if !ok {
					return false
				}
				store, ok := istore.(monitor.KeyValueStore)
				if !ok {
					return false
				}
				providers := &builtin.Providers{
					Items: map[string]int32{
						id.UUID: int32(id.Component),
					},
				}
				expected := tools.EncodeMsgp(providers)
				actual, ok := store.Get(builtin.ProvidersKey)
				if !ok {
					return false
				}
				return bytes.Equal(actual, expected)
			}).Should(BeTrue())
		})
	})
})
