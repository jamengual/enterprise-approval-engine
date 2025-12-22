package github

import (
	"testing"
)

func TestPendingDeployment(t *testing.T) {
	// Test PendingDeployment struct
	pd := PendingDeployment{
		EnvironmentID:         123,
		EnvironmentName:       "production",
		RunID:                 456,
		WaitTimer:             0,
		CurrentUserCanApprove: true,
	}

	if pd.EnvironmentID != 123 {
		t.Errorf("EnvironmentID = %d, want 123", pd.EnvironmentID)
	}
	if pd.EnvironmentName != "production" {
		t.Errorf("EnvironmentName = %s, want production", pd.EnvironmentName)
	}
	if pd.RunID != 456 {
		t.Errorf("RunID = %d, want 456", pd.RunID)
	}
	if !pd.CurrentUserCanApprove {
		t.Errorf("CurrentUserCanApprove = false, want true")
	}
}

func TestWorkflowRun(t *testing.T) {
	// Test WorkflowRun struct
	run := WorkflowRun{
		ID:         12345,
		Name:       "Deploy #123",
		Status:     "waiting",
		Conclusion: "",
		HeadBranch: "main",
		HeadSHA:    "abc123",
		RunNumber:  42,
		URL:        "https://github.com/owner/repo/actions/runs/12345",
	}

	if run.ID != 12345 {
		t.Errorf("ID = %d, want 12345", run.ID)
	}
	if run.Status != "waiting" {
		t.Errorf("Status = %s, want waiting", run.Status)
	}
	if run.HeadBranch != "main" {
		t.Errorf("HeadBranch = %s, want main", run.HeadBranch)
	}
}

func TestApproveEnvironmentDeploymentOptions(t *testing.T) {
	// Test ApproveEnvironmentDeploymentOptions struct
	opts := ApproveEnvironmentDeploymentOptions{
		RunID:   12345,
		EnvIDs:  []int64{100, 200},
		Comment: "Approved via test",
	}

	if opts.RunID != 12345 {
		t.Errorf("RunID = %d, want 12345", opts.RunID)
	}
	if len(opts.EnvIDs) != 2 {
		t.Errorf("len(EnvIDs) = %d, want 2", len(opts.EnvIDs))
	}
	if opts.Comment != "Approved via test" {
		t.Errorf("Comment = %s, want 'Approved via test'", opts.Comment)
	}
}
