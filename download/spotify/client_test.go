package spotify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sv4u/spotigo/v2"
)

func TestNewSpotifyClient_InvalidCredentials(t *testing.T) {
	config := &Config{
		ClientID:     "",
		ClientSecret: "",
	}

	_, err := NewSpotifyClient(config)
	if err == nil {
		t.Error("Expected error for invalid credentials")
	}
}

func TestSpotifyClient_GetRateLimitInfo(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Initially, no rate limit should return nil (this is expected)
	_ = client.GetRateLimitInfo()

	// Simulate a rate limit by updating the tracker directly
	client.rateLimitTracker.Update(10)
	info := client.GetRateLimitInfo()
	if info == nil {
		t.Error("GetRateLimitInfo() should return info when rate limit is active")
	}
	if info != nil && !info.Active {
		t.Error("Expected Active to be true")
	}
}

func TestSpotifyClient_ClearCache(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		CacheMaxSize: 10,
		CacheTTL:     3600,
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Add something to cache
	client.cache.Set("test:key", "test:value")

	// Clear cache
	client.ClearCache()

	// Verify cache is empty
	if client.cache.Size() != 0 {
		t.Error("Cache should be empty after ClearCache()")
	}
}

func TestSpotifyClient_GetCacheStats(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		CacheMaxSize: 10,
		CacheTTL:     3600,
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	stats := client.GetCacheStats()
	if stats.MaxSize != 10 {
		t.Errorf("Expected MaxSize 10, got %d", stats.MaxSize)
	}
}

func TestSpotifyClient_Close(t *testing.T) {
	config := &Config{
		ClientID:             "test_id",
		ClientSecret:         "test_secret",
		CacheCleanupInterval: 1 * time.Second,
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close should stop cleanup
	client.Close()

	// Verify cleanup is stopped (no panic on second close)
	client.Close()
}

func TestSpotifyClient_HandleError_Nil(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	err2 := client.handleError(nil)
	if err2 != nil {
		t.Errorf("Expected nil error, got %v", err2)
	}
}

func TestSpotifyClient_HandleError_RateLimit(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Use a real spotigo.SpotifyError with StatusCode() method
	rateLimitErr := &spotigo.SpotifyError{
		HTTPStatus: 429,
		Code:       429,
		Message:    "Too Many Requests",
	}

	err2 := client.handleError(rateLimitErr)
	if err2 == nil {
		t.Error("Expected rate limit error")
	}

	if _, ok := err2.(*RateLimitError); !ok {
		t.Errorf("Expected RateLimitError, got %T", err2)
	}
}

func TestSpotifyClient_HandleError_RegularError(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	regularErr := errors.New("some error")
	err2 := client.handleError(regularErr)
	if err2 == nil {
		t.Error("Expected error")
	}

	if _, ok := err2.(*SpotifyError); !ok {
		t.Errorf("Expected SpotifyError, got %T", err2)
	}
}

func TestSpotifyClient_HandleError_SpotifyNon429(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	notFoundErr := &spotigo.SpotifyError{
		HTTPStatus: 404,
		Code:       404,
		Message:    "Not Found",
	}

	err2 := client.handleError(notFoundErr)
	if err2 == nil {
		t.Error("Expected error")
	}

	if _, ok := err2.(*SpotifyError); !ok {
		t.Errorf("Expected SpotifyError (not RateLimitError), got %T", err2)
	}
}

func TestSpotifyClient_ApplyRateLimiting_ContextCancellation(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = client.applyRateLimiting(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestSpotifyClient_ApplyRateLimiting_GeneralRateLimiter(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		GeneralRateLimiter: &mockRateLimiter{
			shouldError: false,
		},
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	err = client.applyRateLimiting(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestSpotifyClient_ApplyRateLimiting_GeneralRateLimiterError(t *testing.T) {
	expectedErr := errors.New("rate limiter error")
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		GeneralRateLimiter: &mockRateLimiter{
			shouldError: true,
			err:         expectedErr,
		},
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	err = client.applyRateLimiting(ctx)
	if err != expectedErr {
		t.Errorf("Expected rate limiter error, got %v", err)
	}
}

// Mock implementations for testing

type mockRateLimiter struct {
	shouldError bool
	err         error
}

func (m *mockRateLimiter) WaitForRequest(ctx context.Context) error {
	if m.shouldError {
		if m.err != nil {
			return m.err
		}
		return errors.New("rate limiter error")
	}
	return nil
}
