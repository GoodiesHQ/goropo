package goropo

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_NewPool(t *testing.T) {
	pool := NewPool(3, 10)

	if pool.workerCount != 3 {
		t.Fatalf("Expected workerCount 3, got %d", pool.workerCount)
	}
	if pool.queueSize != 10 {
		t.Fatalf("Expected queueSize 10, got %d", pool.queueSize)
	}

	// Test minimum enforcement
	smallPool := NewPool(MIN_WORKER_COUNT-1, MIN_QUEUE_SIZE-1)
	if smallPool.workerCount != MIN_WORKER_COUNT {
		t.Fatalf("Expected minimum workerCount %d, got %d", MIN_WORKER_COUNT, smallPool.workerCount)
	}
	if smallPool.queueSize != MIN_QUEUE_SIZE {
		t.Fatalf("Expected minimum queueSize %d, got %d", MIN_QUEUE_SIZE, smallPool.queueSize)
	}
}

func TestPool_SubmitSuccess(t *testing.T) {
	pool := NewPool(2, 5)
	defer pool.Close()

	ctx := context.Background()
	fut := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		return "success", nil
	})

	result, err := fut.Await(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "success" {
		t.Fatalf("Expected 'success', got '%s'", result)
	}
}

func TestPool_SubmitError(t *testing.T) {
	pool := NewPool(2, 5)
	defer pool.Close()

	ctx := context.Background()
	expectedErr := errors.New("task error")

	fut := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		return "", expectedErr
	})

	result, err := fut.Await(ctx)
	if err != expectedErr {
		t.Fatalf("Expected error %v, got %v", expectedErr, err)
	}
	if result != "" {
		t.Fatalf("Expected empty result, got '%s'", result)
	}
}

func TestPool_StopGraceful(t *testing.T) {
	pool := NewPool(2, 5)

	ctx := context.Background()
	var completed int
	var mu sync.Mutex

	// Submit several tasks
	futures := make([]*Future[int], 5)
	for i := 0; i < 5; i++ {
		i := i
		fut := Submit(pool, ctx, func(ctx context.Context) (int, error) {
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			completed++
			mu.Unlock()
			return i, nil
		})
		futures[i] = fut
	}

	// Stop gracefully - should wait for all tasks to complete
	pool.Stop(StopModeGraceful)

	// All tasks should have completed
	mu.Lock()
	if completed != 5 {
		t.Fatalf("Expected 5 completed tasks, got %d", completed)
	}
	mu.Unlock()

	// All futures should have results
	for i, fut := range futures {
		result, err := fut.Await(ctx)
		if err != nil {
			t.Fatalf("Task %d failed: %v", i, err)
		}
		if result != i {
			t.Fatalf("Task %d: expected %d, got %d", i, i, result)
		}
	}
}

func TestPool_StopAbort(t *testing.T) {
	pool := NewPool(1, 10) // Only 1 worker to ensure sequential processing

	ctx := context.Background()
	var started, completed int
	var mu sync.Mutex

	// Submit tasks that take time
	futures := make([]*Future[int], 5)
	for i := 0; i < 5; i++ {
		i := i
		fut := Submit(pool, ctx, func(ctx context.Context) (int, error) {
			mu.Lock()
			started++
			mu.Unlock()

			time.Sleep(150 * time.Millisecond) // Long enough to be aborted

			mu.Lock()
			completed++
			mu.Unlock()

			return i, nil
		})
		futures[i] = fut
	}

	// Give time for first task to start
	time.Sleep(50 * time.Millisecond)

	// Stop with abort - should cancel remaining tasks
	pool.Stop(StopModeAbort)

	mu.Lock()
	defer mu.Unlock()

	// At least one task should have started
	if started == 0 {
		t.Fatal("Expected at least one task to start")
	}

	// Not all tasks should have completed (some should be aborted)
	if completed == 5 {
		t.Fatal("Expected some tasks to be aborted, but all completed")
	}

	// Check results - first task(s) should succeed, later ones should fail
	for i, fut := range futures {
		result, err := fut.Await(ctx)
		if i < completed {
			// Tasks that completed should have results
			if err != nil {
				t.Fatalf("Task %d should have succeeded, got error: %v", i, err)
			}
			if result != i {
				t.Fatalf("Task %d: expected %d, got %d", i, i, result)
			}
		} else {
			// Tasks that were aborted should have ErrPoolAborted
			if err != ErrPoolAborted {
				t.Fatalf("Task %d should have been aborted, got: %v", i, err)
			}
		}
	}
}

