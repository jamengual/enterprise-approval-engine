package github

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/go-github/v57/github"
)

// Milestone represents a GitHub milestone.
type Milestone struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"` // "open" or "closed"
	OpenIssues  int    `json:"open_issues"`
	ClosedIssues int   `json:"closed_issues"`
	URL         string `json:"url"`
}

// Branch represents a Git branch.
type Branch struct {
	Name      string `json:"name"`
	SHA       string `json:"sha"`
	Protected bool   `json:"protected"`
}

// PRWithLabels represents a PR with its labels for filtering.
type PRWithLabels struct {
	PullRequest
	Labels []string `json:"labels"`
}

// CreateMilestone creates a new milestone.
func (c *Client) CreateMilestone(ctx context.Context, title, description string) (*Milestone, error) {
	state := "open"
	m := &github.Milestone{
		Title:       &title,
		Description: &description,
		State:       &state,
	}

	milestone, _, err := c.client.Issues.CreateMilestone(ctx, c.owner, c.repo, m)
	if err != nil {
		return nil, fmt.Errorf("failed to create milestone %q: %w", title, err)
	}

	return milestoneFromGitHub(milestone), nil
}

// GetMilestoneByTitle finds a milestone by its title.
func (c *Client) GetMilestoneByTitle(ctx context.Context, title string) (*Milestone, error) {
	opts := &github.MilestoneListOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		milestones, resp, err := c.client.Issues.ListMilestones(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list milestones: %w", err)
		}

		for _, m := range milestones {
			if m.GetTitle() == title {
				return milestoneFromGitHub(m), nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil, nil // Not found
}

// CloseMilestone closes a milestone.
func (c *Client) CloseMilestone(ctx context.Context, number int) error {
	state := "closed"
	m := &github.Milestone{State: &state}

	_, _, err := c.client.Issues.EditMilestone(ctx, c.owner, c.repo, number, m)
	if err != nil {
		return fmt.Errorf("failed to close milestone %d: %w", number, err)
	}

	return nil
}

// GetPRsByMilestone returns all PRs assigned to a milestone.
func (c *Client) GetPRsByMilestone(ctx context.Context, milestoneNumber int) ([]PullRequest, error) {
	// GitHub API uses "milestone" filter on issues endpoint (PRs are issues)
	opts := &github.IssueListByRepoOptions{
		Milestone:   fmt.Sprintf("%d", milestoneNumber),
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var prs []PullRequest

	for {
		issues, resp, err := c.client.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issues for milestone %d: %w", milestoneNumber, err)
		}

		for _, issue := range issues {
			// Only include pull requests (issues with PR data)
			if issue.PullRequestLinks == nil {
				continue
			}

			// Fetch full PR details
			pr, _, err := c.client.PullRequests.Get(ctx, c.owner, c.repo, issue.GetNumber())
			if err != nil {
				continue // Skip if we can't fetch
			}

			author := ""
			if pr.User != nil {
				author = pr.User.GetLogin()
			}

			mergedAt := ""
			if pr.MergedAt != nil {
				mergedAt = pr.MergedAt.String()
			}

			prs = append(prs, PullRequest{
				Number:   pr.GetNumber(),
				Title:    pr.GetTitle(),
				Author:   author,
				URL:      pr.GetHTMLURL(),
				MergedAt: mergedAt,
				MergeSHA: pr.GetMergeCommitSHA(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return prs, nil
}

// GetPRsByLabel returns all merged PRs with a specific label.
func (c *Client) GetPRsByLabel(ctx context.Context, label string) ([]PullRequest, error) {
	opts := &github.IssueListByRepoOptions{
		Labels:      []string{label},
		State:       "closed", // Merged PRs are closed
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var prs []PullRequest

	for {
		issues, resp, err := c.client.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issues with label %q: %w", label, err)
		}

		for _, issue := range issues {
			// Only include pull requests
			if issue.PullRequestLinks == nil {
				continue
			}

			// Fetch full PR details to check if merged
			pr, _, err := c.client.PullRequests.Get(ctx, c.owner, c.repo, issue.GetNumber())
			if err != nil {
				continue
			}

			// Only include merged PRs
			if !pr.GetMerged() {
				continue
			}

			author := ""
			if pr.User != nil {
				author = pr.User.GetLogin()
			}

			mergedAt := ""
			if pr.MergedAt != nil {
				mergedAt = pr.MergedAt.String()
			}

			prs = append(prs, PullRequest{
				Number:   pr.GetNumber(),
				Title:    pr.GetTitle(),
				Author:   author,
				URL:      pr.GetHTMLURL(),
				MergedAt: mergedAt,
				MergeSHA: pr.GetMergeCommitSHA(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Sort by merge date (newest first)
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].MergedAt > prs[j].MergedAt
	})

	return prs, nil
}

// CreateLabel creates a label if it doesn't exist.
func (c *Client) CreateLabel(ctx context.Context, name, color, description string) error {
	label := &github.Label{
		Name:        &name,
		Color:       &color,
		Description: &description,
	}

	_, _, err := c.client.Issues.CreateLabel(ctx, c.owner, c.repo, label)
	if err != nil {
		// Check if label already exists (409 Conflict)
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == 422 {
			return nil // Label already exists
		}
		return fmt.Errorf("failed to create label %q: %w", name, err)
	}

	return nil
}

// AddLabelToPR adds a label to a PR.
func (c *Client) AddLabelToPR(ctx context.Context, prNumber int, labels []string) error {
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, prNumber, labels)
	if err != nil {
		return fmt.Errorf("failed to add labels to PR #%d: %w", prNumber, err)
	}
	return nil
}

// RemoveLabelFromPR removes a label from a PR.
func (c *Client) RemoveLabelFromPR(ctx context.Context, prNumber int, label string) error {
	_, err := c.client.Issues.RemoveLabelForIssue(ctx, c.owner, c.repo, prNumber, label)
	if err != nil {
		if IsNotFound(err) {
			return nil // Label not on PR
		}
		return fmt.Errorf("failed to remove label %q from PR #%d: %w", label, prNumber, err)
	}
	return nil
}

// CreateBranch creates a new branch from a source ref.
func (c *Client) CreateBranch(ctx context.Context, branchName, sourceRef string) (*Branch, error) {
	// Get the SHA of the source ref
	var sha string

	if sourceRef == "" {
		// Use default branch
		repo, _, err := c.client.Repositories.Get(ctx, c.owner, c.repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository: %w", err)
		}
		sourceRef = repo.GetDefaultBranch()
	}

	ref, _, err := c.client.Git.GetRef(ctx, c.owner, c.repo, "refs/heads/"+sourceRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get source ref %q: %w", sourceRef, err)
	}
	sha = ref.GetObject().GetSHA()

	// Create the new branch
	refName := "refs/heads/" + branchName
	newRef := &github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: &sha,
		},
	}

	createdRef, _, err := c.client.Git.CreateRef(ctx, c.owner, c.repo, newRef)
	if err != nil {
		return nil, fmt.Errorf("failed to create branch %q: %w", branchName, err)
	}

	return &Branch{
		Name: branchName,
		SHA:  createdRef.GetObject().GetSHA(),
	}, nil
}

// GetBranch gets information about a branch.
func (c *Client) GetBranch(ctx context.Context, branchName string) (*Branch, error) {
	branch, _, err := c.client.Repositories.GetBranch(ctx, c.owner, c.repo, branchName, 0)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get branch %q: %w", branchName, err)
	}

	return &Branch{
		Name:      branch.GetName(),
		SHA:       branch.GetCommit().GetSHA(),
		Protected: branch.GetProtected(),
	}, nil
}

// DeleteBranch deletes a branch.
func (c *Client) DeleteBranch(ctx context.Context, branchName string) error {
	refName := "refs/heads/" + branchName
	_, err := c.client.Git.DeleteRef(ctx, c.owner, c.repo, refName)
	if err != nil {
		if IsNotFound(err) {
			return nil // Branch doesn't exist
		}
		return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
	}
	return nil
}

// GetPRsMergedToBranch returns PRs merged to a specific branch.
func (c *Client) GetPRsMergedToBranch(ctx context.Context, branchName string) ([]PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State:       "closed",
		Base:        branchName,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var prs []PullRequest

	for {
		pullRequests, resp, err := c.client.PullRequests.List(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list PRs for branch %q: %w", branchName, err)
		}

		for _, pr := range pullRequests {
			// Only include merged PRs
			if !pr.GetMerged() {
				continue
			}

			author := ""
			if pr.User != nil {
				author = pr.User.GetLogin()
			}

			mergedAt := ""
			if pr.MergedAt != nil {
				mergedAt = pr.MergedAt.String()
			}

			prs = append(prs, PullRequest{
				Number:   pr.GetNumber(),
				Title:    pr.GetTitle(),
				Author:   author,
				URL:      pr.GetHTMLURL(),
				MergedAt: mergedAt,
				MergeSHA: pr.GetMergeCommitSHA(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Sort by merge date (newest first)
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].MergedAt > prs[j].MergedAt
	})

	return prs, nil
}

// GetCommitsBetweenBranches returns commits in head branch that are not in base branch.
func (c *Client) GetCommitsBetweenBranches(ctx context.Context, baseBranch, headBranch string) ([]Commit, error) {
	return c.CompareCommits(ctx, baseBranch, headBranch)
}

func milestoneFromGitHub(m *github.Milestone) *Milestone {
	return &Milestone{
		Number:       m.GetNumber(),
		Title:        m.GetTitle(),
		Description:  m.GetDescription(),
		State:        m.GetState(),
		OpenIssues:   m.GetOpenIssues(),
		ClosedIssues: m.GetClosedIssues(),
		URL:          m.GetHTMLURL(),
	}
}
