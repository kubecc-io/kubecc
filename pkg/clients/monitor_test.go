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

package clients_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/apps/monitor"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
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

var _ = Describe("Monitor Clients", func() {
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
	providerUuid := atomic.NewString("")

	Context("Functionality", func() {
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
					if uuid != providerUuid.Load() {
						return
					}
					listenerEvents["providerAdded"] <- uuid
					listener.OnValueChanged(uuid, func(k1 *test.Test1) {
						listenerEvents["testKey1Changed"] <- k1.Counter
					}).OrExpired(func() clients.RetryOptions {
						listenerEvents["testKey1Expired"] <- struct{}{}
						return clients.NoRetry
					})
					listener.OnValueChanged(uuid, func(k2 *test.Test2) {
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
				providerUuid.Store(meta.UUID(ctx))
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
				Eventually(listenerEvents["providerAdded"]).Should(Receive(Equal(providerUuid.Load())))
				Expect(listenerEvents["providerRemoved"]).NotTo(Receive())
				// ensure the context is not canceled and no duplicates occur
				Consistently(listenerEvents["providerAdded"]).ShouldNot(Receive())
				Consistently(listenerEvents["providerRemoved"]).ShouldNot(Receive())
			})
		})
		When("The provider updates a key", func() {
			It("should succeed", func() {
				provider.Post(&test.Test1{
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
					if uuid != providerUuid.Load() {
						return
					}
					lateJoinListenerEvents["providerAdded"] <- uuid
					listener.OnValueChanged(uuid, func(k1 *test.Test1) {
						lateJoinListenerEvents["testKey1Changed"] <- k1.Counter
					}).OrExpired(func() clients.RetryOptions {
						lateJoinListenerEvents["testKey1Expired"] <- struct{}{}
						return clients.NoRetry
					})
					<-pctx.Done()
					lateJoinListenerEvents["providerRemoved"] <- struct{}{}
				})
				Eventually(lateJoinListenerEvents["providerAdded"]).Should(Receive(Equal(providerUuid.Load())))
				Eventually(lateJoinListenerEvents["testKey1Changed"]).Should(Receive(Equal(int32(1))))
			})
		})
		When("The provider updates a different key", func() {
			It("should succeed", func() {
				provider.Post(&test.Test2{
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
				provider.Post(&test.Test2{
					Value: "test",
				})
				provider.Post(&test.Test1{
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
	})
	Context("Performance", func() {
		numProviders := 2
		numListenersPerKey := 10
		numUpdatesPerKey := 1000
		callbackTimeout := 10 * time.Second
		perfTestLoops := 5
		if test.IsRaceDetectorEnabled() {
			numListenersPerKey = 10
			numUpdatesPerKey = 100
			perfTestLoops = 3
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
			func(k *test.Test1) {
				totals[0].Inc()
			},
			func(k *test.Test2) {
				totals[1].Inc()
			},
			func(k *test.Test3) {
				totals[2].Inc()
			},
			func(k *test.Test4) {
				totals[3].Inc()
			},
		}

		cancels := []context.CancelFunc{}
		Specify("Creating providers", func() {
			test.SkipInGithubWorkflow()
			for i := 0; i < numProviders; i++ {
				ctx := meta.NewContext(
					meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
					meta.WithProvider(identity.UUID),
					meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.Agent,
						logkc.WithLogLevel(zapcore.ErrorLevel)))),
					meta.WithProvider(tracing.Tracer),
				)
				ctx, providerCancel := context.WithCancel(ctx)
				cancels = append(cancels, providerCancel)
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
		allActiveListeners := atomic.NewInt32(0)
		Measure("Creating listeners for each key", func(b Benchmarker) {
			test.SkipInGithubWorkflow()
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
			activeListeners := atomic.NewInt32(0)
			l.OnProviderAdded(func(ctx context.Context, uuid string) {
				if _, ok := uuids[uuid]; !ok {
					return
				}
				activeListeners.Inc()
				defer activeListeners.Dec()
				allActiveListeners.Inc()
				defer allActiveListeners.Dec()
				l.OnValueChanged(uuid, handler)
				<-ctx.Done()
			})
			Eventually(func() int {
				return int(activeListeners.Load())
			}, 5*time.Second, 10*time.Millisecond).Should(Equal(len(providers)))
		}, len(listeners)) // This is the loop
		Measure("Updating keys rapidly for each provider", func(b Benchmarker) {
			test.SkipInGithubWorkflow()
			if test.IsRaceDetectorEnabled() {
				testLog.Warn("Race detector enabled: Data volume limited to 10%")
			}
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 1 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.Test1{Counter: int32(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 2 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.Test2{Value: fmt.Sprint(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 3 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.Test3{Counter: int32(i)})
					}
				})
			}()
			go func() {
				defer GinkgoRecover()
				b.Time(fmt.Sprintf("%d Key 4 updates", numUpdatesPerKey), func() {
					for i := 0; i < numUpdatesPerKey; i++ {
						providers[i%len(providers)].Post(&test.Test4{Value: fmt.Sprint(i)})
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
		}, perfTestLoops)
		When("the provider is canceled", func() {
			It("should cancel associated listeners", func() {
				for _, cancel := range cancels {
					cancel()
				}
				Eventually(func() int {
					return int(allActiveListeners.Load())
				}, 5*time.Second, 10*time.Millisecond).Should(Equal(0))
			})
		})
	})
	Context("Connection", func() {
		var testEnv *test.Environment
		addAgent := make(chan struct{}, 1)
		removeAgent := make(chan struct{}, 1)
		addCd := make(chan struct{}, 1)
		removeCd := make(chan struct{}, 1)
		Specify("setup", func() {
			testEnv = test.NewEnvironmentWithLogLevel(zapcore.ErrorLevel)
			testEnv.SpawnMonitor(test.WaitForReady())
		})

		It("should handle disconnect/reconnect", func() {
			// very important that this context is not the environment's context
			// this is because WaitForReady will create a listener under the
			// environment's context, and when it is done, all listeners under
			// the environment's context will be closed, including this one.
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.Agent,
					logkc.WithLogLevel(zapcore.ErrorLevel)))),
				meta.WithProvider(tracing.Tracer),
			)
			client := testEnv.NewMonitorClient(ctx)
			l := clients.NewMetricsListener(ctx, client)
			l.OnProviderAdded(func(c context.Context, s string) {
				whois, err := client.Whois(ctx, &types.WhoisRequest{
					UUID: s,
				})
				Expect(err).NotTo(HaveOccurred())
				if whois.Component != types.Agent {
					return
				}
				addAgent <- struct{}{}
				<-c.Done()
				removeAgent <- struct{}{}
			})
			l.OnProviderAdded(func(c context.Context, s string) {
				whois, err := client.Whois(ctx, &types.WhoisRequest{
					UUID: s,
				})
				Expect(err).NotTo(HaveOccurred())
				if whois.Component != types.Consumerd {
					return
				}
				addCd <- struct{}{}
				<-c.Done()
				removeCd <- struct{}{}
			})

			randomOrder := func(a, b func()) {
				if rand.Int31()%2 == 0 {
					a()
					b()
				} else {
					b()
					a()
				}
			}
			for i := 0; i < 10; i++ {
				var caAgent context.CancelFunc
				var caConsumerd context.CancelFunc
				randomOrder(
					func() {
						_, caAgent = testEnv.SpawnAgent(test.WaitForReady())
					}, func() {
						_, caConsumerd = testEnv.SpawnConsumerd(test.WaitForReady())
					},
				)
				randomOrder(
					func() {
						Eventually(addAgent).Should(Receive())
					},
					func() {
						Eventually(addCd).Should(Receive())
					},
				)
				randomOrder(
					func() {
						caAgent()
					},
					func() {
						caConsumerd()
					},
				)
				randomOrder(
					func() {
						Eventually(removeAgent).Should(Receive())
					},
					func() {
						Eventually(removeCd).Should(Receive())
					},
				)
			}
		})
	})
})
