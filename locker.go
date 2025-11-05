package goropo

import (
	"sync"
)

// Locker locks the given sync.Locker and returns a function that unlocks it.
// usage: defer Locker(&mu)()
func Locker(mu sync.Locker) func() {
	mu.Lock()
	return func() {
		mu.Unlock()
	}
}
