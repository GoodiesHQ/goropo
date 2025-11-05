package goropo

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestFuture_Success(t *testing.T) {
	fut := newFuture[string]()

	// Resolve the future in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		fut.resolve("success", nil)
	}()

	// Await the result
	result, err := fut.Await(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "success" {
		t.Fatalf("Expected 'success', got '%s'", result)
	}
}

func TestFuture_Error(t *testing.T) {
	fut := newFuture[string]()
	expectedErr := errors.New("test error")

	// Resolve with error
	go func() {
		time.Sleep(10 * time.Millisecond)
		var zero string
		fut.resolve(zero, expectedErr)
	}()

	result, err := fut.Await(context.Background())
	if err != expectedErr {
		t.Fatalf("Expected error %v, got %v", expectedErr, err)
	}
	if result != "" {
		t.Fatalf("Expected empty result, got '%s'", result)
	}
}

func TestFuture_IsComplete(t *testing.T) {
	fut := newFuture[string]()

	// Initially not complete
	if fut.IsComplete() {
		t.Fatal("Future should not be complete initially")
	}

	// Resolve the future
	fut.resolve("done", nil)

	// Now should be complete
	if !fut.IsComplete() {
		t.Fatal("Future should be complete after resolution")
	}

	// Multiple calls to IsComplete should work
	if !fut.IsComplete() {
		t.Fatal("Future should remain complete")
	}
}

func TestFuture_AwaitAfterComplete(t *testing.T) {
	fut := newFuture[string]()

	// Resolve first
	fut.resolve("cached", nil)

	// Multiple awaits should return the same result immediately
	for i := 0; i < 3; i++ {
		result, err := fut.Await(context.Background())
		if err != nil {
			t.Fatalf("Expected no error on await %d, got %v", i, err)
		}
		if result != "cached" {
			t.Fatalf("Expected 'cached' on await %d, got '%s'", i, result)
		}
	}
}

func TestFuture_ContextCancellation(t *testing.T) {
	fut := newFuture[string]()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context before awaiting
	cancel()

	result, err := fut.Await(ctx)
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled, got %v", err)
	}
	if result != "" {
		t.Fatalf("Expected empty result on cancellation, got '%s'", result)
	}
}

func TestFuture_ResultType(t *testing.T) {
	type s struct {
		x int
		y string
		z bool
	}

	isSlice := func(a any) bool {
		return reflect.TypeOf(a).Kind() == reflect.Slice
	}

	// Test with different types
	testCases := []struct {
		name     string
		value    any
		expected any
	}{
		{"string", "hello", "hello"},
		{"int", 73, 73},
		{"bool", true, true},
		{"struct", s{73, "hello", true}, s{73, "hello", true}},
		{"slice", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fut := newFuture[any]()

			go func() {
				fut.resolve(tc.value, nil)
			}()

			result, err := fut.Await(context.Background())
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if isSlice(tc.expected) {
				// For slices, use DeepEqual
				if !reflect.DeepEqual(result, tc.expected) {
					t.Fatalf("Expected %v, got %v", tc.expected, result)
				}
			} else {
				if result != tc.expected {
					t.Fatalf("Expected %v, got %v", tc.expected, result)
				}
			}
		})
	}
}

func TestResult_Ok(t *testing.T) {
	// Success result
	success := Result[string]{Value: "ok", Err: nil}
	if !success.Ok() {
		t.Fatal("Success result should be Ok()")
	}

	// Error result
	failure := Result[string]{Value: "", Err: errors.New("fail")}
	if failure.Ok() {
		t.Fatal("Error result should not be Ok()")
	}
}
