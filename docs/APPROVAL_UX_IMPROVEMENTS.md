# Approval UX Improvements: Implementation Plan

This document explores options for improving the approval user experience beyond comment-based approvals.

## Current State

Users approve deployments by commenting on issues with keywords like `approve`, `lgtm`, `yes`.

**Limitations:**
- Not immediately obvious how to approve (requires reading instructions)
- No visual "button" to click
- Easy to miss approval requests in notification noise

---

## Issue Close Protection

### The Problem

Anyone with **Triage** access or higher (Write, Maintain, Admin) can close any issue in a repository. This means unauthorized users could close approval issues, bypassing the approval workflow.

### GitHub's Native Capabilities

| Feature | Can Prevent Unauthorized Close? |
|---------|--------------------------------|
| Repository Permissions | ❌ No - Triage+ can close any issue |
| Branch Protection | ❌ No - Only protects branches, not issues |
| Issue Lock | ⚠️ Partial - Prevents ALL comments, too restrictive |
| CODEOWNERS | ❌ No - Only for code review, not issues |

**Conclusion:** GitHub has no native feature to restrict who can close specific issues.

### Solution: Guardian Workflow

We implement a "guardian" workflow that:
1. Triggers on `issues.closed` event
2. Checks if the closer is authorized (in the approval group)
3. If unauthorized: reopens the issue + posts warning comment
4. If authorized: allows the close and processes approval

```yaml
# .github/workflows/protect-approval-issues.yml
name: Protect Approval Issues

on:
  issues:
    types: [closed]

jobs:
  guard:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        with:
          action: validate-close
          issue_number: ${{ github.event.issue.number }}
          token: ${{ secrets.GITHUB_TOKEN }}
```

### How It Works

```
User closes issue #100
        │
        ▼
┌───────────────────────────────┐
│ Guardian Workflow Triggers    │
│ on: issues.closed             │
└───────────────────────────────┘
        │
        ▼
┌───────────────────────────────┐
│ Check: Is closer authorized?  │
│ - In approval group?          │
│ - Is the assigned approver?   │
│ - Has admin override?         │
└───────────────────────────────┘
        │
        ├─── YES ───▶ Process approval normally
        │
        └─── NO ────▶ ┌─────────────────────────────┐
                      │ 1. Reopen issue             │
                      │ 2. Post warning comment     │
                      │ 3. Log unauthorized attempt │
                      └─────────────────────────────┘
```

### Warning Comment Template

```markdown
⚠️ **Unauthorized Close Attempt**

@{{closer}} attempted to close this issue but is not an authorized approver.

**Authorized approvers for this stage:**
{{#each approvers}}
- @{{this}}
{{/each}}

This issue has been automatically reopened.

---
*If you believe this is an error, please contact a repository administrator.*
```

### Configuration

```yaml
# .github/approvals.yml
defaults:
  protect_issues: true              # Enable guardian workflow
  allow_admin_override: true        # Admins can always close
  unauthorized_close_action: reopen # or "warn_only"

workflows:
  deploy:
    protection:
      strict: true                  # Even assignees must be in approval group
      audit_log: true               # Log all close attempts
```

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| Admin closes issue | Allowed (if `allow_admin_override: true`) |
| Bot closes issue | Allowed (bot is trusted) |
| PR auto-close | Prevented (issue not linked to approval) |
| Assignee not in group closes | Configurable (`strict: true` = reopen) |
| Workflow failure | Issue stays closed, alert sent |

### Sub-Issue Protection

For sub-issues, protection is even more important:

```yaml
sub_issue_settings:
  protection:
    only_assignee_can_close: true   # Only the assigned approver
    require_approval_comment: false # Must comment "approve" before close
    prevent_parent_close: true      # Parent can't be closed until all sub-issues done
```

### Implementation Notes

1. **Race Condition**: The guardian must act fast before other workflows process the close
2. **Recursion Prevention**: Reopening triggers `issues.reopened`, not `issues.closed`
3. **Rate Limits**: Consider batching for repos with many approval issues
4. **Audit Trail**: All unauthorized attempts should be logged

---

## Option 1: Sub-Issues for Approvals (Recommended)

### Overview

Create child issues for each approval stage/group, assigned to the relevant approvers. Approvers simply close their assigned issue to approve.

```
┌─────────────────────────────────────────────────────────────┐
│ Parent Issue #100: "🚀 Deploy v1.2.0 to Production"         │
│ Labels: [deployment, pipeline, v1.2.0]                      │
│                                                             │
│ ## Pipeline Flow                                            │
│ [Mermaid diagram]                                           │
│                                                             │
│ ## Approval Progress                                        │
│ Sub-Issues:                                                 │
│ ├─ #101 ✅ "Approve DEV" (closed by @dev-lead)              │
│ ├─ #102 ✅ "Approve QA" (closed by @qa-lead)                │
│ ├─ #103 ⏳ "Approve STAGE" → assigned to @tech-leads        │
│ └─ #104 ⬜ "Approve PROD" → assigned to @sre-team           │
└─────────────────────────────────────────────────────────────┘
```

