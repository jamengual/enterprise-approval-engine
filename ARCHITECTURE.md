# IssueOps Approvals Action - Architecture Design

## Overview

A GitHub Action for policy-based approval workflows with:
- **Per-group approval requirements** (X-of-N approvers per group)
- **OR logic between groups** (any group can satisfy approval)
- **AND logic within groups** (minimum approvers required)
- **Semver tag creation** upon approval
- **Issue-based workflow** for transparency and audit trail

## Core Concepts

### Approval Model

```
Approval Request
├── Group A (OR)         ← Any ONE group satisfying its requirement = approved
│   ├── User 1 (AND)     ← Within group: need X of N users/teams
│   ├── User 2
│   └── Team X
├── Group B (OR)
│   ├── Team Y
│   └── Team Z
└── Group C (OR)
    └── User 3
```

**Logic:**
- **Between groups**: OR (any group meeting its threshold approves the request)
- **Within groups**: AND with threshold (need X of N members to approve)

### Example Scenarios

1. **Simple**: Any 2 of [alice, bob, charlie] approve → approved
2. **Team-based**: Any 1 from `@org/platform-team` approve → approved
3. **Multi-group OR**: (2 from platform-team) OR (1 from security-team) → approved
4. **Escalation**: (2 from dev-team) OR (1 from managers) → approved

---

## Configuration Format

### Location

```
.github/
├── approvals.yml           # Main config file
└── ISSUE_TEMPLATE/
    └── approval-request.yml  # Optional: custom issue template
```

### Schema: `.github/approvals.yml`

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/owner/issueops-approvals/main/schema.json
version: 1

# Global defaults (optional)
defaults:
  timeout: 72h                    # Max wait time for approvals
  allow_self_approval: false      # Requestor cannot approve their own request
  issue_labels:                   # Labels added to approval issues
    - "approval-required"
    - "issueops"

# Approval policies - define reusable approval groups
policies:
  # Simple policy: any 2 of these users
  dev-review:
    approvers:
      - alice
      - bob
      - charlie
    min_approvals: 2

  # Team-based policy
  platform-team:
    approvers:
      - team:platform-engineers    # Prefix 'team:' for org teams
    min_approvals: 1

  # Mixed users and teams
  security-review:
    approvers:
      - team:security
      - security-lead              # Individual user
    min_approvals: 1

# Workflows define when and how approvals are requested
workflows:
  # Production deployment approval
  production-deploy:
    description: "Production deployment requires approval"

    # Trigger conditions (matched against workflow inputs)
    trigger:
      environment: production

    # Approval requirement: policies combined with OR logic
    # Any ONE of these policy groups being satisfied = approved
    require:
      # Option 1: 2 platform engineers approve
      - policy: platform-team
        min_approvals: 2          # Override policy default

      # Option 2: 1 security team member approves
      - policy: security-review

      # Option 3: Both alice AND bob approve (inline group)
      - approvers: [alice, bob]
        min_approvals: 2          # ALL must approve

    # Issue configuration
    issue:
      title: "Approval Required: Production Deploy - {{version}}"
      labels:
        - "production"
        - "deploy"
      assignees_from_policy: true   # Assign users from approval policies

    # Actions after approval
    on_approved:
      create_tag: true              # Create semver tag
      close_issue: true
      comment: "Approved! Tag {{version}} created."

    on_denied:
      close_issue: true
      comment: "Deployment denied by {{denier}}."

  # Staging deployment - simpler requirements
  staging-deploy:
    description: "Staging deployment"
    trigger:
      environment: staging
    require:
      - policy: dev-review
        min_approvals: 1
    on_approved:
      create_tag: true
      tag_prefix: "staging-"       # Creates: staging-v1.2.3

