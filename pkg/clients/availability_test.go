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
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/types"
)

var _ = Describe("Status Manager", func() {
	var avListener *clients.AvailabilityChecker
	When("creating an availability listener", func() {
		It("should succeed", func() {
			avListener = clients.NewAvailabilityChecker(
				clients.ComponentFilter(types.TestComponent))
		})
	})

	numListeners := 1
	Measure("should ensure component availability", func(b Benchmarker) {
		available := make([]chan struct{}, numListeners)
		unavailable := make([]chan struct{}, numListeners)
		for i := 0; i < numListeners; i++ {
			available[i] = make(chan struct{})
			unavailable[i] = make(chan struct{})
			go func(i int) {
				defer GinkgoRecover()
				for {
					ctx := avListener.EnsureAvailable()
					available[i] <- struct{}{}
					<-ctx.Done()
					unavailable[i] <- struct{}{}
				}
			}(i)
		}

		By("checking if EnsureAvailable is blocked")
		completed := atomic.NewInt32(int32(numListeners))
		for _, ch := range available {
			go func(ch chan struct{}) {
				defer GinkgoRecover()
				defer completed.Dec()
				Consistently(
					ch,
					100*time.Millisecond,
					10*time.Millisecond,
				).ShouldNot(Receive())
			}(ch)
		}
		Eventually(completed.Load).Should(Equal(int32(0)))

		By("connecting the component")
		var cancel context.CancelFunc
		go func() {
			defer GinkgoRecover()
			ctx, ctxCancel := context.WithCancel(context.Background())
			cancel = ctxCancel
			avListener.OnComponentAvailable(ctx, &types.WhoisResponse{
				UUID:      uuid.NewString(),
				Address:   "0.0.0.0",
				Component: types.TestComponent,
			})
		}()

		By("checking if EnsureAvailable unblocked")
		completed.Store(int32(numListeners))
		for _, ch := range available {
			go func(ch chan struct{}) {
				defer GinkgoRecover()
				defer completed.Dec()
				Eventually(ch).Should(Receive())
			}(ch)
		}
		b.Time("checking if EnsureAvailable unblocked", func() {
			Eventually(completed.Load).Should(Equal(int32(0)))
		})
		for _, ch := range available {
			Expect(ch).NotTo(Receive())
		}

		By("disconnecting the component")
		completed.Store(int32(numListeners))
		cancel()
		for _, ch := range unavailable {
			go func(ch chan struct{}) {
				defer GinkgoRecover()
				defer completed.Dec()
				Eventually(ch).Should(Receive())
			}(ch)
		}
		b.Time("disconnecting the component", func() {
			Eventually(completed.Load).Should(Equal(int32(0)))
		})
		for _, ch := range unavailable {
			Expect(ch).NotTo(Receive())
		}

		By("checking if EnsureAvailable is blocked again")
		completed.Store(int32(numListeners))
		for _, ch := range available {
			go func(ch chan struct{}) {
				defer GinkgoRecover()
				defer completed.Dec()
				Consistently(
					ch,
					100*time.Millisecond,
					10*time.Millisecond,
				).ShouldNot(Receive())
			}(ch)
		}
		Eventually(completed.Load).Should(Equal(int32(0)))

		numListeners++
	}, 10)
})
