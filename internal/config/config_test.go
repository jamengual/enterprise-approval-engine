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
