package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState string

const (
	CircuitClosed   CircuitBreakerState = "closed"   // Normal operation
	CircuitOpen     CircuitBreakerState = "open"      // Failing, reject requests
	CircuitHalfOpen CircuitBreakerState = "half_open" // Testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern for download operations.
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitBreakerState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	lastStateChange  time.Time
}

// CircuitBreakerStatus is the JSON-serializable status of the circuit breaker.
type CircuitBreakerStatus struct {
	State            string `json:"state"`
	FailureCount     int    `json:"failureCount"`
	SuccessCount     int    `json:"successCount"`
	FailureThreshold int    `json:"failureThreshold"`
	SuccessThreshold int    `json:"successThreshold"`
	ResetTimeoutSec  int    `json:"resetTimeoutSec"`
	LastFailureAt    int64  `json:"lastFailureAt"`
	LastStateChange  int64  `json:"lastStateChange"`
	CanRetry         bool   `json:"canRetry"`
}

// NewCircuitBreaker creates a new circuit breaker.
// failureThreshold: consecutive failures before opening circuit.
// successThreshold: consecutive successes in half-open before closing circuit.
// resetTimeout: time to wait before transitioning from open to half-open.
func NewCircuitBreaker(failureThreshold, successThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		resetTimeout:     resetTimeout,
		lastStateChange:  time.Now(),
	}
}

// AllowRequest checks if a request should be allowed through.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if reset timeout has elapsed
		if time.Since(cb.lastFailureTime) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.lastStateChange = time.Now()
			cb.successCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.lastStateChange = time.Now()
			cb.failureCount = 0
			cb.successCount = 0
		}
	case CircuitClosed:
		cb.failureCount = 0 // Reset consecutive failure count on success
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
		}
	case CircuitHalfOpen:
		// Any failure in half-open reopens the circuit
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
		cb.successCount = 0
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()
}

// GetStatus returns the current circuit breaker status.
func (cb *CircuitBreaker) GetStatus() CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	canRetry := cb.state == CircuitClosed || cb.state == CircuitHalfOpen
	if cb.state == CircuitOpen {
		canRetry = time.Since(cb.lastFailureTime) >= cb.resetTimeout
	}

	return CircuitBreakerStatus{
		State:            string(cb.state),
		FailureCount:     cb.failureCount,
		SuccessCount:     cb.successCount,
		FailureThreshold: cb.failureThreshold,
		SuccessThreshold: cb.successThreshold,
		ResetTimeoutSec:  int(cb.resetTimeout.Seconds()),
		LastFailureAt:    cb.lastFailureTime.Unix(),
		LastStateChange:  cb.lastStateChange.Unix(),
		CanRetry:         canRetry,
	}
}

// ResumeState tracks which items have been downloaded so incomplete runs can resume.
type ResumeState struct {
	mu             sync.RWMutex
	CompletedItems map[string]bool `json:"completedItems"` // key: item URL or ID
	FailedItems    map[string]FailedItemInfo `json:"failedItems"`
	TotalItems     int  `json:"totalItems"`
	filePath       string
}

// FailedItemInfo holds details about a failed download item.
type FailedItemInfo struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	Error       string `json:"error"`
	Attempts    int    `json:"attempts"`
	LastAttempt int64  `json:"lastAttempt"`
	Retryable   bool   `json:"retryable"`
}

// ResumeStatus is the JSON-serializable resume state.
type ResumeStatus struct {
	HasResumeData  bool                      `json:"hasResumeData"`
	CompletedCount int                       `json:"completedCount"`
	FailedCount    int                       `json:"failedCount"`
	TotalItems     int                       `json:"totalItems"`
	RemainingCount int                       `json:"remainingCount"`
	FailedItems    map[string]FailedItemInfo `json:"failedItems"`
}

// NewResumeState creates a new resume state, loading from disk if available.
func NewResumeState(cacheDir string) *ResumeState {
	filePath := filepath.Join(cacheDir, "resume_state.json")
	rs := &ResumeState{
		CompletedItems: make(map[string]bool),
		FailedItems:    make(map[string]FailedItemInfo),
		filePath:       filePath,
	}
	rs.load()
	return rs
}

