package action

import (
	"strings"
	"testing"
	"time"

	"github.com/jamengual/enterprise-approval-engine/internal/config"
)

func TestGeneratePipelineTable(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "dev", Environment: "development"},
			{Name: "staging", Environment: "staging"},
			{Name: "production", Environment: "production"},
		},
	}

	state := &IssueState{
		CurrentStage: 1,
		StageHistory: []StageCompletion{
			{
				Stage:      "dev",
				ApprovedBy: "alice",
				ApprovedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	table := GeneratePipelineTable(state, pipeline)

	// Check table structure
	if !strings.Contains(table, "| Stage |") {
		t.Error("Expected table header")
	}
	if !strings.Contains(table, "DEV") {
		t.Error("Expected DEV stage")
	}
	if !strings.Contains(table, "✅ Deployed") {
		t.Error("Expected deployed status for dev")
	}
	if !strings.Contains(table, "@alice") {
		t.Error("Expected approver name")
	}
	if !strings.Contains(table, "⏳ Awaiting") {
		t.Error("Expected awaiting status for current stage")
	}
	if !strings.Contains(table, "⬜ Pending") {
		t.Error("Expected pending status for future stage")
	}
}

func TestGeneratePipelineTable_Empty(t *testing.T) {
	table := GeneratePipelineTable(&IssueState{}, nil)
	if table != "" {
		t.Errorf("Expected empty string, got %q", table)
	}

	table = GeneratePipelineTable(&IssueState{}, &config.PipelineConfig{})
	if table != "" {
		t.Errorf("Expected empty string for empty stages, got %q", table)
	}
}

func TestGeneratePipelineTable_AutoApprove(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "dev", AutoApprove: true},
			{Name: "staging", AutoApprove: true},
			{Name: "production", AutoApprove: false},
		},
	}

	state := &IssueState{
		CurrentStage: 0,
		StageHistory: []StageCompletion{},
	}

	table := GeneratePipelineTable(state, pipeline)

	// Auto-approve stages should show robot emoji
	if !strings.Contains(table, "🤖") {
		t.Error("Expected robot emoji for auto-approve stages")
	}
}

func TestGeneratePipelineTable_AutoApprovedStages(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "dev", AutoApprove: true},
			{Name: "staging", AutoApprove: false},
		},
	}

	state := &IssueState{
		CurrentStage: 1,
		StageHistory: []StageCompletion{
			{
				Stage:      "dev",
				ApprovedBy: "[auto]",
				ApprovedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	table := GeneratePipelineTable(state, pipeline)

	// Auto-approved should show auto-deployed
	if !strings.Contains(table, "🤖 Auto-deployed") {
		t.Error("Expected auto-deployed status")
	}
	if !strings.Contains(table, "| auto |") {
		t.Error("Expected 'auto' as approver")
	}
}

func TestGeneratePipelineTable_InvalidTimestamp(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "dev"},
		},
	}

	state := &IssueState{
		CurrentStage: 1,
		StageHistory: []StageCompletion{
			{
				Stage:      "dev",
				ApprovedBy: "alice",
				ApprovedAt: "invalid-timestamp",
			},
		},
	}

	table := GeneratePipelineTable(state, pipeline)

	// Should still generate table even with invalid timestamp
	if !strings.Contains(table, "DEV") {
		t.Error("Expected table to be generated")
	}
}

func TestGeneratePRTable(t *testing.T) {
	prs := []PRInfo{
		{
			Number: 123,
			Title:  "Fix bug in login",
			Author: "alice",
			URL:    "https://github.com/org/repo/pull/123",
		},
		{
			Number: 124,
			Title:  "Add new feature",
			Author: "bob",
			URL:    "https://github.com/org/repo/pull/124",
		},
	}

	table := GeneratePRTable(prs)

	// Check table structure
	if !strings.Contains(table, "| PR |") {
		t.Error("Expected table header")
	}
	if !strings.Contains(table, "[#123]") {
		t.Error("Expected PR number with link")
	}
	if !strings.Contains(table, "Fix bug in login") {
		t.Error("Expected PR title")
	}
	if !strings.Contains(table, "@alice") {
		t.Error("Expected PR author")
	}
}

func TestGeneratePRTable_Empty(t *testing.T) {
	table := GeneratePRTable(nil)
	if !strings.Contains(table, "No PRs") {
		t.Errorf("Expected empty message, got %q", table)
	}
}

