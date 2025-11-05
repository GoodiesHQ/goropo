package goropo

import (
	"context"
	"fmt"
	"sync"
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
	once   sync.Once      // ensures the Future is resolved only once
	ch     chan Result[T] // channel to signal task completion
	result *Result[T]     // cached result after task completion
}

type FuturePtr[T any] = *Future[T]

type FutureAny = Future[any]

// newFuture creates a new Future instance
func newFuture[T any]() *Future[T] {
	return &Future[T]{
		ch:     make(chan Result[T], 1),
		result: nil,
	}
}

// Done returns a channel that is closed when the Future is resolved
func (f *Future[T]) Done() <-chan Result[T] {
	return f.ch
}

// IsComplete returns true if the Future has been resolved (completed)
func (f *Future[T]) IsComplete() bool {
	return f.result != nil
}

// Await blocks until the Future is resolved or the context is cancelled
func (f *Future[T]) Await(ctx context.Context) (T, error) {
	// Non-blocking check for a previously cached result
	if f.result != nil {
		return f.result.Value, f.result.Err
	}

	select {
	// Context cancelled, return zero value and ctx error
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case result, ok := <-f.Done():
		if !ok {
			// Channel closed unexpectedly, return zero value and error
			var zero T
			return zero, fmt.Errorf("future channel closed unexpectedly")
		}
		// Result should already be cached by resolve()
		if f.result == nil {
			f.result = &result
		}
		return result.Value, result.Err
	}
}

// resolve sets the result of the Future and closes the channel, signaling completion
func (f *Future[T]) resolve(value T, err error) {
	f.once.Do(func() {
		result := Result[T]{
			Value: value,
			Err:   err,
		}
		f.result = &result // cache the result first
		f.ch <- result     // Send the result and close the channel
		close(f.ch)        // Close the channel to signal completion
	})
}