### How It Works

1. **Request Action**: Creates parent issue + sub-issues for each stage
2. **Approve**: Approver closes their assigned sub-issue
3. **Pipeline Advances**: `issues.closed` event triggers next stage
4. **Deny**: Approver closes with "won't fix" or comments "deny"
5. **Complete**: All sub-issues closed → parent issue closed

### Advantages

| Aspect | Benefit |
|--------|---------|
| **UX** | Clear ownership - "close this issue to approve" |
| **Notifications** | Approvers get direct assignment notifications |
| **Visibility** | Sub-issues show in assignee's issue list |
| **Audit Trail** | Each approval is a separate issue with history |
| **Progress** | Parent issue shows sub-issue completion progress |
| **Native UI** | Uses GitHub's built-in sub-issues visualization |

### Disadvantages

- Creates more issues in the repository
- Slightly more complex state management
- Requires GitHub's sub-issues feature (relatively new)
- More API calls to create/link issues

### Configuration

```yaml
# .github/approvals.yml
workflows:
  deploy:
    description: "Production deployment pipeline"
    approval_mode: sub_issues  # NEW: "comments" (default) or "sub_issues"
    pipeline:
      stages:
        - name: dev
          environment: development
          policy: dev-team
          # Sub-issue will be assigned to dev-team members
        - name: qa
          environment: qa
          policy: qa-team
        - name: prod
          environment: production
          policy: prod-approvers
          create_tag: true
          is_final: true

    sub_issue_settings:  # NEW: Optional customization
      title_template: "⏳ Approve: {{stage}} for {{version}}"  # ✅ when approved
      body_template: |
        ## Approval Request

        **Stage:** {{stage}}
        **Version:** {{version}}
        **Parent Issue:** #{{parent_issue}}

        ### To Approve
        Close this issue to approve the {{stage}} deployment.

        ### To Deny
        Comment `deny` with a reason, then close.
      labels: [approval-sub-issue]
      auto_close_remaining: true  # Close remaining sub-issues when denied
```

### API Requirements

```go
// New endpoints needed (all available as of Dec 2024)
POST /repos/{owner}/{repo}/issues                              // Create sub-issue
POST /repos/{owner}/{repo}/issues/{issue_number}/sub_issues    // Link as sub-issue
GET  /repos/{owner}/{repo}/issues/{issue_number}/sub_issues    // List sub-issues
GET  /repos/{owner}/{repo}/issues/{issue_number}/parent        // Get parent
```

### Workflow Changes

```yaml
# .github/workflows/handle-sub-issue-close.yml
name: Handle Approval Sub-Issue Close

on:
  issues:
    types: [closed]

jobs:
  process:
    # Only process sub-issues with our label
    if: contains(github.event.issue.labels.*.name, 'approval-sub-issue')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        with:
          action: process-sub-issue-close
          issue_number: ${{ github.event.issue.number }}
          token: ${{ secrets.GITHUB_TOKEN }}
```

### Implementation Stages

#### Stage 1: GitHub Client Extensions (~2 hours)
```go
// internal/github/sub_issues.go
func (c *Client) CreateSubIssue(ctx context.Context, parentNum int, title, body string, assignees []string) (*Issue, error)
func (c *Client) AddSubIssue(ctx context.Context, parentNum, childNum int) error
func (c *Client) ListSubIssues(ctx context.Context, issueNum int) ([]*Issue, error)
func (c *Client) GetParentIssue(ctx context.Context, issueNum int) (*Issue, error)
func (c *Client) RemoveSubIssue(ctx context.Context, parentNum, childNum int) error
```

#### Stage 2: Configuration (~1 hour)
```go
// internal/config/types.go
type Workflow struct {
    // ... existing fields
    ApprovalMode      string             `yaml:"approval_mode,omitempty"`      // "comments" or "sub_issues"
    SubIssueSettings  *SubIssueSettings  `yaml:"sub_issue_settings,omitempty"`
}

type SubIssueSettings struct {
    TitleTemplate        string   `yaml:"title_template,omitempty"`
    BodyTemplate         string   `yaml:"body_template,omitempty"`
    Labels               []string `yaml:"labels,omitempty"`
    AutoCloseRemaining   bool     `yaml:"auto_close_remaining,omitempty"`
}
```

#### Stage 3: Request Action Changes (~3 hours)
- Create parent issue (existing logic)
- For each stage, create sub-issue with assignees
- Link sub-issues to parent
- Update parent issue body with sub-issue links

