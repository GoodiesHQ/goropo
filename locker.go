package goropo

import "sync"

func Locker(mu *sync.Mutex) func() {
	mu.Lock()
	return func() {
		mu.Unlock()
	}
}
