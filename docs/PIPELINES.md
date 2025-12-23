# Multi-Stage Deployment Pipelines

Track deployments through multiple environments with a single approval issue.

## Table of Contents

- [Overview](#overview)
- [Pipeline Configuration](#pipeline-configuration)
- [Workflow Setup](#workflow-setup)
- [Stage Options](#stage-options)
- [Approval Modes](#approval-modes)
- [Auto-Approve](#auto-approve-for-lower-environments)
- [PR and Commit Tracking](#pr-and-commit-tracking)
- [Pipeline Visualization](#pipeline-visualization)
- [Release Strategies](#release-strategies)

## Overview

Progressive deployment pipelines allow you to:
- Track a single deployment through dev → qa → stage → prod
- Require different approvers at each stage
- Visualize progress with color-coded diagrams
- Automatically advance through stages

## Pipeline Configuration

```yaml
version: 1

policies:
  developers:
    approvers: [dev1, dev2, dev3]
    min_approvals: 1

  qa-team:
    approvers: [qa1, qa2]
    min_approvals: 1

  tech-leads:
    approvers: [lead1, lead2]
    min_approvals: 1

  production-approvers:
    approvers: [sre1, sre2, security-lead]
    require_all: true

workflows:
  deploy:
    description: "Deploy through all environments"
    require:
      - policy: developers  # Initial approval to start pipeline
    pipeline:
      track_prs: true       # Include PRs in the issue body
      track_commits: true   # Include commits in the issue body
      stages:
        - name: dev
          environment: development
          policy: developers
          on_approved: "DEV deployment approved! Proceeding to QA..."
        - name: qa
          environment: qa
          policy: qa-team
          on_approved: "QA deployment approved! Proceeding to STAGING..."
        - name: stage
          environment: staging
          policy: tech-leads
          on_approved: "STAGING deployment approved! Ready for PRODUCTION..."
        - name: prod
          environment: production
          policy: production-approvers
          on_approved: "PRODUCTION deployment complete!"
          create_tag: true
          is_final: true
    on_approved:
      close_issue: true
      comment: "Deployment Complete! Version `{{version}}` deployed to all environments."
```

## Workflow Setup

```yaml
# .github/workflows/request-pipeline.yml
name: Request Pipeline Deployment

on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to deploy'
        required: true
        type: string

permissions:
  contents: write
  issues: write
  pull-requests: read  # Required for PR tracking

jobs:
  request:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Needed for commit/PR comparison

      - uses: jamengual/enterprise-approval-engine@v1
        id: approval
        with:
          action: request
          workflow: deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Output Results
        run: |
          echo "## Pipeline Deployment Started" >> $GITHUB_STEP_SUMMARY
          echo "- **Issue:** #${{ steps.approval.outputs.issue_number }}" >> $GITHUB_STEP_SUMMARY
          echo "- **URL:** ${{ steps.approval.outputs.issue_url }}" >> $GITHUB_STEP_SUMMARY
```

## Stage Options

| Option | Description |
|--------|-------------|
| `name` | Stage name (displayed in table) |
| `environment` | GitHub environment name |
| `policy` | Approval policy for this stage |
| `approvers` | Inline approvers (alternative to policy) |
| `on_approved` | Message to post when stage is approved |
| `create_tag` | Create a git tag at this stage |
| `is_final` | Close the issue after this stage |
| `auto_approve` | Automatically approve without human intervention |
| `approval_mode` | Override workflow approval mode for this stage |

## Approval Modes

Choose how approvers interact with approval requests:

| Mode | Description |
|------|-------------|
| `comments` | (Default) Approvers comment `/approve` on the issue |
| `sub_issues` | Creates a sub-issue for each stage - close to approve |
| `hybrid` | Mix modes per stage using `approval_mode` |

### Comments Mode (Default)

Approvers comment on the main issue to advance stages:

```yaml
workflows:
  deploy:
    approval_mode: comments  # Default
    pipeline:
      stages:
        - name: dev
          policy: developers
        - name: prod
          policy: production-approvers
```

### Sub-Issue Mode

Creates dedicated sub-issues for each stage:

```yaml
workflows:
  deploy:
    approval_mode: sub_issues
    sub_issue_settings:
      title_template: "Approve: {{stage}} for {{version}}"
      labels: [approval-stage]
      protection:
        only_assignee_can_close: true
        prevent_parent_close: true
    pipeline:
      stages:
        - name: dev
          policy: developers
        - name: prod
          policy: production-approvers
```

The parent issue shows a table of approval sub-issues:

```markdown
### Approval Sub-Issues

| Stage | Sub-Issue | Status | Assignees |
|-------|-----------|--------|----------|
| DEV | #124 | Awaiting | @alice, @bob |
| PROD | #125 | Awaiting | @sre1, @sre2 |
```

### Hybrid Mode

Mix approval modes per stage:

```yaml
workflows:
  deploy:
    approval_mode: comments  # Default for this workflow
    pipeline:
      stages:
        - name: dev
          policy: developers
          # Uses comments (workflow default)
        - name: prod
          policy: production-approvers
          approval_mode: sub_issues  # Override for production
```

## Auto-Approve for Lower Environments

Use `auto_approve: true` for stages that don't require human intervention:

```yaml
workflows:
  deploy:
    pipeline:
      stages:
        - name: dev
          environment: development
          auto_approve: true              # Automatically approved
          on_approved: "DEV auto-deployed"
        - name: integration
          environment: integration
          auto_approve: true              # Automatically approved
          on_approved: "INTEGRATION auto-deployed"
        - name: staging
          environment: staging
          policy: qa-team                 # Requires manual approval
          on_approved: "STAGING approved"
        - name: production
          environment: production
          policy: production-approvers    # Requires manual approval
          create_tag: true
          is_final: true
```

**How it works:**
1. When a pipeline issue is created, all initial `auto_approve: true` stages are automatically completed
2. When a stage is manually approved, consecutive `auto_approve: true` stages that follow are also completed
3. Auto-approved stages show with a robot indicator in the pipeline table
4. The approver is recorded as `[auto]` in the stage history

## PR and Commit Tracking

Include merged PRs and commits in the approval issue:

```yaml
pipeline:
  track_prs: true        # Include merged PRs
  track_commits: true    # Include commits since last tag
  compare_from_tag: "v*" # Tag pattern to compare from
```

The issue will include:

```markdown
### Pull Requests in this Release

| PR | Title | Author |
|----|-------|--------|
| [#42](https://...) | Add user authentication | @alice |
| [#45](https://...) | Fix payment processing bug | @bob |

### Commits

- [`abc1234`](https://...) feat: add OAuth2 support
- [`def5678`](https://...) fix: handle null payments
```

**Note:** PR tracking requires `pull-requests: read` permission.

## Pipeline Visualization

The issue includes a Mermaid flowchart showing progress:

```markdown
### Pipeline Flow

​```mermaid
flowchart LR
    DEV(DEV)
    QA(QA)
    STAGE(STAGE)
    PROD(PROD)
    DEV --> QA --> STAGE --> PROD

    classDef completed fill:#28a745,stroke:#1e7e34,color:#fff
    classDef current fill:#ffc107,stroke:#d39e00,color:#000
    classDef pending fill:#6c757d,stroke:#545b62,color:#fff
    class DEV completed
    class QA current
    class STAGE,PROD pending
​```
```

Color meanings:
- **Green** - Completed stages
- **Yellow** - Current stage awaiting approval
- **Gray** - Pending stages
- **Cyan** - Auto-approve stages

To disable the diagram:

```yaml
pipeline:
  show_mermaid_diagram: false
```

## Release Strategies

For enterprise environments where PRs merged to main aren't always immediate release candidates.

See [Release Strategies](RELEASE_STRATEGIES.md) for:
- Tag-based releases (default)
- Branch-based releases (GitFlow)
- Label-based releases (flexible batching)
- Milestone-based releases (roadmap alignment)
- Auto-creation on completion
- Hotfix deployment patterns
