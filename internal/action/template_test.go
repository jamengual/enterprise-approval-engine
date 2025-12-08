package action

import (
	"strings"
	"testing"

	"github.com/issueops/approvals/internal/approval"
	"github.com/issueops/approvals/internal/config"
)

func TestGenerateIssueBody(t *testing.T) {
	data := TemplateData{
		Title:       "Test Approval",
		Version:     "v1.2.3",
		Requestor:   "testuser",
		Environment: "production",
		Groups: []GroupTemplateData{
			{
				Name:        "dev-team",
				Required:    "2 of 3",
				Current:     1,
				StatusEmoji: "⏳",
				StatusText:  "Pending",
			},
		},
		State: IssueState{
			Workflow:  "production-deploy",
			Version:   "v1.2.3",
			Requestor: "testuser",
		},
	}

	body, err := GenerateIssueBody(data)
	if err != nil {
		t.Fatalf("GenerateIssueBody failed: %v", err)
	}

	// Check for expected content
	checks := []string{
		"Test Approval",
		"@testuser",
		"v1.2.3",
		"production",
		"dev-team",
		"2 of 3",
		"⏳ Pending",
		"issueops-state:",
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("Expected body to contain %q, but it didn't.\nBody:\n%s", check, body)
		}
	}
}

func TestGenerateIssueBodyWithCustomTemplate(t *testing.T) {
	customTemplate := `# {{.Title}}
Version: {{.Version}}
By: @{{.Requestor}}
{{.GroupsTable}}`

	data := TemplateData{
		Title:     "Custom Test",
		Version:   "v2.0.0",
		Requestor: "alice",
		Groups: []GroupTemplateData{
			{
				Name:        "approvers",
				Required:    "1 of 2",
				Current:     0,
				StatusEmoji: "⏳",
				StatusText:  "Pending",
			},
		},
		State: IssueState{
			Workflow: "test",
		},
	}

	body, err := GenerateIssueBodyWithTemplate(data, customTemplate)
	if err != nil {
		t.Fatalf("GenerateIssueBodyWithTemplate failed: %v", err)
	}

	// Check for expected content
	if !strings.Contains(body, "# Custom Test") {
		t.Errorf("Expected body to contain custom title")
	}
	if !strings.Contains(body, "Version: v2.0.0") {
		t.Errorf("Expected body to contain version")
	}
	if !strings.Contains(body, "@alice") {
		t.Errorf("Expected body to contain requestor")
	}
	if !strings.Contains(body, "approvers") {
		t.Errorf("Expected body to contain group table")
	}
	if !strings.Contains(body, "issueops-state:") {
		t.Errorf("Expected body to contain state marker")
	}
}

func TestParseIssueState(t *testing.T) {
	body := `## Test Issue

Some content here.

<!-- issueops-state:{"workflow":"production-deploy","version":"v1.2.3","requestor":"testuser"} -->`

	state, err := ParseIssueState(body)
	if err != nil {
		t.Fatalf("ParseIssueState failed: %v", err)
	}

	if state.Workflow != "production-deploy" {
		t.Errorf("Expected workflow 'production-deploy', got %q", state.Workflow)
	}
	if state.Version != "v1.2.3" {
		t.Errorf("Expected version 'v1.2.3', got %q", state.Version)
	}
	if state.Requestor != "testuser" {
		t.Errorf("Expected requestor 'testuser', got %q", state.Requestor)
	}
}

func TestParseIssueState_NotFound(t *testing.T) {
	body := `## Test Issue

No state marker here.`

	_, err := ParseIssueState(body)
	if err == nil {
		t.Error("Expected error when state marker not found")
	}
}

func TestUpdateIssueState(t *testing.T) {
	body := `## Test Issue

<!-- issueops-state:{"workflow":"test","version":"v1.0.0","requestor":"alice"} -->`

	newState := IssueState{
		Workflow:  "test",
		Version:   "v1.0.0",
		Requestor: "alice",
		Tag:       "v1.0.0",
	}

	updated, err := UpdateIssueState(body, newState)
	if err != nil {
		t.Fatalf("UpdateIssueState failed: %v", err)
	}

	// Parse the updated state
	parsed, err := ParseIssueState(updated)
	if err != nil {
		t.Fatalf("Failed to parse updated state: %v", err)
	}

	if parsed.Tag != "v1.0.0" {
		t.Errorf("Expected tag 'v1.0.0', got %q", parsed.Tag)
	}
}