func TestPool_SubmitAfterClose(t *testing.T) {
	pool := NewPool(2, 5)
	pool.Close() // Close the pool

	ctx := context.Background()
	fut := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		return "should not run", nil
	})

	result, err := fut.Await(ctx)
	if err != ErrPoolClosed {
		t.Fatalf("Expected ErrPoolClosed, got %v", err)
	}
	if result != "" {
		t.Fatalf("Expected empty result, got '%s'", result)
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	pool := NewPool(2, 5)
	defer pool.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Submit a task that would take time
	fut := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return "completed", nil
		}
	})

	// Cancel context quickly
	cancel()

	result, err := fut.Await(context.Background())
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled, got %v", err)
	}
	if result != "" {
		t.Fatalf("Expected empty result, got '%s'", result)
	}
}

func TestPool_QueueFull(t *testing.T) {
	// Create pool with very small queue
	pool := NewPool(1, 1) // 1 worker, queue size 1
	defer pool.Close()

	ctx := context.Background()

	// Submit first task - should succeed
	fut1 := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "task1", nil
	})
	fut2 := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "task2", nil
	})

	fut3, err := TrySubmit(pool, ctx, func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "task3", nil
	})
	if err != ErrPoolQueueFull {
		t.Fatalf("Expected ErrPoolQueueFull, got %v", err)
	}

	result1, err1 := fut1.Await(ctx)
	if err1 != nil || result1 != "task1" {
		t.Fatalf("Task1 failed: %v, result: %s", err1, result1)
	}

	result2, err2 := fut2.Await(ctx)
	if err2 != nil || result2 != "task2" {
		t.Fatalf("Task2 failed: %v, result: %s", err2, result2)
	}

	if fut3 != nil {
		t.Fatal("Expected fut3 to be nil due to queue full")
	}
}

func TestGo_Function(t *testing.T) {
	ctx := context.Background()

	fut := Go(ctx, func(ctx context.Context) (string, error) {
		return "goroutine result", nil
	})

	result, err := fut.Await(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "goroutine result" {
		t.Fatalf("Expected 'goroutine result', got '%s'", result)
	}
}

func TestGo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fut := Go(ctx, func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return "should not complete", nil
		}
	})

	cancel()

	result, err := fut.Await(context.Background())
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled, got %v", err)
	}
	if result != "" {
		t.Fatalf("Expected empty result, got '%s'", result)
	}
}

func TestPool_TaskPanic(t *testing.T) {
	pool := NewPool(2, 5)
	ctx := context.Background()

	var complete atomic.Bool
	complete.Store(false)

	handler := func(recovered any) {
		// Just log the panic for test purposes
		if recovered == nil {
			t.Fatal("Expected non-nil panic recovery")
		}
		if recovered.(string) != "panic" {
			t.Fatalf("Expected panic message 'panic', got '%v'", recovered)
		}

		complete.Store(true)
	}
	workload := func(ctx context.Context) (string, error) {
		panic("panic")
	}

	pool.SetPanicHandler(handler)
	fut := Submit(pool, ctx, workload)
	pool.Stop(StopModeGraceful)
	_, err := fut.Await(ctx)
	if !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("Expected ErrTaskPanicked, got %v", err)
	}

	if !complete.Load() {
		t.Fatal("Expected panic handler to be invoked")
	}

	pool.Wait()
}

func TestPool_Wait(t *testing.T) {
	pool := NewPool(2, 5)
	ctx := context.Background()

	var complete atomic.Bool
	complete.Store(false)

	fut := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		complete.Store(true)
		return "done", nil
	})

	pool.Stop(StopModeGraceful)

	if !complete.Load() {
		t.Fatal("Expected task to be completed before Wait returns")
	}

	result, err := fut.Await(ctx)
	if err != nil {
		t.Fatalf("Expected no error from future, got: %v", err)
	}
	if result != "done" {
		t.Fatalf("Expected result 'done', got: %s", result)
	}
}

func TestPool_WaitIdle(t *testing.T) {
	pool := NewPool(2, 5)
	ctx := context.Background()

	var complete atomic.Bool
	complete.Store(false)

	fut := Submit(pool, ctx, func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		complete.Store(true)
		return "done", nil
	})

	pool.WaitIdle()

	if !complete.Load() {
		t.Fatal("Expected task to be completed before WaitIdle returns")
	}

	if fut.IsComplete() == false {
		t.Fatal("Expected future to be done after WaitIdle")
	}

	result, err := fut.Await(ctx)
	if err != nil {
		t.Fatalf("Expected no error from future, got: %v", err)
	}
	if result != "done" {
		t.Fatalf("Expected result 'done', got: %s", result)
	}

	pool.Close()
	pool.Wait()
}
