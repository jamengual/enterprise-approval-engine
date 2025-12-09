package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryTransport_Success(t *testing.T) {
	// Server that succeeds on first request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	transport := newRetryTransport(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_RateLimitRetry(t *testing.T) {
	var attempts int32

	// Server that rate limits first 2 requests, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt <= 2 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", "0") // Immediate retry for test
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message": "rate limit exceeded"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	transport := newRetryTransport(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryTransport_429Retry(t *testing.T) {
	var attempts int32

	// Server that returns 429, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message": "too many requests"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	transport := newRetryTransport(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryTransport_MaxRetriesExceeded(t *testing.T) {
	var attempts int32

	// Server that always rate limits
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message": "rate limit exceeded"}`))
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:        2,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	transport := newRetryTransport(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Expected response (not error), got: %v", err)
	}
	defer resp.Body.Close()

	// Should return the rate limit response after max retries
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}

	// Initial attempt + 2 retries = 3 total
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryTransport_ContextCancellation(t *testing.T) {
	// Server that always rate limits
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("Retry-After", "60") // Long retry
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    time.Second,
		MaxBackoff:        time.Minute,
		BackoffMultiplier: 2.0,
	}

	transport := newRetryTransport(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := client.Do(req)

	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

func TestRetryTransport_NonRateLimitError(t *testing.T) {
	var attempts int32

	// Server that returns 500 (not a rate limit)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message": "internal error"}`))
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	transport := newRetryTransport(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Expected response, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Should NOT retry for non-rate-limit errors
	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry for 500), got %d", attempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    time.Second,
		MaxBackoff:        time.Minute,
		BackoffMultiplier: 2.0,
	}

	transport := &retryTransport{config: config}

	// Test server-suggested backoff is respected
	serverSuggested := 30 * time.Second
	backoff := transport.calculateBackoff(0, serverSuggested)
	// Should be serverSuggested + 1 second buffer
	if backoff != 31*time.Second {
		t.Errorf("Expected 31s, got %v", backoff)
	}

	// Test exponential backoff (attempt 0, no server suggestion)
	backoff = transport.calculateBackoff(0, 0)
	// Should be around 1s (±25% jitter)
	if backoff < 750*time.Millisecond || backoff > 1250*time.Millisecond {
		t.Errorf("Expected ~1s, got %v", backoff)
	}

	// Test exponential growth (attempt 2)
	backoff = transport.calculateBackoff(2, 0)
	// Should be around 4s (1s * 2^2) ±25%
	if backoff < 3*time.Second || backoff > 5*time.Second {
		t.Errorf("Expected ~4s, got %v", backoff)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", config.MaxRetries)
	}
	if config.InitialBackoff != time.Second {
		t.Errorf("Expected InitialBackoff 1s, got %v", config.InitialBackoff)
	}
	if config.MaxBackoff != 2*time.Minute {
		t.Errorf("Expected MaxBackoff 2m, got %v", config.MaxBackoff)
	}
	if config.BackoffMultiplier != 2.0 {
		t.Errorf("Expected BackoffMultiplier 2.0, got %f", config.BackoffMultiplier)
	}
}

func TestIsRateLimited(t *testing.T) {
	transport := &retryTransport{config: DefaultRetryConfig()}

	tests := []struct {
		name       string
		statusCode int
		headers    map[string]string
		expected   bool
	}{
		{
			name:       "429 Too Many Requests",
			statusCode: http.StatusTooManyRequests,
			headers:    nil,
			expected:   true,
		},
		{
			name:       "403 with rate limit remaining 0",
			statusCode: http.StatusForbidden,
			headers:    map[string]string{"X-RateLimit-Remaining": "0"},
			expected:   true,
		},
		{
			name:       "403 with Retry-After",
			statusCode: http.StatusForbidden,
			headers:    map[string]string{"Retry-After": "60"},
			expected:   true,
		},
		{
			name:       "403 without rate limit headers",
			statusCode: http.StatusForbidden,
			headers:    nil,
			expected:   false,
		},
		{
			name:       "200 OK",
			statusCode: http.StatusOK,
			headers:    nil,
			expected:   false,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			headers:    nil,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     make(http.Header),
			}
			for k, v := range tt.headers {
				resp.Header.Set(k, v)
			}

			result := transport.isRateLimited(resp)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWrapWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	var attempts int
	operation := func() (string, error) {
		attempts++
		return "success", nil
	}

	result, err := WrapWithRetry(ctx, config, operation)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %q", result)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestWrapWithRetry_NonRateLimitError(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	var attempts int
	expectedErr := fmt.Errorf("some other error")
	operation := func() (string, error) {
		attempts++
		return "", expectedErr
	}

	_, err := WrapWithRetry(ctx, config, operation)
	if err != expectedErr {
		t.Errorf("Expected original error, got: %v", err)
	}
	// Should NOT retry for non-rate-limit errors
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestWrapWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	operation := func() (string, error) {
		return "success", nil
	}

	result, err := WrapWithRetry(ctx, config, operation)
	// Should still succeed since operation completes before context check
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %q", result)
	}
}