func TestGenerateCommitList(t *testing.T) {
	commits := []CommitInfo{
		{
			SHA:     "abc123456789",
			Message: "Fix critical bug\n\nThis is a longer description",
			Author:  "alice",
			URL:     "https://github.com/org/repo/commit/abc123456789",
		},
		{
			SHA:     "def456",
			Message: "Add feature",
			Author:  "bob",
			URL:     "https://github.com/org/repo/commit/def456",
		},
	}

	list := GenerateCommitList(commits)

	// Check list structure
	if !strings.Contains(list, "[`abc1234`]") {
		t.Error("Expected truncated SHA")
	}
	if !strings.Contains(list, "Fix critical bug") {
		t.Error("Expected first line of commit message")
	}
	if strings.Contains(list, "longer description") {
		t.Error("Should not contain second line of commit message")
	}
	if !strings.Contains(list, "[`def456`]") {
		t.Error("Expected short SHA preserved when already short")
	}
}

func TestGenerateCommitList_Empty(t *testing.T) {
	list := GenerateCommitList(nil)
	if !strings.Contains(list, "No commits") {
		t.Errorf("Expected empty message, got %q", list)
	}
}

func TestGeneratePipelineIssueBody(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "dev", Environment: "development"},
			{Name: "staging", Environment: "staging"},
			{Name: "production", Environment: "production"},
		},
	}

	state := &IssueState{
		Workflow:     "pipeline-deploy",
		Version:      "v1.0.0",
		Requestor:    "alice",
		CurrentStage: 1,
		StageHistory: []StageCompletion{
			{Stage: "dev", ApprovedBy: "bob", ApprovedAt: time.Now().Format(time.RFC3339)},
		},
		PRs: []PRInfo{
			{Number: 123, Title: "Fix bug", Author: "alice", URL: "https://example.com"},
		},
		Commits: []CommitInfo{
			{SHA: "abc123456789", Message: "Fix bug", Author: "alice", URL: "https://example.com"},
		},
	}

	data := &TemplateData{
		Version:     "v1.0.0",
		Description: "Deploying new version",
		Requestor:   "alice",
		Branch:      "main",
		CommitSHA:   "abc123456789",
		CommitURL:   "https://example.com/commit/abc123",
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	body := GeneratePipelineIssueBody(data, state, pipeline)

	// Check body structure
	if !strings.Contains(body, "Deployment Pipeline") {
		t.Error("Expected pipeline title")
	}
	if !strings.Contains(body, "v1.0.0") {
		t.Error("Expected version")
	}
	if !strings.Contains(body, "@alice") {
		t.Error("Expected requestor")
	}
	if !strings.Contains(body, "STAGING") {
		t.Error("Expected current stage info")
	}
	if !strings.Contains(body, "Pull Requests") {
		t.Error("Expected PRs section")
	}
	if !strings.Contains(body, "Commits") {
		t.Error("Expected commits section")
	}
	if !strings.Contains(body, "issueops-state:") {
		t.Error("Expected state marker")
	}
}

func TestGeneratePipelineIssueBody_NoOptionalFields(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "production"},
		},
	}

	state := &IssueState{
		Workflow:     "deploy",
		Version:      "v1.0.0",
		Requestor:    "alice",
		CurrentStage: 0,
	}

	data := &TemplateData{
		Version:     "v1.0.0",
		Description: "Deploy",
		Requestor:   "alice",
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	body := GeneratePipelineIssueBody(data, state, pipeline)

	// Should still generate valid body without optional fields
	if !strings.Contains(body, "v1.0.0") {
		t.Error("Expected version")
	}
	// Should not have empty sections
	if strings.Contains(body, "Pull Requests") {
		t.Error("Should not have PRs section when empty")
	}
	if strings.Contains(body, "Commits") {
		t.Error("Should not have Commits section when empty")
	}
}

func TestGeneratePipelineIssueBody_PipelineComplete(t *testing.T) {
	pipeline := &config.PipelineConfig{
		Stages: []config.PipelineStage{
			{Name: "dev"},
			{Name: "production"},
		},
	}

	state := &IssueState{
		Workflow:     "deploy",
		Version:      "v1.0.0",
		Requestor:    "alice",
		CurrentStage: 2, // Past all stages
		StageHistory: []StageCompletion{
			{Stage: "dev", ApprovedBy: "bob", ApprovedAt: time.Now().Format(time.RFC3339)},
			{Stage: "production", ApprovedBy: "charlie", ApprovedAt: time.Now().Format(time.RFC3339)},
		},
	}

	data := &TemplateData{
		Version:     "v1.0.0",
		Description: "Deploy",
		Requestor:   "alice",
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	body := GeneratePipelineIssueBody(data, state, pipeline)

	// Should not show "Current Stage" when pipeline is complete
	if strings.Contains(body, "Current Stage") {
		t.Error("Should not show current stage when pipeline is complete")
	}
}
