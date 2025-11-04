package goropo

import "context"

// Submit submits a task to the worker pool for execution
func Submit[T any](p *Pool, ctx context.Context, fn func(context.Context) (T, error)) *Future[T] {
	// create a new future to hold the result of the task
	fut := newFuture[T]()

	run := func() {
		// check for submission context cancellation before executing the function
		if err := ctx.Err(); err != nil {
			var zero T
			fut.resolve(zero, err)
			return
		}

		// pass the submission context to the function and execute it
		value, err := fn(ctx)

		// resolve the future with the function's result
		fut.resolve(value, err)
	}

	// function to call if the task is dropped (e.g. pool aborted)
	onDrop := func() {
		var zero T
		fut.resolve(zero, ErrPoolAborted) // inform that the task was dropped
	}

	// wrap the function to be executed in a task
	task := workerTask{
		run:    run,
		onDrop: onDrop,
	}

	// check if the pool has been closed before submitting the task
	select {
	case <-p.chClosed:
		var zero T
		fut.resolve(zero, ErrPoolClosed)
		return fut
	default:
	}

	select {
	// check if the pool has been closed
	case <-p.chAbort:
		var zero T
		fut.resolve(zero, ErrPoolClosed)
	// check if the context has been cancelled
	case <-ctx.Done():
		var zero T
		fut.resolve(zero, ctx.Err())
	// submit the task to the pool
	case p.chTasks <- task:
	}
	return fut
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
		// execute the function and resolve the future with its result
		value, err := fn(ctx)
		f.resolve(value, err)
	}()
	return f
}
