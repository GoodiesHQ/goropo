package goropo

import (
	"context"
	"errors"
	"testing"
)

func TestTask_TaskRun(t *testing.T) {
	ctx := context.Background()
	fut := newFuture[int]()

	task := newWorkerTask(ctx, fut, func(ctx context.Context) (int, error) {
		return 73, nil
	})

	task.run()
	value, err := fut.Await(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if value != 73 {
		t.Fatalf("Expected value 73, got %d", value)
	}
}

func TestTask_TaskDrop(t *testing.T) {
	ctx := context.Background()
	fut := newFuture[int]()

	task := newWorkerTask(ctx, fut, func(ctx context.Context) (int, error) {
		return 73, nil
	})

	task.onDrop()
	value, err := fut.Await(ctx)
	if !errors.Is(err, ErrPoolAborted) {
		t.Fatalf("Expected no error, got %v", err)
	}

	if value == 73 {
		t.Fatalf("Expected value 73, got %d", value)
	}
}

func TestTask_TaskPanic(t *testing.T) {
	ctx := context.Background()
	fut := newFuture[int]()

	task := newWorkerTask(ctx, fut, func(ctx context.Context) (int, error) {
		if true {
			panic("panic")
		}
		return 73, nil
	})

	func() {
		defer func() {
			if r := recover(); r != nil {
				task.onPanic(r)
			}
		}()
		task.run()
	}()

	result, err := fut.Await(ctx)
	if !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("Expected ErrTaskPanicked, got %v", err)
	}

	if result == 73 {
		t.Fatalf("Expected no result, got %d", result)
	}

	var panicErr *TaskPanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("Expected error to be of type TaskPanicError, got %T", err)
	}

	if r, ok := panicErr.Recovered.(string); !ok || r != "panic" {
		t.Fatalf("Expected recovered value 'panic', got %v", panicErr.Recovered)
	}
}
