package goropo

import (
	"context"
)

// Submit submits a task to the worker pool for execution
func Submit[T any](p *Pool, ctx context.Context, fn Task[T]) *Future[T] {
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

	// increment outstanding before the send
	p.outstanding.Add(1)

	select {
	case <-p.chClosed:
		p.outstanding.Add(-1)
		var zero T
		fut.resolve(zero, ErrPoolClosed)
	case <-ctx.Done():
		p.outstanding.Add(-1)
		var zero T
		fut.resolve(zero, ctx.Err())
	case p.chTasks <- task: // submit the task to the pool
	}
	return fut
}

// TrySubmit attempts to submit a task to the worker pool without blocking
// Returns ErrPoolQueueFull if the queue is full and the task cannot be queued immediately
func TrySubmit[T any](p *Pool, ctx context.Context, fn Task[T]) (*Future[T], error) {
	// use background context if none provided
	if ctx == nil {
		ctx = context.Background()
	}

	fut := newFuture[T]()

	// check if the pool has been closed before submitting the task
	if p.IsClosed() {
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

	// increment outstanding before the non-blocking send attempt
	p.outstanding.Add(1)

	select {
	case p.chTasks <- task:
		return fut, nil
	default:
		p.outstanding.Add(-1)
		return nil, ErrPoolQueueFull
	}
}

// Go is a convenience function that creates a new goroutine to execute the given function, no pool involved
func Go[T any](ctx context.Context, fn Task[T]) *Future[T] {
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
				f.resolve(zero, NewTaskPanicError(r))
			}
		}()
		// execute the function and resolve the future with its result
		value, err := fn(ctx)
		f.resolve(value, err)
	}()
	return f
}
