package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_MinimalConfig(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice, bob]
workflows:
  default:
    require:
      - policy: approvers
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Version)
	assert.Len(t, cfg.Policies, 1)
	assert.Len(t, cfg.Workflows, 1)
}

func TestParse_FullConfig(t *testing.T) {
	yaml := `
version: 1
defaults:
  timeout: 48h
  allow_self_approval: false
  issue_labels: [approval-required]
policies:
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2
  security-team:
    approvers: [team:security]
    require_all: true
workflows:
  production-deploy:
    description: "Production deployment"
    trigger:
      environment: production
    require:
      - policy: dev-team
      - policy: security-team
        min_approvals: 1
    issue:
      title: "Approval: {{version}}"
      labels: [production]
    on_approved:
      create_tag: true
      close_issue: true
semver:
  prefix: "v"
  strategy: input
  validate: true
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Defaults
	assert.Equal(t, 48*time.Hour, cfg.Defaults.Timeout.Duration)
	assert.False(t, cfg.Defaults.AllowSelfApproval)
	assert.Equal(t, []string{"approval-required"}, cfg.Defaults.IssueLabels)

	// Policies
	assert.Len(t, cfg.Policies, 2)
	assert.Equal(t, []string{"alice", "bob", "charlie"}, cfg.Policies["dev-team"].Approvers)
	assert.Equal(t, 2, cfg.Policies["dev-team"].MinApprovals)
	assert.True(t, cfg.Policies["security-team"].RequireAll)

	// Workflows
	workflow := cfg.Workflows["production-deploy"]
	assert.Equal(t, "Production deployment", workflow.Description)
	assert.Len(t, workflow.Require, 2)
	assert.True(t, workflow.OnApproved.CreateTag)
}

func TestParse_InvalidVersion(t *testing.T) {
	yaml := `
version: 2
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "unsupported config version")
}

func TestParse_MissingPolicies(t *testing.T) {
	yaml := `
version: 1
workflows:
  default:
    require:
      - approvers: [alice]
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "at least one policy must be defined")
}

func TestParse_MissingWorkflows(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "at least one workflow must be defined")
}

func TestParse_UndefinedPolicyReference(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: nonexistent
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "undefined policy")
}

func TestParse_EmptyApprovers(t *testing.T) {
	yaml := `
version: 1
policies:
  empty:
    approvers: []
workflows:
  default:
    require:
      - policy: empty
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "must have either 'approvers' or 'from' defined")
}

func TestParse_MinApprovalsExceedsCount(t *testing.T) {
	yaml := `
version: 1
policies:
  small:
    approvers: [alice]
    min_approvals: 5
workflows:
  default:
    require:
      - policy: small
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "exceeds approver count")
}

func TestParse_TeamApproverAllowsHighMinApprovals(t *testing.T) {
	// Teams can have many members, so min_approvals > len(approvers) is OK
	yaml := `
version: 1
policies:
  team-policy:
    approvers: [team:platform]
    min_approvals: 5
workflows:
  default:
    require:
      - policy: team-policy
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.Policies["team-policy"].MinApprovals)
}

func TestParse_InlineApprovers(t *testing.T) {
	yaml := `
version: 1
policies:
  placeholder:
    approvers: [unused]
workflows:
  default:
    require:
      - approvers: [alice, bob]
        min_approvals: 1
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, cfg.Workflows["default"].Require[0].Approvers)
}

func TestParse_RequirementNeedsPolicyOrApprovers(t *testing.T) {
	yaml := `
version: 1
policies:
  placeholder:
    approvers: [unused]
workflows:
  default:
    require:
      - min_approvals: 2
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "must specify policy or approvers")
}

func TestParse_RequirementCannotHaveBoth(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
        approvers: [bob]
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "cannot specify both policy and approvers")
}

