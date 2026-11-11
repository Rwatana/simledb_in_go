package buffer

import (
	"testing"
	"time"
)

func TestNewManagerBindsAvailabilityConditionToItsMutex(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if manager.availableChanged == nil {
		t.Fatal("availableChanged = nil, want an initialized condition variable")
	}
	if manager.availableChanged.L != &manager.mu {
		t.Errorf("availableChanged locker = %p, want Manager mutex %p", manager.availableChanged.L, &manager.mu)
	}
}

func TestManagerUnpinWakesAvailabilityWaiterOnlyOnFinalRelease(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	block, _ := appendDataBlock(t, files, 1)
	frame, pinned, err := manager.tryToPin(block)
	if err != nil || !pinned {
		t.Fatalf("first tryToPin() = (%p, %t, %v), want a pinned frame", frame, pinned, err)
	}
	if _, pinned, err := manager.tryToPin(block); err != nil || !pinned {
		t.Fatalf("second tryToPin() = (_, %t, %v), want a pinned frame", pinned, err)
	}

	ready, released := startAvailabilityWaiter(manager)
	<-ready

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("first Unpin() error = %v", err)
	}
	select {
	case <-released:
		t.Fatal("waiter proceeded before the final pin was released")
	case <-time.After(50 * time.Millisecond):
	}

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("second Unpin() error = %v", err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("waiter did not proceed after the final pin was released")
	}
}

func TestManagerInvalidUnpinDoesNotPublishAvailability(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	block, _ := appendDataBlock(t, files, 1)
	frame, pinned, err := manager.tryToPin(block)
	if err != nil || !pinned {
		t.Fatalf("tryToPin() = (%p, %t, %v), want a pinned frame", frame, pinned, err)
	}

	ready, released := startAvailabilityWaiter(manager)
	<-ready
	if err := manager.Unpin(nil); err == nil {
		t.Fatal("Unpin(nil) error = nil, want an error")
	}
	select {
	case <-released:
		t.Fatal("waiter proceeded after invalid unpin")
	case <-time.After(50 * time.Millisecond):
	}

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("cleanup Unpin() error = %v", err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("waiter did not proceed after cleanup release")
	}
}

func startAvailabilityWaiter(manager *Manager) (<-chan struct{}, <-chan struct{}) {
	ready := make(chan struct{})
	released := make(chan struct{})
	go func() {
		manager.mu.Lock()
		close(ready)
		for manager.available == 0 {
			manager.availableChanged.Wait()
		}
		manager.mu.Unlock()
		close(released)
	}()
	return ready, released
}
