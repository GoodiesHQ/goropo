# goropo
**Go** **Ro**utine **Po**ol - Simple, generic buffered worker pool for executing tasks in parallel with ease.


### Future and Result

A `Future[T]` is a generic promise from the worker pool (or background goroutine) that will resolve once the underlying task has been completed. A `Future[T]` is returned during the following functions:

    goropo.Submit[T](...)    // submits a new task to the queue, blocks until the task is queued
    goropo.TrySubmit[T](...) // non-blocking attempt to submit a task to the queue, returns `ErrPoolQueueFull` if not
    goropo.Go[T](...)        // runs a background worker function in a new goroutine without a pool


A `Result[T]` is a simple structure that is resulted from a resolved future and contains two public attributes.

    type Result[T any] struct {
        Value T
        Err   error
    }

