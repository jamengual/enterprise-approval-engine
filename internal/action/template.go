package action

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/issueops/approvals/internal/approval"
	"github.com/issueops/approvals/internal/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// IssueState contains hidden state embedded in the issue body.
type IssueState struct {
	Workflow     string   `json:"workflow"`
	Version      string   `json:"version,omitempty"`
	Requestor    string   `json:"requestor"`
	RunID        string   `json:"run_id,omitempty"`
	Tag          string   `json:"tag,omitempty"`          // Tag that was created (for deletion on close)
	ApprovedAt   string   `json:"approved_at,omitempty"`  // Timestamp when approved
	DeploymentID int64    `json:"deployment_id,omitempty"` // GitHub deployment ID
	Environment  string   `json:"environment,omitempty"`   // Target environment
	JiraIssues   []string `json:"jira_issues,omitempty"`   // Jira issue keys in this release
	PreviousTag  string   `json:"previous_tag,omitempty"`  // Previous tag for comparison

	// Progressive deployment fields
	Pipeline      []string          `json:"pipeline,omitempty"`       // Ordered list of environments: ["dev", "qa", "stage", "prod"]
	CurrentStage  int               `json:"current_stage,omitempty"`  // Index of current stage in pipeline (0-based)
	StageHistory  []StageCompletion `json:"stage_history,omitempty"`  // History of completed stages
	PRs           []PRInfo          `json:"prs,omitempty"`            // PRs included in this release
	Commits       []CommitInfo      `json:"commits,omitempty"`        // Commits included in this release

	// Release strategy fields
	ReleaseStrategy   string `json:"release_strategy,omitempty"`   // Strategy used: "tag", "branch", "label", "milestone"
	ReleaseIdentifier string `json:"release_identifier,omitempty"` // e.g., branch name, label, milestone title

	// Auto-approval tracking
	AutoApprovedStages []string `json:"auto_approved_stages,omitempty"` // Stages that were automatically approved
}

// StageCompletion records when a stage was completed.
type StageCompletion struct {
	Stage      string `json:"stage"`
	ApprovedBy string `json:"approved_by"`
	ApprovedAt string `json:"approved_at"`
}

// PRInfo contains information about a PR included in the release.
type PRInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"`
}

// CommitInfo contains information about a commit included in the release.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	URL     string `json:"url"`
}

// TemplateData contains data for issue body templates.
// All fields are available as template variables.
type TemplateData struct {
	// Core fields
	Title       string
	Description string
	Version     string
	Requestor   string
	Environment string

	// URLs and references
	RunURL      string
	RepoURL     string
	CommitSHA   string
	CommitURL   string
	Branch      string

	// Approval groups
	Groups         []GroupTemplateData
	GroupsTable    string // Pre-rendered markdown table
	ApprovalStatus string // "pending", "approved", "denied"

	// Timestamps
	CreatedAt string
	Timestamp string

	// Custom variables (from workflow inputs or environment)
	Vars map[string]string

	// Jira integration
	JiraIssues      []JiraIssueData // Jira issues in this release
	JiraIssuesTable string          // Pre-rendered markdown table of Jira issues
	JiraBaseURL     string          // Jira base URL for linking
	HasJiraIssues   bool            // Whether there are Jira issues

	// Deployment tracking
	DeploymentPipeline []DeploymentStageData // Deployment stages
	PipelineTable      string                // Pre-rendered deployment pipeline
	CurrentDeployment  *DeploymentStageData  // Current deployment stage

	// Release info
	PreviousVersion string // Previous version/tag
	CommitsCount    int    // Number of commits in this release
	ReleaseNotes    string // Auto-generated release notes

	// Internal state (serialized to hidden comment)
	State IssueState
}

// JiraIssueData contains data for a Jira issue.
type JiraIssueData struct {
	Key       string // e.g., "PROJ-123"
	Summary   string
	Type      string // e.g., "Bug", "Feature", "Task"
	TypeEmoji string // e.g., "🐛", "✨", "📋"
	Status    string // e.g., "Done", "In Progress"
	StatusEmoji string
	URL       string // Full URL to the issue
	Assignee  string
}

