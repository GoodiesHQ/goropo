package goropo

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const MIN_WORKER_COUNT = 1
const MIN_QUEUE_SIZE = 0

// Generic function that will be executed within a task
type fnRunner func()

// Generic function that will be executed when a task is dropped
type fnOnDrop func()

type Task[T any] func(context.Context) (T, error)
type TaskAny = Task[any]

var (
	ErrPoolClosed  = errors.New("pool is closed")             // pool has been closed, no new tasks can be submitted
	ErrPoolAborted = errors.New("pool aborted, task dropped") // pool has been aborted, task was dropped and failed to complete
)

type StopMode int

const (
	StopModeGraceful StopMode = iota // gracefully stop the pool, allowing all queued tasks to complete but no new tasks
	StopModeAbort    StopMode = iota // immediately abort the pool, finish currently running tasks but drop all unresolved queued tasks
)

// Task is a wrapper around a function to be executed by the worker pool
type workerTask struct {
	run    fnRunner // wrapped function to execute the submitted task
	onDrop fnOnDrop // function to perform if/when the task is dropped by the pool (e.g. aborted)
}

type Pool struct {
	mu          sync.Mutex
	wg          sync.WaitGroup  // wait group to track active workers
	chTasks     chan workerTask // channel of tasks to be executed by the worker pool
	chClosed    chan struct{}   // channel to signal that the pool has been closed (gracefully or aborted)
	chAbort     chan struct{}   // channel to signal immediate abortion of workers
	stopped     atomic.Bool     // indicates whether the pool has been stopped
	onPanic     func(any)       // optional handler for panics occurring within task execution
	workerCount int             // number of worker goroutines in the pool
	queueSize   int             // maximum size of the task queue
	// ctx         context.Context    // context to manage cancellation of workers
	// cancel      context.CancelFunc // function to cancel the pool's context
}

// SetHandlerPanic sets a handler function to be called when a panic occurs within a task
// The handler receives the recovered panic value as an argument
func (p *Pool) SetHandlerPanic(handler func(any)) {
	defer Locker(&p.mu)()
	p.onPanic = handler
}

// worker is the main loop for each worker goroutine in the pool
func (p *Pool) worker() {
	// signal that this worker is done when the function exits
	defer p.wg.Done()

	for {
		select {
		case <-p.chAbort: // check for abort signal
			return
		case t, ok := <-p.chTasks: // get the next task from the channel
			if !ok {
				// channel closed, exit the worker
				return
			}
			// execute the task
			func() {
				// recover from panics within the task execution to ensure the worker continues running
				defer func() {
					if r := recover(); r != nil && p.onPanic != nil {
						p.onPanic(r)
					}
				}()
				t.run()
			}()
		}
	}
}

// NewPool creates a new worker pool with the specified number of workers and queue size
func NewPool(workerCount, queueSize int) *Pool {
	// enforce minimums
	if workerCount < MIN_WORKER_COUNT {
		workerCount = MIN_WORKER_COUNT
	}
	if queueSize < MIN_QUEUE_SIZE {
		queueSize = MIN_QUEUE_SIZE
	}

	// create the pool instance
	p := &Pool{
		wg:          sync.WaitGroup{},
		workerCount: workerCount,
		queueSize:   queueSize,
	}

	p.reset()

	return p
}

// reset initializes or re-initializes the pool's internal state
func (p *Pool) reset() {
	p.chTasks = make(chan workerTask, p.queueSize)
	p.chAbort = make(chan struct{})
	p.chClosed = make(chan struct{})
	p.stopped.Store(false)

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// Wait blocks until all workers have completed their tasks
func (p *Pool) Wait() {
	p.wg.Wait()
}

// Stop stops the worker pool according to the specified StopMode
func (p *Pool) Stop(mode StopMode) {
	defer Locker(&p.mu)()
	if p.stopped.Swap(true) {
		return
	}

	// signal that the pool is closed
	close(p.chClosed)

	switch mode {
	case StopModeGraceful:
		// close the tasks channel to stop accepting new tasks, but allow existing tasks to complete
		close(p.chTasks)
	case StopModeAbort:
		// close the abort channel to signal immediate termination of all workers
		close(p.chAbort)
	drainLoop:
		for {
			// drain the remaining queued tasks and invoke their onDrop handlers
			select {
			case t, ok := <-p.chTasks:
				// if the channel is closed and empty, exit the loop
				if !ok {
					break drainLoop
				}
				// invoke the onDrop handler if it exists
				if t.onDrop != nil {
					t.onDrop()
				}
			default:
				break drainLoop
			}
		}
		// close the tasks channel to stop accepting new tasks
		close(p.chTasks)

	}
	p.wg.Wait()
}

// Close is a convenience method that stops the pool gracefully after completing all queued tasks
func (p *Pool) Close() {
	p.Stop(StopModeGraceful)
}

// Abort is a convenience method that aborts the pool immediately and drops queued tasks
func (p *Pool) Abort() {
	p.Stop(StopModeAbort)
}

// Submit submits a task to the worker pool for execution, returning a Future holding an any type
func (p *Pool) Submit(ctx context.Context, fn func(context.Context) (any, error)) *FutureAny {
	return Submit(p, ctx, fn)
}
