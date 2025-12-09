// Package config handles parsing and validation of approvals.yml configuration files.
package config

import "time"

// Config represents the complete approvals.yml configuration.
type Config struct {
	Version   int                  `yaml:"version"`
	Defaults  Defaults             `yaml:"defaults,omitempty"`
	Policies  map[string]Policy    `yaml:"policies"`
	Workflows map[string]Workflow  `yaml:"workflows"`
	Semver    SemverConfig         `yaml:"semver,omitempty"`
}

// Defaults contains default values applied to all workflows.
type Defaults struct {
	Timeout           Duration `yaml:"timeout,omitempty"`
	AllowSelfApproval bool     `yaml:"allow_self_approval,omitempty"`
	IssueLabels       []string `yaml:"issue_labels,omitempty"`
}

// Policy defines a reusable group of approvers with a threshold.
type Policy struct {
	// Simple format: list of approvers with group-level threshold
	Approvers    []string `yaml:"approvers,omitempty"`
	MinApprovals int      `yaml:"min_approvals,omitempty"` // X of N required (0 = use require_all)
	RequireAll   bool     `yaml:"require_all,omitempty"`   // If true, ALL approvers must approve (AND logic)

	// Advanced format: per-source thresholds for fine-grained control
	From  []ApproverSource `yaml:"from,omitempty"`
	Logic string           `yaml:"logic,omitempty"` // "and" or "or" - how to combine sources (default: "and")
}

// ApproverSource defines an approver (user or team) with its own threshold.
// This allows "2 from team:platform AND 1 from team:security" in a single policy.
type ApproverSource struct {
	Team         string   `yaml:"team,omitempty"`          // Team slug (e.g., "platform" or "org/platform")
	User         string   `yaml:"user,omitempty"`          // Individual user
	Users        []string `yaml:"users,omitempty"`         // List of users
	MinApprovals int      `yaml:"min_approvals,omitempty"` // Required from this source (default: 1)
	RequireAll   bool     `yaml:"require_all,omitempty"`   // All from this source must approve
	Logic        string   `yaml:"logic,omitempty"`         // Logic to next source: "and" or "or" (default: uses policy logic)
}

// UsesAdvancedFormat returns true if the policy uses the "from" format.
func (p Policy) UsesAdvancedFormat() bool {
	return len(p.From) > 0
}

// GetLogic returns the logic type for combining sources ("and" or "or").
func (p Policy) GetLogic() string {
	if p.Logic == "" {
		return "and" // Default to AND logic
	}
	return p.Logic
}

// GetApprovers returns the list of approvers for the source.
func (s ApproverSource) GetApprovers() []string {
	if s.Team != "" {
		return []string{"team:" + s.Team}
	}
	if s.User != "" {
		return []string{s.User}
	}
	return s.Users
}

// GetMinApprovals returns the minimum approvals needed for this source.
func (s ApproverSource) GetMinApprovals() int {
	if s.RequireAll {
		return 0 // 0 signals "require all" mode
	}
	if s.MinApprovals > 0 {
		return s.MinApprovals
	}
	return 1 // Default to 1
}

// GetRequireAll returns whether all approvers from this source must approve.
func (s ApproverSource) GetRequireAll() bool {
	// If it's a single user, require_all is implicit
	if s.User != "" {
		return true
	}
	return s.RequireAll
}

// Workflow defines an approval workflow with triggers and requirements.
type Workflow struct {
	Description string            `yaml:"description,omitempty"`
	Trigger     map[string]string `yaml:"trigger,omitempty"`
	Require     []Requirement     `yaml:"require"`
	Issue       IssueConfig       `yaml:"issue,omitempty"`
	OnApproved  ActionConfig      `yaml:"on_approved,omitempty"`
	OnDenied    ActionConfig      `yaml:"on_denied,omitempty"`
	OnClosed    OnClosedConfig    `yaml:"on_closed,omitempty"` // Actions when issue is manually closed

	// Progressive deployment pipeline
	Pipeline *PipelineConfig `yaml:"pipeline,omitempty"` // Multi-stage deployment pipeline
}

// PipelineConfig defines a progressive deployment pipeline.
type PipelineConfig struct {
	Stages         []PipelineStage `yaml:"stages"`                     // Ordered list of deployment stages
	TrackPRs       bool            `yaml:"track_prs,omitempty"`        // Include PRs in release tracking
	TrackCommits   bool            `yaml:"track_commits,omitempty"`    // Include commits in release tracking
	CompareFromTag string          `yaml:"compare_from_tag,omitempty"` // Tag pattern to compare from (e.g., "v*")
}

// PipelineStage defines a single stage in a deployment pipeline.
type PipelineStage struct {
	Name        string   `yaml:"name"`                   // Stage name (e.g., "dev", "qa", "prod")
	Environment string   `yaml:"environment,omitempty"`  // GitHub environment name
	Policy      string   `yaml:"policy,omitempty"`       // Policy for this stage
	Approvers   []string `yaml:"approvers,omitempty"`    // Inline approvers (alternative to policy)
	OnApproved  string   `yaml:"on_approved,omitempty"`  // Comment to post when stage is approved
	CreateTag   bool     `yaml:"create_tag,omitempty"`   // Create tag at this stage
	IsFinal     bool     `yaml:"is_final,omitempty"`     // If true, close issue after this stage
}

