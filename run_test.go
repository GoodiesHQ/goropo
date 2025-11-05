package goropo

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// Go is a convenience function that creates a new goroutine to execute the given function, no pool involved
func TestRun_Go(t *testing.T) {
	var complete atomic.Bool
	complete.Store(false)
	ch := make(chan struct{})
	ctx := context.Background()

	fut := Go[any](ctx, func(ctx context.Context) (any, error) {
		complete.Store(true)
		close(ch)
		return nil, nil
	})

	<-ch

	if !complete.Load() {
		t.Fatal("Expected task to be completed")
	}

	_, err := fut.Await(context.Background())
	if err != nil {
		t.Fatalf("Expected no error from future, got: %v", err)
	}
}

func TestRun_GoPanic(t *testing.T) {
	var complete atomic.Bool
	complete.Store(false)
	ch := make(chan struct{})
	ctx := context.Background()

	fut := Go[any](ctx, func(ctx context.Context) (any, error) {
		defer close(ch)
		// ensure panic happens before complete is set
		if !complete.Load() {
			panic("panic")
		}
		complete.Store(true)
		return nil, nil
	})

	<-ch

	_, err := fut.Await(context.Background())
	if !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("Expected ErrTaskPanicked, got %v", err)
	}

	if complete.Load() {
		t.Fatal("Expected task to be incomplete")
	}
}
