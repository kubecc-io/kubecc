package storage_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/pkg/storage"
)

// Test ExpirationHeap
func TestExpirationHeap(t *testing.T) {
	heap := storage.NewExpirationHeap()

	// Ensure that the heap is empty
	if heap.Len() != 0 {
		t.Errorf("Expected heap to be empty, but found %d", heap.Len())
	}

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
	// Insert the id and expiry pairs into the heap

	// Insert 100 pairs into the heap
	for i := 0; i < 100; i++ {
		heap.Push(ids[i], expiries[i])

		// Ensure that the underlaying array is sorted correctly
		sorted := isHeapSorted(heap.UnderlyingArray())
		if !sorted {
			t.Errorf("Expected heap to be sorted, but found %t", sorted)
			return
		}
	}

	// Ensure that the heap is not empty
	if heap.Len() != 100 {
		t.Errorf("Expected heap to be 100, but found %d", heap.Len())
		return
	}

	// Ensure that the heap is sorted correctly
	sorted := isHeapSorted(heap.UnderlyingArray())
	if !sorted {
		t.Errorf("Expected heap to be sorted, but found %t", sorted)
		return
	}

	// Pop all ids and expiry pairs from the heap and ensure that they are sorted correctly
	times := make([]time.Time, 100)
	for i := 0; i < 100; i++ {
		item := heap.Pop()
		expiry := item.Date
		times[i] = expiry
		// Ensure that the underlaying array is sorted correctly
		sorted := isHeapSorted(heap.UnderlyingArray())
		if !sorted {
			t.Errorf("Expected heap to be sorted, but found %t", sorted)
			return
		}
	}

	// Ensure that the list of times is sorted correctly
	for i := 1; i < 100; i++ {
		if times[i-1].After(times[i]) {
			t.Errorf("Expected times to be sorted, but found %v", times)
			return
		}
	}

	// Ensure that the heap is empty
	if heap.Len() != 0 {
		t.Errorf("Expected heap to be empty, but found %d", heap.Len())
		return
	}
}

// Checks if a binary heap is in min-heap order
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

func TestExpirationNotifier(t *testing.T) {
	// Create a new ExpirationNotifier
	notifier := storage.NewExpirationNotifier()

	// Ensure that the notifier is empty by checking if the next expiration
	// contains default values
	nextHash, nextTime := notifier.NextExpiration()
	emptyTime := time.Time{}
	if nextHash != "" || nextTime != emptyTime {
		t.Errorf("Expected nextHash and nextTime to be empty, but found %s and %s", nextHash, nextTime)
		return
	}

	// Generate 100 id and expiry pairs
	ids := make([]string, 100)
	expiries := make([]time.Time, 100)
	for i := 0; i < 100; i++ {
		ids[i] = uuid.New().String()
		expiries[i] = time.Now().Add(time.Duration(i+1) * time.Second)
	}

	// Add the id and expiry pairs to the notifier in reverse order
	for i := 99; i >= 0; i-- {
		notifier.Add(ids[i], expiries[i])
	}

	// Ensure that the next expiration is the first expiry
	nextHash, nextTime = notifier.NextExpiration()
	if nextHash != ids[0] || nextTime != expiries[0] {
		t.Errorf("Expected nextHash and nextTime to be %s and %s, but found %s and %s", ids[0], expiries[0], nextHash, nextTime)
		return
	}

	// Wait for the next expiration and check to make sure it takes one second
	start := time.Now()
	ctx, ca := context.WithCancel(testCtx)
	go notifier.WaitForExpiration(ctx)
	select {
	case <-notifier.Notify():
	case <-time.After(2 * time.Second):
		t.Errorf("Expected to wait for the next expiration, but it did not")
		ca()
		return
	}
	ca()
	end := time.Now()
	if end.Sub(start) < time.Second || end.Sub(start) > time.Second+100*time.Millisecond {
		t.Errorf("Expected wait for expiration to take at least 1 second, but it took %v", end.Sub(start))
		return
	}

	// Check to make sure that force expiration works during a wait
	// Make sure the next expiration is the second expiry
	nextHash, nextTime = notifier.NextExpiration()
	if nextHash != ids[1] || nextTime != expiries[1] {
		t.Errorf("Expected nextHash and nextTime to be %s and %s, but found %s and %s", ids[1], expiries[1], nextHash, nextTime)
		return
	}

	start = time.Now()
	go func() {
		time.Sleep(time.Second)
		notifier.ForceExpire()
	}()
	ctx, ca = context.WithCancel(testCtx)
	go notifier.WaitForExpiration(ctx)
	select {
	case <-notifier.Notify():
	case <-time.After(2 * time.Second):
		t.Errorf("Expected to wait for the next expiration, but it did not")
		ca()
		return
	}
	ca()
	end = time.Now()
	if end.Sub(start) < time.Second || end.Sub(start) > time.Second+100*time.Millisecond {
		t.Errorf("Expected wait for expiration to take at least 1 second, but it took %v", end.Sub(start))
		return
	}

	// Ensure that the next expiration is the third expiry
	nextHash, nextTime = notifier.NextExpiration()
	if nextHash != ids[2] || nextTime != expiries[2] {
		t.Errorf("Expected nextHash and nextTime to be %s and %s, but found %s and %s", ids[2], expiries[2], nextHash, nextTime)
		return
	}

	// Check to make sure that adding newer expirations than the next expiration correctly updates the next expiration
	id := uuid.New()
	expiry := time.Now().Add(time.Second)
	notifier.Add(id.String(), expiry)
	nextHash, nextTime = notifier.NextExpiration()
	if nextHash != id.String() || nextTime != expiry {
		t.Errorf("Expected nextHash and nextTime to be %s and %s, but found %s and %s", id.String(), expiry, nextHash, nextTime)
		return
	}

	// Check to make sure that adding newer expirations works during a wait
	start = time.Now()
	go func() {
		time.Sleep(time.Second)
		notifier.Add(id.String(), expiry)
	}()
	ctx, ca = context.WithCancel(testCtx)
	go notifier.WaitForExpiration(ctx)
	select {
	case <-notifier.Notify():
	case <-time.After(4 * time.Second):
		t.Errorf("Expected to wait for the next expiration, but it did not")
		ca()
		return
	}
	ca()
	end = time.Now()
	// Ensure that the wait for expiration took 2 seconds
	if end.Sub(start) < 2*time.Second || end.Sub(start) > 2*time.Second+100*time.Millisecond {
		t.Errorf("Expected wait for expiration to take at least 2 seconds, but it took %v", end.Sub(start))
		return
	}

}
