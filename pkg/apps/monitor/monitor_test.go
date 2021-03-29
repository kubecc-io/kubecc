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
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
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

type TestStoreCreator struct {
	Count  *atomic.Int32
	Stores sync.Map // map[string]monitor.KeyValueStore
}

func (c *TestStoreCreator) NewStore(ctx context.Context) monitor.KeyValueStore {
	store := monitor.InMemoryStoreCreator.NewStore(ctx)
	c.Stores.Store(ctx, store)
	i := int32(0)
	c.Stores.Range(func(key, value interface{}) bool {
		i++
		return true
	})
	c.Count.Store(i)
	return store
}

func drain(c chan interface{}) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

// todo: these tests are flaky, figure out why

var _ = Describe("Monitor", func() {
	var listener *bufconn.Listener
	var monitorCtx context.Context
	var storeCreator *TestStoreCreator

	Specify("Monitor server setup", func() {
		storeCreator = &TestStoreCreator{
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
		mon := monitor.NewMonitorServer(monitorCtx, config.MonitorSpec{}, storeCreator)
		listener = bufconn.Listen(1024 * 1024)
		srv := servers.NewServer(monitorCtx, servers.WithServerOpts(
			grpc.NumStreamWorkers(24),
		))
		types.RegisterMonitorServer(srv, mon)
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

	bug := atomic.NewBool(true)
	var providerUuid string

	When("A listener connects", func() {
		It("should succeed", func() {
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.CLI,
					logkc.WithLogLevel(zapcore.WarnLevel)))),
				meta.WithProvider(tracing.Tracer),
			)
			cc, err := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			mc := types.NewMonitorClient(cc)
			listener := clients.NewMetricsListener(ctx, mc)
			listener.OnProviderAdded(func(pctx context.Context, uuid string) {
				if uuid != providerUuid {
					return
				}
				listenerEvents["providerAdded"] <- uuid
				listener.OnValueChanged(uuid, func(k1 *testutil.Test1) {
					listenerEvents["testKey1Changed"] <- k1.Counter
				}).OrExpired(func() clients.RetryOptions {
					listenerEvents["testKey1Expired"] <- struct{}{}
					return clients.NoRetry
				})
				listener.OnValueChanged(uuid, func(k2 *testutil.Test2) {
					listenerEvents["testKey2Changed"] <- k2.Value
				}).OrExpired(func() clients.RetryOptions {
					listenerEvents["testKey2Expired"] <- struct{}{}
					return clients.NoRetry
				})
				<-pctx.Done()
				if bug.Load() {
					// ! Put a breakpoint here to catch the GC issue
					testLog.Warn("pctx done " + uuid)
				}
				listenerEvents["providerRemoved"] <- uuid
			})
		})
	})

	var provider clients.MetricsProvider
	var providerCancel context.CancelFunc
	When("A provider connects", func() {
		It("should succeed", func() {
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
			cc, err := servers.Dial(cctx, uuid.NewString(), servers.WithDialOpts(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			mc := types.NewMonitorClient(cc)
			provider = clients.NewMetricsProvider(cctx, mc, clients.Buffered)
			Expect(provider).NotTo(BeNil())
		})
		It("should create a store", func() {
			Eventually(func() int32 {
				return storeCreator.Count.Load()
			}).Should(BeEquivalentTo(2))
		})
		It("should notify the listener", func() {
			Eventually(listenerEvents["providerAdded"]).Should(Receive(Equal(providerUuid)))
			Expect(listenerEvents["providerRemoved"]).NotTo(Receive())
			// ensure the context is not canceled and no duplicates occur
			Consistently(listenerEvents["providerAdded"]).ShouldNot(Receive())
			Consistently(listenerEvents["providerRemoved"]).ShouldNot(Receive())
		})
	})
	When("The provider updates a key", func() {
		It("should succeed", func() {
			provider.Post(&testutil.Test1{
				Counter: 1,
			})
		})
		It("should notify the listener", func() {
			Eventually(listenerEvents["testKey1Changed"]).Should(Receive(Equal(int32(1))))
			Expect(listenerEvents["testKey2Changed"]).ShouldNot(Receive())
			Consistently(listenerEvents["testKey1Changed"]).ShouldNot(Receive())
		})
	})
	When("A late-joining listener connects", func() {
		It("should be notified of existing data", func() {
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.CLI,
					logkc.WithLogLevel(zapcore.WarnLevel)))),
				meta.WithProvider(tracing.Tracer),
			)
			cc, err := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
				grpc.WithContextDialer(
					func(context.Context, string) (net.Conn, error) {
						return listener.Dial()
					}),
			))
			Expect(err).NotTo(HaveOccurred())
			mc := types.NewMonitorClient(cc)
			listener := clients.NewMetricsListener(ctx, mc)
			listener.OnProviderAdded(func(pctx context.Context, uuid string) {
				if uuid != providerUuid {
					return
				}
				lateJoinListenerEvents["providerAdded"] <- uuid
				listener.OnValueChanged(uuid, func(k1 *testutil.Test1) {
					lateJoinListenerEvents["testKey1Changed"] <- k1.Counter
				}).OrExpired(func() clients.RetryOptions {
					lateJoinListenerEvents["testKey1Expired"] <- struct{}{}
					return clients.NoRetry
				})
				<-pctx.Done()
				lateJoinListenerEvents["providerRemoved"] <- struct{}{}
			})
			Eventually(lateJoinListenerEvents["providerAdded"]).Should(Receive(Equal(providerUuid)))
			Eventually(lateJoinListenerEvents["testKey1Changed"]).Should(Receive(Equal(int32(1))))
		})
	})
	When("The provider updates a different key", func() {
		It("should succeed", func() {
			provider.Post(&testutil.Test2{
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
			provider.Post(&testutil.Test2{
				Value: "test",
			})
			provider.Post(&testutil.Test1{
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
			bug.Toggle() // ! Enables GC issue breakpoint
			Eventually(listenerEvents["providerRemoved"]).Should(Receive())
			Eventually(lateJoinListenerEvents["providerRemoved"]).Should(Receive())
		})
		It("should expire the corresponding bucket", func() {
			Eventually(listenerEvents["testKey1Expired"]).Should(Receive())
			Eventually(lateJoinListenerEvents["testKey1Expired"]).Should(Receive())
			Eventually(listenerEvents["testKey2Expired"]).Should(Receive())
		})
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
		providers := make([]clients.MetricsProvider, numProviders)
		listeners := make([]clients.MetricsListener, numListenersPerKey*4)
		uuids := map[string]struct{}{}
		totals := []*atomic.Int32{
			atomic.NewInt32(0),
			atomic.NewInt32(0),
			atomic.NewInt32(0),
			atomic.NewInt32(0),
		}
		handlers := []interface{}{
			func(k *testutil.Test1) {
				totals[0].Inc()
			},
			func(k *testutil.Test2) {
				totals[1].Inc()
			},
			func(k *testutil.Test3) {
				totals[2].Inc()
			},
			func(k *testutil.Test4) {
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
				uuids[meta.UUID(ctx)] = struct{}{}
				cc, _ := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
					grpc.WithContextDialer(
						func(context.Context, string) (net.Conn, error) {
							return listener.Dial()
						}),
				))
				mc := types.NewMonitorClient(cc)
				provider := clients.NewMetricsProvider(ctx, mc, clients.Buffered)
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
			mc := types.NewMonitorClient(cc)
			l := clients.NewMetricsListener(ctx, mc)
			listeners[sampleIdx] = l
			handler := handlers[sampleIdx%4]
			testCh := make(chan struct{}, len(providers))
			l.OnProviderAdded(func(ctx context.Context, uuid string) {
				if _, ok := uuids[uuid]; !ok {
					return
				}
				testCh <- struct{}{}
				l.OnValueChanged(uuid, handler)
				<-ctx.Done()
			})
			Eventually(func() int {
				return len(testCh)
			}, 10*time.Second, 1*time.Millisecond).Should(Equal(len(providers)))
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
						providers[i%len(providers)].Post(&testutil.Test1{Counter: int32(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 2 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&testutil.Test2{Value: fmt.Sprint(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 3 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&testutil.Test3{Counter: int32(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 4 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&testutil.Test4{Value: fmt.Sprint(i)})
					}
				})
			}()
			total := int32(numUpdatesPerKey * numListenersPerKey)
			var wg sync.WaitGroup
			wg.Add(4)
			for i := 0; i < 4; i++ {
				go func(i int) {
					defer GinkgoRecover()
					defer wg.Done()
					b.Time(fmt.Sprintf("%d key %d callbacks", total, i+1), func() {
						timeout := time.NewTimer(callbackTimeout)
						for totals[i].Load() < total {
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
