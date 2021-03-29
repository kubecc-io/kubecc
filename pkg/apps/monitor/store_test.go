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
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubecc-io/kubecc/internal/testutil"
	"github.com/kubecc-io/kubecc/pkg/apps/monitor"
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
		store.Set("key1", &testutil.Test1{Counter: 1})
		value, ok := store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test1{Counter: 1}))
		Expect(store.Len()).To(Equal(1))
		store.Set("key2", &testutil.Test2{Value: "1"})
		value, ok = store.Get("key2")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test2{Value: "1"}))
		Expect(store.Len()).To(Equal(2))
		store.Set("key3", &testutil.Test3{Counter: 2})
		value, ok = store.Get("key3")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test3{Counter: 2}))
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
		Expect(store.CAS("key1", &testutil.Test1{Counter: 2})).To(BeTrue())
		value, ok := store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test1{Counter: 2}))

		Expect(store.CAS("key1", &testutil.Test1{Counter: 2})).To(BeFalse())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test1{Counter: 2}))

		Expect(store.CAS("key1", &testutil.Test1{Counter: 3})).To(BeTrue())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test1{Counter: 3}))

		Expect(store.CAS("key1", &testutil.Test1{Counter: 3})).To(BeFalse())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test1{Counter: 3}))

		Expect(store.CAS("key1", &testutil.Test1{Counter: 2})).To(BeTrue())
		value, ok = store.Get("key1")
		Expect(ok).To(BeTrue())
		Expect(value).To(BeEquivalentTo(&testutil.Test1{Counter: 2}))
	})
	Measure("Performance", func(b Benchmarker) {
		store = monitor.InMemoryStoreCreator.NewStore(context.Background())
		b.Time("10B payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: "0123456789"})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("100B payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: strings.Repeat("0123456789", 10)})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("1KB payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: strings.Repeat("0123456789", 100)})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("10KB payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: strings.Repeat("0123456789", 1000)})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("100KB payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: strings.Repeat("0123456789", 10000)})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("1MB payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: strings.Repeat("0123456789", 1e5)})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		b.Time("10MB payload Set/Get", func() {
			store.Set("key1", &testutil.Test2{Value: strings.Repeat("0123456789", 1e6)})
			_, _ = store.Get("key1")
		})
		store.Delete("key1")
		Expect(store.Len()).To(Equal(0))
	}, 100)
	Measure("Throughput", func(b Benchmarker) {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			store.CAS("throughput", &testutil.Test1{Counter: int32(i)})
		}
		elapsed := time.Since(start)
		b.RecordValueWithPrecision("Updates per second",
			float64(1e12/elapsed.Nanoseconds())/1e6, "M", 3)
	}, 100)
})
