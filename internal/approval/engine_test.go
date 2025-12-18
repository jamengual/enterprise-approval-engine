package approval

import (
	"testing"
	"time"

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

// Additional tests for comprehensive coverage

func TestEngine_WaitForApproval_ApprovedImmediately(t *testing.T) {
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

	callCount := 0
	getComments := func() ([]Comment, error) {
		callCount++
		return []Comment{{User: "alice", Body: "approve"}}, nil
	}

	req := &Request{Config: cfg, Workflow: workflow}
	result, err := engine.WaitForApproval(req, time.Second, 10*time.Millisecond, getComments)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.Equal(t, 1, callCount) // Should only poll once
}

func TestEngine_WaitForApproval_DeniedImmediately(t *testing.T) {
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

	getComments := func() ([]Comment, error) {
		return []Comment{{User: "alice", Body: "deny"}}, nil
	}

	req := &Request{Config: cfg, Workflow: workflow}
	result, err := engine.WaitForApproval(req, time.Second, 10*time.Millisecond, getComments)
	require.NoError(t, err)
	assert.Equal(t, StatusDenied, result.Status)
	assert.Equal(t, "alice", result.Denier)
}

func TestEngine_WaitForApproval_Timeout(t *testing.T) {
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

	getComments := func() ([]Comment, error) {
		return []Comment{}, nil // No approvals
	}

	req := &Request{Config: cfg, Workflow: workflow}
	result, err := engine.WaitForApproval(req, 50*time.Millisecond, 10*time.Millisecond, getComments)
	require.NoError(t, err)
	assert.Equal(t, StatusTimeout, result.Status)
}

func TestEngine_WaitForApproval_GetCommentsError(t *testing.T) {
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

	getComments := func() ([]Comment, error) {
		return nil, assert.AnError
	}

	req := &Request{Config: cfg, Workflow: workflow}
	result, err := engine.WaitForApproval(req, time.Second, 10*time.Millisecond, getComments)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEngine_WaitForApproval_ApprovalAfterPolling(t *testing.T) {
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

	callCount := 0
	getComments := func() ([]Comment, error) {
		callCount++
		if callCount >= 3 {
			return []Comment{{User: "alice", Body: "approve"}}, nil
		}
		return []Comment{}, nil
	}

	req := &Request{Config: cfg, Workflow: workflow}
	result, err := engine.WaitForApproval(req, time.Second, 10*time.Millisecond, getComments)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	assert.GreaterOrEqual(t, callCount, 3)
}

func TestParser_FormatApprovalKeywords(t *testing.T) {
	parser := NewParser()
	formatted := parser.FormatApprovalKeywords()
	assert.Contains(t, formatted, `"approve"`)
	assert.Contains(t, formatted, `"approved"`)
	assert.Contains(t, formatted, `"lgtm"`)
	assert.Contains(t, formatted, `"yes"`)
	assert.Contains(t, formatted, `"/approve"`)
}

func TestParser_FormatDenialKeywords(t *testing.T) {
	parser := NewParser()
	formatted := parser.FormatDenialKeywords()
	assert.Contains(t, formatted, `"deny"`)
	assert.Contains(t, formatted, `"denied"`)
	assert.Contains(t, formatted, `"reject"`)
	assert.Contains(t, formatted, `"rejected"`)
	assert.Contains(t, formatted, `"no"`)
	assert.Contains(t, formatted, `"/deny"`)
}

func TestParser_CustomKeywords(t *testing.T) {
	parser := NewParserWithKeywords([]string{"+1", "ship-it"}, []string{"hold", "-1"})

	// Test custom approval keywords
	assert.True(t, parser.IsApproval("+1"))
	assert.True(t, parser.IsApproval("ship-it"))
	// Default still work
	assert.True(t, parser.IsApproval("approve"))

	// Test custom denial keywords
	assert.True(t, parser.IsDenial("hold"))
	assert.True(t, parser.IsDenial("-1"))
	// Default still work
	assert.True(t, parser.IsDenial("deny"))
}

func TestEngine_ExpandApprovers_NilTeamResolver(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [team:unknown, alice]
    min_approvals: 1
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil) // No team resolver

	// Team reference won't be expanded, alice should still work
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

// errorTeamResolver returns an error
type errorTeamResolver struct{}

func (e *errorTeamResolver) GetTeamMembers(team string) ([]string, error) {
	return nil, assert.AnError
}

func TestEngine_ExpandApprovers_TeamResolverError(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [team:platform]
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, &errorTeamResolver{})

	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{}}
	_, err := engine.Evaluate(req)
	assert.Error(t, err)
}