func TestResolveRequirement_FromPolicy(t *testing.T) {
	yaml := `
version: 1
policies:
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2
workflows:
  default:
    require:
      - policy: dev-team
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	approvers, minApprovals, requireAll := cfg.ResolveRequirement(cfg.Workflows["default"].Require[0])
	assert.Equal(t, []string{"alice", "bob", "charlie"}, approvers)
	assert.Equal(t, 2, minApprovals)
	assert.False(t, requireAll)
}

func TestResolveRequirement_OverrideMinApprovals(t *testing.T) {
	yaml := `
version: 1
policies:
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2
workflows:
  default:
    require:
      - policy: dev-team
        min_approvals: 1
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	approvers, minApprovals, requireAll := cfg.ResolveRequirement(cfg.Workflows["default"].Require[0])
	assert.Equal(t, []string{"alice", "bob", "charlie"}, approvers)
	assert.Equal(t, 1, minApprovals) // Overridden
	assert.False(t, requireAll)
}

func TestResolveRequirement_RequireAll(t *testing.T) {
	yaml := `
version: 1
policies:
  strict:
    approvers: [alice, bob]
    require_all: true
workflows:
  default:
    require:
      - policy: strict
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	approvers, minApprovals, requireAll := cfg.ResolveRequirement(cfg.Workflows["default"].Require[0])
	assert.Equal(t, []string{"alice", "bob"}, approvers)
	assert.Equal(t, 0, minApprovals)
	assert.True(t, requireAll)
}

func TestResolveRequirement_DefaultsToRequireAll(t *testing.T) {
	yaml := `
version: 1
policies:
  no-threshold:
    approvers: [alice, bob]
workflows:
  default:
    require:
      - policy: no-threshold
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	_, minApprovals, requireAll := cfg.ResolveRequirement(cfg.Workflows["default"].Require[0])
	assert.Equal(t, 0, minApprovals)
	assert.True(t, requireAll) // Defaults to require all when neither is set
}

func TestIsTeam(t *testing.T) {
	assert.True(t, IsTeam("team:platform"))
	assert.True(t, IsTeam("team:org/security"))
	assert.False(t, IsTeam("alice"))
	assert.False(t, IsTeam("team")) // Too short
	assert.False(t, IsTeam(""))
}

func TestParseTeam(t *testing.T) {
	assert.Equal(t, "platform", ParseTeam("team:platform"))
	assert.Equal(t, "org/security", ParseTeam("team:org/security"))
	assert.Equal(t, "", ParseTeam("alice"))
	assert.Equal(t, "", ParseTeam(""))
}

func TestApplyDefaults(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Check defaults were applied
	assert.Equal(t, DefaultTimeout, cfg.Defaults.Timeout.Duration)
	assert.Equal(t, "v", cfg.Semver.Prefix)
	assert.Equal(t, "input", cfg.Semver.Strategy)
}

func TestTaggingConfig_GetPrefix(t *testing.T) {
	tests := []struct {
		name     string
		config   TaggingConfig
		expected string
	}{
		{"explicit prefix", TaggingConfig{Prefix: "release-"}, "release-"},
		{"infer v from start_version", TaggingConfig{StartVersion: "v1.0.0"}, "v"},
		{"infer no prefix", TaggingConfig{StartVersion: "1.0.0"}, ""},
		{"default to v", TaggingConfig{}, "v"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.GetPrefix())
		})
	}
}

func TestTaggingConfig_GetStartVersion(t *testing.T) {
	tests := []struct {
		name     string
		config   TaggingConfig
		expected string
	}{
		{"with v prefix", TaggingConfig{StartVersion: "v1.2.3"}, "1.2.3"},
		{"without prefix", TaggingConfig{StartVersion: "1.2.3"}, "1.2.3"},
		{"empty default", TaggingConfig{}, "0.0.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.GetStartVersion())
		})
	}
}

func TestTaggingConfig_FormatTag(t *testing.T) {
	tests := []struct {
		name     string
		config   TaggingConfig
		version  string
		expected string
	}{
		{
			"with v prefix",
			TaggingConfig{StartVersion: "v1.0.0"},
			"1.2.3",
			"v1.2.3",
		},
		{
			"without prefix",
			TaggingConfig{StartVersion: "1.0.0"},
			"1.2.3",
			"1.2.3",
		},
		{
			"with env prefix",
			TaggingConfig{StartVersion: "v1.0.0", EnvPrefix: "dev-"},
			"1.2.3",
			"dev-v1.2.3",
		},
		{
			"strip input prefix",
			TaggingConfig{StartVersion: "1.0.0"},
			"v1.2.3",
			"1.2.3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.FormatTag(tc.version))
		})
	}
}

