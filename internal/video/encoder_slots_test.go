package video

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A 1080p encode peaks around 300 MB and the chart requests 512Mi, so the pod is
// sized for one at a time. Nothing bounded how many ran at once, which is why
// #202's sizing could not survive two concurrent edits.
func TestEncoderSlots_BoundsConcurrency(t *testing.T) {
	limit := 2
	slots := newEncoderSlots(limit)

	var running, peak atomic.Int64
	var wg sync.WaitGroup

	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := slots.acquire(context.Background())
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			defer release()

			now := running.Add(1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			running.Add(-1)
		}()
	}
	wg.Wait()

	if got := peak.Load(); got > int64(limit) {
		t.Errorf("peak concurrency %d exceeded the limit of %d", got, limit)
	}
	if running.Load() != 0 {
		t.Errorf("%d encodes still holding a slot after completion", running.Load())
	}
}

// Waiting for a slot has to respect the job's deadline. Otherwise a queue of
// edits outlives the contexts that were supposed to bound them.
func TestEncoderSlots_WaitRespectsContext(t *testing.T) {
	slots := newEncoderSlots(1)

	release, err := slots.acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := slots.acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("second acquire returned %v, want context.DeadlineExceeded", err)
	}
}

// Releasing has to return the slot, or the pool drains to zero and every later
// encode blocks until its deadline.
func TestEncoderSlots_ReleaseReturnsTheSlot(t *testing.T) {
	slots := newEncoderSlots(1)

	for i := range 3 {
		release, err := slots.acquire(context.Background())
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		release()
	}
}

// A limit below one would deadlock every encode, so it is clamped rather than
// trusted: the value reaches here from an env var.
func TestEncoderSlots_ClampsNonPositiveLimits(t *testing.T) {
	for _, limit := range []int{0, -1} {
		slots := newEncoderSlots(limit)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		release, err := slots.acquire(ctx)
		cancel()
		if err != nil {
			t.Errorf("limit %d: acquire failed, so the pool deadlocks: %v", limit, err)
			continue
		}
		release()
	}
}
