package buffer

import (
	"sync"
	"testing"
)

func TestUnpinDecrementsCountAndReportsOnlyTheAvailableTransition(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	frame.pin()
	frame.pin()

	becameAvailable, err := frame.unpin()
	if err != nil {
		t.Fatalf("first unpin() error = %v", err)
	}
	if becameAvailable {
		t.Error("first unpin() transition = true, want false for count 2 to 1")
	}
	if !frame.IsPinned() || frame.PinCount() != 1 {
		t.Errorf("after first unpin: IsPinned() = %t, PinCount() = %d; want true, 1", frame.IsPinned(), frame.PinCount())
	}

	becameAvailable, err = frame.unpin()
	if err != nil {
		t.Fatalf("second unpin() error = %v", err)
	}
	if !becameAvailable {
		t.Error("second unpin() transition = false, want true for count 1 to 0")
	}
	if frame.IsPinned() || frame.PinCount() != 0 {
		t.Errorf("after second unpin: IsPinned() = %t, PinCount() = %d; want false, 0", frame.IsPinned(), frame.PinCount())
	}

	becameAvailable, err = frame.unpin()
	if err == nil {
		t.Fatal("third unpin() error = nil, want an underflow error")
	}
	if becameAvailable {
		t.Error("rejected unpin() transition = true, want false")
	}
	if frame.PinCount() != 0 {
		t.Errorf("PinCount() after rejected unpin = %d, want 0", frame.PinCount())
	}
}

func TestUnpinIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	const goroutines = 32
	for range goroutines {
		frame.pin()
	}

	type result struct {
		becameAvailable bool
		err             error
	}
	results := make(chan result, goroutines)
	var wait sync.WaitGroup
	wait.Add(goroutines)
	for range goroutines {
		go func() {
			defer wait.Done()
			becameAvailable, err := frame.unpin()
			results <- result{becameAvailable: becameAvailable, err: err}
		}()
	}
	wait.Wait()
	close(results)

	transitionCount := 0
	for got := range results {
		if got.err != nil {
			t.Errorf("concurrent unpin() error = %v", got.err)
		}
		if got.becameAvailable {
			transitionCount++
		}
	}
	if transitionCount != 1 {
		t.Errorf("pinned-to-available transitions = %d, want 1", transitionCount)
	}
	if frame.PinCount() != 0 {
		t.Errorf("PinCount() = %d, want 0", frame.PinCount())
	}
}