#### Stage 4: New Action Handler (~3 hours)
```go
// internal/action/sub_issue.go
func (h *Handler) ProcessSubIssueClose(ctx context.Context) error {
    // 1. Get parent issue
    // 2. Determine which stage this sub-issue represents
    // 3. Check if closed as approved or denied
    // 4. Update pipeline state
    // 5. If approved, advance to next stage (create/open next sub-issue)
    // 6. If denied, close remaining sub-issues
    // 7. Update parent issue body with progress
}
```

#### Stage 5: Tests & Documentation (~3 hours)

### Estimated Total: 12-14 hours

---

## Option 2: GitHub Check Runs with Action Buttons

### Overview

Create Check Runs attached to a commit that display approval buttons in the PR/commit checks UI.

```
┌─────────────────────────────────────────────────────────────┐
│ Checks                                                      │
│                                                             │
│ ⏳ Deploy v1.2.0 - Awaiting Approval                        │
│    [Approve DEV] [Deny]                    Details →        │
│                                                             │
│ ✅ CI / Build                                               │
│ ✅ CI / Test                                                │
└─────────────────────────────────────────────────────────────┘
```

### How It Works

1. Create a Check Run with `status: "in_progress"` and `actions` buttons
2. User clicks "Approve" button
3. GitHub sends `check_run.requested_action` webhook
4. App updates check run status and advances pipeline

### Critical Limitation ⚠️

**Check Runs can only be created by GitHub Apps, not GitHub Actions with GITHUB_TOKEN.**

Additionally, when using GITHUB_TOKEN:
> Events triggered by the GITHUB_TOKEN will not create a new workflow run.

This means:
- Must distribute as a **GitHub App**, not just an Action
- Users must install the App on their repository
- Significantly more complex setup

### Configuration (if implemented)

```yaml
workflows:
  deploy:
    approval_mode: check_runs  # Requires GitHub App installation
    check_run_settings:
      name: "Deploy {{version}}"
      details_url: "{{issue_url}}"
      actions:
        - label: "Approve"
          description: "Approve this stage"
          identifier: "approve"
        - label: "Deny"
          description: "Deny deployment"
          identifier: "deny"
```

### Advantages

- Clean, integrated UI in the Checks tab
- Familiar approval pattern for developers
- Works on commits/PRs (not just issues)

### Disadvantages

- **Requires GitHub App** (not just Action)
- More complex installation for users
- Check runs are commit-scoped, not ideal for multi-stage pipelines
- Limited to 3 action buttons
- `requested_action` webhook doesn't trigger Actions workflows

### Verdict: Not Recommended for This Use Case

The GitHub App requirement significantly increases complexity for users. Better suited for CI/CD tools that are already distributed as Apps.

---

## Option 3: Slash Commands (Enhanced Comments)

### Overview

Improve the comment-based UX with better prompts and slash command support.

```
┌─────────────────────────────────────────────────────────────┐
│ ## Quick Actions                                            │
│                                                             │
│ Type one of these commands to take action:                  │
│                                                             │
│ `/approve` - Approve this stage                             │
│ `/deny [reason]` - Deny with reason                         │
│ `/skip` - Skip to next stage (admin only)                   │
│ `/status` - Show current approval status                    │
│                                                             │
│ Or simply comment: `approve`, `lgtm`, `yes`                 │
└─────────────────────────────────────────────────────────────┘
```

### How It Works

Uses existing `issue_comment` trigger but with better UX:
1. Issue body includes clear command instructions
2. Bot reacts to comments with 👀 (seen) → ✅/❌ (result)
3. Rich feedback comments show what happened

### Enhancements

```yaml
workflows:
  deploy:
    approval_mode: comments  # Default, enhanced
    comment_settings:
      show_quick_actions: true     # Show command help in issue
      react_to_comments: true      # React with emoji feedback
      require_slash_prefix: false  # Allow both /approve and approve
      custom_commands:
        approve: ["/approve", "approve", "lgtm", "yes", "👍"]
        deny: ["/deny", "deny", "reject", "no", "👎"]
```

### Advantages

- No additional setup required
- Works with existing infrastructure
- Familiar GitHub comment workflow
- Can be improved incrementally

### Disadvantages

- Still requires typing (no click-to-approve)
- Not as discoverable as buttons
- Comments can get noisy

### Implementation: ~4 hours

---

## Option 4: GitHub Environment Protection Rules

### Overview

Leverage GitHub's built-in environment protection with required reviewers.

```yaml
# In repository Settings > Environments > production
Required reviewers: @team-leads, @sre-team
```

### How It Works

1. Workflow references an environment with protection rules
2. GitHub shows native "Review deployments" UI
3. Reviewers approve via GitHub's built-in interface
4. Workflow continues after approval

