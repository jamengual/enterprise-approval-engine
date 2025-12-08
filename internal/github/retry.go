package github

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/google/go-github/v57/github"
)

// RetryConfig configures retry behavior for API calls.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// InitialBackoff is the initial backoff duration.
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration.
	MaxBackoff time.Duration
	// BackoffMultiplier is multiplied to backoff after each retry.
	BackoffMultiplier float64
}

// DefaultRetryConfig returns sensible defaults for retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    time.Second,
		MaxBackoff:        time.Minute * 2,
		BackoffMultiplier: 2.0,
	}
}

// retryTransport wraps an http.RoundTripper with retry logic for rate limits.
type retryTransport struct {
	base   http.RoundTripper
	config RetryConfig
}

// newRetryTransport creates a new retry transport wrapping the given base transport.
func newRetryTransport(base http.RoundTripper, config RetryConfig) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &retryTransport{
		base:   base,
		config: config,
	}
}

// RoundTrip implements http.RoundTripper with retry logic.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.config.MaxRetries; attempt++ {
		// Clone the request for retry (body may have been consumed)
		reqCopy := req.Clone(req.Context())

		resp, err = t.base.RoundTrip(reqCopy)
		if err != nil {
			// Network error - retry with backoff
			if attempt < t.config.MaxRetries {
				backoff := t.calculateBackoff(attempt, 0)
				if waitErr := t.wait(req.Context(), backoff); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return nil, err
		}

		// Check if we hit a rate limit
		if !t.isRateLimited(resp) {
			return resp, nil
		}

		// Don't retry if we've exhausted attempts
		if attempt >= t.config.MaxRetries {
			return resp, nil
		}

		// Calculate backoff, respecting Retry-After header
		retryAfter := t.parseRetryAfter(resp)
		backoff := t.calculateBackoff(attempt, retryAfter)

		// Close the response body before retrying
		resp.Body.Close()

		// Wait before retrying
		if waitErr := t.wait(req.Context(), backoff); waitErr != nil {
			return nil, waitErr
		}
	}

	return resp, err
}

// isRateLimited checks if the response indicates a rate limit.
func (t *retryTransport) isRateLimited(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	// HTTP 429 Too Many Requests (secondary rate limit)
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	// HTTP 403 with rate limit headers (primary rate limit)
	if resp.StatusCode == http.StatusForbidden {
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		if remaining == "0" {
			return true
		}
		// Also check for abuse rate limit message
		// GitHub returns 403 for secondary rate limits too
		if resp.Header.Get("Retry-After") != "" {
			return true
		}
	}

	return false
}

// parseRetryAfter extracts the retry-after duration from response headers.
func (t *retryTransport) parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	// Check Retry-After header (seconds)
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}

	// Check X-RateLimit-Reset header (Unix timestamp)
	if resetStr := resp.Header.Get("X-RateLimit-Reset"); resetStr != "" {
		if resetTime, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			resetAt := time.Unix(resetTime, 0)
			waitDuration := time.Until(resetAt)
			if waitDuration > 0 {
				return waitDuration
			}
		}
	}

	return 0
}

// calculateBackoff calculates the backoff duration for the given attempt.
func (t *retryTransport) calculateBackoff(attempt int, serverSuggested time.Duration) time.Duration {
	// If server suggested a duration, use it (with a small buffer)
	if serverSuggested > 0 {
		// Add 1 second buffer to ensure we're past the rate limit
		return serverSuggested + time.Second
	}

	// Exponential backoff with jitter
	backoff := float64(t.config.InitialBackoff) * math.Pow(t.config.BackoffMultiplier, float64(attempt))

	// Add jitter (±25%)
	jitter := backoff * 0.25 * (rand.Float64()*2 - 1)
	backoff += jitter

	// Cap at max backoff
	if backoff > float64(t.config.MaxBackoff) {
		backoff = float64(t.config.MaxBackoff)
	}

	return time.Duration(backoff)
}

// wait waits for the specified duration, respecting context cancellation.
func (t *retryTransport) wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// IsRateLimitError checks if an error is a GitHub rate limit error.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*github.RateLimitError)
	if ok {
		return true
	}
	// Also check for AbuseRateLimitError (secondary rate limit)
	_, ok = err.(*github.AbuseRateLimitError)
	return ok
}

// GetRateLimitReset returns the time when the rate limit resets.
func GetRateLimitReset(err error) time.Time {
	if rlErr, ok := err.(*github.RateLimitError); ok {
		return rlErr.Rate.Reset.Time
	}
	if abuseErr, ok := err.(*github.AbuseRateLimitError); ok {
		if abuseErr.RetryAfter != nil {
			return time.Now().Add(*abuseErr.RetryAfter)
		}
	}
	return time.Time{}
}

// WrapWithRetry wraps an operation with retry logic for rate limits.
// This is useful for operations that don't go through the HTTP transport.
func WrapWithRetry[T any](ctx context.Context, config RetryConfig, operation func() (T, error)) (T, error) {
	var result T
	var err error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		result, err = operation()
		if err == nil {
			return result, nil
		}

		// Check if it's a rate limit error
		if !IsRateLimitError(err) {
			return result, err
		}

		// Don't retry if we've exhausted attempts
		if attempt >= config.MaxRetries {
			return result, fmt.Errorf("rate limit exceeded after %d retries: %w", config.MaxRetries, err)
		}

		// Calculate backoff
		var backoff time.Duration
		resetTime := GetRateLimitReset(err)
		if !resetTime.IsZero() {
			backoff = time.Until(resetTime) + time.Second
		} else {
			backoff = time.Duration(float64(config.InitialBackoff) * math.Pow(config.BackoffMultiplier, float64(attempt)))
		}

		// Cap at max backoff
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}

		// Wait before retrying
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, ctx.Err()
		case <-timer.C:
		}
	}

	return result, err
}
