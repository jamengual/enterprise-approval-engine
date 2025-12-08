package approval

import (
	"testing"

	"github.com/issueops/approvals/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTeamResolver implements TeamResolver for testing.
type mockTeamResolver struct {
	teams map[string][]string
}

func (m *mockTeamResolver) GetTeamMembers(team string) ([]string, error) {
	if members, ok := m.teams[team]; ok {
		return members, nil
	}
	return []string{}, nil
}

func newMockTeamResolver() *mockTeamResolver {
	return &mockTeamResolver{
		teams: map[string][]string{
			"platform": {"alice", "bob", "charlie"},
			"security": {"dave", "eve"},
		},
	}
}

func parseConfig(t *testing.T, yaml string) *config.Config {
	cfg, err := config.Parse([]byte(yaml))
	require.NoError(t, err)
	return cfg
}

func TestEngine_SingleApprover_RequireAll(t *testing.T) {
	yaml := `
version: 1
policies:
  single:
    approvers: [alice]
    require_all: true
workflows:
  test:
    require:
      - policy: single
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// No comments - pending
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// Alice approves - approved
	req.Comments = []Comment{{User: "alice", Body: "approve"}}
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_MultipleApprovers_RequireAll(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, bob]
    require_all: true
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Only alice approves - still pending
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "lgtm"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)
	assert.Equal(t, 1, result.Groups[0].Current)
	assert.False(t, result.Groups[0].Satisfied)

	// Both approve - approved
	req.Comments = []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approved"},
	}
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.True(t, result.Groups[0].Satisfied)
}

func TestEngine_MinApprovals_Threshold(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, bob, charlie]
    min_approvals: 2
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// One approval - pending
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// Two approvals - approved
	req.Comments = []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "yes"},
	}
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_MultipleGroups_ORLogic(t *testing.T) {
	yaml := `
version: 1
policies:
  dev-team:
    approvers: [alice, bob]
    min_approvals: 2
  security:
    approvers: [dave]
    require_all: true
workflows:
  test:
    require:
      - policy: dev-team
      - policy: security
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Only alice approves - neither group satisfied
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// Dave approves (security group satisfied) - approved via OR logic
	req.Comments = []Comment{
		{User: "alice", Body: "approve"},
		{User: "dave", Body: "lgtm"},
	}
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.Equal(t, "security", result.SatisfiedGroup)
}

func TestEngine_Denial_ImmediatelyFails(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, bob]
    min_approvals: 1
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Bob denies
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "bob", Body: "deny"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusDenied, result.Status)
	assert.Equal(t, "bob", result.Denier)
}

