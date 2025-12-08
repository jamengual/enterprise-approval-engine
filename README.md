# Enterprise Approval Engine

Enterprise-grade GitHub Action for policy-based approval workflows with per-group thresholds (X of N), OR logic between groups, and automatic semver tag creation.

## Features

- **Flexible Approval Logic**: Support for both AND (all must approve) and threshold (X of N) logic within groups
- **OR Logic Between Groups**: Multiple approval paths - any one group meeting requirements approves the request
- **Mixed Approvers**: Combine individual users and GitHub teams in the same group
- **Semver Tag Creation**: Automatically create git tags upon approval
- **Policy-Based Configuration**: Define reusable approval policies in YAML
- **Issue-Based Workflow**: Transparent audit trail through GitHub issues
- **No External Dependencies**: Pure GitHub Actions, no external services required

## Quick Start

### 1. Create Configuration

Create `.github/approvals.yml` in your repository:

```yaml
version: 1

policies:
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2

  platform-team:
    approvers: [team:platform-engineers]
    require_all: true

workflows:
  production-deploy:
    require:
      # OR logic: either path satisfies approval
      - policy: dev-team        # 2 of 3 developers
      - policy: platform-team   # ALL platform engineers
    on_approved:
      create_tag: true
      close_issue: true
```

### 2. Request Approval Workflow

Create `.github/workflows/request-approval.yml`:

```yaml
name: Request Deployment Approval

on:
  workflow_dispatch:
    inputs:
      version:
        required: true
        type: string

jobs:
  request:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: issueops/approvals@v1
        with:
          action: request
          workflow: production-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}
```

### 3. Handle Approval Comments

Create `.github/workflows/handle-approval.yml`:

```yaml
name: Handle Approval Comments

on:
  issue_comment:
    types: [created]

jobs:
  process:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: issueops/approvals@v1
        with:
          action: process-comment
          issue_number: ${{ github.event.issue.number }}
          token: ${{ secrets.GITHUB_TOKEN }}
```

## Configuration Reference

### Policies

Policies define reusable groups of approvers. There are two formats:

#### Simple Format

```yaml
policies:
  # Threshold-based: X of N must approve
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2

  # All must approve (AND logic)
  security:
    approvers: [team:security, security-lead]
    require_all: true

  # Mixed teams and individuals
  production:
    approvers:
      - team:sre
      - tech-lead
      - product-owner
    min_approvals: 2
```

#### Advanced Format (per-source thresholds)

For complex requirements like "2 from platform AND 1 from security":

```yaml
policies:
  # Complex AND gate
  production-gate:
    from:
      - team: platform-engineers
        min_approvals: 2        # 2 of the platform team
      - team: security
        min_approvals: 1        # 1 of the security team
      - user: alice             # alice must also approve
    logic: and                  # ALL sources must be satisfied

  # Flexible OR gate
  flexible-review:
    from:
      - team: security
        require_all: true       # All security team
      - team: platform
        min_approvals: 2        # OR 2 platform members
    logic: or                   # ANY source is enough

  # Executive approval: any one exec
  exec-approval:
    from:
      - user: ceo
      - user: cto
      - user: vp-engineering
    logic: or

  # User list with threshold
  leads:
    from:
      - users: [tech-lead, product-lead, design-lead]
        min_approvals: 2
```

**Source types:**

- `team: slug` - GitHub team (requires App token)
- `user: username` - Single user (implicit require_all)
- `users: [a, b, c]` - List of users

**Policy-level logic:**

- `logic: and` - ALL sources must be satisfied (default)
- `logic: or` - ANY source being satisfied is enough

#### Inline Logic (mix AND/OR)

For complex expressions, use `logic:` on each source to specify how it connects to the next:

```yaml
policies:
  # (2 security AND 2 platform) OR alice
  complex-gate:
    from:
      - team: security
        min_approvals: 2
        logic: and              # AND with next source
      - team: platform
        min_approvals: 2
        logic: or               # OR with next source
      - user: alice            # alice alone can satisfy

  # (security AND platform) OR (alice AND bob) OR manager
  multi-path:
    from:
      - team: security
        min_approvals: 1
        logic: and
      - team: platform
        min_approvals: 1
        logic: or               # End first AND group
      - user: alice
        logic: and
      - user: bob
        logic: or               # End second AND group
      - user: manager          # Third path
```