# Semver configuration
semver:
  # Tag format
  prefix: "v"                       # v1.2.3

  # How to determine next version
  strategy: input                   # 'input' = from workflow input
                                    # 'auto' = auto-increment based on labels
                                    # 'conventional' = from conventional commits

  # For 'auto' strategy
  auto:
    major_labels: ["breaking", "major"]
    minor_labels: ["feature", "enhancement"]
    patch_labels: ["fix", "bugfix", "patch"]

  # Validation
  validate: true                    # Ensure valid semver format
  allow_prerelease: true            # Allow v1.2.3-beta.1
```

### Minimal Configuration

For simple use cases:

```yaml
version: 1

policies:
  approvers:
    approvers: [alice, bob, charlie]
    min_approvals: 2

workflows:
  default:
    require:
      - policy: approvers
```

---

## Action Interface

### Inputs

```yaml
- uses: owner/issueops-approvals@v1
  with:
    # Required
    action: request|check|approve|deny    # What operation to perform

    # For 'request' action
    workflow: production-deploy           # Which workflow config to use
    version: "1.2.3"                      # Semver version (if creating tag)

    # For 'check' action
    issue_number: 123                     # Issue to check status

    # For 'approve'/'deny' actions (used by comment workflow)
    # These are typically triggered by issue_comment events

    # Authentication (required for team membership checks)
    token: ${{ secrets.GITHUB_TOKEN }}    # Basic operations
    app_id: ${{ vars.APP_ID }}            # For team membership (optional)
    app_private_key: ${{ secrets.KEY }}   # For team membership (optional)

    # Optional overrides
    config_path: .github/approvals.yml    # Custom config location
    timeout: 48h                          # Override timeout
```

### Outputs

```yaml
outputs:
  status: approved|pending|denied|timeout
  issue_number: "123"
  issue_url: "https://github.com/..."
  approvers: "alice,bob"                  # Who approved
  tag: "v1.2.3"                           # Created tag (if applicable)
  approval_groups_satisfied: "platform-team"
```

---

## Workflow Patterns

### Pattern 1: Request and Wait (Blocking)

```yaml
name: Deploy with Approval

on:
  workflow_dispatch:
    inputs:
      environment:
        type: choice
        options: [staging, production]
      version:
        type: string
        required: true

jobs:
  request-approval:
    runs-on: ubuntu-latest
    outputs:
      issue_number: ${{ steps.request.outputs.issue_number }}
    steps:
      - uses: owner/issueops-approvals@v1
        id: request
        with:
          action: request
          workflow: ${{ inputs.environment }}-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}

  wait-for-approval:
    needs: request-approval
    runs-on: ubuntu-latest
    steps:
      - uses: owner/issueops-approvals@v1
        id: approval
        with:
          action: check
          issue_number: ${{ needs.request-approval.outputs.issue_number }}
          token: ${{ secrets.GITHUB_TOKEN }}
          wait: true                      # Poll until resolved
          timeout: 24h

      - if: steps.approval.outputs.status != 'approved'
        run: exit 1

  deploy:
    needs: wait-for-approval
    runs-on: ubuntu-latest
    steps:
      - run: echo "Deploying..."
```

### Pattern 2: Event-Driven (Non-Blocking)

```yaml
# Workflow 1: Request approval (non-blocking)
name: Request Deployment Approval

on:
  workflow_dispatch:
    inputs:
      environment:
        type: choice
        options: [staging, production]
      version:
        type: string

jobs:
  request:
    runs-on: ubuntu-latest
    steps:
      - uses: owner/issueops-approvals@v1
        with:
          action: request
          workflow: ${{ inputs.environment }}-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}
      # Workflow exits - approval handled by comment workflow
```

```yaml
# Workflow 2: Handle approval comments
name: Process Approval Comments

on:
  issue_comment:
    types: [created]

jobs:
  process:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: owner/issueops-approvals@v1
        id: process
        with:
          action: process-comment
          token: ${{ secrets.GITHUB_TOKEN }}
          app_id: ${{ vars.APP_ID }}
          app_private_key: ${{ secrets.APP_PRIVATE_KEY }}

      - if: steps.process.outputs.status == 'approved'
        uses: actions/github-script@v7
        with:
          script: |
            await github.rest.actions.createWorkflowDispatch({
              owner: context.repo.owner,
              repo: context.repo.repo,
              workflow_id: 'deploy.yml',
              ref: 'main',
              inputs: {
                version: '${{ steps.process.outputs.version }}'
              }
            })
