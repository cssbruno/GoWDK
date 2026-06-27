package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInMemoryStoreConcurrentTakeAndCleanup(t *testing.T) {
	store := NewInMemoryStore(InMemoryOptions{CleanupInterval: time.Nanosecond})
	const workers = 16
	const iterations = 50
	base := time.Unix(1000, 0)

	var done sync.WaitGroup
	for worker := 0; worker < workers; worker++ {

		done.Add(1)
		go func() {
			defer done.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				key := "client-" + string(rune('a'+worker%26))
				now := base.Add(time.Duration(iteration) * time.Millisecond)
				if _, err := store.Take(context.Background(), key, 3, time.Millisecond, now); err != nil {
					t.Errorf("take: %v", err)
				}
			}
		}()
	}
	done.Wait()

	result, err := store.Take(context.Background(), "client-z", 1, time.Second, base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.Remaining != 0 {
		t.Fatalf("unexpected post-cleanup result: %#v", result)
	}
}
