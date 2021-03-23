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

package storage_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/storage"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
)

var testCtx = meta.NewContext(
	meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
	meta.WithProvider(identity.UUID),
	meta.WithProvider(logkc.Logger),
	meta.WithProvider(tracing.Tracer),
)

var _ = Describe("Volatile Storage Provider", func() {
	Context("Configuration checking", func() {
		It("Should fail when given an invalid configuration", func() {
			By("Testing empty config")
			vsp := storage.NewVolatileStorageProvider(testCtx, config.LocalStorageSpec{})
			Expect(vsp.Configure()).To(MatchError(storage.ConfigurationError))
			By("Testing wrong storage limit kind")
			vsp = storage.NewVolatileStorageProvider(testCtx, config.LocalStorageSpec{
				Limits: config.StorageLimitsSpec{
					Disk: "1Mi",
				},
			})
			Expect(vsp.Configure()).To(MatchError(storage.ConfigurationError))
			By("Invalid memory limit string")
			vsp = storage.NewVolatileStorageProvider(testCtx, config.LocalStorageSpec{
				Limits: config.StorageLimitsSpec{
					Memory: "",
				},
			})
			Expect(vsp.Configure()).To(MatchError(storage.ConfigurationError))
		})
		It("Should not fail when given a valid configuration", func() {
			Expect(storage.NewVolatileStorageProvider(testCtx, config.LocalStorageSpec{
				Limits: config.StorageLimitsSpec{
					Memory: "1Mi",
				},
			})).NotTo(BeNil())
		})
	})
	Context("Basic functionality", func() {
		vsp := storage.NewVolatileStorageProvider(testCtx, config.LocalStorageSpec{
			Limits: config.StorageLimitsSpec{
				Memory: "1Ki",
			},
		})
		startTime := time.Now().Unix()
		It("Should configure successfully", func() {
			Expect(vsp.Configure()).To(Succeed())
		})
		It("Should have the proper location set", func() {
			Expect(vsp.Location()).To(Equal(types.Memory))
		})
		It("Should be empty when first created", func() {
			By("Checking usage info")
			Expect(vsp.UsageInfo().ObjectCount).To(Equal(int64(0)))
			Expect(vsp.UsageInfo().TotalSize).To(Equal(int64(0)))
			Expect(vsp.UsageInfo().UsagePercent).To(Equal(float64(0)))
		})
		It("Should persist stored objects", func() {
			for i := 1; i <= 100; i++ {
				Expect(vsp.Put(testCtx, &types.CacheKey{
					Hash: fmt.Sprint(i),
				}, &types.CacheObject{
					Data: []byte("12345"),
				})).To(Succeed())
				obj, err := vsp.Get(testCtx, &types.CacheKey{
					Hash: fmt.Sprint(i),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.Data).To(BeEquivalentTo([]byte("12345")))
				Expect(vsp.UsageInfo().ObjectCount).To(Equal(int64(i)))
				Expect(vsp.UsageInfo().TotalSize).To(Equal(int64(5 * i)))
				// float math is deterministic so comparing floats is ok here
				Expect(vsp.UsageInfo().UsagePercent).
					To(Equal(float64(5*i) / float64(1024)))
			}
		})
		It("Should not store the same object twice", func() {
			for i := 1; i <= 100; i++ {
				err := vsp.Put(testCtx, &types.CacheKey{
					Hash: fmt.Sprint(i),
				}, &types.CacheObject{
					Data: []byte("12345"),
				})
				Expect(status.Code(err)).To(Equal(codes.AlreadyExists))
			}
		})
		It("Should query existing objects", func() {
			keys := []*types.CacheKey{}
			for i := 1; i <= 100; i++ {
				keys = append(keys, &types.CacheKey{
					Hash: fmt.Sprint(i),
				})
			}
			results, err := vsp.Query(testCtx, keys)
			Expect(err).NotTo(HaveOccurred())
			for _, result := range results {
				Expect(result).NotTo(BeNil())
				Expect(result.ManagedFields.Size).To(Equal(int64(5)))
			}
		})
		It("Should store managed fields", func() {
			obj, err := vsp.Get(testCtx, &types.CacheKey{
				Hash: "1",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.GetMetadata()).NotTo(BeNil())
			Expect(obj.GetMetadata().GetManagedFields().GetLocation()).
				To(BeEquivalentTo(vsp.Location()))
			Expect(obj.GetMetadata().GetManagedFields().GetSize()).
				To(Equal(int64(5)))
			Expect(obj.GetMetadata().GetManagedFields().GetTimestamp()).
				To(And(
					BeNumerically(">=", startTime),
					BeNumerically("<=", time.Now().Unix()),
				))
		})
		It("Should store tags", func() {
			tags := map[string]string{
				"tag1": "value1",
				"tag2": "value2",
				"tag3": "value3",
			}
			err := vsp.Put(testCtx, &types.CacheKey{
				Hash: "999",
			}, &types.CacheObject{
				Data: []byte("12345"),
				Metadata: &types.CacheObjectMeta{
					Tags: tags,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			meta, err := vsp.Get(testCtx, &types.CacheKey{
				Hash: "999",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(meta.Metadata.GetTags()).To(BeEquivalentTo(tags))
		})
	})
	Context("Expiration", func() {
		vsp := storage.NewVolatileStorageProvider(testCtx, config.LocalStorageSpec{
			Limits: config.StorageLimitsSpec{
				Memory: "1Ki",
			},
		})
		It("Should configure successfully", func() {
			Expect(vsp.Configure()).To(Succeed())
		})
		It("Should handle key expiration", func() {
			By("Adding a new key")
			start := time.Now()
			Expect(vsp.Put(testCtx, &types.CacheKey{
				Hash: "1",
			}, &types.CacheObject{
				Data: []byte("1"),
				Metadata: &types.CacheObjectMeta{
					ExpirationDate: start.Add(1 * time.Second).UnixNano(),
				},
			})).To(Succeed())
			By("Ensuring the key exists")
			Eventually(func() bool {
				items, _ := vsp.Query(testCtx, []*types.CacheKey{
					{
						Hash: "1",
					},
				})
				return items[0] != nil
			}, 1*time.Second, 10*time.Millisecond).Should(BeTrue())
			By("Waiting until the key is deleted")
			Eventually(func() bool {
				items, _ := vsp.Query(testCtx, []*types.CacheKey{
					{
						Hash: "1",
					},
				})
				return items[0] == nil
			}, 1200*time.Millisecond, 10*time.Millisecond).Should(BeTrue())
			Expect(time.Since(start)).To(BeNumerically(">=", 990*time.Millisecond))
			Expect(vsp.UsageInfo().ObjectCount).To(Equal(int64(0)))
			// Wait for the OnDelete callback to occur
			Eventually(func() int64 {
				return vsp.UsageInfo().TotalSize
			}).Should(Equal(int64(0)))
			Eventually(func() float64 {
				return vsp.UsageInfo().UsagePercent
			}).Should(Equal(0.0))
		})
	})
})
