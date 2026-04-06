package goropo

import (
	"context"
	"sync"
	"sync/atomic"
)

// Generic return type for a Future, either a value of type T or an error
type Result[T any] struct {
	Value T
	Err   error
}

// Ok returns true if the Result contains no error
func (r Result[T]) Ok() bool {
	return r.Err == nil
}

// Future represents a task that will complete in the future
type Future[T any] struct {
	once   sync.Once                 // ensures the Future is resolved only once
	done   chan struct{}             // closed when resolved; closing broadcasts to all waiters
	result atomic.Pointer[Result[T]] // atomically stored result, safe for concurrent reads
}

type FuturePtr[T any] = *Future[T]

type FutureAny = Future[any]

// newFuture creates a new Future instance
func newFuture[T any]() *Future[T] {
	return &Future[T]{
		done: make(chan struct{}),
	}
}

// Done returns a channel that is closed when the Future is resolved.
// Multiple goroutines may wait on this channel concurrently.
func (f *Future[T]) Done() <-chan struct{} {
	return f.done
}

// IsComplete returns true if the Future has been resolved (completed)
func (f *Future[T]) IsComplete() bool {
	return f.result.Load() != nil
}

// Result returns the cached result if available, or nil if not yet resolved
func (f *Future[T]) Result() *Result[T] {
	return f.result.Load()
}

// Await blocks until the Future is resolved or the context is cancelled.
// Safe to call concurrently from multiple goroutines.
func (f *Future[T]) Await(ctx context.Context) (T, error) {
	// Non-blocking check for a previously cached result
	if r := f.result.Load(); r != nil {
		return r.Value, r.Err
	}

	select {
	// Context cancelled, return zero value and ctx error
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case <-f.done:
		r := f.result.Load()
		return r.Value, r.Err
	}
}

// resolve sets the result of the Future and closes the done channel, signaling all waiters
func (f *Future[T]) resolve(value T, err error) {
	f.once.Do(func() {
		result := Result[T]{
			Value: value,
			Err:   err,
		}
		f.result.Store(&result) // atomic write before closing done
		close(f.done)           // unblocks all Await() callers simultaneously
	})
}
