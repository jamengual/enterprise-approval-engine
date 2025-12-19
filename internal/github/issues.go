package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v57/github"
)

// Issue represents a GitHub issue.
type Issue struct {
	Number  int
	Title   string
	Body    string
	State   string
	HTMLURL string
	Labels  []string
}

// IssueComment represents a comment on a GitHub issue.
type IssueComment struct {
	ID        int64
	User      string
	Body      string
	CreatedAt string
}

// CreateIssueOptions contains options for creating an issue.
type CreateIssueOptions struct {
	Title     string
	Body      string
	Labels    []string
	Assignees []string
}

// CreateIssue creates a new issue in the repository.
func (c *Client) CreateIssue(ctx context.Context, opts CreateIssueOptions) (*Issue, error) {
	req := &github.IssueRequest{
		Title: &opts.Title,
		Body:  &opts.Body,
	}

	if len(opts.Labels) > 0 {
		req.Labels = &opts.Labels
	}

	// GitHub limits assignees to 10 per issue
	if len(opts.Assignees) > 0 {
		assignees := opts.Assignees
		if len(assignees) > 10 {
			assignees = assignees[:10]
		}
		req.Assignees = &assignees
	}

	issue, _, err := c.client.Issues.Create(ctx, c.owner, c.repo, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return issueFromGitHub(issue), nil
}

// GetIssue retrieves an issue by number.
func (c *Client) GetIssue(ctx context.Context, number int) (*Issue, error) {
	issue, _, err := c.client.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue %d: %w", number, err)
	}

	return issueFromGitHub(issue), nil
}

// UpdateIssueBody updates the body of an issue.
func (c *Client) UpdateIssueBody(ctx context.Context, number int, body string) error {
	req := &github.IssueRequest{Body: &body}
	_, _, err := c.client.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return fmt.Errorf("failed to update issue %d: %w", number, err)
	}
	return nil
}

// UpdateIssueTitle updates the title of an issue.
func (c *Client) UpdateIssueTitle(ctx context.Context, number int, title string) error {
	req := &github.IssueRequest{Title: &title}
	_, _, err := c.client.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return fmt.Errorf("failed to update issue title %d: %w", number, err)
	}
	return nil
}

// CloseIssue closes an issue.
func (c *Client) CloseIssue(ctx context.Context, number int) error {
	state := "closed"
	req := &github.IssueRequest{State: &state}
	_, _, err := c.client.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return fmt.Errorf("failed to close issue %d: %w", number, err)
	}
	return nil
}

// AddLabels adds labels to an issue.
func (c *Client) AddLabels(ctx context.Context, number int, labels []string) error {
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, number, labels)
	if err != nil {
		return fmt.Errorf("failed to add labels to issue %d: %w", number, err)
	}
	return nil
}

// CreateComment adds a comment to an issue.
func (c *Client) CreateComment(ctx context.Context, number int, body string) error {
	comment := &github.IssueComment{Body: &body}
	_, _, err := c.client.Issues.CreateComment(ctx, c.owner, c.repo, number, comment)
	if err != nil {
		return fmt.Errorf("failed to create comment on issue %d: %w", number, err)
	}
	return nil
}

// ListComments retrieves all comments on an issue.
func (c *Client) ListComments(ctx context.Context, number int) ([]IssueComment, error) {
	var allComments []IssueComment

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list comments for issue %d: %w", number, err)
		}

		for _, comment := range comments {
			allComments = append(allComments, IssueComment{
				ID:        comment.GetID(),
				User:      comment.GetUser().GetLogin(),
				Body:      comment.GetBody(),
				CreatedAt: comment.GetCreatedAt().String(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// AddReaction adds a reaction to an issue comment.
func (c *Client) AddReaction(ctx context.Context, commentID int64, reaction string) error {
	_, _, err := c.client.Reactions.CreateIssueCommentReaction(ctx, c.owner, c.repo, commentID, reaction)
	if err != nil {
		return fmt.Errorf("failed to add reaction to comment %d: %w", commentID, err)
	}
	return nil
}

// GetIssueByNumber retrieves the full GitHub issue object by number.
// Returns the underlying github.Issue which includes the ID field.
func (c *Client) GetIssueByNumber(ctx context.Context, number int) (*github.Issue, *github.Response, error) {
	issue, resp, err := c.client.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, resp, fmt.Errorf("failed to get issue %d: %w", number, err)
	}
	return issue, resp, nil
}

// ReopenIssue reopens a closed issue.
func (c *Client) ReopenIssue(ctx context.Context, number int) error {
	state := "open"
	req := &github.IssueRequest{State: &state}
	_, _, err := c.client.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return fmt.Errorf("failed to reopen issue %d: %w", number, err)
	}
	return nil
}

func issueFromGitHub(issue *github.Issue) *Issue {
	var labels []string
	for _, label := range issue.Labels {
		labels = append(labels, label.GetName())
	}

	return &Issue{
		Number:  issue.GetNumber(),
		Title:   issue.GetTitle(),
		Body:    issue.GetBody(),
		State:   issue.GetState(),
		HTMLURL: issue.GetHTMLURL(),
		Labels:  labels,
	}
}