**Operator precedence:** AND binds tighter than OR (standard boolean logic).

The expression `A and B or C and D` is evaluated as `(A AND B) OR (C AND D)`.

### Tagging Configuration

Control how tags are created per workflow:

```yaml
workflows:
  dev-deploy:
    on_approved:
      tagging:
        enabled: true
        start_version: "0.1.0"      # No 'v' prefix, start at 0.1.0
        auto_increment: patch        # Auto-bump: 0.1.0 -> 0.1.1 -> 0.1.2
        env_prefix: "dev-"           # Creates: dev-0.1.0, dev-0.1.1

  staging-deploy:
    on_approved:
      tagging:
        enabled: true
        start_version: "v1.0.0"     # 'v' prefix (inferred from start_version)
        auto_increment: minor        # v1.0.0 -> v1.1.0 -> v1.2.0
        env_prefix: "staging-"       # Creates: staging-v1.0.0

  production-deploy:
    on_approved:
      tagging:
        enabled: true
        start_version: "v1.0.0"     # Manual version required (no auto_increment)
```

**Tagging options:**

- `enabled` - Enable tag creation
- `start_version` - Starting version and format (e.g., "v1.0.0" or "1.0.0")
- `prefix` - Version prefix (inferred from `start_version` if not set)
- `auto_increment` - Auto-bump: "major", "minor", "patch", or omit for manual
- `env_prefix` - Environment prefix (e.g., "dev-" creates "dev-v1.0.0")

### Workflows

Workflows define approval requirements and actions:

```yaml
workflows:
  my-workflow:
    description: "Optional description"

    # Trigger conditions (for filtering)
    trigger:
      environment: production

    # Approval requirements (OR logic between items)
    require:
      - policy: dev-team
      - policy: security
      # Or inline approvers:
      - approvers: [alice, bob]
        require_all: true

    # Issue configuration
    issue:
      title: "Approval: {{version}}"
      body: |                          # Inline custom template (optional)
        ## My Custom Approval Issue
        Version: {{.Version}}
        Requested by: @{{.Requestor}}
        {{.GroupsTable}}
      body_file: "templates/my-template.md"  # Or load from file
      labels: [production, deploy]
      assignees_from_policy: true

    # Actions on approval
    on_approved:
      create_tag: true
      tag_prefix: "v"  # Creates v1.2.3
      close_issue: true
      comment: "Approved! Tag {{version}} created."

    # Actions on denial
    on_denied:
      close_issue: true
      comment: "Denied by {{denier}}."

    # Actions when issue is manually closed
    on_closed:
      delete_tag: true   # Delete the tag if issue is closed
      comment: "Deployment cancelled. Tag {{tag}} deleted."
```

### Custom Issue Templates

You can fully customize the issue body using Go templates. Use `body` for inline templates or `body_file` to load from a file.

**Available template variables:**

| Variable | Description |
|----------|-------------|
| `{{.Title}}` | Issue title |
| `{{.Description}}` | Workflow description |
| `{{.Version}}` | Semver version |
| `{{.Requestor}}` | GitHub username who requested |
| `{{.Environment}}` | Environment name |
| `{{.RunURL}}` | Link to workflow run |
| `{{.RepoURL}}` | Repository URL |
| `{{.CommitSHA}}` | Full commit SHA |
| `{{.CommitURL}}` | Link to commit |
| `{{.Branch}}` | Branch name |
| `{{.GroupsTable}}` | Pre-rendered approval status table |
| `{{.Timestamp}}` | Request timestamp |
| `{{.Vars.key}}` | Custom variables |

**Template functions:**

- `{{slice .CommitSHA 0 7}}` - Substring (short SHA)
- `{{.Environment | title}}` - Title case
- `{{.Version | upper}}` - Uppercase
- `{{join .Groups ","}}` - Join array

**Example custom template file** (`.github/templates/deploy.md`):