// DeploymentStageData contains data for a deployment stage.
type DeploymentStageData struct {
	Environment string // e.g., "development", "staging", "production"
	Status      string // e.g., "deployed", "pending", "not_started"
	StatusEmoji string
	Version     string // Version deployed to this environment
	DeployedAt  string
	IsCurrent   bool // Is this the current target environment?
}

// GroupTemplateData contains data for a single approval group.
type GroupTemplateData struct {
	Name        string
	Approvers   []string
	Required    string // "all" or "X of Y"
	Current     int
	StatusEmoji string
	StatusText  string
	Satisfied   bool
}

// DefaultIssueTemplate is a comprehensive default template that can be fully customized.
const DefaultIssueTemplate = `## 🚀 {{.Title}}

{{- if .Description}}
{{.Description}}
{{- end}}

### Request Information

{{- if .Requestor}}
- **Requested by:** @{{.Requestor}}
{{- end}}
{{- if .Version}}
- **Version:** ` + "`{{.Version}}`" + `{{if .PreviousVersion}} (from ` + "`{{.PreviousVersion}}`" + `){{end}}
{{- end}}
{{- if .Environment}}
- **Environment:** {{.Environment}}
{{- end}}
{{- if .Branch}}
- **Branch:** ` + "`{{.Branch}}`" + `
{{- end}}
{{- if .CommitSHA}}
- **Commit:** [{{slice .CommitSHA 0 7}}]({{.CommitURL}})
{{- end}}
{{- if .CommitsCount}}
- **Commits:** {{.CommitsCount}} commits in this release
{{- end}}
{{- if .RunURL}}
- **Workflow Run:** [View Run]({{.RunURL}})
{{- end}}
- **Requested at:** {{.Timestamp}}
{{if .HasJiraIssues}}
---

### 📋 Jira Issues in this Release

| Key | Summary | Type | Status |
|-----|---------|------|--------|
{{- range .JiraIssues}}
| [{{.Key}}]({{.URL}}) | {{.Summary}} | {{.TypeEmoji}} {{.Type}} | {{.StatusEmoji}} {{.Status}} |
{{- end}}
{{end}}{{if .PipelineTable}}
---

### 🛤️ Deployment Pipeline

{{.PipelineTable}}
{{end}}
---

### ✅ Approval Requirements

This request can be approved by **any one** of the following groups:

| Group | Required | Current | Status |
|-------|----------|---------|--------|
{{- range .Groups}}
| {{.Name}} | {{.Required}} | {{.Current}} | {{.StatusEmoji}} {{.StatusText}} |
{{- end}}

---

### How to Respond

**To Approve:** Comment with one of:
` + "` approve ` ` approved ` ` lgtm ` ` yes ` ` /approve `" + `

**To Deny:** Comment with one of:
` + "` deny ` ` denied ` ` reject ` ` no ` ` /deny `" + `

---
`

// MinimalIssueTemplate is a simpler template for basic use cases.
const MinimalIssueTemplate = `## {{.Title}}

**Requested by:** @{{.Requestor}}
{{- if .Version}} | **Version:** {{.Version}}{{end}}
{{- if .Environment}} | **Environment:** {{.Environment}}{{end}}

### Approval Status

{{.GroupsTable}}

**Approve:** Comment ` + "`approve`" + ` | **Deny:** Comment ` + "`deny`" + `
`

// stateMarkerStart is the marker for the hidden state in issue body.
const stateMarkerStart = "<!-- issueops-state:"
const stateMarkerEnd = " -->"

// GenerateIssueBody generates the issue body from template data using the default template.
func GenerateIssueBody(data TemplateData) (string, error) {
	return GenerateIssueBodyWithTemplate(data, DefaultIssueTemplate)
}