```

---

## Implementation Architecture

### Components

```
issueops-approvals/
├── cmd/
│   └── action/
│       └── main.go              # Action entrypoint
├── internal/
│   ├── config/
│   │   ├── config.go            # Config parsing
│   │   ├── schema.go            # JSON Schema validation
│   │   └── types.go             # Config types
│   ├── approval/
│   │   ├── engine.go            # Approval logic engine
│   │   ├── policy.go            # Policy evaluation
│   │   └── status.go            # Status tracking
│   ├── github/
│   │   ├── client.go            # GitHub API client
│   │   ├── issues.go            # Issue operations
│   │   ├── teams.go             # Team membership
│   │   └── tags.go              # Tag creation
│   ├── semver/
│   │   ├── parse.go             # Semver parsing
│   │   ├── validate.go          # Validation
│   │   └── increment.go         # Auto-increment logic
│   └── action/
│       ├── request.go           # Request action
│       ├── check.go             # Check action
│       ├── process.go           # Process comment action
│       └── outputs.go           # Action outputs
├── schema.json                   # JSON Schema for config validation
├── action.yml                    # Action metadata
├── Dockerfile                    # Docker action
└── README.md
```

### Core Types

```go
// Config represents the approvals.yml configuration
type Config struct {
    Version   int                    `yaml:"version"`
    Defaults  Defaults               `yaml:"defaults"`
    Policies  map[string]Policy      `yaml:"policies"`
    Workflows map[string]Workflow    `yaml:"workflows"`
    Semver    SemverConfig           `yaml:"semver"`
}

// Policy defines a reusable approval group
type Policy struct {
    Approvers    []string `yaml:"approvers"`     // Users or "team:name"
    MinApprovals int      `yaml:"min_approvals"` // Required count
}

// Workflow defines an approval workflow
type Workflow struct {
    Description string           `yaml:"description"`
    Trigger     map[string]any   `yaml:"trigger"`
    Require     []Requirement    `yaml:"require"`   // OR between these
    Issue       IssueConfig      `yaml:"issue"`
    OnApproved  ActionConfig     `yaml:"on_approved"`
    OnDenied    ActionConfig     `yaml:"on_denied"`
}

// Requirement is one approval path (policies combined with OR)
type Requirement struct {
    Policy       string   `yaml:"policy"`        // Reference to policy
    Approvers    []string `yaml:"approvers"`     // Inline approvers
    MinApprovals int      `yaml:"min_approvals"` // Override or inline
}

// ApprovalStatus tracks the current state
type ApprovalStatus struct {
    State              string                      // pending|approved|denied
    GroupsStatus       map[string]GroupStatus      // Per-group status
    Approvals          []Approval                  // All approvals received
    Denials            []Denial                    // All denials received
    SatisfiedGroup     string                      // Which group was satisfied
}

type GroupStatus struct {
    Required  int        // Min approvals needed
    Current   int        // Approvals received
    Approvers []string   // Who approved
    Satisfied bool       // Met threshold?
}
```

### Approval Engine Logic

```go
// CheckApprovalStatus evaluates all requirements (OR logic)
func (e *Engine) CheckApprovalStatus(req *ApprovalRequest) *ApprovalStatus {
    status := &ApprovalStatus{
        State:        "pending",
        GroupsStatus: make(map[string]GroupStatus),
    }

    // Collect all approvals from issue comments
    approvals := e.collectApprovals(req.IssueNumber)
    denials := e.collectDenials(req.IssueNumber)

    // Check if ANY denial exists (configurable)
    if len(denials) > 0 && req.Workflow.FailOnDeny {
        status.State = "denied"
        status.Denials = denials
        return status
    }

    // Evaluate each requirement group (OR logic between groups)
    for _, requirement := range req.Workflow.Require {
        groupStatus := e.evaluateRequirement(requirement, approvals)
        status.GroupsStatus[requirement.Name()] = groupStatus

        // OR logic: if ANY group is satisfied, approval is complete
        if groupStatus.Satisfied {
            status.State = "approved"
            status.SatisfiedGroup = requirement.Name()
            break
        }
    }

    return status
}

