package aws

import (
	"context"
	"sync"
)

// ForEachParallel runs fn(i) for i in [0, n) with at most limit concurrent
// invocations. It blocks until all scheduled fn calls return. When ctx is
// cancelled, no NEW fn calls are started (in-flight ones finish); it returns
// ctx.Err() in that case, nil otherwise. limit <= 1 or n <= 1 degrade to a
// plain sequential loop (in ascending order). fn is responsible for its own
// result collection and locking.
func ForEachParallel(ctx context.Context, n, limit int, fn func(i int)) error {
	if n <= 0 {
		return nil
	}
	if limit <= 1 || n <= 1 {
		for i := range n {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			fn(i)
		}
		return nil
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i := range n {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}

	wg.Wait()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
