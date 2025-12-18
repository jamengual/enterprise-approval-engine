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

// Additional tests for comprehensive coverage

func TestRenderJiraIssuesTable(t *testing.T) {
	issues := []JiraIssueData{
		{
			Key:         "PROJ-123",
			Summary:     "Fix login bug",
			Type:        "Bug",
			TypeEmoji:   "🐛",
			Status:      "Done",
			StatusEmoji: "✅",
			URL:         "https://jira.example.com/browse/PROJ-123",
			Assignee:    "alice",
		},
		{
			Key:         "PROJ-124",
			Summary:     "Add new feature with a very long summary that should be truncated to fit the table",
			Type:        "Feature",
			TypeEmoji:   "✨",
			Status:      "In Progress",
			StatusEmoji: "🔄",
			URL:         "https://jira.example.com/browse/PROJ-124",
			Assignee:    "bob",
		},
	}

	table := renderJiraIssuesTable(issues)

	// Check table structure
	if !strings.Contains(table, "| Key |") {
		t.Error("Expected table header")
	}
	if !strings.Contains(table, "PROJ-123") {
		t.Error("Expected first issue key")
	}
	if !strings.Contains(table, "Fix login bug") {
		t.Error("Expected first issue summary")
	}
	if !strings.Contains(table, "🐛 Bug") {
		t.Error("Expected bug type with emoji")
	}
	if !strings.Contains(table, "...") {
		t.Error("Expected truncated summary")
	}
}

func TestRenderJiraIssuesTable_Empty(t *testing.T) {
	table := renderJiraIssuesTable(nil)
	if !strings.Contains(table, "No Jira issues") {
		t.Errorf("Expected empty message, got %q", table)
	}
}

func TestRenderJiraIssuesTable_WithPipe(t *testing.T) {
	// Test that pipe characters are escaped
	issues := []JiraIssueData{
		{
			Key:       "PROJ-125",
			Summary:   "Fix bug with | pipe character",
			Type:      "Bug",
			TypeEmoji: "🐛",
			Status:    "Done",
			URL:       "https://jira.example.com/browse/PROJ-125",
		},
	}

	table := renderJiraIssuesTable(issues)

	// Pipe should be escaped
	if !strings.Contains(table, "\\|") {
		t.Error("Expected escaped pipe character")
	}
}

func TestRenderPipelineTable(t *testing.T) {
	stages := []DeploymentStageData{
		{
			Environment: "development",
			Status:      "deployed",
			StatusEmoji: "✅",
			Version:     "v1.0.0",
			IsCurrent:   false,
		},
		{
			Environment: "staging",
			Status:      "awaiting approval",
			StatusEmoji: "⏳",
			Version:     "v0.9.0",
			IsCurrent:   true,
		},
		{
			Environment: "production",
			Status:      "not started",
			StatusEmoji: "⬜",
			Version:     "",
			IsCurrent:   false,
		},
	}

	table := renderPipelineTable(stages)

	// Check table structure
	if !strings.Contains(table, "| Environment |") {
		t.Error("Expected table header")
	}
	if !strings.Contains(table, "development") {
		t.Error("Expected development environment")
	}
	if !strings.Contains(table, "**staging**") {
		t.Error("Expected bold staging (current)")
	}
	if !strings.Contains(table, "**⏳ awaiting approval**") {
		t.Error("Expected bold status for current stage")
	}
	if !strings.Contains(table, "| - |") {
		t.Error("Expected dash for empty version")
	}
}

func TestRenderPipelineTable_Empty(t *testing.T) {
	table := renderPipelineTable(nil)
	if table != "" {
		t.Errorf("Expected empty string, got %q", table)
	}
}

func TestBuildDeploymentPipeline(t *testing.T) {
	stages := BuildDeploymentPipeline("staging", "v1.0.0", "v0.9.0")

	if len(stages) != 3 {
		t.Fatalf("Expected 3 stages, got %d", len(stages))
	}

	// Check staging is marked as current
	found := false
	for _, stage := range stages {
		if stage.Environment == "staging" {
			if !stage.IsCurrent {
				t.Error("Expected staging to be current")
			}
			if stage.Status != "awaiting approval" {
				t.Errorf("Expected awaiting approval, got %q", stage.Status)
			}
			found = true
		}
	}
	if !found {
		t.Error("Staging environment not found")
	}
}

func TestBuildDeploymentPipeline_Production(t *testing.T) {
	stages := BuildDeploymentPipeline("production", "v2.0.0", "v1.0.0")

	for _, stage := range stages {
		if stage.Environment == "production" {
			if !stage.IsCurrent {
				t.Error("Expected production to be current")
			}
			break
		}
	}
}