func TestTaggingConfig_IsEnabled(t *testing.T) {
	assert.True(t, TaggingConfig{Enabled: true}.IsEnabled())
	assert.True(t, TaggingConfig{StartVersion: "v1.0.0"}.IsEnabled())
	assert.False(t, TaggingConfig{}.IsEnabled())
}

func TestTaggingConfig_GetAutoIncrement(t *testing.T) {
	tests := []struct {
		name     string
		config   TaggingConfig
		expected string
	}{
		{"major", TaggingConfig{AutoIncrement: "major"}, "major"},
		{"minor", TaggingConfig{AutoIncrement: "minor"}, "minor"},
		{"patch", TaggingConfig{AutoIncrement: "patch"}, "patch"},
		{"invalid", TaggingConfig{AutoIncrement: "invalid"}, ""},
		{"empty", TaggingConfig{}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.GetAutoIncrement())
		})
	}
}

func TestGetWorkflow(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
workflows:
  production:
    description: "Production workflow"
    require:
      - policy: approvers
  staging:
    require:
      - policy: approvers
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Existing workflow
	workflow, err := cfg.GetWorkflow("production")
	require.NoError(t, err)
	assert.Equal(t, "Production workflow", workflow.Description)

	// Another existing workflow
	workflow, err = cfg.GetWorkflow("staging")
	require.NoError(t, err)
	assert.NotNil(t, workflow)

	// Non-existent workflow
	_, err = cfg.GetWorkflow("nonexistent")
	assert.ErrorContains(t, err, `workflow "nonexistent" not found`)
}