func TestReplaceTemplateVars(t *testing.T) {
	tests := []struct {
		input    string
		vars     map[string]string
		expected string
	}{
		{
			input:    "Hello {{name}}!",
			vars:     map[string]string{"name": "World"},
			expected: "Hello World!",
		},
		{
			input:    "Version {{version}} deployed to {{env}}",
			vars:     map[string]string{"version": "v1.2.3", "env": "production"},
			expected: "Version v1.2.3 deployed to production",
		},
		{
			input:    "No variables here",
			vars:     map[string]string{},
			expected: "No variables here",
		},
	}

	for _, tt := range tests {
		result := ReplaceTemplateVars(tt.input, tt.vars)
		if result != tt.expected {
			t.Errorf("ReplaceTemplateVars(%q, %v) = %q, want %q",
				tt.input, tt.vars, result, tt.expected)
		}
	}
}

func TestBuildGroupTemplateData(t *testing.T) {
	cfg := &config.Config{
		Policies: map[string]config.Policy{
			"dev-team": {
				Approvers:    []string{"alice", "bob", "charlie"},
				MinApprovals: 2,
			},
		},
		Workflows: map[string]config.Workflow{
			"test": {
				Require: []config.Requirement{
					{Policy: "dev-team"},
				},
			},
		},
	}

	workflow := cfg.Workflows["test"]
	groups := BuildGroupTemplateData(cfg, &workflow, nil)

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.Name != "dev-team" {
		t.Errorf("Expected name 'dev-team', got %q", g.Name)
	}
	if g.Required != "2 of 3" {
		t.Errorf("Expected required '2 of 3', got %q", g.Required)
	}
	if g.StatusEmoji != "⏳" {
		t.Errorf("Expected pending emoji, got %q", g.StatusEmoji)
	}
}

func TestBuildGroupTemplateData_WithResult(t *testing.T) {
	cfg := &config.Config{
		Policies: map[string]config.Policy{
			"dev-team": {
				Approvers:    []string{"alice", "bob"},
				MinApprovals: 1,
			},
		},
		Workflows: map[string]config.Workflow{
			"test": {
				Require: []config.Requirement{
					{Policy: "dev-team"},
				},
			},
		},
	}

	workflow := cfg.Workflows["test"]
	result := &approval.ApprovalResult{
		Status: approval.StatusApproved,
		Groups: []approval.GroupStatus{
			{
				Name:      "dev-team",
				Current:   1,
				Satisfied: true,
			},
		},
	}

	groups := BuildGroupTemplateData(cfg, &workflow, result)

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.Current != 1 {
		t.Errorf("Expected current 1, got %d", g.Current)
	}
	if g.StatusEmoji != "✅" {
		t.Errorf("Expected satisfied emoji, got %q", g.StatusEmoji)
	}
	if !g.Satisfied {
		t.Error("Expected group to be satisfied")
	}
}

func TestRenderGroupsTable(t *testing.T) {
	groups := []GroupTemplateData{
		{
			Name:        "dev-team",
			Required:    "2 of 3",
			Current:     1,
			StatusEmoji: "⏳",
			StatusText:  "Pending",
		},
		{
			Name:        "security",
			Required:    "all (2)",
			Current:     2,
			StatusEmoji: "✅",
			StatusText:  "Satisfied",
		},
	}

	table := renderGroupsTable(groups)

	// Check table structure
	if !strings.Contains(table, "| Group |") {
		t.Error("Expected table header")
	}
	if !strings.Contains(table, "| dev-team |") {
		t.Error("Expected dev-team row")
	}
	if !strings.Contains(table, "| security |") {
		t.Error("Expected security row")
	}
	if !strings.Contains(table, "⏳ Pending") {
		t.Error("Expected pending status")
	}
	if !strings.Contains(table, "✅ Satisfied") {
		t.Error("Expected satisfied status")
	}
}

func TestRenderGroupsTable_Empty(t *testing.T) {
	table := renderGroupsTable(nil)
	if !strings.Contains(table, "No approval groups") {
		t.Errorf("Expected empty message, got %q", table)
	}
}