// IsPipeline returns true if this workflow uses a progressive pipeline.
func (w *Workflow) IsPipeline() bool {
	return w.Pipeline != nil && len(w.Pipeline.Stages) > 0
}

// Requirement defines one approval path. Multiple requirements form OR logic.
// Within a requirement, use RequireAll for AND logic or MinApprovals for threshold.
type Requirement struct {
	Policy       string   `yaml:"policy,omitempty"`        // Reference to a defined policy
	Approvers    []string `yaml:"approvers,omitempty"`     // Inline approvers (alternative to policy)
	MinApprovals int      `yaml:"min_approvals,omitempty"` // X of N required (overrides policy)
	RequireAll   bool     `yaml:"require_all,omitempty"`   // ALL must approve (overrides policy)
}

// IssueConfig defines how approval issues are created.
type IssueConfig struct {
	Title               string   `yaml:"title,omitempty"`
	Body                string   `yaml:"body,omitempty"`     // Custom issue body template (Go template syntax)
	BodyFile            string   `yaml:"body_file,omitempty"` // Path to custom template file (relative to .github/)
	Labels              []string `yaml:"labels,omitempty"`
	AssigneesFromPolicy bool     `yaml:"assignees_from_policy,omitempty"`
}

// ActionConfig defines actions to take on approval/denial.
type ActionConfig struct {
	CreateTag  bool   `yaml:"create_tag,omitempty"`
	CloseIssue bool   `yaml:"close_issue,omitempty"`
	Comment    string `yaml:"comment,omitempty"`

	// Tagging configuration
	Tagging TaggingConfig `yaml:"tagging,omitempty"`
}

// OnClosedConfig defines actions when an approval issue is manually closed.
type OnClosedConfig struct {
	DeleteTag bool   `yaml:"delete_tag,omitempty"` // Delete the associated tag if issue is closed without approval
	Comment   string `yaml:"comment,omitempty"`    // Comment to post when issue is closed
}

// TaggingConfig defines how tags are created for a workflow.
type TaggingConfig struct {
	Enabled       bool   `yaml:"enabled,omitempty"`        // Enable tag creation (alternative to create_tag)
	StartVersion  string `yaml:"start_version,omitempty"`  // Initial version (e.g., "v1.0.0" or "1.0.0")
	Prefix        string `yaml:"prefix,omitempty"`         // Tag prefix (inferred from start_version if not set)
	AutoIncrement string `yaml:"auto_increment,omitempty"` // "major", "minor", "patch", or "" for manual
	EnvPrefix     string `yaml:"env_prefix,omitempty"`     // Environment prefix (e.g., "dev-" creates "dev-v1.0.0")
}

// IsEnabled returns true if tagging is enabled for this workflow.
func (t TaggingConfig) IsEnabled() bool {
	return t.Enabled || t.StartVersion != ""
}

// GetPrefix returns the version prefix (e.g., "v" or "").
// Inferred from StartVersion if not explicitly set.
func (t TaggingConfig) GetPrefix() string {
	if t.Prefix != "" {
		return t.Prefix
	}
	// Infer from start_version
	if t.StartVersion != "" {
		if len(t.StartVersion) > 0 && (t.StartVersion[0] == 'v' || t.StartVersion[0] == 'V') {
			return "v"
		}
		return ""
	}
	return "v" // Default to v prefix
}

// GetStartVersion returns the starting version without prefix.
func (t TaggingConfig) GetStartVersion() string {
	if t.StartVersion == "" {
		return "0.0.0"
	}
	v := t.StartVersion
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}

// GetAutoIncrement returns the auto-increment type (major, minor, patch, or "").
func (t TaggingConfig) GetAutoIncrement() string {
	switch t.AutoIncrement {
	case "major", "minor", "patch":
		return t.AutoIncrement
	default:
		return ""
	}
}

// FormatTag formats a version number as a complete tag.
func (t TaggingConfig) FormatTag(version string) string {
	// Remove any existing prefix from version
	v := version
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	return t.EnvPrefix + t.GetPrefix() + v
}

// SemverConfig defines semantic versioning behavior.
type SemverConfig struct {
	Prefix          string     `yaml:"prefix,omitempty"`
	Strategy        string     `yaml:"strategy,omitempty"`
	Auto            AutoConfig `yaml:"auto,omitempty"`
	Validate        bool       `yaml:"validate,omitempty"`
	AllowPrerelease bool       `yaml:"allow_prerelease,omitempty"`
}

// AutoConfig defines label-based auto-increment settings.
type AutoConfig struct {
	MajorLabels []string `yaml:"major_labels,omitempty"`
	MinorLabels []string `yaml:"minor_labels,omitempty"`
	PatchLabels []string `yaml:"patch_labels,omitempty"`
}

// Duration is a wrapper for time.Duration that supports YAML unmarshaling.
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements yaml.Unmarshaler for Duration.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		d.Duration = 0
		return nil
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

// Name returns a human-readable name for the requirement.
func (r Requirement) Name() string {
	if r.Policy != "" {
		return r.Policy
	}
	if len(r.Approvers) > 0 {
		return "custom"
	}
	return "unknown"
}

// IsTeam returns true if the approver is a team reference (team:org/name).
func IsTeam(approver string) bool {
	return len(approver) > 5 && approver[:5] == "team:"
}

// ParseTeam extracts the team slug from a team reference.
func ParseTeam(approver string) string {
	if !IsTeam(approver) {
		return ""
	}
	return approver[5:]
}
