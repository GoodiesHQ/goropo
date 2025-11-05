package goropo

import "errors"

var (
	ErrPoolClosed    = errors.New("pool is closed, task rejected")     // pool has been closed, no new tasks can be submitted
	ErrPoolAborted   = errors.New("pool aborted, task dropped")        // pool has been aborted, task was dropped and failed to complete
	ErrPoolQueueFull = errors.New("pool queue is full, task rejected") // task queue is full, cannot accept new tasks (only used for TrySubmit)
	ErrTaskPanicked  = errors.New("task panicked during execution")    // task panicked during execution
)