// load reads resume state from disk.
func (rs *ResumeState) load() {
	data, err := os.ReadFile(rs.filePath)
	if err != nil {
		return
	}
	var state struct {
		CompletedItems map[string]bool            `json:"completedItems"`
		FailedItems    map[string]FailedItemInfo   `json:"failedItems"`
		TotalItems     int                         `json:"totalItems"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	if state.CompletedItems != nil {
		rs.CompletedItems = state.CompletedItems
	}
	if state.FailedItems != nil {
		rs.FailedItems = state.FailedItems
	}
	rs.TotalItems = state.TotalItems
}

// save writes resume state to disk.
func (rs *ResumeState) save() {
	dir := filepath.Dir(rs.filePath)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(rs.filePath, data, 0644)
}

// MarkCompleted marks an item as successfully completed.
func (rs *ResumeState) MarkCompleted(itemKey string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.CompletedItems[itemKey] = true
	delete(rs.FailedItems, itemKey)
	rs.save()
}

// MarkFailed marks an item as failed.
func (rs *ResumeState) MarkFailed(itemKey string, info FailedItemInfo) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.FailedItems[itemKey] = info
	rs.save()
}

// IsCompleted checks if an item has already been completed.
func (rs *ResumeState) IsCompleted(itemKey string) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.CompletedItems[itemKey]
}

// SetTotalItems sets the total number of items for this run.
func (rs *ResumeState) SetTotalItems(total int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.TotalItems = total
	rs.save()
}

// GetStatus returns the current resume state.
func (rs *ResumeState) GetStatus() ResumeStatus {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	completed := len(rs.CompletedItems)
	failed := len(rs.FailedItems)
	remaining := rs.TotalItems - completed - failed
	if remaining < 0 {
		remaining = 0
	}
	failedCopy := make(map[string]FailedItemInfo, len(rs.FailedItems))
	for k, v := range rs.FailedItems {
		failedCopy[k] = v
	}
	return ResumeStatus{
		HasResumeData:  completed > 0 || failed > 0,
		CompletedCount: completed,
		FailedCount:    failed,
		TotalItems:     rs.TotalItems,
		RemainingCount: remaining,
		FailedItems:    failedCopy,
	}
}

// Clear resets the resume state (e.g. for a fresh run).
func (rs *ResumeState) Clear() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.CompletedItems = make(map[string]bool)
	rs.FailedItems = make(map[string]FailedItemInfo)
	rs.TotalItems = 0
	rs.save()
}

// RetryableErrors returns only the failed items that are marked as retryable.
func (rs *ResumeState) RetryableErrors() []FailedItemInfo {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	result := make([]FailedItemInfo, 0)
	for _, info := range rs.FailedItems {
		if info.Retryable {
			result = append(result, info)
		}
	}
	return result
}

// RecoveryStatus combines circuit breaker and resume state for the API.
type RecoveryStatus struct {
	CircuitBreaker CircuitBreakerStatus `json:"circuitBreaker"`
	Resume         ResumeStatus         `json:"resume"`
}

// ErrorDetail provides a detailed, user-friendly error message.
type ErrorDetail struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Explanation string `json:"explanation"`
	Suggestion  string `json:"suggestion"`
	Retryable   bool   `json:"retryable"`
	Timestamp   int64  `json:"timestamp"`
}

// ClassifyError converts a raw error into a detailed, user-friendly error.
func ClassifyError(err error) ErrorDetail {
	if err == nil {
		return ErrorDetail{}
	}

	errStr := err.Error()
	now := time.Now().Unix()

	// Rate limit errors
	if containsAny(errStr, "429", "rate limit", "too many requests") {
		return ErrorDetail{
			Code:        "RATE_LIMITED",
			Message:     "Spotify rate limit reached",
			Explanation: "The Spotify API has temporarily blocked requests because too many were sent in a short period.",
			Suggestion:  "Wait for the countdown to finish. The download will automatically resume.",
			Retryable:   true,
			Timestamp:   now,
		}
	}

	// Network errors
	if containsAny(errStr, "connection refused", "no such host", "network unreachable", "dial tcp") {
		return ErrorDetail{
			Code:        "NETWORK_ERROR",
			Message:     "Network connection failed",
			Explanation: "Unable to connect to the remote server. This could be a DNS, firewall, or internet connectivity issue.",
			Suggestion:  "Check your internet connection and try again. If using Docker, ensure the container has network access.",
			Retryable:   true,
			Timestamp:   now,
		}
	}

	// Timeout errors
	if containsAny(errStr, "timeout", "deadline exceeded", "context deadline") {
		return ErrorDetail{
			Code:        "TIMEOUT",
			Message:     "Request timed out",
			Explanation: "The remote server took too long to respond.",
			Suggestion:  "Try again. If this persists, the server may be overloaded. Consider increasing timeout settings.",
			Retryable:   true,
			Timestamp:   now,
		}
	}

	// Authentication errors
	if containsAny(errStr, "401", "unauthorized", "invalid credentials", "authentication") {
		return ErrorDetail{
			Code:        "AUTH_ERROR",
			Message:     "Authentication failed",
			Explanation: "The Spotify API credentials are invalid or expired.",
			Suggestion:  "Check your client_id and client_secret in config.yaml. Generate new credentials from the Spotify Developer Dashboard.",
			Retryable:   false,
			Timestamp:   now,
		}
	}

	// Not found errors
	if containsAny(errStr, "404", "not found") {
		return ErrorDetail{
			Code:        "NOT_FOUND",
			Message:     "Resource not found",
			Explanation: "The requested track, album, artist, or playlist could not be found on Spotify.",
			Suggestion:  "Verify the URL in your config.yaml. The content may have been removed from Spotify.",
			Retryable:   false,
			Timestamp:   now,
		}
	}

	// File system errors
	if containsAny(errStr, "permission denied", "no space left", "read-only file system", "disk quota") {
		return ErrorDetail{
			Code:        "FILESYSTEM_ERROR",
			Message:     "File system error",
			Explanation: fmt.Sprintf("A file system operation failed: %s", errStr),
			Suggestion:  "Check disk space, file permissions, and that the output directory exists and is writable.",
			Retryable:   false,
			Timestamp:   now,
		}
	}

	// yt-dlp errors
	if containsAny(errStr, "yt-dlp", "yt_dlp", "youtube-dl") {
		return ErrorDetail{
			Code:        "YTDLP_ERROR",
			Message:     "Audio download tool error",
			Explanation: fmt.Sprintf("yt-dlp encountered an error: %s", errStr),
			Suggestion:  "Ensure yt-dlp is installed and up to date. Try running 'pip install --upgrade yt-dlp'.",
			Retryable:   true,
			Timestamp:   now,
		}
	}

	// Generic / unknown error
	return ErrorDetail{
		Code:        "UNKNOWN_ERROR",
		Message:     "An unexpected error occurred",
		Explanation: errStr,
		Suggestion:  "Check the logs for more details. If the problem persists, try restarting the service.",
		Retryable:   true,
		Timestamp:   now,
	}
}

// containsAny checks if s contains any of the substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