func TestGenerateIssueBodyWithTemplate_InvalidTemplate(t *testing.T) {
	invalidTemplate := `{{.InvalidField`

	data := TemplateData{
		Title: "Test",
		State: IssueState{Workflow: "test"},
	}

	_, err := GenerateIssueBodyWithTemplate(data, invalidTemplate)
	if err == nil {
		t.Error("Expected error for invalid template")
	}
}

func TestGenerateIssueBodyWithTemplate_ExecutionError(t *testing.T) {
	// Template that will fail during execution
	badTemplate := `{{slice .CommitSHA 100 200}}`

	data := TemplateData{
		CommitSHA: "abc", // Too short for slice range
		State:     IssueState{Workflow: "test"},
	}

	// Should not error, slice function handles out of range
	body, err := GenerateIssueBodyWithTemplate(data, badTemplate)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if body == "" {
		t.Error("Expected non-empty body")
	}
}

func TestGenerateIssueBodyWithTemplate_TemplateFunctions(t *testing.T) {
	template := `
{{upper .Title}}
{{lower .Title}}
{{title .Title}}
{{contains .Title "test"}}
{{replace .Title "test" "demo"}}
{{default "N/A" .Environment}}
{{slice .CommitSHA 0 7}}
`
	data := TemplateData{
		Title:     "test title",
		CommitSHA: "abc1234567890",
		Groups: []GroupTemplateData{
			{Approvers: []string{"alice", "bob"}},
		},
		Environment: "",
		State:       IssueState{Workflow: "test"},
	}

	body, err := GenerateIssueBodyWithTemplate(data, template)
	if err != nil {
		t.Fatalf("GenerateIssueBodyWithTemplate failed: %v", err)
	}

	if !strings.Contains(body, "TEST TITLE") {
		t.Error("Expected upper case")
	}
	if !strings.Contains(body, "test title") {
		t.Error("Expected lower case")
	}
	if !strings.Contains(body, "Test Title") {
		t.Error("Expected title case")
	}
	if !strings.Contains(body, "true") {
		t.Error("Expected contains result")
	}
	if !strings.Contains(body, "demo title") {
		t.Error("Expected replace result")
	}
	if !strings.Contains(body, "N/A") {
		t.Error("Expected default value")
	}
	if !strings.Contains(body, "abc1234") {
		t.Error("Expected slice result")
	}
}

func TestGenerateIssueBodyWithTemplate_WithJiraAndPipeline(t *testing.T) {
	data := TemplateData{
		Title:     "Test",
		Requestor: "alice",
		JiraIssues: []JiraIssueData{
			{Key: "PROJ-1", Summary: "Test issue", URL: "https://jira/PROJ-1"},
		},
		DeploymentPipeline: []DeploymentStageData{
			{Environment: "dev", Status: "deployed", StatusEmoji: "✅"},
		},
		State: IssueState{Workflow: "test"},
	}

	body, err := GenerateIssueBody(data)
	if err != nil {
		t.Fatalf("GenerateIssueBody failed: %v", err)
	}

	if !strings.Contains(body, "PROJ-1") {
		t.Error("Expected Jira issue in body")
	}
	// Check body contains Jira issues section (HasJiraIssues is set internally)
	if !strings.Contains(body, "Jira Issues") {
		t.Error("Expected Jira Issues section in body")
	}
}

func TestParseIssueState_MissingEndMarker(t *testing.T) {
	body := `## Test Issue

<!-- issueops-state:{"workflow":"test"} No end marker`

	_, err := ParseIssueState(body)
	if err == nil {
		t.Error("Expected error when end marker not found")
	}
	if !strings.Contains(err.Error(), "end marker") {
		t.Errorf("Expected end marker error, got: %v", err)
	}
}