func TestEngine_ExpandApprovers_DuplicateUsers(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, Alice, ALICE, bob]
    min_approvals: 2
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Alice (deduplicated) and bob approve - should satisfy min_approvals: 2
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "bob", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	// Approvers list should have deduplicated entries
	assert.Equal(t, 2, len(result.Groups[0].Approvers))
}

func TestEngine_IsEligibleApprover_NotEligible(t *testing.T) {
	yaml := `
version: 1
policies:
  team:
    approvers: [alice, bob]
workflows:
  test:
    require:
      - policy: team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Random user (not in approvers) tries to deny - should be ignored
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "random", Body: "deny"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status) // Not denied because random is not eligible
}

func TestEngine_IsEligibleApprover_MultipleGroups(t *testing.T) {
	yaml := `
version: 1
policies:
  dev:
    approvers: [alice]
  ops:
    approvers: [bob]
workflows:
  test:
    require:
      - policy: dev
      - policy: ops
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Bob (in ops) denies - should work
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "bob", Body: "deny"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusDenied, result.Status)
	assert.Equal(t, "bob", result.Denier)
}

func TestEngine_DeduplicateUsers(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int
	}{
		{
			name:     "no duplicates",
			input:    []string{"alice", "bob", "charlie"},
			expected: 3,
		},
		{
			name:     "case insensitive duplicates",
			input:    []string{"Alice", "alice", "ALICE"},
			expected: 1,
		},
		{
			name:     "mixed duplicates",
			input:    []string{"alice", "Bob", "alice", "bob", "charlie"},
			expected: 3,
		},
		{
			name:     "empty list",
			input:    []string{},
			expected: 0,
		},
		{
			name:     "nil list",
			input:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateUsers(tt.input)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestEngine_FilterUser(t *testing.T) {
	tests := []struct {
		name     string
		users    []string
		exclude  string
		expected int
	}{
		{
			name:     "exclude existing user",
			users:    []string{"alice", "bob", "charlie"},
			exclude:  "bob",
			expected: 2,
		},
		{
			name:     "case insensitive exclude",
			users:    []string{"Alice", "Bob"},
			exclude:  "alice",
			expected: 1,
		},
		{
			name:     "exclude non-existing user",
			users:    []string{"alice", "bob"},
			exclude:  "charlie",
			expected: 2,
		},
		{
			name:     "empty list",
			users:    []string{},
			exclude:  "alice",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterUser(tt.users, tt.exclude)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestEngine_EvaluateExpression_EmptySources(t *testing.T) {
	engine := NewEngine(false, nil)
	result := engine.evaluateExpression(nil, nil, "and")
	assert.False(t, result)
}

func TestEngine_EvaluateExpression_SingleSource(t *testing.T) {
	yaml := `
version: 1
policies:
  single:
    from:
      - user: alice
workflows:
  test:
    require:
      - policy: single
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Alice approves - single source satisfied
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_EvaluateSource_UsersSourceName(t *testing.T) {
	yaml := `
version: 1
policies:
  leads:
    from:
      - users: [alice, bob, charlie]
        min_approvals: 1
workflows:
  test:
    require:
      - policy: leads
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	// Source should be named "users"
	assert.Equal(t, "users", result.Groups[0].Sources[0].Name)
}

func TestEngine_AdvancedFormat_RequireAllInSource(t *testing.T) {
	yaml := `
version: 1
policies:
  strict:
    from:
      - users: [alice, bob]
        require_all: true
workflows:
  test:
    require:
      - policy: strict
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil)

	// Only alice - not satisfied
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)

	// Both approve - satisfied
	req.Comments = append(req.Comments, Comment{User: "bob", Body: "approve"})
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_AdvancedFormat_SelfApprovalBlocked(t *testing.T) {
	yaml := `
version: 1
policies:
  test:
    from:
      - users: [alice, bob]
        min_approvals: 1
workflows:
  test:
    require:
      - policy: test
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, nil) // Self-approval disabled

	// Alice is requestor and tries to approve
	req := &Request{
		Config:    cfg,
		Workflow:  workflow,
		Requestor: "alice",
		Comments:  []Comment{{User: "alice", Body: "approve"}},
	}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status) // Alice filtered out

	// Bob approves - satisfied
	req.Comments = append(req.Comments, Comment{User: "bob", Body: "approve"})
	result, err = engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
}

func TestEngine_AdvancedFormat_TeamResolverError(t *testing.T) {
	yaml := `
version: 1
policies:
  test:
    from:
      - team: platform
        min_approvals: 1
workflows:
  test:
    require:
      - policy: test
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, &errorTeamResolver{})

	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{}}
	_, err := engine.Evaluate(req)
	assert.Error(t, err)
}

func TestParser_Parse_ReturnsKeyword(t *testing.T) {
	parser := NewParser()

	// Approval keyword
	result := parser.Parse("approve")
	assert.True(t, result.IsApproval)
	assert.Equal(t, "approve", result.Keyword)

	// Denial keyword
	result = parser.Parse("deny")
	assert.True(t, result.IsDenial)
	assert.Equal(t, "deny", result.Keyword)

	// Non-matching
	result = parser.Parse("hello world")
	assert.False(t, result.IsApproval)
	assert.False(t, result.IsDenial)
	assert.Empty(t, result.Keyword)
}

func TestEngine_Denial_NonEligibleApproverIgnored(t *testing.T) {
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

	// Random user denies - should be ignored
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "random", Body: "deny"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)
	assert.Empty(t, result.Denials) // No denials recorded
}

func TestEngine_GroupStatus_ReportsCorrectly(t *testing.T) {
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

	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)

	assert.Equal(t, StatusPending, result.Status)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "team", result.Groups[0].Name)
	assert.Equal(t, 3, len(result.Groups[0].Approvers))
	assert.Equal(t, 2, result.Groups[0].MinRequired)
	assert.Equal(t, 1, result.Groups[0].Current)
	assert.False(t, result.Groups[0].Satisfied)
	assert.False(t, result.Groups[0].RequireAll)
}

func TestEngine_AllApprovalsRecorded(t *testing.T) {
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

	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve", CreatedAt: time.Now()},
		{User: "bob", Body: "lgtm", CreatedAt: time.Now().Add(time.Minute)},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)

	// Both approvals should be recorded
	assert.Len(t, result.Approvals, 2)
	assert.Equal(t, "alice", result.Approvals[0].User)
	assert.Equal(t, "bob", result.Approvals[1].User)
}

func TestEngine_EmptyComments(t *testing.T) {
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

	req := &Request{Config: cfg, Workflow: workflow, Comments: nil}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, result.Status)
	assert.Empty(t, result.Approvals)
}

func TestEngine_TeamMemberDuplicatedAcrossTeams(t *testing.T) {
	// Test when a user appears in multiple teams - should deduplicate
	resolver := &mockTeamResolver{
		teams: map[string][]string{
			"team-a": {"alice", "bob"},
			"team-b": {"bob", "charlie"}, // bob in both teams
		},
	}

	yaml := `
version: 1
policies:
  multi-team:
    approvers: [team:team-a, team:team-b]
    min_approvals: 2
workflows:
  test:
    require:
      - policy: multi-team
`
	cfg := parseConfig(t, yaml)
	workflow, _ := cfg.GetWorkflow("test")
	engine := NewEngine(false, resolver)

	// alice and charlie approve (bob counts once, so 3 unique users)
	req := &Request{Config: cfg, Workflow: workflow, Comments: []Comment{
		{User: "alice", Body: "approve"},
		{User: "charlie", Body: "approve"},
	}}
	result, err := engine.Evaluate(req)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, result.Status)
	// Approvers should be deduplicated (alice, bob, charlie = 3)
	assert.Equal(t, 3, len(result.Groups[0].Approvers))
}
