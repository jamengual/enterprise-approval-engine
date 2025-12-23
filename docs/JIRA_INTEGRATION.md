# Jira Integration

Automatically extract Jira issues from commits and update Fix Versions on approval.

## Table of Contents

- [Overview](#overview)
- [Links-Only Mode](#links-only-mode)
- [Full Mode with API](#full-mode-with-api)
- [Configuration](#configuration)
- [Outputs](#outputs)
- [Custom Templates](#custom-templates)

## Overview

The action extracts Jira issue keys (e.g., `PROJ-123`) from:
- Commit messages
- Branch names
- PR titles

Two modes are available:

| Mode | Auth Required | Features |
|------|---------------|----------|
| Links-only | No | Issue keys as clickable links |
| Full | Yes | Links + summary, status, type, Fix Version updates |

## Links-Only Mode

Just provide `jira_base_url` to display issues as clickable links. No authentication required.

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    action: request
    workflow: production-deploy
    version: v1.2.0
    token: ${{ secrets.GITHUB_TOKEN }}
    jira_base_url: https://yourcompany.atlassian.net
```

The approval issue will include:

```markdown
### Jira Issues
- [PROJ-123](https://yourcompany.atlassian.net/browse/PROJ-123)
- [PROJ-456](https://yourcompany.atlassian.net/browse/PROJ-456)
```

## Full Mode with API

Add credentials to fetch issue details and update Fix Versions:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    action: request
    workflow: production-deploy
    version: v1.2.0
    token: ${{ secrets.GITHUB_TOKEN }}
    # Jira configuration
    jira_base_url: https://yourcompany.atlassian.net
    jira_user_email: ${{ secrets.JIRA_EMAIL }}
    jira_api_token: ${{ secrets.JIRA_API_TOKEN }}
    jira_update_fix_version: 'true'
```

The approval issue will include a detailed table:

```markdown
### Jira Issues in this Release

| Key | Summary | Type | Status |
|-----|---------|------|--------|
| [PROJ-123](https://...) | Fix login bug | Bug | Done |
| [PROJ-456](https://...) | Add dark mode | Feature | In Progress |
```

### Creating a Jira API Token

1. Go to https://id.atlassian.com/manage-profile/security/api-tokens
2. Click **Create API token**
3. Give it a name (e.g., "GitHub Actions")
4. Copy the token

### Setting Up Secrets

Add these secrets to your repository:
- `JIRA_EMAIL`: Your Jira account email
- `JIRA_API_TOKEN`: The API token you created

## Configuration

### Action Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `jira_base_url` | Jira Cloud base URL | No | - |
| `jira_user_email` | Jira user email for API | No | - |
| `jira_api_token` | Jira API token | No | - |
| `jira_update_fix_version` | Update Fix Version on approval | No | `true` |
| `include_jira_issues` | Include issues in request body | No | `true` |

### Fix Version Updates

When `jira_update_fix_version: true`, the action updates each Jira issue's Fix Version field when the approval is granted. The version name is taken from the `version` input.

This runs during the `process-comment` action when status becomes `approved`.

## Outputs

| Output | Description | Available For |
|--------|-------------|---------------|
| `jira_issues` | Comma-separated list of issue keys | `request` |
| `jira_issues_json` | JSON array of issue details | `request` |

### Using Outputs

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  id: approval
  with:
    action: request
    jira_base_url: https://company.atlassian.net

- name: Use Jira Outputs
  run: |
    echo "Issues: ${{ steps.approval.outputs.jira_issues }}"
    # Output: PROJ-123,PROJ-456

    echo "Details: ${{ steps.approval.outputs.jira_issues_json }}"
    # Output: [{"key":"PROJ-123","summary":"Fix login bug",...}]
```

## Custom Templates

Use Jira data in custom issue templates:

```yaml
workflows:
  deploy:
    issue:
      body_file: ".github/templates/deploy.md"
```

**.github/templates/deploy.md:**

```markdown
## Deployment Request

Version: {{.Version}}

{{if .HasJiraIssues}}
### Jira Issues

{{.JiraIssuesTable}}

**Total issues:** {{len .JiraIssues}}
{{else}}
No Jira issues found in commits.
{{end}}
```

### Available Template Variables

| Variable | Type | Description |
|----------|------|-------------|
| `{{.HasJiraIssues}}` | bool | Whether any issues were found |
| `{{.JiraIssues}}` | array | Array of issue objects |
| `{{.JiraIssuesTable}}` | string | Pre-formatted markdown table |

### JiraIssues Object Structure

```json
{
  "key": "PROJ-123",
  "summary": "Fix login bug",
  "type": "Bug",
  "status": "Done",
  "url": "https://company.atlassian.net/browse/PROJ-123"
}
```

### Iterating Over Issues

```markdown
{{range .JiraIssues}}
- [{{.key}}]({{.url}}): {{.summary}} ({{.status}})
{{end}}
```