func TestParseIssueState_InvalidJSON(t *testing.T) {
	body := `## Test Issue

<!-- issueops-state:invalid-json -->`

	_, err := ParseIssueState(body)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse issue state") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

func TestParseIssueState_ComplexState(t *testing.T) {
	body := `## Test Issue

<!-- issueops-state:{"workflow":"pipeline","version":"v1.0.0","requestor":"alice","pipeline":["dev","staging","prod"],"current_stage":1,"jira_issues":["PROJ-1","PROJ-2"]} -->`

	state, err := ParseIssueState(body)
	if err != nil {
		t.Fatalf("ParseIssueState failed: %v", err)
	}

	if state.Workflow != "pipeline" {
		t.Errorf("Expected workflow 'pipeline', got %q", state.Workflow)
	}
	if state.CurrentStage != 1 {
		t.Errorf("Expected current_stage 1, got %d", state.CurrentStage)
	}
	if len(state.Pipeline) != 3 {
		t.Errorf("Expected 3 pipeline stages, got %d", len(state.Pipeline))
	}
	if len(state.JiraIssues) != 2 {
		t.Errorf("Expected 2 jira issues, got %d", len(state.JiraIssues))
	}
}

func TestUpdateIssueState_NoExistingState(t *testing.T) {
	body := `## Test Issue

No state marker here.`

	newState := IssueState{
		Workflow:  "test",
		Requestor: "bob",
	}

	updated, err := UpdateIssueState(body, newState)
	if err != nil {
		t.Fatalf("UpdateIssueState failed: %v", err)
	}

	// Should append state
	if !strings.Contains(updated, "issueops-state:") {
		t.Error("Expected state marker to be appended")
	}

	// Parse and verify
	parsed, err := ParseIssueState(updated)
	if err != nil {
		t.Fatalf("Failed to parse updated state: %v", err)
	}
	if parsed.Workflow != "test" {
		t.Errorf("Expected workflow 'test', got %q", parsed.Workflow)
	}
}

func TestUpdateIssueState_MissingEndMarker(t *testing.T) {
	body := `<!-- issueops-state:{"workflow":"test"} no end marker`

	newState := IssueState{Workflow: "updated"}

	_, err := UpdateIssueState(body, newState)
	if err == nil {
		t.Error("Expected error when end marker not found")
	}
}

func TestUpdateIssueState_WithContentAfterState(t *testing.T) {
	body := `## Test Issue

<!-- issueops-state:{"workflow":"old"} -->

Footer content here.`

	newState := IssueState{Workflow: "new"}

	updated, err := UpdateIssueState(body, newState)
	if err != nil {
		t.Fatalf("UpdateIssueState failed: %v", err)
	}

	// Should preserve footer
	if !strings.Contains(updated, "Footer content") {
		t.Error("Expected footer content to be preserved")
	}

	// Parse and verify new state
	parsed, err := ParseIssueState(updated)
	if err != nil {
		t.Fatalf("Failed to parse updated state: %v", err)
	}
	if parsed.Workflow != "new" {
		t.Errorf("Expected workflow 'new', got %q", parsed.Workflow)
	}
}

func TestBuildGroupTemplateData_RequireAll(t *testing.T) {
	cfg := &config.Config{
		Policies: map[string]config.Policy{
			"strict": {
				Approvers:  []string{"alice", "bob"},
				RequireAll: true,
			},
		},
		Workflows: map[string]config.Workflow{
			"test": {
				Require: []config.Requirement{
					{Policy: "strict"},
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
	if g.Required != "all (2)" {
		t.Errorf("Expected 'all (2)', got %q", g.Required)
	}
}

func TestBuildGroupTemplateData_Denied(t *testing.T) {
	cfg := &config.Config{
		Policies: map[string]config.Policy{
			"team": {
				Approvers:    []string{"alice"},
				MinApprovals: 1,
			},
		},
		Workflows: map[string]config.Workflow{
			"test": {
				Require: []config.Requirement{
					{Policy: "team"},
				},
			},
		},
	}

	workflow := cfg.Workflows["test"]
	result := &approval.ApprovalResult{
		Status: approval.StatusDenied,
		Groups: []approval.GroupStatus{
			{Name: "team", Current: 0, Satisfied: false},
		},
	}

	groups := BuildGroupTemplateData(cfg, &workflow, result)

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	// When not satisfied, should show pending status
	if g.StatusEmoji != "⏳" {
		t.Errorf("Expected pending emoji, got %q", g.StatusEmoji)
	}
}

func TestBuildGroupTemplateData_MultipleGroups(t *testing.T) {
	cfg := &config.Config{
		Policies: map[string]config.Policy{
			"dev": {
				Approvers:    []string{"alice", "bob"},
				MinApprovals: 1,
			},
			"ops": {
				Approvers:  []string{"charlie"},
				RequireAll: true,
			},
		},
		Workflows: map[string]config.Workflow{
			"test": {
				Require: []config.Requirement{
					{Policy: "dev"},
					{Policy: "ops"},
				},
			},
		},
	}

	workflow := cfg.Workflows["test"]
	result := &approval.ApprovalResult{
		Status: approval.StatusApproved,
		Groups: []approval.GroupStatus{
			{Name: "dev", Current: 1, Satisfied: true},
			{Name: "ops", Current: 0, Satisfied: false},
		},
	}

	groups := BuildGroupTemplateData(cfg, &workflow, result)

	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	// First group should be satisfied
	if !groups[0].Satisfied {
		t.Error("Expected first group to be satisfied")
	}
	if groups[0].StatusEmoji != "✅" {
		t.Errorf("Expected satisfied emoji, got %q", groups[0].StatusEmoji)
	}

	// Second group should not be satisfied
	if groups[1].Satisfied {
		t.Error("Expected second group to not be satisfied")
	}
}
