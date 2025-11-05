package goropo

import (
	"sync"
	"sync/atomic"
	"testing"
)

// MutexWithCheck is a sync.Mutex wrapper that tracks its locked state
// implements sync.Locker interface
type MutexWithCheck struct {
	mu     sync.Mutex
	locked atomic.Bool
}

// Lock locks the mutex and sets the locked state to true
func (mu *MutexWithCheck) Lock() {
	mu.mu.Lock()
	mu.locked.Store(true)
}

// Unlock unlocks the mutex and sets the locked state to false
func (mu *MutexWithCheck) Unlock() {
	mu.mu.Unlock()
	mu.locked.Store(false)
}

// IsLocked returns true if the mutex is currently locked
func (mu *MutexWithCheck) IsLocked() bool {
	return mu.locked.Load()
}

func TestLocker_LockUnlock(t *testing.T) {
	locker := &MutexWithCheck{}

	if locker.IsLocked() {
		t.Fatal("Expected mutex to be initially unlocked")
	}

	unlock := Locker(locker)
	if !locker.IsLocked() {
		t.Fatal("Expected mutex to be locked after Locker call")
	}

	unlock()
	if locker.IsLocked() {
		t.Fatal("Expected mutex to be unlocked after unlock call")
	}
}

func TestLocker_Deferred(t *testing.T) {
	locker := &MutexWithCheck{}

	if locker.IsLocked() {
		t.Fatal("Expected mutex to be initially unlocked")
	}

	func() {
		defer Locker(locker)()
		if !locker.IsLocked() {
			t.Fatal("Expected mutex to be locked inside deferred function")
		}
	}()

	if locker.IsLocked() {
		t.Fatal("Expected mutex to be unlocked after deferred function")
	}
}
