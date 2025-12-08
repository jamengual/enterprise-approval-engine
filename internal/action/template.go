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
)

// IssueState contains hidden state embedded in the issue body.
type IssueState struct {
	Workflow   string `json:"workflow"`
	Version    string `json:"version,omitempty"`
	Requestor  string `json:"requestor"`
	RunID      string `json:"run_id,omitempty"`
	Tag        string `json:"tag,omitempty"`        // Tag that was created (for deletion on close)
	ApprovedAt string `json:"approved_at,omitempty"` // Timestamp when approved
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

	// Internal state (serialized to hidden comment)
	State IssueState
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
- **Version:** ` + "`{{.Version}}`" + `
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
{{- if .RunURL}}
- **Workflow Run:** [View Run]({{.RunURL}})
{{- end}}
- **Requested at:** {{.Timestamp}}

---

### Approval Requirements

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
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
		"title":    strings.Title,
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