// GenerateIssueBodyWithTemplate generates the issue body using a custom template string.
func GenerateIssueBodyWithTemplate(data TemplateData, templateStr string) (string, error) {
	// Pre-render the groups table for {{.GroupsTable}} convenience variable
	data.GroupsTable = renderGroupsTable(data.Groups)

	// Pre-render Jira issues table
	if len(data.JiraIssues) > 0 {
		data.JiraIssuesTable = renderJiraIssuesTable(data.JiraIssues)
		data.HasJiraIssues = true
	}

	// Pre-render deployment pipeline table
	if len(data.DeploymentPipeline) > 0 {
		data.PipelineTable = renderPipelineTable(data.DeploymentPipeline)
	}

	// Set timestamp if not provided
	if data.Timestamp == "" {
		data.Timestamp = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	}

	// Create template with custom functions
	funcMap := template.FuncMap{
		"slice": func(s string, start, end int) string {
			if start < 0 {
				start = 0
			}
			if end > len(s) {
				end = len(s)
			}
			if start >= len(s) {
				return ""
			}
			return s[start:end]
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": func(s string) string {
			return cases.Title(language.English).String(s)
		},
		"contains": strings.Contains,
		"replace":  strings.ReplaceAll,
		"join":     strings.Join,
		"default": func(def, val string) string {
			if val == "" {
				return def
			}
			return val
		},
	}

	tmpl, err := template.New("issue").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Append the hidden state marker
	stateJSON, err := json.Marshal(data.State)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	body := buf.String()
	body += "\n" + stateMarkerStart + string(stateJSON) + stateMarkerEnd

	return body, nil
}

// GenerateIssueBodyFromConfig generates the issue body using template from config.
func GenerateIssueBodyFromConfig(data TemplateData, issueConfig *config.IssueConfig) (string, error) {
	templateStr := DefaultIssueTemplate

	// Check for custom template in config
	if issueConfig != nil {
		if issueConfig.Body != "" {
			templateStr = issueConfig.Body
		} else if issueConfig.BodyFile != "" {
			// Load template from file
			content, err := loadTemplateFile(issueConfig.BodyFile)
			if err != nil {
				return "", fmt.Errorf("failed to load template file: %w", err)
			}
			templateStr = content
		}
	}

	return GenerateIssueBodyWithTemplate(data, templateStr)
}

