package scheduler

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testCd1 = &Consumerd{
		remoteInfo: remoteInfo{
			Context: context.Background(),
			UUID:    uuid.NewString(),
		},
		Toolchains: &metrics.Toolchains{
			Items: []*types.Toolchain{
				clang_c,
			},
		},
		RWMutex: &sync.RWMutex{},
	}
	testCd2 = &Consumerd{
		remoteInfo: remoteInfo{
			Context: context.Background(),
			UUID:    uuid.NewString(),
		},
		Toolchains: &metrics.Toolchains{
			Items: []*types.Toolchain{
				gnu_c,
			},
		},
		RWMutex: &sync.RWMutex{},
	}
	testAgent1 = &Agent{
		remoteInfo: remoteInfo{
			Context: context.Background(),
			UUID:    uuid.NewString(),
		},
		Toolchains: &metrics.Toolchains{
			Items: []*types.Toolchain{
				clang_c,
			},
		},
	}
	testAgent2 = &Agent{
		remoteInfo: remoteInfo{
			Context: context.Background(),
			UUID:    uuid.NewString(),
		},
		Toolchains: &metrics.Toolchains{
			Items: []*types.Toolchain{
				gnu_c,
			},
		},
	}
)

var _ = Describe("Router", func() {
	Context("Basic functionality", func() {
		router := NewRouter(testCtx)
		When("a router is created", func() {
			It("should be empty", func() {
				Expect(router.senders).To(BeEmpty())
				Expect(router.receivers).To(BeEmpty())
				Expect(router.routes).To(BeEmpty())
			})
		})
		When("a sender is added", func() {
			var rt *route
			It("should succeed", func() {
				router.AddSender(testCd1)
				Expect(len(router.routes)).To(Equal(1))
				Expect(len(router.senders)).To(Equal(1))
			})
			It("should add a new task channel", func() {
				rt = router.routes[tcHash(clang_c)]
				ch2 := router.routeForToolchain(clang_c)
				Expect(ch2).To(Equal(rt)) // checking pointer here
				Expect(len(router.routes)).To(Equal(1))
				Eventually(rt.txRefCount.Load).Should(BeEquivalentTo(1))
				Eventually(rt.rxRefCount.Load).Should(BeEquivalentTo(0))
			})
			It("should not be able to send on that channel", func() {
				Expect(rt.CanSend()).To(BeFalse())
				err := router.Send(context.Background(), sample_req1)
				Expect(err).To(MatchError(ErrNoAgents))
			})
		})
		When("sending an invalid request", func() {
			It("should return the correct errors", func() {
				err := router.Send(context.Background(), &types.CompileRequest{})
				Expect(err).To(MatchError(ErrInvalidToolchain))
			})
		})
		When("adding a receiver", func() {
			var rx <-chan request
			It("should succeed", func() {
				rx = router.AddReceiver(testAgent1)
				Expect(len(router.routes)).To(Equal(1))
				Expect(len(router.senders)).To(Equal(1))
				Expect(len(router.receivers)).To(Equal(1))
			})
			It("should be able to send on that channel", func() {
				rt := router.routeForToolchain(clang_c)
				Eventually(rt.CanSend).Should(BeTrue())
				Expect(router.Send(context.Background(), sample_req1)).To(Succeed())
				Eventually(rx).Should(Receive(Equal(sample_req1)))
			})
		})
		When("adding a sender with non-matching toolchains", func() {
			It("should succeed", func() {
				router.AddSender(testCd2)
				Expect(len(router.senders)).To(Equal(2))
				Expect(len(router.receivers)).To(Equal(1))
				Expect(len(router.routes)).To(Equal(2))
			})
			It("should not be able to send", func() {
				rt := router.routeForToolchain(gnu_c)
				Expect(rt.CanSend()).To(BeFalse())
				err := router.Send(context.Background(), sample_req2)
				Expect(err).To(MatchError(ErrNoAgents))
			})
		})
		When("adding a matching receiver", func() {
			var rx <-chan request
			It("should succeed", func() {
				rx = router.AddReceiver(testAgent2)
				Expect(len(router.routes)).To(Equal(2))
				Expect(len(router.senders)).To(Equal(2))
				Expect(len(router.receivers)).To(Equal(2))
			})
			It("should be able to send on that channel", func() {
				rt := router.routeForToolchain(gnu_c)
				Eventually(rt.CanSend).Should(BeTrue())
				Expect(router.Send(context.Background(), sample_req2)).To(Succeed())
				Eventually(rx).Should(Receive(Equal(sample_req2)))
			})
		})
	})
})
