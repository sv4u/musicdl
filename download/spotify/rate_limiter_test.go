package spotify

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_Disabled(t *testing.T) {
	rl := NewRateLimiter(false, 10, 1.0)

	// Should not block when disabled
	start := time.Now()
	err := rl.WaitIfNeeded(context.Background())
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if duration > 100*time.Millisecond {
		t.Error("Should not wait when disabled")
	}
}

func TestRateLimiter_Basic(t *testing.T) {
	rl := NewRateLimiter(true, 2, 1.0) // 2 requests per second

	// First two requests should not block
	start := time.Now()
	rl.WaitIfNeeded(context.Background())
	rl.WaitIfNeeded(context.Background())
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Error("First requests should not block")
	}

	// Third request should block
	start = time.Now()
	rl.WaitIfNeeded(context.Background())
	duration = time.Since(start)

	if duration < 900*time.Millisecond {
		t.Errorf("Third request should block for ~1 second, blocked for %v", duration)
	}
}

func TestRateLimiter_WindowSliding(t *testing.T) {
	rl := NewRateLimiter(true, 2, 1.0) // 2 requests per second

	// Make 2 requests
	rl.WaitIfNeeded(context.Background())
	rl.WaitIfNeeded(context.Background())

	// Wait for window to slide
	time.Sleep(1100 * time.Millisecond)

	// Should be able to make 2 more requests immediately
	start := time.Now()
	rl.WaitIfNeeded(context.Background())
	rl.WaitIfNeeded(context.Background())
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Error("Should not block after window slides")
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := NewRateLimiter(true, 1, 1.0) // 1 request per second

	// Make first request
	rl.WaitIfNeeded(context.Background())

	// Second request should block, but we'll cancel context
	ctx, cancel := context.WithCancel(context.Background())
	
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- rl.WaitIfNeeded(ctx)
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should return context error
	err := <-done
	duration := time.Since(start)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
	if duration > 200*time.Millisecond {
		t.Error("Should cancel quickly")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(true, 10, 1.0) // 10 requests per second
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Make 20 concurrent requests
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.WaitIfNeeded(context.Background()); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}
}
