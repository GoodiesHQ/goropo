package goropo

import (
	"context"
)

// Task is a wrapper around a function to be executed by the worker pool
type workerTask struct {
	run     fnRunner  // wrapped function to execute the submitted task
	onDrop  fnOnDrop  // function to perform if/when the task is dropped by the pool (e.g. aborted)
	onPanic fnOnPanic // panic handler specific to this task
}

func newWorkerTask[T any](ctx context.Context, fut *Future[T], fn func(context.Context) (T, error)) workerTask {
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

	// function to call if the task panics during execution
	onPanic := func(r any) {
		var zero T
		fut.resolve(zero, NewTaskPanicError(r))
	}

	return workerTask{
		run:     run,
		onDrop:  onDrop,
		onPanic: onPanic,
	}
}
