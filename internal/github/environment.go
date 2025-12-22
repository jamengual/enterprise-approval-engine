package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v57/github"
)

// PendingDeployment represents a deployment waiting for approval.
type PendingDeployment struct {
	EnvironmentID         int64  `json:"environment_id"`
	EnvironmentName       string `json:"environment_name"`
	RunID                 int64  `json:"run_id"`
	WaitTimer             int    `json:"wait_timer"`
	WaitTimerStart        string `json:"wait_timer_start,omitempty"`
	CurrentUserCanApprove bool   `json:"current_user_can_approve"`
}

// pendingDeploymentResponse is the API response for a pending deployment.
type pendingDeploymentResponse struct {
	Environment struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"environment"`
	WaitTimer             int    `json:"wait_timer"`
	WaitTimerStartedAt    string `json:"wait_timer_started_at,omitempty"`
	CurrentUserCanApprove bool   `json:"current_user_can_approve"`
}

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"` // queued, in_progress, completed, waiting
	Conclusion string `json:"conclusion,omitempty"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	RunNumber  int    `json:"run_number"`
	URL        string `json:"url"`
}

// GetPendingDeployments returns deployments waiting for approval for a workflow run.
// Note: go-github v57 does not provide a dedicated typed method for this endpoint,
// so we use the generic client.NewRequest/client.Do helpers.
func (c *Client) GetPendingDeployments(ctx context.Context, runID int64) ([]PendingDeployment, error) {
	url := fmt.Sprintf("repos/%s/%s/actions/runs/%d/pending_deployments", c.owner, c.repo, runID)

	req, err := c.client.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var response []pendingDeploymentResponse
	resp, err := c.client.Do(ctx, req, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending deployments for run %d: %w", runID, err)
	}
	defer resp.Body.Close()

	var result []PendingDeployment
	for _, p := range response {
		result = append(result, PendingDeployment{
			EnvironmentID:         p.Environment.ID,
			EnvironmentName:       p.Environment.Name,
			RunID:                 runID,
			WaitTimer:             p.WaitTimer,
			WaitTimerStart:        p.WaitTimerStartedAt,
			CurrentUserCanApprove: p.CurrentUserCanApprove,
		})
	}
	return result, nil
}

// ApproveEnvironmentDeploymentOptions contains options for approving environment deployments.
type ApproveEnvironmentDeploymentOptions struct {
	RunID   int64    // Workflow run ID
	EnvIDs  []int64  // Environment IDs to approve (if empty, approves all pending)
	Comment string   // Approval comment
}

// ApproveEnvironmentDeployment approves pending environment deployments for a workflow run.
// This requires the authenticated user to be a Required Reviewer on the environment.
func (c *Client) ApproveEnvironmentDeployment(ctx context.Context, opts ApproveEnvironmentDeploymentOptions) error {
	// If no specific env IDs provided, get all pending deployments
	envIDs := opts.EnvIDs
	if len(envIDs) == 0 {
		pending, err := c.GetPendingDeployments(ctx, opts.RunID)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			// No pending deployments, nothing to approve
			return nil
		}
		for _, p := range pending {
			envIDs = append(envIDs, p.EnvironmentID)
		}
	}

	comment := opts.Comment
	if comment == "" {
		comment = "Approved via Enterprise Approval Engine"
	}

	// Call the GitHub API to approve
	req := github.PendingDeploymentsRequest{
		EnvironmentIDs: envIDs,
		State:          "approved",
		Comment:        comment,
	}

	_, _, err := c.client.Actions.PendingDeployments(ctx, c.owner, c.repo, opts.RunID, &req)
	if err != nil {
		return fmt.Errorf("failed to approve environment deployment for run %d: %w", opts.RunID, err)
	}

	return nil
}

// RejectEnvironmentDeployment rejects pending environment deployments for a workflow run.
func (c *Client) RejectEnvironmentDeployment(ctx context.Context, runID int64, envIDs []int64, comment string) error {
	// If no specific env IDs provided, get all pending deployments
	if len(envIDs) == 0 {
		pending, err := c.GetPendingDeployments(ctx, runID)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		for _, p := range pending {
			envIDs = append(envIDs, p.EnvironmentID)
		}
	}

	if comment == "" {
		comment = "Rejected via Enterprise Approval Engine"
	}

	req := github.PendingDeploymentsRequest{
		EnvironmentIDs: envIDs,
		State:          "rejected",
		Comment:        comment,
	}

	_, _, err := c.client.Actions.PendingDeployments(ctx, c.owner, c.repo, runID, &req)
	if err != nil {
		return fmt.Errorf("failed to reject environment deployment for run %d: %w", runID, err)
	}

	return nil
}

// ListWaitingWorkflowRuns returns workflow runs that are in "waiting" status.
func (c *Client) ListWaitingWorkflowRuns(ctx context.Context) ([]WorkflowRun, error) {
	opts := &github.ListWorkflowRunsOptions{
		Status:      "waiting",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	runs, _, err := c.client.Actions.ListRepositoryWorkflowRuns(ctx, c.owner, c.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list waiting workflow runs: %w", err)
	}

	var result []WorkflowRun
	for _, r := range runs.WorkflowRuns {
		result = append(result, WorkflowRun{
			ID:         r.GetID(),
			Name:       r.GetName(),
			Status:     r.GetStatus(),
			Conclusion: r.GetConclusion(),
			HeadBranch: r.GetHeadBranch(),
			HeadSHA:    r.GetHeadSHA(),
			RunNumber:  r.GetRunNumber(),
			URL:        r.GetHTMLURL(),
		})
	}
	return result, nil
}

// FindWaitingRunByIssue searches for a waiting workflow run that matches an issue number.
// This looks for runs where the issue number is stored in the run name or inputs.
func (c *Client) FindWaitingRunByIssue(ctx context.Context, issueNumber int) (*WorkflowRun, error) {
	runs, err := c.ListWaitingWorkflowRuns(ctx)
	if err != nil {
		return nil, err
	}

	issueStr := fmt.Sprintf("#%d", issueNumber)
	for _, run := range runs {
		// Check if run name contains the issue number
		if strings.Contains(run.Name, issueStr) {
			return &run, nil
		}
	}

	return nil, nil // No matching run found
}

// GetWorkflowRun returns a workflow run by ID.
func (c *Client) GetWorkflowRun(ctx context.Context, runID int64) (*WorkflowRun, error) {
	run, _, err := c.client.Actions.GetWorkflowRunByID(ctx, c.owner, c.repo, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow run %d: %w", runID, err)
	}

	return &WorkflowRun{
		ID:         run.GetID(),
		Name:       run.GetName(),
		Status:     run.GetStatus(),
		Conclusion: run.GetConclusion(),
		HeadBranch: run.GetHeadBranch(),
		HeadSHA:    run.GetHeadSHA(),
		RunNumber:  run.GetRunNumber(),
		URL:        run.GetHTMLURL(),
	}, nil
}

// IsRunWaiting checks if a workflow run is in "waiting" status.
func (c *Client) IsRunWaiting(ctx context.Context, runID int64) (bool, error) {
	run, err := c.GetWorkflowRun(ctx, runID)
	if err != nil {
		return false, err
	}
	return run.Status == "waiting", nil
}

