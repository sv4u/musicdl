package spotify

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
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
	info := client.GetRateLimitInfo()
	// nil is valid when there's no active rate limit

	// Simulate a rate limit by updating the tracker directly
	client.rateLimitTracker.Update(10)
	info = client.GetRateLimitInfo()
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
		ClientID:            "test_id",
		ClientSecret:        "test_secret",
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

	// Create a mock rate limit error
	rateLimitErr := &mockHTTPError{
		statusCode: http.StatusTooManyRequests,
		message:    "429 Too Many Requests",
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

func TestSpotifyClient_IsRateLimitError_HTTP429(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	rateLimitErr := &mockHTTPError{
		statusCode: http.StatusTooManyRequests,
		message:    "429 Too Many Requests",
	}

	if !client.isRateLimitError(rateLimitErr) {
		t.Error("Expected isRateLimitError to return true for HTTP 429")
	}
}

func TestSpotifyClient_IsRateLimitError_Message(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test error message containing "429"
	err1 := errors.New("HTTP 429 error")
	if !client.isRateLimitError(err1) {
		t.Error("Expected isRateLimitError to return true for error with '429'")
	}

	// Test error message containing "rate limit"
	err2 := errors.New("rate limit exceeded")
	if !client.isRateLimitError(err2) {
		t.Error("Expected isRateLimitError to return true for error with 'rate limit'")
	}

	// Test error message containing "too many requests"
	err3 := errors.New("too many requests")
	if !client.isRateLimitError(err3) {
		t.Error("Expected isRateLimitError to return true for error with 'too many requests'")
	}
}

func TestSpotifyClient_ExtractRetryAfter_WithRetryAfter(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	mockErr := &mockHTTPError{
		statusCode:  http.StatusTooManyRequests,
		retryAfter:  5,
		message:     "429 Too Many Requests",
	}

	retryAfter := client.extractRetryAfter(mockErr)
	if retryAfter != 5 {
		t.Errorf("Expected retryAfter 5, got %d", retryAfter)
	}
}

func TestSpotifyClient_ExtractRetryAfter_Default(t *testing.T) {
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
	retryAfter := client.extractRetryAfter(regularErr)
	if retryAfter != 1 {
		t.Errorf("Expected default retryAfter 1, got %d", retryAfter)
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

type mockHTTPError struct {
	statusCode int
	retryAfter int
	message    string
}

func (e *mockHTTPError) StatusCode() int {
	return e.statusCode
}

func (e *mockHTTPError) RetryAfter() int {
	return e.retryAfter
}

func (e *mockHTTPError) Error() string {
	return e.message
}

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
