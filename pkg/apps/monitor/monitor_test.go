package monitor_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
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
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func drain(c chan interface{}) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

func recycle(c chan context.CancelFunc) {
	for {
		select {
		case f := <-c:
			f()
		default:
			return
		}
	}
}

var _ = Describe("Monitor", func() {
	var listener *bufconn.Listener
	var monitorCtx context.Context
	var storeCreator *test.TestStoreCreator

	Specify("Monitor server setup", func() {
		storeCreator = &test.TestStoreCreator{
			Stores: sync.Map{},
			Count:  atomic.NewInt32(0),
		}
		srvUuid := uuid.NewString()
		monitorCtx = meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
			meta.WithProvider(identity.UUID, meta.WithValue(srvUuid)),
			meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.Monitor,
				logkc.WithLogLevel(zapcore.WarnLevel)))),
			meta.WithProvider(tracing.Tracer),
		)
		mon := monitor.NewMonitorServer(monitorCtx, storeCreator)
		listener = bufconn.Listen(1024 * 1024)
		srv := servers.NewServer(monitorCtx, servers.WithServerOpts(
			grpc.NumStreamWorkers(12),
		))
		types.RegisterInternalMonitorServer(srv, mon)
		types.RegisterExternalMonitorServer(srv, mon)
		go func() {
			Expect(srv.Serve(listener)).NotTo(HaveOccurred())
		}()
		Eventually(func() int32 {
			return storeCreator.Count.Load()
		}).Should(BeEquivalentTo(1))
	})

	listenerEvents := map[string]chan interface{}{
		"providerAdded":   make(chan interface{}, 100),
		"providerRemoved": make(chan interface{}, 100),
		"testKey1Changed": make(chan interface{}, 100),
		"testKey2Changed": make(chan interface{}, 100),
		"testKey1Expired": make(chan interface{}, 100),
		"testKey2Expired": make(chan interface{}, 100),
	}

	lateJoinListenerEvents := map[string]chan interface{}{
		"providerAdded":   make(chan interface{}, 100),
		"providerRemoved": make(chan interface{}, 100),
		"testKey1Changed": make(chan interface{}, 100),
		"testKey1Expired": make(chan interface{}, 100),
	}

	When("A listener connects", func() {
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.CLI,
				logkc.WithLogLevel(zapcore.WarnLevel)))),
			meta.WithProvider(tracing.Tracer),
		)

		It("should succeed", func() {
			cc, err := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
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
	var providerUuid string
	When("A provider connects", func() {
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.Agent,
				logkc.WithLogLevel(zapcore.WarnLevel)))),
			meta.WithProvider(tracing.Tracer),
		)
		providerUuid = meta.UUID(ctx)
		cctx, cancel := context.WithCancel(ctx)
		providerCancel = cancel
		It("should succeed", func() {
			cc, err := servers.Dial(cctx, uuid.NewString(), servers.WithDialOpts(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			client := types.NewInternalMonitorClient(cc)
			provider = metrics.NewMonitorProvider(cctx, client, metrics.Buffered|metrics.Block)
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
			// ensure the context is not canceled and no duplicates occur
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
	When("A late-joining listener connects", func() {
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.CLI,
				logkc.WithLogLevel(zapcore.WarnLevel)))),
			meta.WithProvider(tracing.Tracer),
		)
		It("should be notified of existing data", func() {
			cc, err := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			client := types.NewExternalMonitorClient(cc)
			listener := metrics.NewListener(ctx, client)
			listener.OnProviderAdded(func(pctx context.Context, uuid string) {
				lateJoinListenerEvents["providerAdded"] <- uuid
				listener.OnValueChanged(uuid, func(k1 *test.TestKey1) {
					lateJoinListenerEvents["testKey1Changed"] <- k1.Counter
				}).OrExpired(func() metrics.RetryOptions {
					lateJoinListenerEvents["testKey1Expired"] <- struct{}{}
					return metrics.NoRetry
				})
				<-pctx.Done()
				lateJoinListenerEvents["providerRemoved"] <- struct{}{}
			})
			Eventually(lateJoinListenerEvents["providerAdded"]).Should(Receive(Equal(providerUuid)))
			Eventually(lateJoinListenerEvents["testKey1Changed"]).Should(Receive(Equal(1)))
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
			Expect(lateJoinListenerEvents["testKey1Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider posts a key with the same value", func() {
		It("should succeed", func() {
			provider.Post(&test.TestKey2{
				Value: "test",
			})
			provider.Post(&test.TestKey1{
				Counter: 1,
			})
		})
		It("should not notify the listener", func() {
			Consistently(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey1Changed"]).ShouldNot(Receive())
			Consistently(lateJoinListenerEvents["testKey1Changed"]).ShouldNot(Receive())
		})
	})
	When("The provider exits", func() {
		It("should cancel its context", func() {
			providerCancel()
			Eventually(listenerEvents["providerRemoved"]).Should(Receive())
			Eventually(lateJoinListenerEvents["providerRemoved"]).Should(Receive())
		})
		It("should expire the corresponding bucket", func() {
			Eventually(listenerEvents["testKey1Expired"]).Should(Receive())
			Eventually(lateJoinListenerEvents["testKey1Expired"]).Should(Receive())
			Eventually(listenerEvents["testKey2Expired"]).Should(Receive())
		})
		for _, c := range listenerEvents {
			drain(c)
		}
		for _, c := range lateJoinListenerEvents {
			drain(c)
		}
	})
	Context("Stress Test", func() {
		numProviders := 2
		numListenersPerKey := 10
		numUpdatesPerKey := 1000
		callbackTimeout := 10 * time.Second
		stressTestLoops := 5
		if testutil.IsRaceDetectorEnabled() {
			numListenersPerKey = 10
			numUpdatesPerKey = 100
			stressTestLoops = 3
		}
		providers := make([]metrics.Provider, numProviders)
		listeners := make([]metrics.Listener, numListenersPerKey*4)
		totals := []*atomic.Int32{
			atomic.NewInt32(0),
			atomic.NewInt32(0),
			atomic.NewInt32(0),
			atomic.NewInt32(0),
		}
		handlers := []interface{}{
			func(k *test.TestKey1) {
				totals[0].Inc()
			},
			func(k *test.TestKey2) {
				totals[1].Inc()
			},
			func(k *test.TestKey3) {
				totals[2].Inc()
			},
			func(k *test.TestKey4) {
				totals[3].Inc()
			},
		}

		Specify("Creating providers", func() {
			testutil.SkipInGithubWorkflow()
			for i := 0; i < numProviders; i++ {
				ctx := meta.NewContext(
					meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
					meta.WithProvider(identity.UUID),
					meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.Agent,
						logkc.WithLogLevel(zapcore.ErrorLevel)))),
					meta.WithProvider(tracing.Tracer),
				)
				cc, _ := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
					grpc.WithContextDialer(
						func(context.Context, string) (net.Conn, error) {
							return listener.Dial()
						}),
				))
				client := types.NewInternalMonitorClient(cc)
				provider := metrics.NewMonitorProvider(ctx, client, metrics.Buffered)
				providers[i] = provider
			}
		})
		sampleIdx := 0
		Measure("Creating listeners for each key", func(b Benchmarker) {
			testutil.SkipInGithubWorkflow()
			defer func() {
				sampleIdx++
			}()
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.CLI,
					logkc.WithLogLevel(zapcore.ErrorLevel)))),
				meta.WithProvider(tracing.Tracer),
			)
			cc, _ := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			client := types.NewExternalMonitorClient(cc)
			l := metrics.NewListener(ctx, client)
			listeners[sampleIdx] = l
			handler := handlers[sampleIdx%4]
			b.Time("Handling provider add callbacks", func() {
				testCh := make(chan struct{}, len(providers))
				l.OnProviderAdded(func(ctx context.Context, uuid string) {
					testCh <- struct{}{}
					l.OnValueChanged(uuid, handler)
					<-ctx.Done()
				})
				Eventually(func() int {
					return len(testCh)
				}, 10*time.Second, 1*time.Millisecond).Should(Equal(len(providers)))
			})
		}, len(listeners)) // This is the loop
		Measure("Updating keys rapidly for each provider", func(b Benchmarker) {
			testutil.SkipInGithubWorkflow()
			if testutil.IsRaceDetectorEnabled() {
				testLog.Warn("Race detector enabled: Data volume limited to 10%")
			}
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 1 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.TestKey1{Counter: i})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 2 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.TestKey2{Value: fmt.Sprint(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 3 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.TestKey3{Counter: i})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 4 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.TestKey4{Value: fmt.Sprint(i)})
					}
				})
			}()
			total := int32(numUpdatesPerKey * numListenersPerKey)
			var wg sync.WaitGroup
			wg.Add(4)
			for i := 0; i < 4; i++ {
				go func(j int) {
					defer GinkgoRecover()
					defer wg.Done()
					b.Time(fmt.Sprintf("%d key %d callbacks", total, j+1), func() {
						timeout := time.NewTimer(callbackTimeout)
						for totals[j].Load() < total {
							select {
							case <-timeout.C:
								return
							default:
							}
							time.Sleep(10 * time.Millisecond)
						}
					})
				}(i)
			}
			wg.Wait()
			Expect([]int32{
				totals[0].Swap(0),
				totals[1].Swap(0),
				totals[2].Swap(0),
				totals[3].Swap(0),
			}).To(Equal([]int32{total, total, total, total}))
		}, stressTestLoops)
	})
})
