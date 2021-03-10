package monitor_test

import (
	"bytes"
	"context"

	mapset "github.com/deckarep/golang-set"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cobalt77/kubecc/pkg/apps/monitor"
)

var _ = Describe("Store", func() {
	var store monitor.KeyValueStore
	When("creating the store", func() {
		It("should be empty", func() {
			store = monitor.InMemoryStoreCreator.NewStore(context.Background())
			Expect(store.Len()).To(Equal(0))
		})
	})
	It("Should handle setting and retrieving keys", func() {
		store.Set("key1", []byte("test"))
		value, ok := store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("test")))
		Expect(store.Len()).To(Equal(1))
		store.Set("key2", []byte("test2"))
		value, ok = store.Get("key2")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("test2")))
		Expect(store.Len()).To(Equal(2))
		store.Set("key3", []byte("test3"))
		value, ok = store.Get("key3")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("test3")))
		Expect(store.Len()).To(Equal(3))
	})
	It("Should list the available keys", func() {
		set := mapset.NewSet()
		for _, k := range store.Keys() {
			set.Add(k)
		}
		Expect(set.Cardinality()).To(Equal(3))
		Expect(set.Contains("key1", "key2", "key3")).To(BeTrue())
	})
	It("Should handle deleting keys", func() {
		store.Delete("key1")
		value, ok := store.Get("key1")
		Expect(ok).To(BeFalse())
		Expect(value).To(BeNil())
		Expect(store.Len()).To(Equal(2))
		store.Delete("key2")
		value, ok = store.Get("key2")
		Expect(ok).To(BeFalse())
		Expect(value).To(BeNil())
		Expect(store.Len()).To(Equal(1))
		store.Delete("key3")
		value, ok = store.Get("key3")
		Expect(ok).To(BeFalse())
		Expect(value).To(BeNil())
		Expect(store.Len()).To(Equal(0))
	})
	It("Should list the available keys (empty)", func() {
		Expect(store.Keys()).To(BeEmpty())
	})
	It("Should handle compare-and-swap", func() {

		Expect(store.CAS("key1", []byte("a"))).To(BeTrue())
		value, ok := store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("a")))

		Expect(store.CAS("key1", []byte("a"))).To(BeFalse())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("a")))

		Expect(store.CAS("key1", []byte("b"))).To(BeTrue())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("b")))

		Expect(store.CAS("key1", []byte("b"))).To(BeFalse())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("b")))

		Expect(store.CAS("key1", []byte("a"))).To(BeTrue())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo([]byte("a")))
	})
	Measure("Performance", func(b Benchmarker) {
		store = monitor.InMemoryStoreCreator.NewStore(context.Background())
		b.Time("10B payload Set/Get", func() {
			store.Set("key1", []byte("0123456789"))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("100B payload Set/Get", func() {
			store.Set("key1", bytes.Repeat([]byte("0123456789"), 10))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("1KB payload Set/Get", func() {
			store.Set("key1", bytes.Repeat([]byte("0123456789"), 100))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("10KB payload Set/Get", func() {
			store.Set("key1", bytes.Repeat([]byte("0123456789"), 1000))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("100KB payload Set/Get", func() {
			store.Set("key1", bytes.Repeat([]byte("0123456789"), 10000))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("1MB payload Set/Get", func() {
			store.Set("key1", bytes.Repeat([]byte("0123456789"), 1e5))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("10MB payload Set/Get", func() {
			store.Set("key1", bytes.Repeat([]byte("0123456789"), 1e6))
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		Expect(store.Len()).To(Equal(0))
	}, 1000)
})
