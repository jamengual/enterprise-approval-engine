// Package jira provides a client for the Jira Cloud REST API.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the Jira REST API.
// It supports two modes:
// - Links-only mode: Only baseURL is required, can generate issue links
// - Full mode: With email+apiToken, can fetch issue details and update Fix Versions
type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

// ClientConfig contains configuration for creating a Jira client.
type ClientConfig struct {
	BaseURL  string // e.g., "https://yourcompany.atlassian.net" (required)
	Email    string // Jira user email (optional - for API access)
	APIToken string // Jira API token (optional - for API access)
}

// NewClient creates a new Jira client.
// If only BaseURL is provided, the client works in links-only mode.
// If Email and APIToken are also provided, the client can make API calls.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("jira base URL is required")
	}

	client := &Client{
		baseURL: cfg.BaseURL,
	}

	// If credentials are provided, enable API access
	if cfg.Email != "" && cfg.APIToken != "" {
		client.email = cfg.Email
		client.apiToken = cfg.APIToken
		client.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return client, nil
}

// NewLinksOnlyClient creates a Jira client that can only generate links.
// Use this when you don't need to fetch issue details or update Fix Versions.
func NewLinksOnlyClient(baseURL string) (*Client, error) {
	return NewClient(ClientConfig{BaseURL: baseURL})
}

// doRequest performs an authenticated HTTP request to Jira.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Basic auth with email:api_token
	req.SetBasicAuth(c.email, c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// decodeResponse decodes a JSON response body.
func decodeResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira API error (status %d): %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// IsConfigured returns true if the client has at least a base URL configured.
func (c *Client) IsConfigured() bool {
	return c != nil && c.baseURL != ""
}

// CanMakeAPICalls returns true if the client has credentials for API access.
// When false, the client can only generate links.
func (c *Client) CanMakeAPICalls() bool {
	return c != nil && c.baseURL != "" && c.email != "" && c.apiToken != "" && c.httpClient != nil
}

// BaseURL returns the Jira base URL.
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}
