package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v57/github"
)

// SubIssueRequest represents a request to add a sub-issue to a parent issue.
type SubIssueRequest struct {
	SubIssueID int64 `json:"sub_issue_id"`
}

// SubIssueResponse represents the response from sub-issue operations.
type SubIssueResponse struct {
	ID        int64  `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	Assignees []*github.User `json:"assignees,omitempty"`
}

// AddSubIssue links an existing issue as a sub-issue of a parent issue.
// This uses the GitHub Sub-Issues API: POST /repos/{owner}/{repo}/issues/{issue_number}/sub_issues
// Available since December 2024.
func (c *Client) AddSubIssue(ctx context.Context, parentIssueNumber int, subIssueID int64) error {
	url := fmt.Sprintf("repos/%s/%s/issues/%d/sub_issues", c.owner, c.repo, parentIssueNumber)

	req := &SubIssueRequest{
		SubIssueID: subIssueID,
	}

	httpReq, err := c.client.NewRequest(http.MethodPost, url, req)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// The sub-issues API requires this preview header
	httpReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(ctx, httpReq, nil)
	if err != nil {
		return fmt.Errorf("failed to add sub-issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ListSubIssues retrieves all sub-issues for a given parent issue.
// GET /repos/{owner}/{repo}/issues/{issue_number}/sub_issues
func (c *Client) ListSubIssues(ctx context.Context, parentIssueNumber int) ([]*github.Issue, error) {
	url := fmt.Sprintf("repos/%s/%s/issues/%d/sub_issues", c.owner, c.repo, parentIssueNumber)

	httpReq, err := c.client.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/vnd.github+json")

	var subIssues []*github.Issue
	resp, err := c.client.Do(ctx, httpReq, &subIssues)
	if err != nil {
		return nil, fmt.Errorf("failed to list sub-issues: %w", err)
	}
	defer resp.Body.Close()

	return subIssues, nil
}

// GetParentIssue retrieves the parent issue for a given sub-issue.
// GET /repos/{owner}/{repo}/issues/{issue_number}/parent
func (c *Client) GetParentIssue(ctx context.Context, subIssueNumber int) (*github.Issue, error) {
	url := fmt.Sprintf("repos/%s/%s/issues/%d/parent", c.owner, c.repo, subIssueNumber)

	httpReq, err := c.client.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/vnd.github+json")

	var parent github.Issue
	resp, err := c.client.Do(ctx, httpReq, &parent)
	if err != nil {
		// 404 means no parent (not a sub-issue)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get parent issue: %w", err)
	}
	defer resp.Body.Close()

	return &parent, nil
}

// RemoveSubIssue removes a sub-issue from its parent.
// DELETE /repos/{owner}/{repo}/issues/{issue_number}/sub_issue
func (c *Client) RemoveSubIssue(ctx context.Context, parentIssueNumber int, subIssueID int64) error {
	url := fmt.Sprintf("repos/%s/%s/issues/%d/sub_issue", c.owner, c.repo, parentIssueNumber)

	req := &SubIssueRequest{
		SubIssueID: subIssueID,
	}

	httpReq, err := c.client.NewRequest(http.MethodDelete, url, req)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(ctx, httpReq, nil)
	if err != nil {
		return fmt.Errorf("failed to remove sub-issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// CreateApprovalSubIssue creates a new issue and links it as a sub-issue of a parent.
// This is a convenience method that combines CreateIssue + AddSubIssue.
func (c *Client) CreateApprovalSubIssue(ctx context.Context, parentIssueNumber int, title, body string, labels, assignees []string) (*Issue, error) {
	// First, create the issue
	issue, err := c.CreateIssue(ctx, CreateIssueOptions{
		Title:     title,
		Body:      body,
		Labels:    labels,
		Assignees: assignees,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-issue: %w", err)
	}

	// Get the issue ID from GitHub API (we need to fetch it)
	ghIssue, _, err := c.client.Issues.Get(ctx, c.owner, c.repo, issue.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue ID: %w", err)
	}

	// Then link it as a sub-issue
	if err := c.AddSubIssue(ctx, parentIssueNumber, ghIssue.GetID()); err != nil {
		// If linking fails, close the orphaned issue
		_ = c.CloseIssue(ctx, issue.Number)
		return nil, fmt.Errorf("failed to link sub-issue #%d to parent #%d: %w", issue.Number, parentIssueNumber, err)
	}

	return issue, nil
}

// SubIssueState represents the state of a sub-issue for approval tracking.
type SubIssueState struct {
	IssueNumber int    `json:"issue_number"`
	Stage       string `json:"stage"`
	Status      string `json:"status"` // "pending", "approved", "denied"
	ClosedBy    string `json:"closed_by,omitempty"`
	ClosedAt    string `json:"closed_at,omitempty"`
}

// IsSubIssue checks if an issue is a sub-issue (has a parent).
func (c *Client) IsSubIssue(ctx context.Context, issueNumber int) (bool, error) {
	parent, err := c.GetParentIssue(ctx, issueNumber)
	if err != nil {
		return false, err
	}
	return parent != nil, nil
}