// loadTemplateFile loads a template file from the .github directory.
func loadTemplateFile(path string) (string, error) {
	// If path doesn't start with .github/, prepend it
	if !strings.HasPrefix(path, ".github/") && !strings.HasPrefix(path, ".github\\") {
		path = filepath.Join(".github", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// renderGroupsTable renders the approval groups as a markdown table.
func renderGroupsTable(groups []GroupTemplateData) string {
	if len(groups) == 0 {
		return "_No approval groups configured_"
	}

	var buf bytes.Buffer
	buf.WriteString("| Group | Required | Current | Status |\n")
	buf.WriteString("|-------|----------|---------|--------|\n")

	for _, g := range groups {
		buf.WriteString(fmt.Sprintf("| %s | %s | %d | %s %s |\n",
			g.Name, g.Required, g.Current, g.StatusEmoji, g.StatusText))
	}

	return buf.String()
}

// renderJiraIssuesTable renders Jira issues as a markdown table.
func renderJiraIssuesTable(issues []JiraIssueData) string {
	if len(issues) == 0 {
		return "_No Jira issues found_"
	}

	var buf bytes.Buffer
	buf.WriteString("| Key | Summary | Type | Status |\n")
	buf.WriteString("|-----|---------|------|--------|\n")

	for _, issue := range issues {
		summary := issue.Summary
		// Truncate long summaries
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}
		// Escape pipe characters in summary
		summary = strings.ReplaceAll(summary, "|", "\\|")

		buf.WriteString(fmt.Sprintf("| [%s](%s) | %s | %s %s | %s %s |\n",
			issue.Key, issue.URL, summary,
			issue.TypeEmoji, issue.Type,
			issue.StatusEmoji, issue.Status))
	}

	return buf.String()
}

// renderPipelineTable renders the deployment pipeline as a markdown table.
func renderPipelineTable(stages []DeploymentStageData) string {
	if len(stages) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("| Environment | Version | Status |\n")
	buf.WriteString("|-------------|---------|--------|\n")

	for _, stage := range stages {
		env := stage.Environment
		if stage.IsCurrent {
			env = "**" + env + "**"
		}

		version := stage.Version
		if version == "" {
			version = "-"
		}

		status := stage.StatusEmoji + " " + stage.Status
		if stage.IsCurrent {
			status = "**" + status + "**"
		}

		buf.WriteString(fmt.Sprintf("| %s | %s | %s |\n", env, version, status))
	}

	return buf.String()
}

// BuildDeploymentPipeline creates deployment stage data for common environments.
func BuildDeploymentPipeline(targetEnv string, version string, previousVersion string) []DeploymentStageData {
	// Common deployment pipeline stages
	stages := []DeploymentStageData{
		{Environment: "development", Status: "deployed", StatusEmoji: "✅", Version: version},
		{Environment: "staging", Status: "deployed", StatusEmoji: "✅", Version: version},
		{Environment: "production", Status: "awaiting approval", StatusEmoji: "⏳", Version: previousVersion},
	}

	// Mark the target environment as current and update status
	for i := range stages {
		if stages[i].Environment == targetEnv {
			stages[i].IsCurrent = true
			stages[i].Status = "awaiting approval"
			stages[i].StatusEmoji = "⏳"
			// Previous stages are already deployed
			break
		}
	}

	return stages
}

// BuildGroupTemplateData converts config requirements to template data.
func BuildGroupTemplateData(cfg *config.Config, workflow *config.Workflow, result *approval.ApprovalResult) []GroupTemplateData {
	var groups []GroupTemplateData

	for i, req := range workflow.Require {
		approvers, minApprovals, requireAll := cfg.ResolveRequirement(req)

		var required string
		if requireAll {
			required = fmt.Sprintf("all (%d)", len(approvers))
		} else {
			required = fmt.Sprintf("%d of %d", minApprovals, len(approvers))
		}

		// Get current status from result if available
		current := 0
		statusEmoji := "⏳"
		statusText := "Pending"

		if result != nil && i < len(result.Groups) {
			group := result.Groups[i]
			current = group.Current
			if group.Satisfied {
				statusEmoji = "✅"
				statusText = "Satisfied"
			}
		}

		satisfied := false
		if result != nil && i < len(result.Groups) {
			satisfied = result.Groups[i].Satisfied
		}

		groups = append(groups, GroupTemplateData{
			Name:        req.Name(),
			Approvers:   approvers,
			Required:    required,
			Current:     current,
			StatusEmoji: statusEmoji,
			StatusText:  statusText,
			Satisfied:   satisfied,
		})
	}

	return groups
}

// ParseIssueState extracts the hidden state from an issue body.
func ParseIssueState(body string) (*IssueState, error) {
	startIdx := strings.Index(body, stateMarkerStart)
	if startIdx == -1 {
		return nil, fmt.Errorf("issue state not found in body")
	}

	startIdx += len(stateMarkerStart)
	endIdx := strings.Index(body[startIdx:], stateMarkerEnd)
	if endIdx == -1 {
		return nil, fmt.Errorf("issue state end marker not found")
	}

	stateJSON := body[startIdx : startIdx+endIdx]

	var state IssueState
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return nil, fmt.Errorf("failed to parse issue state: %w", err)
	}

	return &state, nil
}

// UpdateIssueState updates the state marker in an issue body.
func UpdateIssueState(body string, state IssueState) (string, error) {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	startIdx := strings.Index(body, stateMarkerStart)
	if startIdx == -1 {
		// No existing state, append it
		return body + "\n" + stateMarkerStart + string(stateJSON) + stateMarkerEnd, nil
	}

	endIdx := strings.Index(body[startIdx:], stateMarkerEnd)
	if endIdx == -1 {
		return "", fmt.Errorf("issue state end marker not found")
	}

	// Replace the state
	newBody := body[:startIdx] + stateMarkerStart + string(stateJSON) + stateMarkerEnd
	remainder := body[startIdx+endIdx+len(stateMarkerEnd):]
	if remainder != "" {
		newBody += remainder
	}

	return newBody, nil
}

// ReplaceTemplateVars replaces template variables in a string.
func ReplaceTemplateVars(s string, vars map[string]string) string {
	for key, value := range vars {
		s = strings.ReplaceAll(s, "{{"+key+"}}", value)
	}
	return s
}
