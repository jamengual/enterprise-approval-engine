# Troubleshooting

Common issues and solutions for the Enterprise Approval Engine.

## Table of Contents

- [Issue Not Created](#issue-not-created)
- [Approval Comment Not Recognized](#approval-comment-not-recognized)
- [Team Membership Not Working](#team-membership-not-working)
- [Tag Creation Failed](#tag-creation-failed)
- [Rate Limiting](#rate-limiting)
- [Configuration Validation Errors](#configuration-validation-errors)
- [Pipeline Stages Not Advancing](#pipeline-stages-not-advancing)
- [Debug Logging](#debug-logging)

## Issue Not Created

**Symptom:** Action runs but no issue is created.

### Check workflow permissions

Ensure your workflow has `issues: write` permission:

```yaml
permissions:
  contents: write
  issues: write
```

### Verify config file path

Default path is `.github/approvals.yml`. If using a custom path:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    config_path: .github/my-approvals.yml
```

### Validate YAML syntax

```bash
# Install yamllint
pip install yamllint

# Validate config
yamllint .github/approvals.yml
```

### Check workflow name exists

The workflow name must match exactly:

```yaml
# In approvals.yml
workflows:
  production-deploy:  # This name
    require:
      - policy: approvers

# In your workflow
- uses: jamengual/enterprise-approval-engine@v1
  with:
    workflow: production-deploy  # Must match exactly
```

## Approval Comment Not Recognized

**Symptom:** User comments "approve" but nothing happens.

### Verify issue has the correct label

The default label is `approval-required`. Check your workflow condition:

```yaml
jobs:
  process:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
```

### Check approver is in policy

Usernames are **case-sensitive**:

```yaml
policies:
  my-policy:
    approvers: [Alice]  # Must match GitHub username exactly
```

### Verify comment contains approval keyword

Valid approval keywords: `approve`, `approved`, `lgtm`, `yes`, `/approve`

Valid denial keywords: `deny`, `denied`, `reject`, `rejected`, `no`, `/deny`

### Ensure it's not a PR comment

The action only processes issue comments, not PR comments:

```yaml
if: github.event.issue.pull_request == null
```

### Check self-approval setting

If `allow_self_approval: false` (default), the requestor cannot approve their own request:

```yaml
defaults:
  allow_self_approval: false  # Requestor cannot approve
```

## Team Membership Not Working

**Symptom:** Team members can't approve even though they're in the team.

### Use a GitHub App token

The standard `GITHUB_TOKEN` cannot list team members. Use a GitHub App:

```yaml
- uses: actions/create-github-app-token@v2
  id: app-token
  with:
    app-id: ${{ vars.APP_ID }}
    private-key: ${{ secrets.APP_PRIVATE_KEY }}

- uses: jamengual/enterprise-approval-engine@v1
  with:
    token: ${{ steps.app-token.outputs.token }}
```

### Check GitHub App permissions

The App needs `Organization > Members: Read` permission.

### Verify team slug format

Use `team:slug` format, not the display name:

```yaml
policies:
  platform:
    approvers:
      - team:platform-engineers  # Use slug, not "Platform Engineers"
```

### Check organization membership

The user must be a member of the organization, not just the team.

## Tag Creation Failed

**Symptom:** Approval works but tag is not created.

### Check contents permission

```yaml
permissions:
  contents: write  # Required for tag creation
```

### Verify version format

If `semver.validate: true`, the version must be valid semver:

```yaml
# Valid
version: "1.2.3"
version: "v1.2.3"
version: "1.0.0-beta.1"

# Invalid
version: "1.2"
version: "release-1"
```

### Check tag doesn't already exist

The action won't overwrite existing tags. Delete the tag first or use a new version.

### Verify on_approved settings

```yaml
on_approved:
  create_tag: true  # Must be true
```

## Rate Limiting

**Symptom:** Action fails with 403 or rate limit error.

### Automatic retry

The action automatically retries with exponential backoff:
- Initial delay: 1 second
- Max delay: 60 seconds
- Max retries: 5
- Random jitter: 0-500ms

### Reduce API calls

Use external config to avoid re-parsing:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    config_repo: myorg/.github  # Shared config repo
```

### Use a PAT with higher limits

GitHub PATs have higher rate limits than `GITHUB_TOKEN`:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    token: ${{ secrets.PAT_TOKEN }}
```

## Configuration Validation Errors

**Symptom:** Action fails with "invalid configuration" error.

### Use JSON Schema validation

Add schema reference to your config:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jamengual/enterprise-approval-engine/main/schema.json
version: 1

policies:
  # ...
```

### Common configuration mistakes

**Missing version:**
```yaml
# Wrong
policies:
  approvers: [alice]

# Correct
version: 1
policies:
  my-policy:
    approvers: [alice]
```

**Invalid policy reference:**
```yaml
# Wrong - policy name doesn't exist
workflows:
  deploy:
    require:
      - policy: non-existent

# Correct
policies:
  my-policy:
    approvers: [alice]
workflows:
  deploy:
    require:
      - policy: my-policy
```

**Missing required fields:**
```yaml
# Wrong - missing approvers
policies:
  empty-policy: {}

# Correct
policies:
  my-policy:
    approvers: [alice]
    min_approvals: 1
```

## Pipeline Stages Not Advancing

**Symptom:** Approval is recorded but pipeline doesn't advance to next stage.

### Verify stage policy

Each stage needs its own policy:

```yaml
pipeline:
  stages:
    - name: dev
      policy: dev-team     # Must exist in policies
    - name: prod
      policy: prod-team    # Must exist in policies
```

### Check current stage

The issue body shows which stage is current. Approvals only count for the current stage.

### Verify approver is in stage policy

An approver for stage 1 cannot approve stage 2 (unless they're in both policies).

## Debug Logging

Enable debug logging to see detailed action output:

### Repository-level debug

Add a repository secret:
- Name: `ACTIONS_STEP_DEBUG`
- Value: `true`

### Workflow-level debug

```yaml
env:
  ACTIONS_STEP_DEBUG: true
```

### Check action outputs

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  id: approval

- name: Debug outputs
  run: |
    echo "Status: ${{ steps.approval.outputs.status }}"
    echo "Issue: ${{ steps.approval.outputs.issue_number }}"
    echo "Approvers: ${{ steps.approval.outputs.approvers }}"
    echo "Satisfied group: ${{ steps.approval.outputs.satisfied_group }}"
```

## Getting Help

If you're still stuck:

1. Check the [GitHub Issues](https://github.com/jamengual/enterprise-approval-engine/issues) for similar problems
2. Open a new issue with:
   - Your `approvals.yml` configuration (redact sensitive info)
   - The workflow file
   - Error messages from the action logs
   - Steps to reproduce