func TestEngine_SelfApproval_Blocked(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, bob]
    min_approvals: 1
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil) // Self-approval disabled

	// Alice is the requestor and tries to approve
	req := &Request{
		Config:    cfg,
		Workflow:  workflow,
		Requestor: "alice",
		Comments:  []Comment{{User: "alice", Body: "approve"}},
	}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	// Alice's approval should not count (she's the requestor)
	assert.Equal(t, StatusPending, result.Status)

	// Bob approves - now it's approved
	req.Comments = append(req.Comments, Comment{User: "bob", Body: "approve"})
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_SelfApproval_Allowed(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(true, nil) // Self-approval enabled

	req := &Request{
		Config:    cfg,
		Workflow:  workflow,
		Requestor: "alice",
		Comments:  []Comment{{User: "alice", Body: "approve"}},
	}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_TeamExpansion(t *testing.T) {
	yaml := `
version: 1
policies:
  platform:
    approvers: [team:platform]
    min_approvals: 2
workflows:
  test:
    require:
      - policy: platform
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Alice and bob (team members) approve
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "lgtm"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_MixedTeamAndIndividual(t *testing.T) {
	yaml := `
version: 1
policies:
  mixed:
    approvers: [team:security, frank]
    min_approvals: 2
workflows:
  test:
    require:
      - policy: mixed
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Dave (from security team) and frank approve
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "dave", Body: "approve"},
		{User: "frank", Body: "yes"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_InlineApprovers(t *testing.T) {
	yaml := `
version: 1
policies:
  placeholder:
    approvers: [unused]
workflows:
  test:
    require:
      - approvers: [alice, bob]
        require_all: true
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_NonApproverCommentIgnored(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice]
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Random user comments "approve" - should be ignored
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "random", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// Alice approves - now approved
	req.Comments = append(req.Comments, Comment{User: "alice", Body: "approve"})
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_CaseInsensitiveUsername(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [Alice]
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// alice (lowercase) approves
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_DuplicateApprovalsCounted_Once(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, bob]
    min_approvals: 2
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Alice approves twice - should count as 1
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "alice", Body: "lgtm"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)
	assert.Equal(t, 1, result.Groups[0].Current)
}

func TestParser_ApprovalKeywords(t *testing.T) {
	parser := NewParser()

	approvals := []string{
		"approve",
		"Approve",
		"APPROVE",
		"approved",
		"lgtm",
		"LGTM",
		"yes",
		"/approve",
		"approve!",
		"approve.",
		"  approve  ",
	}

	for _, kw := range approvals {
		t.Run(kw, func(t *testing.T) {
			assert.True(t, parser.IsApproval(kw), "should be approval: %q", kw)
		})
	}

	nonApprovals := []string{
		"approve please",
		"I approve",
		"approving",
		"not approved",
		"hello",
	}

	for _, kw := range nonApprovals {
		t.Run(kw, func(t *testing.T) {
			assert.False(t, parser.IsApproval(kw), "should not be approval: %q", kw)
		})
	}
}

func TestParser_DenialKeywords(t *testing.T) {
	parser := NewParser()

	denials := []string{
		"deny",
		"denied",
		"reject",
		"rejected",
		"no",
		"/deny",
		"DENY",
	}

	for _, kw := range denials {
		t.Run(kw, func(t *testing.T) {
			assert.True(t, parser.IsDenial(kw), "should be denial: %q", kw)
		})
	}
}

func TestParser_DenialTakesPrecedence(t *testing.T) {
	parser := NewParser()
	// This shouldn't happen in practice, but denial takes precedence
	result := parser.Parse("deny")
	assert.True(t, result.IsDenial)
	assert.False(t, result.IsApproval)
}

// Tests for advanced "from" format

func TestEngine_AdvancedFormat_ANDLogic(t *testing.T) {
	yaml := `
version: 1
policies:
  production-gate:
    from:
      - team: platform
        min_approvals: 2
      - team: security
        min_approvals: 1
    logic: and
workflows:
  test:
    require:
      - policy: production-gate
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Only platform approvals - not enough (need security too)
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// Add security approval - now satisfied
	req.Comments = append(req.Comments, Comment{User: "dave", Body: "approve"})
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.Len(t, result.Groups[0].Sources, 2)
}

func TestEngine_AdvancedFormat_ORLogic(t *testing.T) {
	yaml := `
version: 1
policies:
  flexible-review:
    from:
      - team: security
        require_all: true
      - team: platform
        min_approvals: 2
    logic: or
workflows:
  test:
    require:
      - policy: flexible-review
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// 2 platform members approve - satisfied via OR logic
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.Equal(t, "or", result.Groups[0].Logic)
}

func TestEngine_AdvancedFormat_SingleUser(t *testing.T) {
	yaml := `
version: 1
policies:
  exec-approval:
    from:
      - user: ceo
      - user: cto
    logic: or
workflows:
  test:
    require:
      - policy: exec-approval
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// CEO approves - satisfied
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "ceo", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_AdvancedFormat_UsersList(t *testing.T) {
	yaml := `
version: 1
policies:
  leads:
    from:
      - users: [tech-lead, product-lead, design-lead]
        min_approvals: 2
workflows:
  test:
    require:
      - policy: leads
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Only 1 approval - pending
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "tech-lead", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// 2 approvals - satisfied
	req.Comments = append(req.Comments, Comment{User: "product-lead", Body: "approve"})
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_AdvancedFormat_MixedSourcesAND(t *testing.T) {
	// Complex case: 2 from platform AND 1 from security AND alice
	yaml := `
version: 1
policies:
  complex-gate:
    from:
      - team: platform
        min_approvals: 2
      - team: security
        min_approvals: 1
      - user: alice
    logic: and
workflows:
  test:
    require:
      - policy: complex-gate
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Platform (alice, bob) + Security (dave) but alice is also required as individual
	// alice counts for platform but also needs to approve as herself
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"}, // Counts for platform AND alice source
		{User: "bob", Body: "approve"},   // Counts for platform
		{User: "dave", Body: "approve"},  // Counts for security
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.Len(t, result.Groups[0].Sources, 3)
}

func TestEngine_AdvancedFormat_DefaultLogicIsAND(t *testing.T) {
	yaml := `
version: 1
policies:
  no-logic-specified:
    from:
      - team: platform
        min_approvals: 1
      - team: security
        min_approvals: 1
workflows:
  test:
    require:
      - policy: no-logic-specified
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Only platform - should not be satisfied (AND is default)
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)
}

// Tests for inline "then" logic

func TestEngine_AdvancedFormat_InlineThenLogic(t *testing.T) {
	// (2 from security AND 2 from platform) OR alice
	yaml := `
version: 1
policies:
  complex:
    from:
      - team: security
        min_approvals: 2
        logic: and
      - team: platform
        min_approvals: 2
        logic: or
      - user: alice
workflows:
  test:
    require:
      - policy: complex
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Only alice approves - satisfied via OR path
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_AdvancedFormat_InlineThenLogic_ANDPath(t *testing.T) {
	// (2 from security AND 2 from platform) OR alice
	yaml := `
version: 1
policies:
  complex:
    from:
      - team: security
        min_approvals: 2
        logic: and
      - team: platform
        min_approvals: 2
        logic: or
      - user: alice
workflows:
  test:
    require:
      - policy: complex
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// 2 security (dave, eve) AND 2 platform (alice, bob) - satisfied via AND path
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "dave", Body: "approve"},
		{User: "eve", Body: "approve"},
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_AdvancedFormat_InlineThenLogic_NotEnough(t *testing.T) {
	// (2 from security AND 2 from platform) OR alice
	yaml := `
version: 1
policies:
  complex:
    from:
      - team: security
        min_approvals: 2
        logic: and
      - team: platform
        min_approvals: 2
        logic: or
      - user: alice
workflows:
  test:
    require:
      - policy: complex
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Only 2 security, not enough platform (AND group incomplete)
	// And not alice (OR path not satisfied)
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "dave", Body: "approve"},
		{User: "eve", Body: "approve"},
		{User: "bob", Body: "approve"}, // Only 1 platform
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)
}

func TestEngine_AdvancedFormat_MultipleORGroups(t *testing.T) {
	// (security AND platform) OR (alice AND bob) OR manager
	yaml := `
version: 1
policies:
  multi-path:
    from:
      - team: security
        min_approvals: 1
        logic: and
      - team: platform
        min_approvals: 1
        logic: or
      - user: alice
        logic: and
      - user: bob
        logic: or
      - user: manager
workflows:
  test:
    require:
      - policy: multi-path
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, newMockTeamResolver())

	// Just manager approves - satisfied via third OR path
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "manager", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)

	// alice AND bob - satisfied via second OR path
	req2 := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approve"},
	}}
	result2, err := engine.Evaluate(req2)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result2.Status)
}
