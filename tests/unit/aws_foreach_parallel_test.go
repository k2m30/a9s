package unit

// aws_foreach_parallel_test.go — pins the contract for aws.ForEachParallel,
// the shared bounded-concurrency helper Wave-2 issue enrichers will use to
// replace their current sequential per-resource AWS calls.
//
// Contract under test (core/aws/parallel.go, not yet implemented):
//
//	func ForEachParallel(ctx context.Context, n, limit int, fn func(i int)) error
//
//	- Runs fn(i) for i in [0, n) with at most `limit` concurrent invocations.
//	- Blocks until all scheduled fn calls return.
//	- When ctx is cancelled, no NEW fn calls are started (in-flight ones
//	  finish); returns ctx.Err() in that case, nil otherwise.
//	- limit <= 1 or n <= 1 degrade to a plain sequential loop.
//	- fn is responsible for its own result collection and locking.
//
// Panic safety is explicitly out of contract and not tested here.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// TestForEachParallel_AllIndicesVisitedExactlyOnce pins case 1: every index
// in [0,n) is visited exactly once, collected into a mutex-guarded set.
func TestForEachParallel_AllIndicesVisitedExactlyOnce(t *testing.T) {
	const n = 100
	const limit = 8

	var mu sync.Mutex
	seen := make(map[int]int, n)

	err := awsclient.ForEachParallel(context.Background(), n, limit, func(i int) {
		mu.Lock()
		seen[i]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v", err)
	}

	if len(seen) != n {
		t.Fatalf("visited %d distinct indices, want %d; seen = %v", len(seen), n, seen)
	}
	for i := 0; i < n; i++ {
		count, ok := seen[i]
		if !ok {
			t.Errorf("index %d was never visited", i)
			continue
		}
		if count != 1 {
			t.Errorf("index %d visited %d times, want exactly 1", i, count)
		}
	}
}

// TestForEachParallel_ConcurrencyBounded pins case 2: with limit=4, the
// max number of in-flight fn calls never exceeds 4, and — to prove actual
// parallelization is happening rather than a hidden sequential loop — the
// max in-flight count exceeds 1 at some point during the run.
func TestForEachParallel_ConcurrencyBounded(t *testing.T) {
	const n = 40
	const limit = 4

	var current int64
	var max int64

	err := awsclient.ForEachParallel(context.Background(), n, limit, func(_ int) {
		cur := atomic.AddInt64(&current, 1)
		for {
			m := atomic.LoadInt64(&max)
			if cur <= m || atomic.CompareAndSwapInt64(&max, m, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt64(&current, -1)
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v", err)
	}

	finalMax := atomic.LoadInt64(&max)
	if finalMax > limit {
		t.Errorf("max in-flight = %d, want <= %d (limit not enforced)", finalMax, limit)
	}
	if finalMax <= 1 {
		t.Errorf("max in-flight = %d, want > 1 (calls ran sequentially, not in parallel)", finalMax)
	}
}

// TestForEachParallel_ContextCancellation pins case 3: cancelling ctx after
// a handful of fn calls stops new calls from starting, returns
// context.Canceled, and the call returns promptly (no deadlock/leak).
func TestForEachParallel_ContextCancellation(t *testing.T) {
	const n = 50
	const limit = 4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var callCount int
	ranCount := 0

	done := make(chan error, 1)
	go func() {
		done <- awsclient.ForEachParallel(ctx, n, limit, func(_ int) {
			mu.Lock()
			callCount++
			c := callCount
			mu.Unlock()

			if c == 3 {
				cancel()
			}

			mu.Lock()
			ranCount++
			mu.Unlock()
		})
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ForEachParallel did not return within 5s after ctx cancellation — possible deadlock/leak")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}

	mu.Lock()
	finalRan := ranCount
	mu.Unlock()
	if finalRan >= n {
		t.Errorf("ranCount = %d, want < %d (cancellation should have stopped new fn calls from starting)", finalRan, n)
	}
}

// TestForEachParallel_ZeroN pins case 4a: n=0 means fn is never called and
// the error is nil.
func TestForEachParallel_ZeroN(t *testing.T) {
	called := false
	err := awsclient.ForEachParallel(context.Background(), 0, 8, func(_ int) {
		called = true
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v, want nil", err)
	}
	if called {
		t.Error("fn was called with n=0, want no calls")
	}
}

// TestForEachParallel_LimitZero_SequentialInOrder pins case 4b: limit=0
// degrades to a plain sequential loop, visiting indices 0..n-1 in exact
// ascending order.
func TestForEachParallel_LimitZero_SequentialInOrder(t *testing.T) {
	const n = 20
	var order []int

	err := awsclient.ForEachParallel(context.Background(), n, 0, func(i int) {
		order = append(order, i)
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v", err)
	}

	if len(order) != n {
		t.Fatalf("len(order) = %d, want %d; order = %v", len(order), n, order)
	}
	for i := 0; i < n; i++ {
		if order[i] != i {
			t.Fatalf("order[%d] = %d, want %d (sequential degrade must preserve exact ascending order); full order = %v", i, order[i], i, order)
		}
	}
}

// TestForEachParallel_LimitOne_SequentialInOrder pins case 4b: limit=1
// likewise degrades to a plain sequential loop, visiting indices 0..n-1 in
// exact ascending order.
func TestForEachParallel_LimitOne_SequentialInOrder(t *testing.T) {
	const n = 20
	var order []int

	err := awsclient.ForEachParallel(context.Background(), n, 1, func(i int) {
		order = append(order, i)
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v", err)
	}

	if len(order) != n {
		t.Fatalf("len(order) = %d, want %d; order = %v", len(order), n, order)
	}
	for i := 0; i < n; i++ {
		if order[i] != i {
			t.Fatalf("order[%d] = %d, want %d (sequential degrade must preserve exact ascending order); full order = %v", i, order[i], i, order)
		}
	}
}

// TestForEachParallel_NOne pins case 4c: n=1 runs fn exactly once with i=0,
// regardless of limit.
func TestForEachParallel_NOne(t *testing.T) {
	var mu sync.Mutex
	var calls []int

	err := awsclient.ForEachParallel(context.Background(), 1, 8, func(i int) {
		mu.Lock()
		calls = append(calls, i)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v", err)
	}
	if len(calls) != 1 || calls[0] != 0 {
		t.Errorf("calls = %v, want [0]", calls)
	}
}

// TestForEachParallel_RaceGuardedCollector is a race-detector-meaningful
// test (case 6): many goroutines write through a single mutex-guarded
// collector concurrently. Under `go test -race` (make test-race / CI) this
// catches any accidental unsynchronized access introduced by the helper
// itself (e.g. sharing loop variables across goroutines incorrectly).
func TestForEachParallel_RaceGuardedCollector(t *testing.T) {
	const n = 200
	const limit = 16

	var mu sync.Mutex
	results := make(map[int]bool, n)

	err := awsclient.ForEachParallel(context.Background(), n, limit, func(i int) {
		mu.Lock()
		results[i] = true
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("ForEachParallel error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(results) != n {
		t.Errorf("len(results) = %d, want %d", len(results), n)
	}
	for i := 0; i < n; i++ {
		if !results[i] {
			t.Errorf("index %d missing from results", i)
		}
	}
}
