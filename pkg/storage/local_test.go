package storage_test

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"

	"time"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/storage"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Local Storage Provider", func() {
	Context("expiration heap", func() {
		var heap *storage.ExpirationHeap
		When("creating an expiration heap", func() {
			It("should have a heap of size 0", func() {
				heap = storage.NewExpirationHeap()
				Expect(heap.Len()).To(Equal(0))
			})
		})
		When("adding entries into the heap", func() {
			It("should be sorted correctly", func() {
				// Generate 100 id and expiry pairs
				ids := make([]string, 100)
				expiries := make([]time.Time, 100)
				for i := 0; i < 100; i++ {
					ids[i] = uuid.New().String()
					expiries[i] = time.Now().Add(time.Duration(i) * time.Second)
				}

				// shuffle the ids and expiry pairs
				for i := 0; i < 100; i++ {
					j := rand.Intn(100)
					ids[i], ids[j] = ids[j], ids[i]
					expiries[i], expiries[j] = expiries[j], expiries[i]
				}

				// Insert 100 pairs into the heap
				for i := 0; i < 100; i++ {
					heap.Push(ids[i], expiries[i])

					// Ensure that the underlaying array is sorted correctly
					Expect(isHeapSorted(heap.UnderlyingArray())).To(BeTrue())
				}
			})
			When("popping all entries off the heap", func() {
				It("should keep the heap sorted in the correct order", func() {
					// Pop all ids and expiry pairs from the heap and ensure that they are sorted correctly
					times := make([]time.Time, 100)
					for i := 0; i < 100; i++ {
						item := heap.Pop()
						expiry := item.Date
						times[i] = expiry
						// Ensure that the underlaying array is sorted correctly
						Expect(isHeapSorted(heap.UnderlyingArray())).To(BeTrue())
					}

					// Ensure that the list of times is sorted correctly
					for i := 1; i < 100; i++ {
						if times[i].Before(times[i-1]) {
							Fail("Expected times to be sorted")
						}
					}
				})
				It("should be empty", func() {
					Expect(heap.Len()).To(Equal(0))
				})
			})
		})
	})
	Context("expiration notifier", func() {
		notifier := storage.NewExpirationNotifier()

		When("creating an expiration notifier", func() {
			It("should be empty", func() {
				// Ensure that the notifier is empty by checking if the next expiration
				// contains default values
				nextHash, nextTime := notifier.NextExpiration()
				Expect(nextHash).To(Equal(""))
				Expect(nextTime).To(Equal(time.Time{}))
			})
		})
		ids := make([]string, 100)
		expiries := make([]time.Time, 100)
		When("adding entries into the notifier in reverse order", func() {
			Specify("the first entry should be the shortest expiration", func() {
				// Generate 100 id and expiry pairs
				for i := 0; i < 100; i++ {
					ids[i] = uuid.New().String()
					expiries[i] = time.Now().Add(time.Duration(i+1) * time.Second)
				}

				// Add the id and expiry pairs to the notifier in reverse order
				for i := 99; i >= 0; i-- {
					notifier.Add(ids[i], expiries[i])
				}

				// Ensure that the next expiration is the first expiry
				nextHash, nextTime := notifier.NextExpiration()
				Expect(nextHash).To(Equal(ids[0]))
				Expect(nextTime).To(Equal(expiries[0]))
			})
		})
		When("waiting for the first expiration", func() {
			It("should take one second", func() {
				start := time.Now()
				done := make(chan struct{})
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					notifier.WaitOne(ctx)
					close(done)
					cancel()
				}()
				Eventually(done, 2*time.Second, 10*time.Millisecond).Should(BeClosed())
				cancel()
				Expect(time.Since(start)).To(BeNumerically("~", 1*time.Second, 50*time.Millisecond))
			})
			Specify("the next expiration should be the second one", func() {
				nextHash, nextTime := notifier.NextExpiration()
				Expect(nextHash).To(Equal(ids[1]))
				Expect(nextTime).To(Equal(expiries[1]))
			})
		})
		When("forcing an expiration", func() {
			It("should immediately expire", func() {
				start := time.Now()
				done := make(chan struct{})
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					notifier.WaitOne(ctx)
					close(done)
				}()
				go func() {
					defer GinkgoRecover()
					time.Sleep(500 * time.Millisecond)
					Expect(done).NotTo(BeClosed())
					err := notifier.ForceExpiration()
					Expect(err).NotTo(HaveOccurred())
				}()
				Eventually(done, 2*time.Second, 10*time.Millisecond).Should(BeClosed())
				cancel()
				Expect(time.Since(start)).To(BeNumerically("~", 500*time.Millisecond, 50*time.Millisecond))
			})
			Specify("thenext expiration should be the third one", func() {
				nextHash, nextTime := notifier.NextExpiration()
				Expect(nextHash).To(Equal(ids[2]))
				Expect(nextTime).To(Equal(expiries[2]))
			})
		})
		When("adding newer expirations while waiting", func() {
			It("should immediately update", func() {
				start := time.Now()

				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					notifier.WaitOne(ctx)
					close(done)
				}()

				time.Sleep(100 * time.Millisecond)
				// order is important here, generate the expiration after sleeping
				id := uuid.New().String()
				expiry := time.Now().Add(100 * time.Millisecond)

				notifier.Add(id, expiry)
				nextHash, nextTime := notifier.NextExpiration()
				Expect(nextHash).To(Equal(id))
				Expect(nextTime).To(Equal(expiry))
				Eventually(done, 2*time.Second, 10*time.Millisecond).Should(BeClosed())
				cancel()
				Expect(time.Since(start)).To(BeNumerically("~", 200*time.Millisecond, 50*time.Millisecond))
			})
		})
	})
	Context("storage provider", func() {
		// Test that the storage provider works as expected
		var (
			storageProvider *storage.LocalStorageProvider = new(storage.LocalStorageProvider)
			tempDir         string
			itCtx           context.Context
			itCancel        context.CancelFunc
		)
		BeforeEach(func() {
			// Create a new context for each test
			itCtx, itCancel = context.WithCancel(testCtx)

			// Create a new temp directory for the test
			var err error
			tempDir, err = ioutil.TempDir("", "notifier-test")
			Expect(err).NotTo(HaveOccurred())
			// Create a new local storage provider with a 10KiB limit. Each object
			// we write to the storage provider will be 1KiB.
			*storageProvider = *storage.NewLocalStorageProvider(itCtx, config.LocalStorageSpec{
				Path: tempDir,
				Limits: config.StorageLimitsSpec{
					Disk: "10Ki",
				},
			}).(*storage.LocalStorageProvider)
			// Configure the storage provider
			err = storageProvider.Configure()
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			// Delete the temp directory
			itCancel()
			err := os.RemoveAll(tempDir)
			Expect(err).NotTo(HaveOccurred())
		})
		// Test that the storage provider can get and put objects

		It("should get and put objects", func() {
			// Create a new object with 1024 random bytes, calculate its hash, and
			// write it to the storage provider
			obj := make([]byte, 1024)
			rand.Read(obj)
			hasher := md5.New()
			hasher.Write(obj)
			hash := fmt.Sprintf("%x", hasher.Sum(nil))

			err := storageProvider.Put(testCtx, &types.CacheKey{
				Hash: string(hash),
			}, &types.CacheObject{
				Data: obj,
				Metadata: &types.CacheObjectMeta{
					ExpirationDate: time.Now().Add(time.Hour).UnixNano(),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Get the object from the storage provider and check that it's the
			// same as the one we wrote
			obj2, err := storageProvider.Get(testCtx, &types.CacheKey{
				Hash: string(hash),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj2.Data).To(Equal(obj))
		})
		It("should get and put objects with metadata", func() {
			// Create a new object with 1024 random bytes, calculate its hash, and
			// write it to the storage provider
			obj := make([]byte, 1024)
			rand.Read(obj)
			hasher := md5.New()
			hasher.Write(obj)
			hash := fmt.Sprintf("%x", hasher.Sum(nil))

			metadata := map[string]string{
				"foo": "bar",
			}
			err := storageProvider.Put(testCtx, &types.CacheKey{
				Hash: string(hash),
			}, &types.CacheObject{
				Data: obj,
				Metadata: &types.CacheObjectMeta{
					ExpirationDate: time.Now().Add(time.Hour).UnixNano(),
					Tags:           metadata,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Get the object from the storage provider and check that it's the
			// same as the one we wrote
			obj2, err := storageProvider.Get(testCtx, &types.CacheKey{
				Hash: string(hash),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj2.Data).To(Equal(obj))
			Expect(obj2.Metadata.Tags).To(Equal(metadata))
		})
		It("should get and put objects with metadata and expiration", func() {
			// Create a new object with 1024 random bytes, calculate its hash, and
			// write it to the storage provider
			obj := make([]byte, 1024)
			rand.Read(obj)
			hasher := md5.New()
			hasher.Write(obj)
			hash := fmt.Sprintf("%x", hasher.Sum(nil))

			metadata := map[string]string{
				"foo": "bar",
			}
			expirationDate := time.Now().Add(time.Hour).UnixNano()
			err := storageProvider.Put(testCtx, &types.CacheKey{
				Hash: string(hash),
			}, &types.CacheObject{
				Data: obj,
				Metadata: &types.CacheObjectMeta{
					ExpirationDate: expirationDate,
					Tags:           metadata,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Get the object from the storage provider and check that it's the
			// same as the one we wrote
			obj2, err := storageProvider.Get(testCtx, &types.CacheKey{
				Hash: string(hash),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj2.Data).To(Equal(obj))
			Expect(obj2.Metadata.Tags).To(Equal(metadata))
			Expect(obj2.Metadata.ExpirationDate).To(Equal(expirationDate))
		})
		It("should get and put 11 objects with different metadata and expiration", func() {
			// Create 11 new objects with 1024 random bytes, and random metadata
			// and expiration dates, and store them in a map for later use
			objs := make(map[string]*types.CacheObject)
			for i := 0; i < 11; i++ {
				obj := make([]byte, 1024)
				rand.Read(obj)
				hasher := md5.New()
				hasher.Write(obj)
				// Get the hash as a hex string so it can be used as a file name
				hash := fmt.Sprintf("%x", hasher.Sum(nil))

				// Generate a random expiration date
				expirationDate := time.Now().Add(time.Duration(rand.Intn(3600)+60) * time.Second).UnixNano()
				// Generate random tags
				metadata := map[string]string{}
				for i := 0; i < rand.Intn(10); i++ {
					metadata[fmt.Sprintf("foo%d", i)] = fmt.Sprintf("bar%d", i)
				}
				// Store the object in the map
				objs[hash] = &types.CacheObject{
					Data: obj,
					Metadata: &types.CacheObjectMeta{
						ExpirationDate: expirationDate,
						Tags:           metadata,
					},
				}
			}

			// Store the objects in the storage provider
			for hash, obj := range objs {
				err := storageProvider.Put(testCtx, &types.CacheKey{
					Hash: hash,
				}, obj)
				Expect(err).NotTo(HaveOccurred())
			}
			// Get the objects from the storage provider and check that they're
			// the same as the ones we wrote
			for hash, obj := range objs {
				obj2, err := storageProvider.Get(testCtx, &types.CacheKey{
					Hash: hash,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(obj2.Data).To(Equal(obj.Data))
				Expect(obj2.Metadata.Tags).To(BeEquivalentTo(obj.Metadata.Tags))
				Expect(obj2.Metadata.ExpirationDate).To(Equal(obj.Metadata.ExpirationDate))
			}

			// Check the usage info of the storage provider
			usageInfo := storageProvider.UsageInfo()
			Expect(usageInfo.ObjectCount).To(BeEquivalentTo(11))
			Expect(usageInfo.TotalSize).To(BeEquivalentTo(11 * 1024))

			// Trigger the storage provider to delete objects closest to expiring
			// to keep size under the limit. Since each object is at least 1KB
			// (including metadata), this should delete at least one object.
			storageProvider.DeleteObjectsClosestToExpiration()
			// Check that the storage provider has deleted 3 objects by checking
			// the usage info of the storage provider
			usageInfo2 := storageProvider.UsageInfo()
			Expect(usageInfo2.ObjectCount).To(BeNumerically("<", 11))
			Expect(usageInfo2.TotalSize).To(BeNumerically("<", 90*1024*1024))

			// Check to see how many objects were deleted
			deletedObjectCount := usageInfo.ObjectCount - usageInfo2.ObjectCount

			// Check to see if the objects with the closest expiration date
			// have been deleted from the storage provider

			// Sort the obejcts by the expiration date
			var sortedObjs []*types.CacheObject
			for _, obj := range objs {
				sortedObjs = append(sortedObjs, obj)
			}
			sort.Slice(sortedObjs, func(i, j int) bool {
				return sortedObjs[i].Metadata.ExpirationDate < sortedObjs[j].Metadata.ExpirationDate
			})

			for i := 0; i < int(deletedObjectCount); i++ {
				// Compute hash of data
				hasher := md5.New()
				hasher.Write(sortedObjs[i].Data)
				hash := fmt.Sprintf("%x", hasher.Sum(nil))
				// Get the object from the storage provider and check that it's
				// not there
				_, err := storageProvider.Get(testCtx, &types.CacheKey{
					Hash: string(hash),
				})
				Expect(err).To(HaveOccurred())
			}

			// Check that the rest of the objects have been left in place
			for i := int(deletedObjectCount); i < 11; i++ {
				// Compute hash of data
				hasher := md5.New()
				hasher.Write(sortedObjs[i].Data)
				hash := fmt.Sprintf("%x", hasher.Sum(nil))
				// Get the object from the storage provider and check that it's
				// the same as the one we wrote
				obj2, err := storageProvider.Get(testCtx, &types.CacheKey{
					Hash: string(hash),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(obj2.Data).To(Equal(sortedObjs[i].Data))
				Expect(obj2.Metadata.Tags).To(Equal(sortedObjs[i].Metadata.Tags))
				Expect(obj2.Metadata.ExpirationDate).To(Equal(sortedObjs[i].Metadata.ExpirationDate))
			}
		})
	})
})

func isHeapSorted(array []*storage.ExpirationEntry) bool {
	for i := 0; i < len(array); i++ {
		if i > 0 {
			if array[i].Date.Before(array[(i-1)/2].Date) {
				return false
			}
		}
	}
	return true
}
