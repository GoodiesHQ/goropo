package goropo

import (
	"sync"
)

// Locker locks the given sync.Locker and returns a function that unlocks it.
// usage: defer Locker(&mu)()
func Locker(l sync.Locker) func() {
	l.Lock()
	return func() {
		l.Unlock()
	}
}
