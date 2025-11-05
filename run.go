package goropo

import (
	"context"
	"fmt"
)

// Submit submits a task to the worker pool for execution
func Submit[T any](p *Pool, ctx context.Context, fn func(context.Context) (T, error)) *Future[T] {
	// use background context if none provided
	if ctx == nil {
		ctx = context.Background()
	}

	// create a new future to hold the result of the task
	fut := newFuture[T]()

	// wrap the function to be executed in a task
	task := newWorkerTask(ctx, fut, fn)

	// check if the pool has been closed before submitting the task
	if p.IsClosed() {
		var zero T
		fut.resolve(zero, ErrPoolClosed)
		return fut
	}

	select {
	case <-p.chClosed:
		var zero T
		fut.resolve(zero, ErrPoolClosed)
	case <-ctx.Done():
		var zero T
		fut.resolve(zero, ctx.Err())
	case p.chTasks <- task: // submit the task to the pool and wait
	}
	return fut
}

// TrySubmit attempts to submit a task to the worker pool without blocking
// Returns ErrPoolQueueFull if the queue is full and the task cannot be queued immediately
func TrySubmit[T any](p *Pool, ctx context.Context, fn func(context.Context) (T, error)) (*Future[T], error) {
	// use background context if none provided
	if ctx == nil {
		ctx = context.Background()
	}

	fut := newFuture[T]()

	// check if the pool has been closed before submitting the task
	if p.IsClosed() {
		// create a new future to hold the result of the task
		var zero T
		fut.resolve(zero, ErrPoolClosed)
		return fut, nil
	}

	select {
	case <-p.chClosed:
		var zero T
		fut.resolve(zero, ErrPoolClosed)
		return fut, nil
	case <-ctx.Done():
		var zero T
		fut.resolve(zero, ctx.Err())
		return fut, nil
	default:
		break
	}

	task := newWorkerTask(ctx, fut, fn)

	select {
	case p.chTasks <- task:
		return fut, nil
	default:
		return nil, ErrPoolQueueFull
	}
}

// Go is a convenience function that creates a new goroutine to execute the given function, no pool involved
func Go[T any](ctx context.Context, fn func(context.Context) (T, error)) *Future[T] {
	f := newFuture[T]()
	go func() {
		// check for context cancellation before executing the function
		if err := ctx.Err(); err != nil {
			var zero T
			f.resolve(zero, err)
			return
		}
		defer func() {
			if r := recover(); r != nil {
				var zero T
				f.resolve(zero, fmt.Errorf("%w: %v", ErrTaskPanicked, r))
			}
		}()
		// execute the function and resolve the future with its result
		value, err := fn(ctx)
		f.resolve(value, err)
	}()
	return f
}