func TestGetPolicy(t *testing.T) {
	yaml := `
version: 1
policies:
  dev-team:
    approvers: [alice, bob]
    min_approvals: 1
  security:
    approvers: [team:security]
    require_all: true
workflows:
  default:
    require:
      - policy: dev-team
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Existing policy
	policy, err := cfg.GetPolicy("dev-team")
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, policy.Approvers)

	// Another policy
	policy, err = cfg.GetPolicy("security")
	require.NoError(t, err)
	assert.True(t, policy.RequireAll)

	// Non-existent policy
	_, err = cfg.GetPolicy("nonexistent")
	assert.ErrorContains(t, err, `policy "nonexistent" not found`)
}

func TestPolicy_UsesAdvancedFormat(t *testing.T) {
	simple := Policy{Approvers: []string{"alice", "bob"}}
	assert.False(t, simple.UsesAdvancedFormat())

	advanced := Policy{
		From: []ApproverSource{
			{Team: "platform", MinApprovals: 2},
		},
	}
	assert.True(t, advanced.UsesAdvancedFormat())
}

func TestPolicy_GetLogic(t *testing.T) {
	// Default to "and"
	p := Policy{}
	assert.Equal(t, "and", p.GetLogic())

	// Explicit and
	p = Policy{Logic: "and"}
	assert.Equal(t, "and", p.GetLogic())

	// Explicit or
	p = Policy{Logic: "or"}
	assert.Equal(t, "or", p.GetLogic())
}

func TestApproverSource_GetApprovers(t *testing.T) {
	// Team source
	source := ApproverSource{Team: "platform"}
	assert.Equal(t, []string{"team:platform"}, source.GetApprovers())

	// Single user
	source = ApproverSource{User: "alice"}
	assert.Equal(t, []string{"alice"}, source.GetApprovers())

	// Multiple users
	source = ApproverSource{Users: []string{"alice", "bob"}}
	assert.Equal(t, []string{"alice", "bob"}, source.GetApprovers())

	// Empty source
	source = ApproverSource{}
	assert.Nil(t, source.GetApprovers())
}

func TestApproverSource_GetMinApprovals(t *testing.T) {
	// RequireAll mode
	source := ApproverSource{RequireAll: true}
	assert.Equal(t, 0, source.GetMinApprovals())

	// Explicit min
	source = ApproverSource{MinApprovals: 3}
	assert.Equal(t, 3, source.GetMinApprovals())

	// Default to 1
	source = ApproverSource{}
	assert.Equal(t, 1, source.GetMinApprovals())
}

func TestApproverSource_GetRequireAll(t *testing.T) {
	// Single user is implicit require all
	source := ApproverSource{User: "alice"}
	assert.True(t, source.GetRequireAll())

	// Explicit require all
	source = ApproverSource{Users: []string{"alice", "bob"}, RequireAll: true}
	assert.True(t, source.GetRequireAll())

	// Not require all
	source = ApproverSource{Users: []string{"alice", "bob"}}
	assert.False(t, source.GetRequireAll())
}

func TestRequirement_Name(t *testing.T) {
	// Policy reference
	req := Requirement{Policy: "dev-team"}
	assert.Equal(t, "dev-team", req.Name())

	// Inline approvers
	req = Requirement{Approvers: []string{"alice", "bob"}}
	assert.Equal(t, "custom", req.Name())

	// Unknown
	req = Requirement{}
	assert.Equal(t, "unknown", req.Name())
}

func TestWorkflow_IsPipeline(t *testing.T) {
	// No pipeline
	w := Workflow{}
	assert.False(t, w.IsPipeline())

	// Empty pipeline
	w = Workflow{Pipeline: &PipelineConfig{}}
	assert.False(t, w.IsPipeline())

	// With stages
	w = Workflow{
		Pipeline: &PipelineConfig{
			Stages: []PipelineStage{
				{Name: "dev"},
				{Name: "prod"},
			},
		},
	}
	assert.True(t, w.IsPipeline())
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	yaml := `
version: 1
defaults:
  timeout: 24h
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cfg.Defaults.Timeout.Duration)

	// Test empty duration
	yaml = `
version: 1
defaults:
  timeout: ""
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
`
	cfg, err = Parse([]byte(yaml))
	require.NoError(t, err)
	// Empty uses default
	assert.Equal(t, DefaultTimeout, cfg.Defaults.Timeout.Duration)
}

func TestDuration_UnmarshalYAML_Invalid(t *testing.T) {
	yaml := `
version: 1
defaults:
  timeout: invalid
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
`
	_, err := Parse([]byte(yaml))
	assert.Error(t, err)
}

func TestParse_AdvancedPolicy(t *testing.T) {
	yaml := `
version: 1
policies:
  complex-gate:
    from:
      - team: security
        min_approvals: 2
        logic: and
      - team: platform
        min_approvals: 2
        logic: or
      - user: alice
    logic: or
workflows:
  default:
    require:
      - policy: complex-gate
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	policy := cfg.Policies["complex-gate"]
	assert.True(t, policy.UsesAdvancedFormat())
	assert.Len(t, policy.From, 3)
	assert.Equal(t, "or", policy.Logic)

	// Check source details
	assert.Equal(t, "security", policy.From[0].Team)
	assert.Equal(t, 2, policy.From[0].MinApprovals)
	assert.Equal(t, "and", policy.From[0].Logic)
}

func TestParse_InvalidYAML(t *testing.T) {
	yaml := `
version: 1
policies: [invalid array instead of map]
`
	_, err := Parse([]byte(yaml))
	assert.Error(t, err)
}

func TestValidateApproverSource_InvalidFormat(t *testing.T) {
	// This tests the validateApproverSource indirectly through Parse
	yaml := `
version: 1
policies:
  invalid:
    from:
      - team: ""
        min_approvals: 2
workflows:
  default:
    require:
      - policy: invalid
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "must specify 'team', 'user', or 'users'")
}

