# Configuration Reference

Complete reference for `.github/approvals.yml` configuration.

## Table of Contents

- [Top-Level Structure](#top-level-structure)
- [Defaults](#defaults)
- [Policies](#policies)
  - [Simple Format](#simple-format)
  - [Advanced Format](#advanced-format)
  - [Inline Logic](#inline-logic)
- [Workflows](#workflows)
  - [Issue Configuration](#issue-configuration)
  - [On Approved Actions](#on-approved-actions)
  - [On Denied Actions](#on-denied-actions)
  - [On Closed Actions](#on-closed-actions)
- [Tagging Configuration](#tagging-configuration)
- [Custom Issue Templates](#custom-issue-templates)
- [Semver Configuration](#semver-configuration)
- [Schema Validation](#schema-validation)

## Top-Level Structure

```yaml
version: 1                    # Required: config version (always 1)
defaults: { ... }             # Optional: global defaults
policies: { ... }             # Required: reusable approval policies
workflows: { ... }            # Required: approval workflows
semver: { ... }               # Optional: version handling settings
```

## Defaults

Global defaults that apply to all workflows:

```yaml
defaults:
  timeout: 72h                    # Default approval timeout
  allow_self_approval: false      # Whether requestors can approve their own requests
  issue_labels:                   # Labels added to all approval issues
    - approval-required
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `timeout` | duration | `72h` | Timeout for blocking `check` action with `wait: true` |
| `allow_self_approval` | bool | `false` | Whether the requestor can approve their own request |
| `issue_labels` | string[] | `[]` | Labels added to all approval issues |

## Policies

Policies define reusable groups of approvers.

### Simple Format

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

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `approvers` | string[] | - | List of usernames or `team:slug` references |
| `min_approvals` | int | 0 | Number of approvals required (0 = use `require_all`) |
| `require_all` | bool | `false` | If true, ALL approvers must approve |

### Advanced Format

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

### Inline Logic

For complex expressions, use `logic:` on each source:

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

**Operator precedence:** AND binds tighter than OR. The expression `A and B or C and D` evaluates as `(A AND B) OR (C AND D)`.

## Workflows

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
      labels: [production, deploy]
      assignees_from_policy: true

    # Actions on approval/denial/close
    on_approved: { ... }
    on_denied: { ... }
    on_closed: { ... }
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `description` | string | - | Human-readable description |
| `trigger` | map | - | Trigger conditions (for filtering) |
| `require` | requirement[] | - | **Required:** Approval requirements (OR logic) |
| `issue` | object | - | Issue creation settings |
| `on_approved` | object | - | Actions when approved |
| `on_denied` | object | - | Actions when denied |
| `on_closed` | object | - | Actions when issue is manually closed |
| `pipeline` | object | - | Progressive deployment pipeline config |

### `require[]` Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `policy` | string | - | Reference to a defined policy |
| `approvers` | string[] | - | Inline approvers (alternative to policy) |
| `min_approvals` | int | - | Override policy's min_approvals |
| `require_all` | bool | - | Override policy's require_all |

### Issue Configuration

```yaml
issue:
  title: "Approval: {{version}}"
  body: |                          # Inline custom template
    ## My Custom Approval Issue
    Version: {{.Version}}
  body_file: "templates/my-template.md"  # Or load from file
  labels: [production, deploy]
  assignees_from_policy: true
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `title` | string | `"Approval Required: {workflow}"` | Issue title |
| `body` | string | - | Custom issue body template |
| `body_file` | string | - | Path to template file |
| `labels` | string[] | `[]` | Additional labels for this workflow |
| `assignees_from_policy` | bool | `false` | Auto-assign users from policies (max 10) |

### On Approved Actions

```yaml
on_approved:
  create_tag: true
  tag_prefix: "v"  # Creates v1.2.3
  close_issue: true
  comment: "Approved! Tag {{version}} created."
  tagging:          # Advanced tagging options
    enabled: true
    start_version: "0.1.0"
    auto_increment: patch
    env_prefix: "dev-"
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `create_tag` | bool | `false` | Create a git tag |
| `close_issue` | bool | `false` | Close the issue after approval |
| `comment` | string | - | Comment to post |
| `tagging` | object | - | Advanced tagging configuration |

### On Denied Actions

```yaml
on_denied:
  close_issue: true
  comment: "Denied by {{denier}}."
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `close_issue` | bool | `false` | Close the issue after denial |
| `comment` | string | - | Comment to post |

### On Closed Actions

```yaml
on_closed:
  delete_tag: true
  comment: "Deployment cancelled. Tag {{tag}} deleted."
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `delete_tag` | bool | `false` | Delete the associated tag |
| `comment` | string | - | Comment to post |

## Tagging Configuration

Control how tags are created per workflow:

```yaml
on_approved:
  tagging:
    enabled: true
    start_version: "0.1.0"      # No 'v' prefix, start at 0.1.0
    auto_increment: patch        # Auto-bump: 0.1.0 -> 0.1.1
    env_prefix: "dev-"           # Creates: dev-0.1.0
```

| Option | Description |
|--------|-------------|
| `enabled` | Enable tag creation |
| `start_version` | Starting version and format (e.g., "v1.0.0" or "1.0.0") |
| `prefix` | Version prefix (inferred from `start_version` if not set) |
| `auto_increment` | Auto-bump: `major`, `minor`, `patch`, or omit for manual |
| `env_prefix` | Environment prefix (e.g., "dev-" creates "dev-v1.0.0") |

## Custom Issue Templates

Use Go templates to customize the issue body.

**Available variables:**

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
| `{{.PreviousVersion}}` | Previous version/tag |
| `{{.CommitsCount}}` | Number of commits in this release |
| `{{.HasJiraIssues}}` | Boolean - whether Jira issues exist |
| `{{.JiraIssues}}` | Array of Jira issue data |
| `{{.JiraIssuesTable}}` | Pre-rendered Jira issues table |
| `{{.PipelineTable}}` | Pre-rendered deployment pipeline table |
| `{{.PipelineMermaid}}` | Pre-rendered Mermaid flowchart diagram |
| `{{.Vars.key}}` | Custom variables |

**Template functions:**

| Function | Example | Description |
|----------|---------|-------------|
| `slice` | `{{slice .CommitSHA 0 7}}` | Substring (short SHA) |
| `title` | `{{.Environment \| title}}` | Title case |
| `upper` | `{{.Version \| upper}}` | Uppercase |
| `lower` | `{{.Version \| lower}}` | Lowercase |
| `join` | `{{join .Groups ","}}` | Join array |
| `contains` | `{{if contains .Branch "feature"}}` | Check substring |
| `replace` | `{{replace .Version "v" ""}}` | Replace string |
| `default` | `{{default "N/A" .Environment}}` | Default value |

**Example template file** (`.github/templates/deploy.md`):

```markdown
## {{.Title}}

### Release Information

- **Version:** `{{.Version}}`
- **Requested by:** @{{.Requestor}}
{{- if .CommitSHA}}
- **Commit:** [{{slice .CommitSHA 0 7}}]({{.CommitURL}})
{{- end}}
{{- if .CommitsCount}}
- **Changes:** {{.CommitsCount}} commits since {{.PreviousVersion}}
{{- end}}

{{if .HasJiraIssues}}
### Jira Issues

{{.JiraIssuesTable}}
{{end}}

### Approval Status

{{.GroupsTable}}

---

**Approve:** Comment `approve` | **Deny:** Comment `deny`
```

## Semver Configuration

```yaml
semver:
  prefix: "v"              # Tag prefix (v1.2.3)
  strategy: input          # Use version from input
  validate: true           # Validate semver format
  allow_prerelease: true   # Allow v1.0.0-beta.1
  auto:                    # Label-based auto-increment
    major_labels: [breaking, major]
    minor_labels: [feature, minor]
    patch_labels: [fix, patch, bug]
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `prefix` | string | `"v"` | Tag prefix |
| `strategy` | string | `"input"` | Version strategy: `"input"`, `"auto"` |
| `validate` | bool | `false` | Validate semver format |
| `allow_prerelease` | bool | `false` | Allow prerelease versions |
| `auto` | object | - | Label-based auto-increment settings |

## Schema Validation

Validate your configuration using the JSON schema:

```yaml
# .github/approvals.yml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jamengual/enterprise-approval-engine/main/schema.json

version: 1

policies:
  # ... your config
```

Or validate in CI:

```yaml
- name: Validate Config
  run: |
    npm install -g ajv-cli
    ajv validate -s schema.json -d .github/approvals.yml
```
