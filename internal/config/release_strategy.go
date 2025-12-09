package config

// ReleaseStrategyType defines how release candidates are selected.
type ReleaseStrategyType string

const (
	// StrategyTag uses git tags to define release scope (current behavior).
	// All commits/PRs between previous tag and current tag are included.
	StrategyTag ReleaseStrategyType = "tag"

	// StrategyBranch uses release branches (e.g., release/v1.0.0).
	// PRs merged to the release branch are the release candidates.
	StrategyBranch ReleaseStrategyType = "branch"

	// StrategyLabel uses GitHub labels to select PRs for a release.
	// Only PRs with the matching label are included in the release.
	StrategyLabel ReleaseStrategyType = "label"

	// StrategyMilestone uses GitHub milestones to group PRs.
	// PRs assigned to the milestone are the release candidates.
	StrategyMilestone ReleaseStrategyType = "milestone"
)

// ReleaseStrategyConfig defines how release candidates are selected and tracked.
type ReleaseStrategyConfig struct {
	// Type is the strategy type: "tag", "branch", "label", or "milestone"
	Type ReleaseStrategyType `yaml:"type"`

	// Branch strategy settings
	Branch BranchStrategyConfig `yaml:"branch,omitempty"`

	// Label strategy settings
	Label LabelStrategyConfig `yaml:"label,omitempty"`

	// Milestone strategy settings
	Milestone MilestoneStrategyConfig `yaml:"milestone,omitempty"`

	// AutoCreate automatically creates the next release artifact when
	// the final stage (prod) is approved.
	AutoCreate AutoCreateConfig `yaml:"auto_create,omitempty"`
}

// BranchStrategyConfig defines settings for branch-based release selection.
type BranchStrategyConfig struct {
	// Pattern is the branch naming pattern (e.g., "release/{{version}}")
	// Supports {{version}} placeholder which is replaced with the version number.
	Pattern string `yaml:"pattern,omitempty"`

	// BaseBranch is the branch to compare against (default: "main")
	BaseBranch string `yaml:"base_branch,omitempty"`

	// DeleteAfterRelease deletes the release branch after successful prod deployment
	DeleteAfterRelease bool `yaml:"delete_after_release,omitempty"`
}

// LabelStrategyConfig defines settings for label-based release selection.
type LabelStrategyConfig struct {
	// Pattern is the label naming pattern (e.g., "release:{{version}}")
	// Supports {{version}} placeholder.
	Pattern string `yaml:"pattern,omitempty"`

	// PendingLabel is applied to PRs that are merged but not yet in a release
	// (e.g., "pending-release"). Optional.
	PendingLabel string `yaml:"pending_label,omitempty"`

	// RemoveAfterRelease removes the release label after successful prod deployment
	RemoveAfterRelease bool `yaml:"remove_after_release,omitempty"`
}

// MilestoneStrategyConfig defines settings for milestone-based release selection.
type MilestoneStrategyConfig struct {
	// Pattern is the milestone naming pattern (e.g., "v{{version}}" or "Release {{version}}")
	Pattern string `yaml:"pattern,omitempty"`

	// CloseAfterRelease closes the milestone after successful prod deployment
	CloseAfterRelease bool `yaml:"close_after_release,omitempty"`
}

// AutoCreateConfig defines settings for automatically creating the next release artifact.
type AutoCreateConfig struct {
	// Enabled activates auto-creation on final stage completion
	Enabled bool `yaml:"enabled,omitempty"`

	// NextVersion defines how to determine the next version
	// Options: "patch", "minor", "major", or "prompt" (ask user)
	NextVersion string `yaml:"next_version,omitempty"`

	// CreateIssue creates a new approval issue for the next release
	CreateIssue bool `yaml:"create_issue,omitempty"`

	// Comment to post when creating next release artifact
	Comment string `yaml:"comment,omitempty"`
}

// GetType returns the strategy type, defaulting to "tag" if not set.
func (r ReleaseStrategyConfig) GetType() ReleaseStrategyType {
	if r.Type == "" {
		return StrategyTag
	}
	return r.Type
}

// GetBranchPattern returns the branch pattern with default.
func (r ReleaseStrategyConfig) GetBranchPattern() string {
	if r.Branch.Pattern == "" {
		return "release/{{version}}"
	}
	return r.Branch.Pattern
}

// GetBaseBranch returns the base branch with default.
func (r ReleaseStrategyConfig) GetBaseBranch() string {
	if r.Branch.BaseBranch == "" {
		return "main"
	}
	return r.Branch.BaseBranch
}

// GetLabelPattern returns the label pattern with default.
func (r ReleaseStrategyConfig) GetLabelPattern() string {
	if r.Label.Pattern == "" {
		return "release:{{version}}"
	}
	return r.Label.Pattern
}

// GetMilestonePattern returns the milestone pattern with default.
func (r ReleaseStrategyConfig) GetMilestonePattern() string {
	if r.Milestone.Pattern == "" {
		return "v{{version}}"
	}
	return r.Milestone.Pattern
}

// FormatBranchName formats a branch name for a given version.
func (r ReleaseStrategyConfig) FormatBranchName(version string) string {
	return replaceVersion(r.GetBranchPattern(), version)
}

// FormatLabelName formats a label name for a given version.
func (r ReleaseStrategyConfig) FormatLabelName(version string) string {
	return replaceVersion(r.GetLabelPattern(), version)
}

// FormatMilestoneName formats a milestone name for a given version.
func (r ReleaseStrategyConfig) FormatMilestoneName(version string) string {
	return replaceVersion(r.GetMilestonePattern(), version)
}

// replaceVersion replaces {{version}} placeholder in a pattern.
func replaceVersion(pattern, version string) string {
	// Simple string replacement - no complex templating needed
	result := pattern
	for i := 0; i+len("{{version}}") <= len(result); i++ {
		if result[i:i+len("{{version}}")] == "{{version}}" {
			result = result[:i] + version + result[i+len("{{version}}"):]
		}
	}
	return result
}

// IsAutoCreateEnabled returns true if auto-creation is enabled.
func (r ReleaseStrategyConfig) IsAutoCreateEnabled() bool {
	return r.AutoCreate.Enabled
}

// GetNextVersionStrategy returns the next version strategy with default.
func (r ReleaseStrategyConfig) GetNextVersionStrategy() string {
	if r.AutoCreate.NextVersion == "" {
		return "patch"
	}
	return r.AutoCreate.NextVersion
}