func TestValidateApproverSource_NegativeMinApprovals(t *testing.T) {
	yaml := `
version: 1
policies:
  negative:
    from:
      - users: [alice, bob]
        min_approvals: -1
workflows:
  default:
    require:
      - policy: negative
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "min_approvals cannot be negative")
}

func TestValidatePolicy_BothFormats(t *testing.T) {
	yaml := `
version: 1
policies:
  both:
    approvers: [alice]
    from:
      - user: bob
workflows:
  default:
    require:
      - policy: both
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "cannot use both 'approvers' and 'from'")
}

func TestResolveRequirement_InlineApprovers(t *testing.T) {
	yaml := `
version: 1
policies:
  placeholder:
    approvers: [unused]
workflows:
  default:
    require:
      - approvers: [alice, bob, charlie]
        min_approvals: 2
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	approvers, minApprovals, requireAll := cfg.ResolveRequirement(cfg.Workflows["default"].Require[0])
	assert.Equal(t, []string{"alice", "bob", "charlie"}, approvers)
	assert.Equal(t, 2, minApprovals)
	assert.False(t, requireAll)
}

func TestResolveRequirement_InlineRequireAll(t *testing.T) {
	yaml := `
version: 1
policies:
  placeholder:
    approvers: [unused]
workflows:
  default:
    require:
      - approvers: [alice, bob]
        require_all: true
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	_, minApprovals, requireAll := cfg.ResolveRequirement(cfg.Workflows["default"].Require[0])
	assert.Equal(t, 0, minApprovals)
	assert.True(t, requireAll)
}

func TestResolveRequirement_UnknownPolicy(t *testing.T) {
	cfg := &Config{
		Policies: map[string]Policy{},
	}

	approvers, minApprovals, requireAll := cfg.ResolveRequirement(Requirement{Policy: "nonexistent"})
	assert.Nil(t, approvers)
	assert.Equal(t, 0, minApprovals)
	assert.True(t, requireAll)
}