```markdown
## 🚀 {{.Title}}

### Release Information
- **Version:** `{{.Version}}`
- **Requested by:** @{{.Requestor}}
{{- if .CommitSHA}}
- **Commit:** [{{slice .CommitSHA 0 7}}]({{.CommitURL}})
{{- end}}

### Deployment Flow
✅ dev → ⏳ staging → ⏳ production

### Pre-deployment Checklist
- [ ] Tests passing
- [ ] Release notes reviewed
- [ ] Stakeholders notified

### Approval Status
{{.GroupsTable}}

**Approve:** Comment `approve` | **Deny:** Comment `deny`
```

### Tag Deletion on Issue Close

Some teams want to delete tags when an approval issue is manually closed (e.g., deployment cancelled). Configure this per-workflow:

```yaml
workflows:
  dev-deploy:
    on_closed:
      delete_tag: true   # Delete tag when issue is closed
      comment: "🗑️ Cancelled. Tag {{tag}} deleted."

  production-deploy:
    on_closed:
      delete_tag: false  # NEVER delete production tags
```

To handle issue close events, add this workflow:

```yaml
# .github/workflows/handle-close.yml
name: Handle Issue Close
on:
  issues:
    types: [closed]

jobs:
  handle:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: issueops/approvals@v1
        with:
          action: close-issue
          issue_number: ${{ github.event.issue.number }}
          token: ${{ secrets.GITHUB_TOKEN }}
```

### Defaults

Global defaults that apply to all workflows:

```yaml
defaults:
  timeout: 72h
  allow_self_approval: false
  issue_labels:
    - approval-required
```

### Semver

Configure version handling:

```yaml
semver:
  prefix: "v"           # Tag prefix (v1.2.3)
  strategy: input       # Use version from input
  validate: true        # Validate semver format
  allow_prerelease: true
```

## Action Inputs

| Input | Description | Required |
|-------|-------------|----------|
| `action` | Action type: `request`, `check`, `process-comment`, or `close-issue` | Yes |
| `workflow` | Workflow name from config (for `request`) | For request |
| `version` | Semver version for tag creation | No |
| `issue_number` | Issue number (for `check`, `process-comment`, `close-issue`) | For check/close |
| `issue_action` | Issue event action for `close-issue` (`closed`, `reopened`) | No |
| `token` | GitHub token | Yes |
| `config_path` | Path to config file | No (default: `.github/approvals.yml`) |

## Action Outputs

| Output | Description |
|--------|-------------|
| `status` | Approval status: `pending`, `approved`, `denied`, `timeout`, `tag_deleted`, `skipped` |
| `issue_number` | Created/checked issue number |
| `issue_url` | Issue URL |
| `approvers` | Comma-separated list of users who approved |
| `denier` | User who denied the request |
| `satisfied_group` | Name of the group that satisfied approval |
| `tag` | Created tag name |
| `tag_deleted` | Tag that was deleted (for `close-issue` action) |

## Approval Keywords

**Approval**: `approve`, `approved`, `lgtm`, `yes`, `/approve`

**Denial**: `deny`, `denied`, `reject`, `rejected`, `no`, `/deny`

## Team Support

To use GitHub team-based approvers, you need elevated permissions. The standard `GITHUB_TOKEN` cannot list team members. Use a GitHub App token:

```yaml
- uses: actions/create-github-app-token@v2
  id: app-token
  with:
    app-id: ${{ vars.APP_ID }}
    private-key: ${{ secrets.APP_PRIVATE_KEY }}

- uses: issueops/approvals@v1
  with:
    action: process-comment
    token: ${{ steps.app-token.outputs.token }}
```

The GitHub App needs `Organization > Members: Read` permission.

## Examples

See the [examples](./examples) directory for complete workflow examples.

## How It Works

1. **Request**: Creates a GitHub issue with approval requirements table
2. **Approve/Deny**: Users comment with approval keywords
3. **Process**: Action evaluates comments against policy requirements
4. **Complete**: On approval, creates tag and closes issue

### Approval Logic

- **Within a group**: Use `require_all: true` for AND logic, or `min_approvals: N` for threshold
- **Between groups**: OR logic - any one group being satisfied approves the request

Example with 3 groups:

```yaml
require:
  - policy: dev-team      # 2 of 3 developers, OR
  - policy: security      # ALL security team, OR
  - approvers: [manager]  # Just the manager
```

Any ONE of these paths satisfies the approval requirement.

## License

MIT License
