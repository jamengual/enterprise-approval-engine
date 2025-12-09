// Package github provides a wrapper around the GitHub API for IssueOps operations.
package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client with IssueOps-specific operations.
type Client struct {
	client      *github.Client
	owner       string
	repo        string
	retryConfig RetryConfig
}

// NewClient creates a new GitHub client from environment variables.
func NewClient(ctx context.Context) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("INPUT_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN or INPUT_TOKEN environment variable is required")
	}

	repoFullName := os.Getenv("GITHUB_REPOSITORY")
	if repoFullName == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY environment variable is required")
	}

	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid GITHUB_REPOSITORY format: %s", repoFullName)
	}

	return NewClientWithToken(ctx, token, parts[0], parts[1])
}

// NewClientWithToken creates a new GitHub client with explicit parameters.
func NewClientWithToken(ctx context.Context, token, owner, repo string) (*Client, error) {
	return NewClientWithTokenAndRetry(ctx, token, owner, repo, DefaultRetryConfig())
}

// NewClientWithTokenAndRetry creates a new GitHub client with explicit parameters and retry config.
func NewClientWithTokenAndRetry(ctx context.Context, token, owner, repo string, retryConfig RetryConfig) (*Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	baseTransport := oauth2.NewClient(ctx, ts).Transport

	// Wrap with retry transport for rate limit handling
	retryTransport := newRetryTransport(baseTransport, retryConfig)
	httpClient := &http.Client{Transport: retryTransport}

	var ghClient *github.Client

	// Support GitHub Enterprise Server
	serverURL := os.Getenv("GITHUB_SERVER_URL")
	apiURL := os.Getenv("GITHUB_API_URL")

	if serverURL != "" && !strings.Contains(serverURL, "github.com") {
		if apiURL == "" {
			apiURL = serverURL + "/api/v3"
		}
		var err error
		ghClient, err = github.NewClient(httpClient).WithEnterpriseURLs(apiURL, serverURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create enterprise client: %w", err)
		}
	} else {
		ghClient = github.NewClient(httpClient)
	}

	return &Client{
		client:      ghClient,
		owner:       owner,
		repo:        repo,
		retryConfig: retryConfig,
	}, nil
}

// Owner returns the repository owner.
func (c *Client) Owner() string {
	return c.owner
}

// Repo returns the repository name.
func (c *Client) Repo() string {
	return c.repo
}

// GitHubClient returns the underlying go-github client for advanced operations.
func (c *Client) GitHubClient() *github.Client {
	return c.client
}

// IsNotFound returns true if the error is a 404 Not Found error.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		return ghErr.Response.StatusCode == http.StatusNotFound
	}
	return false
}

// IsForbidden returns true if the error is a 403 Forbidden error.
func IsForbidden(err error) bool {
	if err == nil {
		return false
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		return ghErr.Response.StatusCode == http.StatusForbidden
	}
	return false
}

// GetFileContents fetches the contents of a file from a repository.
func (c *Client) GetFileContents(ctx context.Context, owner, repo, path string) ([]byte, error) {
	content, _, _, err := c.client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s from %s/%s: %w", path, owner, repo, err)
	}

	if content == nil {
		return nil, fmt.Errorf("file %s not found in %s/%s", path, owner, repo)
	}

	decoded, err := content.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return []byte(decoded), nil
}

// GetFileContentsFromRepo fetches file contents from any repo (for external config).
// The repo parameter should be in "owner/repo" format.
func (c *Client) GetFileContentsFromRepo(ctx context.Context, repoFullName, path string) ([]byte, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format: %s (expected owner/repo)", repoFullName)
	}

	return c.GetFileContents(ctx, parts[0], parts[1], path)
}