func TestParse_WorkflowWithPipeline(t *testing.T) {
	yaml := `
version: 1
policies:
  dev-team:
    approvers: [alice, bob]
workflows:
  progressive:
    require:
      - policy: dev-team
    pipeline:
      stages:
        - name: dev
          environment: development
          auto_approve: true
          on_approved: "Dev deployed"
        - name: staging
          environment: staging
          policy: dev-team
        - name: production
          environment: production
          policy: dev-team
          create_tag: true
          is_final: true
      track_prs: true
      track_commits: true
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	workflow := cfg.Workflows["progressive"]
	assert.True(t, workflow.IsPipeline())
	assert.Len(t, workflow.Pipeline.Stages, 3)

	// Check stage details
	assert.Equal(t, "dev", workflow.Pipeline.Stages[0].Name)
	assert.True(t, workflow.Pipeline.Stages[0].AutoApprove)
	assert.Equal(t, "Dev deployed", workflow.Pipeline.Stages[0].OnApproved)

	assert.Equal(t, "production", workflow.Pipeline.Stages[2].Name)
	assert.True(t, workflow.Pipeline.Stages[2].CreateTag)
	assert.True(t, workflow.Pipeline.Stages[2].IsFinal)

	assert.True(t, workflow.Pipeline.TrackPRs)
	assert.True(t, workflow.Pipeline.TrackCommits)
}

func TestParse_OnClosedConfig(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
workflows:
  default:
    require:
      - policy: approvers
    on_closed:
      delete_tag: true
      comment: "Issue closed without approval"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	workflow := cfg.Workflows["default"]
	assert.True(t, workflow.OnClosed.DeleteTag)
	assert.Equal(t, "Issue closed without approval", workflow.OnClosed.Comment)
}

func TestParse_EmptyWorkflow(t *testing.T) {
	yaml := `
version: 1
policies:
  approvers:
    approvers: [alice]
workflows:
  empty:
    require: []
`
	_, err := Parse([]byte(yaml))
	assert.ErrorContains(t, err, "must have at least one requirement")
}

func TestLoadWithFallback_MockFetch(t *testing.T) {
	// Create a mock fetch function
	fetchCount := 0
	mockFetch := func(repo, path string) ([]byte, error) {
		fetchCount++
		if path == "myrepo_approvals.yml" {
			return []byte(`
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  deploy:
    require:
      - policy: team
`), nil
		}
		return nil, assert.AnError
	}

	cfg, source, err := LoadWithFallback("org/.github", "myrepo", ".github/approvals.yml", mockFetch)
	require.NoError(t, err)
	assert.Equal(t, "org/.github/myrepo_approvals.yml", source)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, fetchCount)
}

func TestLoadWithFallback_FallbackToShared(t *testing.T) {
	fetchCount := 0
	mockFetch := func(repo, path string) ([]byte, error) {
		fetchCount++
		if path == "approvals.yml" {
			return []byte(`
version: 1
policies:
  team:
    approvers: [bob]
workflows:
  deploy:
    require:
      - policy: team
`), nil
		}
		return nil, assert.AnError
	}

	cfg, source, err := LoadWithFallback("org/.github", "myrepo", ".github/approvals.yml", mockFetch)
	require.NoError(t, err)
	assert.Equal(t, "org/.github/approvals.yml", source)
	assert.NotNil(t, cfg)
	assert.Equal(t, 2, fetchCount) // Tried repo-specific first, then shared
}

// ============================================================================
// Release Strategy Tests
// ============================================================================

func TestReleaseStrategyConfig_GetType(t *testing.T) {
	tests := []struct {
		name     string
		config   ReleaseStrategyConfig
		expected ReleaseStrategyType
	}{
		{"default to tag", ReleaseStrategyConfig{}, StrategyTag},
		{"explicit tag", ReleaseStrategyConfig{Type: StrategyTag}, StrategyTag},
		{"branch", ReleaseStrategyConfig{Type: StrategyBranch}, StrategyBranch},
		{"label", ReleaseStrategyConfig{Type: StrategyLabel}, StrategyLabel},
		{"milestone", ReleaseStrategyConfig{Type: StrategyMilestone}, StrategyMilestone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.config.GetType())
		})
	}
}

func TestReleaseStrategyConfig_GetBranchPattern(t *testing.T) {
	// Default
	cfg := ReleaseStrategyConfig{}
	assert.Equal(t, "release/{{version}}", cfg.GetBranchPattern())

	// Custom
	cfg = ReleaseStrategyConfig{
		Branch: BranchStrategyConfig{Pattern: "releases/{{version}}"},
	}
	assert.Equal(t, "releases/{{version}}", cfg.GetBranchPattern())
}

func TestReleaseStrategyConfig_GetBaseBranch(t *testing.T) {
	// Default
	cfg := ReleaseStrategyConfig{}
	assert.Equal(t, "main", cfg.GetBaseBranch())

	// Custom
	cfg = ReleaseStrategyConfig{
		Branch: BranchStrategyConfig{BaseBranch: "develop"},
	}
	assert.Equal(t, "develop", cfg.GetBaseBranch())
}

func TestReleaseStrategyConfig_GetLabelPattern(t *testing.T) {
	// Default
	cfg := ReleaseStrategyConfig{}
	assert.Equal(t, "release:{{version}}", cfg.GetLabelPattern())

	// Custom
	cfg = ReleaseStrategyConfig{
		Label: LabelStrategyConfig{Pattern: "v{{version}}-release"},
	}
	assert.Equal(t, "v{{version}}-release", cfg.GetLabelPattern())
}

func TestReleaseStrategyConfig_GetMilestonePattern(t *testing.T) {
	// Default
	cfg := ReleaseStrategyConfig{}
	assert.Equal(t, "v{{version}}", cfg.GetMilestonePattern())

	// Custom
	cfg = ReleaseStrategyConfig{
		Milestone: MilestoneStrategyConfig{Pattern: "Release {{version}}"},
	}
	assert.Equal(t, "Release {{version}}", cfg.GetMilestonePattern())
}

func TestReleaseStrategyConfig_FormatBranchName(t *testing.T) {
	cfg := ReleaseStrategyConfig{
		Branch: BranchStrategyConfig{Pattern: "release/{{version}}"},
	}
	assert.Equal(t, "release/v1.2.3", cfg.FormatBranchName("v1.2.3"))

	// Default pattern
	cfg = ReleaseStrategyConfig{}
	assert.Equal(t, "release/1.0.0", cfg.FormatBranchName("1.0.0"))
}

func TestReleaseStrategyConfig_FormatLabelName(t *testing.T) {
	cfg := ReleaseStrategyConfig{
		Label: LabelStrategyConfig{Pattern: "release:{{version}}"},
	}
	assert.Equal(t, "release:v1.2.3", cfg.FormatLabelName("v1.2.3"))

	// Custom pattern
	cfg = ReleaseStrategyConfig{
		Label: LabelStrategyConfig{Pattern: "deploy-{{version}}-ready"},
	}
	assert.Equal(t, "deploy-v2.0.0-ready", cfg.FormatLabelName("v2.0.0"))
}

func TestReleaseStrategyConfig_FormatMilestoneName(t *testing.T) {
	cfg := ReleaseStrategyConfig{
		Milestone: MilestoneStrategyConfig{Pattern: "v{{version}}"},
	}
	assert.Equal(t, "v1.2.3", cfg.FormatMilestoneName("1.2.3"))

	// Custom pattern
	cfg = ReleaseStrategyConfig{
		Milestone: MilestoneStrategyConfig{Pattern: "Release {{version}}"},
	}
	assert.Equal(t, "Release 2.0.0", cfg.FormatMilestoneName("2.0.0"))
}

func TestReleaseStrategyConfig_IsAutoCreateEnabled(t *testing.T) {
	// Disabled by default
	cfg := ReleaseStrategyConfig{}
	assert.False(t, cfg.IsAutoCreateEnabled())

	// Enabled
	cfg = ReleaseStrategyConfig{
		AutoCreate: AutoCreateConfig{Enabled: true},
	}
	assert.True(t, cfg.IsAutoCreateEnabled())
}

func TestReleaseStrategyConfig_GetNextVersionStrategy(t *testing.T) {
	// Default
	cfg := ReleaseStrategyConfig{}
	assert.Equal(t, "patch", cfg.GetNextVersionStrategy())

	// Custom
	cfg = ReleaseStrategyConfig{
		AutoCreate: AutoCreateConfig{NextVersion: "minor"},
	}
	assert.Equal(t, "minor", cfg.GetNextVersionStrategy())

	cfg = ReleaseStrategyConfig{
		AutoCreate: AutoCreateConfig{NextVersion: "major"},
	}
	assert.Equal(t, "major", cfg.GetNextVersionStrategy())
}

func TestParse_ReleaseStrategy(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  progressive:
    require:
      - policy: team
    pipeline:
      stages:
        - name: dev
          auto_approve: true
        - name: prod
          policy: team
          is_final: true
      release_strategy:
        type: branch
        branch:
          pattern: "releases/{{version}}"
          base_branch: develop
          delete_after_release: true
        auto_create:
          enabled: true
          next_version: minor
          create_issue: true
          comment: "Next release ready!"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	workflow := cfg.Workflows["progressive"]
	strategy := workflow.Pipeline.ReleaseStrategy

	assert.Equal(t, StrategyBranch, strategy.GetType())
	assert.Equal(t, "releases/{{version}}", strategy.GetBranchPattern())
	assert.Equal(t, "develop", strategy.GetBaseBranch())
	assert.True(t, strategy.Branch.DeleteAfterRelease)
	assert.True(t, strategy.IsAutoCreateEnabled())
	assert.Equal(t, "minor", strategy.GetNextVersionStrategy())
	assert.True(t, strategy.AutoCreate.CreateIssue)
	assert.Equal(t, "Next release ready!", strategy.AutoCreate.Comment)
}

func TestParse_LabelStrategy(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  label-based:
    require:
      - policy: team
    pipeline:
      stages:
        - name: prod
          is_final: true
      release_strategy:
        type: label
        label:
          pattern: "release:{{version}}"
          pending_label: "pending-release"
          remove_after_release: true
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	strategy := cfg.Workflows["label-based"].Pipeline.ReleaseStrategy
	assert.Equal(t, StrategyLabel, strategy.GetType())
	assert.Equal(t, "release:{{version}}", strategy.GetLabelPattern())
	assert.Equal(t, "pending-release", strategy.Label.PendingLabel)
	assert.True(t, strategy.Label.RemoveAfterRelease)
}

func TestParse_MilestoneStrategy(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  milestone-based:
    require:
      - policy: team
    pipeline:
      stages:
        - name: prod
          is_final: true
      release_strategy:
        type: milestone
        milestone:
          pattern: "Release {{version}}"
          close_after_release: true
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	strategy := cfg.Workflows["milestone-based"].Pipeline.ReleaseStrategy
	assert.Equal(t, StrategyMilestone, strategy.GetType())
	assert.Equal(t, "Release {{version}}", strategy.GetMilestonePattern())
	assert.True(t, strategy.Milestone.CloseAfterRelease)
}

func TestReplaceVersion_MultipleOccurrences(t *testing.T) {
	cfg := ReleaseStrategyConfig{
		Branch: BranchStrategyConfig{Pattern: "{{version}}/release/{{version}}"},
	}
	assert.Equal(t, "v1.0.0/release/v1.0.0", cfg.FormatBranchName("v1.0.0"))
}

func TestReplaceVersion_NoPlaceholder(t *testing.T) {
	cfg := ReleaseStrategyConfig{
		Branch: BranchStrategyConfig{Pattern: "release-branch"},
	}
	assert.Equal(t, "release-branch", cfg.FormatBranchName("v1.0.0"))
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestParse_MultipleRequirementsORLogic(t *testing.T) {
	yaml := `
version: 1
policies:
  team-a:
    approvers: [alice, bob]
    min_approvals: 2
  team-b:
    approvers: [charlie, dave]
    require_all: true
  emergency:
    approvers: [admin]
workflows:
  production:
    require:
      - policy: team-a
      - policy: team-b
      - policy: emergency
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	workflow := cfg.Workflows["production"]
	assert.Len(t, workflow.Require, 3)

	// First requirement - team-a
	approvers, min, all := cfg.ResolveRequirement(workflow.Require[0])
	assert.Equal(t, []string{"alice", "bob"}, approvers)
	assert.Equal(t, 2, min)
	assert.False(t, all)

	// Second requirement - team-b
	approvers, min, all = cfg.ResolveRequirement(workflow.Require[1])
	assert.Equal(t, []string{"charlie", "dave"}, approvers)
	assert.Equal(t, 0, min)
	assert.True(t, all)

	// Third requirement - emergency
	approvers, min, all = cfg.ResolveRequirement(workflow.Require[2])
	assert.Equal(t, []string{"admin"}, approvers)
	assert.Equal(t, 0, min)
	assert.True(t, all) // Defaults to require_all when not specified
}