### Integration Approach

```yaml
# .github/workflows/deploy.yml
jobs:
  deploy-prod:
    environment: production  # Has required reviewers
    runs-on: ubuntu-latest
    steps:
      - uses: jamengual/enterprise-approval-engine@v1
        with:
          action: record-approval  # Just record, don't gate
          workflow: deploy
          version: ${{ inputs.version }}
```

### Advantages

- Native GitHub UI with "Review pending deployments" button
- Built-in notification system
- No custom implementation needed for basic approval
- Works great for simple workflows

### Disadvantages

- **Requires GitHub Team/Enterprise** for private repos
- Limited to 6 required reviewers per environment
- No threshold support (X of N)
- No OR logic between groups
- Can't customize approval issue/tracking
- Doesn't integrate with IssueOps audit trail

### Verdict: Complementary, Not Replacement

Good for simple cases, but doesn't replace the policy-based approval engine for complex workflows.

---

## Option 5: Hybrid Approach (Recommended)

### Overview

Combine sub-issues with enhanced comments for maximum flexibility.

```yaml
workflows:
  deploy:
    approval_mode: hybrid  # Best of both worlds

    pipeline:
      stages:
        - name: dev
          approval_mode: comments    # Simple stages use comments
          auto_approve: true
        - name: qa
          approval_mode: comments
          policy: qa-team
        - name: prod
          approval_mode: sub_issues  # Critical stages get sub-issues
          policy: production-gate
```

### Benefits

- Use lightweight comments for lower environments
- Use explicit sub-issues for production/critical stages
- Gradual migration path for users
- Flexibility per-stage

---

## Comparison Matrix

| Feature | Comments | Sub-Issues | Check Runs | Env Protection |
|---------|----------|------------|------------|----------------|
| Click to approve | ❌ | ✅ (close) | ✅ (button) | ✅ (native) |
| Works with Actions | ✅ | ✅ | ❌ (needs App) | ✅ |
| X of N threshold | ✅ | ✅ | ❌ | ❌ |
| OR logic | ✅ | ✅ | ❌ | ❌ |
| Custom policies | ✅ | ✅ | ❌ | ❌ |
| Audit trail | ✅ | ✅✅ | ⚠️ | ❌ |
| Assignment notifications | ⚠️ | ✅ | ❌ | ✅ |
| Setup complexity | Low | Medium | High | Low |
| Private repo support | ✅ | ✅ | ✅ | Paid only |

---

## Recommended Implementation Order

1. **Phase 1: Enhanced Comments** (4 hours)
   - Better issue body with clear instructions
   - Emoji reactions on comments
   - `/approve` slash command support

2. **Phase 2: Sub-Issues** (12-14 hours)
   - Full sub-issues implementation
   - Per-stage or per-workflow toggle
   - Hybrid mode support

3. **Phase 3 (Optional): GitHub App** (20+ hours)
   - Only if Check Runs buttons are highly requested
   - Significant distribution/installation complexity

---

## Implementation Status

1. ✅ Document options (this file)
2. ✅ Prototype sub-issues API integration (`internal/github/sub_issues.go`)
3. ✅ **Phase 1: Enhanced Comments UX**
   - Emoji reactions on approval/denial comments (👍 approved, 👎 denied, 👀 seen)
   - Quick Actions section in issue body with command reference
   - `comment_settings` configuration for customization
4. ✅ **Phase 2: Sub-Issues for Approvals**
   - `approval_mode: sub_issues` workflow configuration
   - `sub_issue_settings` for title/body templates, labels, protection
   - Per-stage approval mode override for hybrid workflows
   - Sub-issue creation on pipeline issue creation
   - Sub-issue close processing with authorization checks
   - Issue close protection (reopen unauthorized closes)
5. 🔲 Gather user feedback

## New Configuration Options

```yaml
workflows:
  deploy:
    # Enhanced comments UX (Phase 1)
    comment_settings:
      react_to_comments: true    # Add emoji reactions
      show_quick_actions: true   # Show Quick Actions in issue body

    # Sub-issue approvals (Phase 2)
    approval_mode: sub_issues    # "comments" (default), "sub_issues", or "hybrid"
    sub_issue_settings:
      title_template: "⏳ Approve: {{stage}} for {{version}}"  # ✅ when approved
      labels: [approval-stage]
      auto_close_remaining: true  # Close other sub-issues on denial
      protection:
        only_assignee_can_close: true
        require_approval_comment: false
        prevent_parent_close: true

    # Hybrid mode - per-stage override
    pipeline:
      stages:
        - name: dev
          approval_mode: comments   # Simple stages use comments
        - name: prod
          approval_mode: sub_issues # Critical stages use sub-issues
```