// evaluateRequirement checks if a single group meets its threshold
func (e *Engine) evaluateRequirement(req Requirement, approvals []Approval) GroupStatus {
    // Get eligible approvers for this requirement
    eligible := e.getEligibleApprovers(req)

    // Count approvals from eligible users
    count := 0
    var approvers []string
    for _, approval := range approvals {
        if e.isEligible(approval.User, eligible) {
            count++
            approvers = append(approvers, approval.User)
        }
    }

    required := req.MinApprovals
    if required == 0 {
        required = 1 // Default to 1
    }

    return GroupStatus{
        Required:  required,
        Current:   count,
        Approvers: approvers,
        Satisfied: count >= required,
    }
}
```

---

## Issue Template

Generated approval issues use this format:

```markdown
## Approval Request: Production Deploy v1.2.3

**Requested by:** @requester
**Environment:** production
**Version:** v1.2.3
**Requested at:** 2024-01-15 10:30:00 UTC

---

### Approval Requirements

This request can be approved by **any one** of the following:

| Group | Required | Current | Status |
|-------|----------|---------|--------|
| Platform Team | 2 of 3 | 0 | ⏳ Pending |
| Security Review | 1 of 2 | 0 | ⏳ Pending |

### How to Approve

Comment with one of: `approve`, `approved`, `lgtm`, `/approve`

### How to Deny

Comment with one of: `deny`, `denied`, `/deny`

---

### Approval Log

<!-- issueops-approvals-state:{"version":"1.2.3","workflow":"production-deploy"} -->
```

---

## Security Considerations

1. **Token Permissions**
   - Basic GITHUB_TOKEN: Issues read/write, no team access
   - GitHub App: Add `members:read` for team membership checks

2. **Self-Approval Prevention**
   - Configurable per-workflow
   - Default: requestor cannot approve their own request

3. **Validation**
   - Verify commenter is in eligible approver list
   - Verify team membership via API (requires App token)
   - Validate semver format before creating tags

4. **Audit Trail**
   - All approvals/denials recorded as issue comments
   - Timestamps and user attribution preserved
   - Issue timeline provides complete history

---

## Comparison to Existing Actions

| Feature | trstringer/manual-approval | This Action |
|---------|---------------------------|-------------|
| Per-group thresholds | No (total only) | Yes |
| OR logic between groups | No | Yes |
| Config file based | No (inputs only) | Yes |
| Reusable policies | No | Yes |
| Semver tag creation | No | Yes |
| Custom issue templates | Limited | Yes |
| Team support | Yes (with App) | Yes (with App) |
| Event-driven pattern | Polling | Both |

---

## Implementation Phases

### Phase 1: Core MVP
- [ ] Config parsing and validation
- [ ] Basic approval engine (single group, min threshold)
- [ ] Issue creation with status table
- [ ] Comment processing (approve/deny)
- [ ] Semver validation and tag creation

### Phase 2: Multi-Group Support
- [ ] OR logic between requirement groups
- [ ] Per-group threshold tracking
- [ ] Status table updates
- [ ] Satisfied group detection

### Phase 3: Team Integration
- [ ] GitHub App token support
- [ ] Team membership resolution
- [ ] Mixed user/team approvers

### Phase 4: Advanced Features
- [ ] Auto-increment semver strategies
- [ ] Trigger condition matching
- [ ] Custom issue templates
- [ ] Timeout handling
- [ ] Reminder comments

### Phase 5: Polish
- [ ] JSON Schema for config validation
- [ ] Comprehensive documentation
- [ ] Example workflows
- [ ] Test coverage