func TestParse_TaggingConfigComplete(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  deploy:
    require:
      - policy: team
    on_approved:
      tagging:
        enabled: true
        start_version: "v0.1.0"
        prefix: "v"
        auto_increment: minor
        env_prefix: "staging-"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	tagging := cfg.Workflows["deploy"].OnApproved.Tagging
	assert.True(t, tagging.IsEnabled())
	assert.Equal(t, "0.1.0", tagging.GetStartVersion())
	assert.Equal(t, "v", tagging.GetPrefix())
	assert.Equal(t, "minor", tagging.GetAutoIncrement())
	assert.Equal(t, "staging-v1.0.0", tagging.FormatTag("1.0.0"))
}

func TestLoadWithFallback_AllFail(t *testing.T) {
	mockFetch := func(repo, path string) ([]byte, error) {
		return nil, assert.AnError
	}

	_, _, err := LoadWithFallback("org/.github", "myrepo", ".github/approvals.yml", mockFetch)
	assert.Error(t, err)
}

func TestLoadWithFallback_ParseError(t *testing.T) {
	mockFetch := func(repo, path string) ([]byte, error) {
		return []byte("invalid: yaml: ["), nil
	}

	_, _, err := LoadWithFallback("org/.github", "myrepo", ".github/approvals.yml", mockFetch)
	assert.Error(t, err)
}
