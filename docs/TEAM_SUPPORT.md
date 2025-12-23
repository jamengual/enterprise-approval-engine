# Team Support

Use GitHub teams in your approval policies.

## Table of Contents

- [Overview](#overview)
- [Why a GitHub App is Required](#why-a-github-app-is-required)
- [Creating a GitHub App](#creating-a-github-app)
- [Configuring the App](#configuring-the-app)
- [Using Team Approvers](#using-team-approvers)
- [Workflow Configuration](#workflow-configuration)
- [Troubleshooting](#troubleshooting)

## Overview

To use GitHub teams as approvers, you need a GitHub App token. The standard `GITHUB_TOKEN` cannot list team members due to permission limitations.

## Why a GitHub App is Required

| Token Type | Can List Team Members | Reason |
|------------|----------------------|--------|
| `GITHUB_TOKEN` | No | Scoped to repository, not organization |
| Personal Access Token | Yes | Works but tied to a user account |
| **GitHub App** | **Yes** | Recommended: not tied to a user, fine-grained permissions |

## Creating a GitHub App

1. Go to **Organization Settings** → **Developer settings** → **GitHub Apps**
2. Click **New GitHub App**
3. Fill in:
   - **Name**: `Approval Engine` (or similar)
   - **Homepage URL**: Your repository URL
   - **Webhook**: Uncheck "Active" (not needed)
4. Set permissions:
   - **Organization permissions** → **Members**: `Read-only`
5. Click **Create GitHub App**
6. Note the **App ID** (shown on the app page)
7. Generate a **private key** and download it

## Configuring the App

### Install the App

1. On the App page, click **Install App**
2. Select your organization
3. Choose **All repositories** or select specific ones
4. Click **Install**

### Add Secrets to Repository

Add these to your repository:
- `APP_ID`: The App ID (add as a **variable**, not secret)
- `APP_PRIVATE_KEY`: The private key contents (add as a **secret**)

## Using Team Approvers

Reference teams with the `team:` prefix:

```yaml
# .github/approvals.yml
policies:
  # Single team
  platform-team:
    approvers:
      - team:platform-engineers
    min_approvals: 2

  # Mixed users and teams
  production:
    approvers:
      - team:sre
      - tech-lead
      - security-lead
    min_approvals: 2

  # Multiple teams
  security-review:
    approvers:
      - team:security
      - team:compliance
    min_approvals: 1
```

### Team Slug Format

Use the team **slug**, not the display name:

| Display Name | Slug | Config Value |
|--------------|------|--------------|
| Platform Engineers | platform-engineers | `team:platform-engineers` |
| SRE Team | sre-team | `team:sre-team` |
| Security & Compliance | security-compliance | `team:security-compliance` |

Find the slug in your team's URL: `github.com/orgs/ORG/teams/SLUG`

## Workflow Configuration

### Handle Approval Comments

```yaml
name: Handle Approval Comments

on:
  issue_comment:
    types: [created]

permissions:
  contents: write
  issues: write

jobs:
  process:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Generate GitHub App token
      - uses: actions/create-github-app-token@v2
        id: app-token
        with:
          app-id: ${{ vars.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}

      # Use the app token for team membership checks
      - uses: jamengual/enterprise-approval-engine@v1
        with:
          action: process-comment
          issue_number: ${{ github.event.issue.number }}
          token: ${{ steps.app-token.outputs.token }}
```

### Request Approval (Optional App Token)

For the `request` action, an App token is only needed if you want to resolve team members for the issue assignees:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    action: request
    workflow: deploy
    version: ${{ inputs.version }}
    token: ${{ secrets.GITHUB_TOKEN }}  # Standard token works here
```

## Troubleshooting

### "Unable to list team members"

**Cause:** Token doesn't have permission to list team members.

**Solution:** Use a GitHub App token as shown above.

### "Team not found"

**Cause:** Team slug is incorrect or team doesn't exist.

**Solution:** Verify the slug in the team URL: `github.com/orgs/ORG/teams/SLUG`

### "User is not a member of the organization"

**Cause:** The commenter is not in the organization.

**Solution:** Team membership is checked via organization membership. The user must be an org member.

### App token not working

**Checklist:**
1. App is installed on the organization
2. App has `Organization > Members: Read` permission
3. `APP_ID` is set as a variable (not secret)
4. `APP_PRIVATE_KEY` contains the full private key including headers

### Permission denied errors

The App needs these minimum permissions:
- **Organization permissions** → **Members**: `Read-only`

To update permissions after creation:
1. Go to the App settings
2. Update permissions
3. **Reinstall** the App (required for permission changes)
