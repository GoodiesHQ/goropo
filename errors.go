package goropo

import (
	"errors"
	"fmt"
)

var (
	ErrPoolClosed    = errors.New("pool is closed, task rejected")     // pool has been closed, no new tasks can be submitted
	ErrPoolAborted   = errors.New("pool aborted, task dropped")        // pool has been aborted, task was dropped and failed to complete
	ErrPoolQueueFull = errors.New("pool queue is full, task rejected") // task queue is full, cannot accept new tasks (only used for TrySubmit)
	ErrTaskPanicked  = errors.New("task panicked during execution")    // task panicked during execution
)

type TaskPanicError struct {
	Recovered any
	Cause     error
}

func (e *TaskPanicError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%v: %v", ErrTaskPanicked, e.Cause)
	}
	return fmt.Sprintf("%v: %v", ErrTaskPanicked, e.Recovered)
}

func (e *TaskPanicError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrTaskPanicked, e.Cause}
	}
	return []error{ErrTaskPanicked}
}

func IsTaskPanickedError(err error) bool {
	return errors.Is(err, ErrTaskPanicked)
}

func AsTaskPanickedError(err error) (*TaskPanicError, bool) {
	var target *TaskPanicError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func NewTaskPanicError(recovered any) *TaskPanicError {
	var cause error
	if err, ok := recovered.(error); ok {
		cause = err
	}

	return &TaskPanicError{
		Recovered: recovered,
		Cause:     cause,
	}
}
