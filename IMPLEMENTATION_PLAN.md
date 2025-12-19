# Implementation Plan: IssueOps Approvals Action

## Stage 1: Project Foundation
**Goal**: Establish project structure, Go module, and basic CI
**Success Criteria**:
- Go module initializes correctly
- Basic project structure in place
- CI runs and passes
- Action metadata file is valid

**Tests**:
- `go build ./...` succeeds
- `go test ./...` passes (even if no tests yet)
- GitHub Action syntax validation passes

**Status**: Complete

### Tasks
1. Initialize Go module (`go mod init github.com/owner/issueops-approvals`)
2. Create directory structure per ARCHITECTURE.md
3. Create `action.yml` with input/output definitions
4. Create `Dockerfile` for Docker-based action
5. Set up GitHub Actions CI workflow
6. Add basic Makefile for local development

---

## Stage 2: Configuration System
**Goal**: Parse and validate `.github/approvals.yml` configuration files
**Success Criteria**:
- Can parse valid YAML configs into Go structs
- Validates required fields and structure
- Returns clear error messages for invalid configs
- Supports minimal and full configurations

**Tests**:
- Parse minimal config (policies + workflows only)
- Parse full config with all features
- Reject config with missing required fields
- Reject config with invalid policy references
- Handle both `team:name` and plain username formats

**Status**: Complete

### Tasks
1. Define Go types for config schema (`internal/config/types.go`)
2. Implement YAML parsing with `gopkg.in/yaml.v3`
3. Add validation logic for:
   - Required fields (version, policies, workflows)
   - Policy references in workflows exist
   - Approver format validation
4. Create JSON Schema for IDE autocompletion
5. Write comprehensive config parsing tests

---

## Stage 3: GitHub API Client
**Goal**: Wrapper around GitHub API for issues, comments, teams, and tags
**Success Criteria**:
- Can create/update/close issues
- Can read and parse comments
- Can list team members (with App token)
- Can create tags and releases
- Handles rate limiting gracefully

**Tests**:
- Create issue with labels and assignees
- Read comments from issue
- Parse approval keywords from comments
- Mock team membership checks
- Create annotated git tag

**Status**: Complete

### Tasks
1. Create GitHub client wrapper (`internal/github/client.go`)
2. Implement issue operations (`internal/github/issues.go`):
   - CreateIssue, UpdateIssue, CloseIssue
   - AddLabels, AddAssignees
   - ListComments
3. Implement team operations (`internal/github/teams.go`):
   - GetTeamMembers (requires App token)
   - IsUserInTeam
4. Implement tag operations (`internal/github/tags.go`):
   - CreateTag
   - ValidateTagDoesNotExist
5. Handle authentication (GITHUB_TOKEN vs App token)
6. Add retry logic for rate limits

---

## Stage 4: Semver Engine
**Goal**: Parse, validate, and optionally auto-increment semantic versions
**Success Criteria**:
- Validates semver format (with/without 'v' prefix)
- Supports prerelease versions (v1.2.3-beta.1)
- Can increment major/minor/patch based on strategy
- Generates valid git tag names

**Tests**:
- Parse "1.2.3" and "v1.2.3"
- Parse prerelease "1.2.3-alpha.1"
- Reject invalid formats
- Increment patch: 1.2.3 → 1.2.4
- Increment with prefix: v1.2.3 → v1.2.4

**Status**: Complete

### Tasks
1. Implement semver parsing (`internal/semver/parse.go`)
2. Add validation logic (`internal/semver/validate.go`)
3. Implement increment strategies (`internal/semver/increment.go`):
   - `input`: Use provided version
   - `auto`: Increment based on labels
   - `conventional`: Parse from commit messages (future)
4. Add tag name generation with configurable prefix
5. Write comprehensive semver tests

---

## Stage 5: Approval Engine - Single Group
**Goal**: Core approval logic for single approval group
**Success Criteria**:
- Tracks approvals per issue
- Counts approvals toward threshold
- Detects when threshold is met
- Handles approve/deny keywords
- Prevents self-approval (if configured)

**Tests**:
- Single approver, threshold 1 → approved after 1
- Three approvers, threshold 2 → approved after 2
- Self-approval blocked when configured
- Denial immediately fails (if configured)
- Ignore comments from non-approvers

**Status**: Complete

### Tasks
1. Define approval types (`internal/approval/types.go`):
   - ApprovalRequest, ApprovalStatus, Approval, Denial
2. Implement comment parser (`internal/approval/parser.go`):
   - Extract approve/deny keywords
   - Handle variations (approve, approved, lgtm, /approve)
3. Implement single-group engine (`internal/approval/engine.go`):
   - CollectApprovals from comments
   - EvaluateThreshold
   - CheckSelfApproval
4. Implement status tracking (`internal/approval/status.go`):
   - Pending/Approved/Denied states
   - Who approved/denied
5. Write approval engine tests

---

## Stage 6: Approval Engine - Multi-Group OR Logic
**Goal**: Support multiple requirement groups with OR logic
**Success Criteria**:
- Multiple groups in `require:` array
- Any ONE group meeting threshold = approved
- Tracks per-group status independently
- Reports which group was satisfied

**Tests**:
- Two groups: first group meets threshold → approved
- Two groups: second group meets threshold → approved
- Two groups: neither meets threshold → pending
- Mixed: inline approvers + policy reference
- Override policy min_approvals in requirement

**Status**: Complete

### Tasks
1. Extend engine for multi-group evaluation
2. Implement policy resolution (lookup by name)
3. Add per-group status tracking
4. Implement "first satisfied group wins" logic
5. Add requirement name generation (policy name or hash)
6. Update status table generation for multiple groups
7. Write multi-group tests

---

## Stage 7: Actions Implementation
**Goal**: Implement action entrypoints (request, check, process-comment)
**Success Criteria**:
- `request` creates approval issue
- `check` returns current status
- `process-comment` handles approve/deny comments
- All actions set proper outputs

**Tests**:
- Request creates issue with correct body
- Check returns pending for new issue
- Process-comment updates status correctly
- Outputs match expected format

**Status**: Complete

### Tasks
1. Implement request action (`internal/action/request.go`):
   - Load config, resolve workflow
   - Create issue with status table
   - Set outputs (issue_number, issue_url)
2. Implement check action (`internal/action/check.go`):
   - Load issue comments
   - Evaluate approval status
   - Set outputs (status, approvers)
3. Implement process-comment action (`internal/action/process.go`):
   - Parse triggering comment
   - Update approval status
   - Post status update comment
   - Create tag if approved
   - Close issue if configured
4. Implement main entrypoint (`cmd/action/main.go`)
5. Write integration-style tests

---

## Stage 8: Team Integration
**Goal**: Resolve team members for team-based approvers
**Success Criteria**:
- Detect `team:org/name` format in approvers
- Resolve team members via GitHub API
- Works with GitHub App token
- Graceful error when token lacks permissions

**Tests**:
- Resolve team:org/engineers to member list
- Handle team not found error
- Handle permission denied error
- Mixed team + individual approvers

**Status**: Complete

### Tasks
1. Add team detection in approver parsing
2. Implement team member resolution
3. Cache team members per request (avoid repeated API calls)
4. Add clear error messages for auth issues
5. Document App token requirements
6. Write team integration tests (mocked)

---

## Stage 9: Issue Templates & UX
**Goal**: Generate clear, informative approval issues
**Success Criteria**:
- Status table shows all groups with progress
- Template variables work ({{version}}, etc.)
- Updates table when status changes
- Provides clear approval/deny instructions

**Tests**:
- Template variables replaced correctly
- Status table renders as expected
- Table updates on approval
- Markdown renders correctly on GitHub

**Status**: Complete

### Tasks
1. Create issue body template
2. Implement template variable substitution
3. Create status table generator
4. Add update logic (edit issue body on status change)
5. Add hidden state marker for tracking (JSON in comment)
6. Test markdown rendering

---

## Stage 10: End-to-End Testing & Polish
**Goal**: Complete testing, documentation, and release preparation
**Success Criteria**:
- All unit tests pass
- Integration tests with mocked GitHub API
- Example workflows documented
- README complete with usage examples
- Action works in real GitHub workflow

**Tests**:
- Full workflow: request → approve → tag created
- Full workflow: request → deny → closed
- Timeout scenario
- Invalid config error handling
- Permission error handling

**Status**: Complete

### Tasks
1. Write E2E tests with GitHub API mocks
2. Create example workflows in `examples/`
3. Write comprehensive README
4. Create CONTRIBUTING guide
5. Set up release workflow (goreleaser)
6. Test in real repository
7. Create v1.0.0 release

---

## Technology Decisions

### Language: Go
**Rationale**:
- Compiles to single binary (fast action startup)
- Strong typing catches config errors at parse time
- Excellent GitHub API libraries (`google/go-github`)
- Developer familiarity

### Action Type: Docker
**Rationale**:
- Consistent environment across runners
- No dependency on runner's Go installation
- Better reproducibility
- Slightly slower startup, but acceptable for approval workflows

### Configuration: YAML
**Rationale**:
- Familiar to GitHub Actions users
- Supports comments for documentation
- Easy to read and edit
- JSON Schema provides IDE support

### State Storage: Issue Body + Comments
**Rationale**:
- No external dependencies
- Complete audit trail in GitHub
- Works with GitHub's native UI
- Searchable and filterable

---

## Dependencies

### Go Packages
```go
require (
    github.com/google/go-github/v57 v57.0.0
    github.com/sethvargo/go-githubactions v1.1.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/Masterminds/semver/v3 v3.2.1
    github.com/stretchr/testify v1.8.4
)
```

### External Tools
- Docker (for building action image)
- goreleaser (for releases)
- golangci-lint (for linting)

---

## Risk Mitigation

### Risk: GitHub API Rate Limits
**Mitigation**:
- Use conditional requests (ETags)
- Implement exponential backoff
- Cache team membership results
- Recommend GitHub App for higher limits

### Risk: Token Expiration (1 hour for App tokens)
**Mitigation**:
- Document limitation clearly
- Suggest event-driven pattern for long approvals
- Implement timeout with clear messaging

### Risk: Complex Config Errors
**Mitigation**:
- JSON Schema for IDE validation
- Clear error messages with line numbers
- Validate config before any operations
- Provide minimal config examples

### Risk: Team Membership Permissions
**Mitigation**:
- Clear documentation on App requirements
- Graceful fallback for individual users
- Explicit error when team lookup fails

---

## Stage 11: Progressive Deployment Pipelines
**Goal**: Single-issue tracking through multiple environments (dev → qa → stage → prod)
**Success Criteria**:
- Single issue tracks deployment through all stages
- Each stage has its own approval policy
- Progress table updates as stages are approved
- PR and commit tracking shows what's being deployed
- Tags created at configured stages

**Tests**:
- Pipeline issue created with all stages pending
- Approve stage 1 → advances to stage 2
- Progress table updates correctly
- PR tracking populates from git history
- Final stage closes issue

**Status**: Complete

### Implementation

#### Files Created/Modified:
- `internal/action/pipeline.go` - Pipeline processor for stage management
- `internal/action/pipeline_template.go` - Pipeline-specific issue body generation
- `internal/github/commits.go` - Git comparison and PR extraction APIs

#### Key Types:
```go
// PipelineConfig in config/types.go
type PipelineConfig struct {
    Stages         []PipelineStage
    TrackPRs       bool
    TrackCommits   bool
    CompareFromTag string
    ReleaseStrategy ReleaseStrategyConfig
}

type PipelineStage struct {
    Name        string
    Environment string
    Policy      string
    Approvers   []string
    OnApproved  string
    CreateTag   bool
    IsFinal     bool
}
```

#### Key Functions:
- `PipelineProcessor.EvaluatePipelineStage()` - Evaluates current stage approval
- `PipelineProcessor.ProcessPipelineApproval()` - Advances pipeline on approval
- `GeneratePipelineIssueBody()` - Creates progress table with PR/commit tracking
- `Client.GetMergedPRsBetween()` - Fetches PRs between two refs
- `Client.CompareCommits()` - Gets commits between refs

#### Configuration Example:
```yaml
workflows:
  deploy:
    pipeline:
      track_prs: true
      track_commits: true
      stages:
        - name: dev
          policy: developers
          on_approved: "✅ DEV approved!"
        - name: qa
          policy: qa-team
        - name: prod
          policy: production-approvers
          create_tag: true
          is_final: true
```

#### Workflow Requirements:
- `pull-requests: read` permission for PR tracking
- `contents: write` for tag creation
- `issues: write` for issue management

---

## Stage 12: Release Candidate Strategies
**Goal**: Support multiple strategies for selecting which PRs belong to a release
**Success Criteria**:
- Four strategies: tag, branch, label, milestone
- Auto-creation of next release artifact on completion
- Optional cleanup (close milestone, remove labels, delete branch)
- Hotfix workflow support (bypass stages)

**Tests**:
- Tag strategy: PRs between v1.0 and v2.0
- Branch strategy: PRs merged to release/v1.2.0
- Label strategy: PRs with release:v1.2.0 label
- Milestone strategy: PRs in v1.2.0 milestone
- Auto-create next milestone on completion

**Status**: Complete

### Implementation

#### Files Created:
- `internal/config/release_strategy.go` - Strategy configuration types
- `internal/github/releases.go` - GitHub API for milestones, labels, branches
- `internal/action/release_tracker.go` - Strategy-aware PR/commit fetcher

#### Key Types:
```go
// ReleaseStrategyType enum
const (
    StrategyTag       ReleaseStrategyType = "tag"
    StrategyBranch    ReleaseStrategyType = "branch"
    StrategyLabel     ReleaseStrategyType = "label"
    StrategyMilestone ReleaseStrategyType = "milestone"
)

// ReleaseStrategyConfig in config/release_strategy.go
type ReleaseStrategyConfig struct {
    Type      ReleaseStrategyType
    Branch    BranchStrategyConfig
    Label     LabelStrategyConfig
    Milestone MilestoneStrategyConfig
    AutoCreate AutoCreateConfig
}

type AutoCreateConfig struct {
    Enabled     bool
    NextVersion string   // "patch", "minor", "major"
    CreateIssue bool
    Comment     string
}
```

#### Key Functions:
```go
// ReleaseTracker methods
func (r *ReleaseTracker) GetReleaseContents(ctx, previousTag) (*ReleaseContents, error)
func (r *ReleaseTracker) CreateNextReleaseArtifact(ctx, nextVersion) error
func (r *ReleaseTracker) CleanupCurrentRelease(ctx, prs) error

// GitHub client methods
func (c *Client) GetPRsByMilestone(ctx, milestoneNumber) ([]PullRequest, error)
func (c *Client) GetPRsByLabel(ctx, label) ([]PullRequest, error)
func (c *Client) GetPRsMergedToBranch(ctx, branchName) ([]PullRequest, error)
func (c *Client) CreateMilestone(ctx, title, description) (*Milestone, error)
func (c *Client) CreateBranch(ctx, branchName, sourceRef) (*Branch, error)
func (c *Client) CreateLabel(ctx, name, color, description) error
```

#### Configuration Examples:

**Milestone Strategy:**
```yaml
pipeline:
  release_strategy:
    type: milestone
    milestone:
      pattern: "v{{version}}"
      close_after_release: true
    auto_create:
      enabled: true
      next_version: minor
      create_issue: true
```

**Branch Strategy:**
```yaml
pipeline:
  release_strategy:
    type: branch
    branch:
      pattern: "release/{{version}}"
      base_branch: main
      delete_after_release: false
```

**Label Strategy:**
```yaml
pipeline:
  release_strategy:
    type: label
    label:
      pattern: "release:{{version}}"
      pending_label: "pending-release"
      remove_after_release: true
```

**Hotfix Workflow (separate workflow, tag strategy):**
```yaml
workflows:
  hotfix:
    description: "Emergency hotfix - direct to prod"
    pipeline:
      release_strategy:
        type: tag   # No cleanup, no auto-create
      stages:
        - name: prod
          policy: production-approvers
          create_tag: true
          is_final: true
```

#### Cleanup Options (all default to false):
| Strategy | Option | Description |
|----------|--------|-------------|
| Branch | `delete_after_release` | Delete release branch |
| Label | `remove_after_release` | Remove labels from PRs |
| Milestone | `close_after_release` | Close the milestone |

#### Auto-Creation Flow:
1. Final stage (prod) approved
2. Calculate next version (patch/minor/major)
3. Create next artifact (branch/label/milestone)
4. Optionally create new approval issue
5. Post comment about next release

---

## Stage 13: Pipeline Visualization (Mermaid Diagrams)
**Goal**: Add visual flowchart diagrams to pipeline approval issues
**Success Criteria**:
- Mermaid diagram shows pipeline stages with colored nodes
- Colors update based on stage status (completed, current, pending, auto-approve)
- Can be disabled via configuration

**Tests**:
- Generate diagram with all stages pending
- Generate diagram with completed stages
- Generate diagram with auto-approve stages
- Generate diagram when disabled (returns empty string)

**Status**: Complete

### Implementation

#### Files Modified:
- `internal/action/pipeline.go` - Added `GeneratePipelineMermaid()` function
- `internal/action/template.go` - Added `PipelineMermaid` field to `TemplateData`
- `internal/config/types.go` - Added `ShowMermaidDiagram` option to `PipelineConfig`

#### Key Functions:
```go
// GeneratePipelineMermaid generates a Mermaid flowchart for the pipeline
func GeneratePipelineMermaid(state *IssueState, pipeline *config.PipelineConfig) string

// ShouldShowMermaidDiagram returns whether to show the diagram (default: true)
func (p *PipelineConfig) ShouldShowMermaidDiagram() bool
```

#### Color Scheme:
| Status | Color | Hex Code |
|--------|-------|----------|
| Completed | Green | `#28a745` |
| Current | Yellow/Amber | `#ffc107` |
| Pending | Gray | `#6c757d` |
| Auto-approve | Cyan | `#17a2b8` |

#### Emojis in Labels:
- ✅ - Completed (manual approval)
- 🤖 - Auto-approved or auto-approve pending
- ⏳ - Current stage awaiting approval
- ⬜ - Pending future stages

#### Configuration:
```yaml
pipeline:
  show_mermaid_diagram: true  # Default: true
  stages:
    - name: dev
    - name: prod
```

---

## Stage 14: Enhanced Approval UX (Sub-Issues & Comments)
**Goal**: Provide interactive approval experiences via sub-issues and enhanced comments
**Success Criteria**:
- Sub-issues created for each pipeline stage when configured
- Closing sub-issue = approving the stage
- Emoji reactions on approval/denial comments
- Quick Actions section in issue body
- Issue close protection (reopen unauthorized closes)
- Hybrid mode: mix comments and sub-issues per stage

**Tests**:
- Create pipeline with sub-issues mode → sub-issues created and linked
- Close sub-issue → stage approved, parent issue updated
- Unauthorized close → issue reopened with warning
- Hybrid mode respects per-stage overrides
- Comment reactions added on approval/denial

**Status**: Complete

### Implementation

#### Phase 1: Enhanced Comments UX
- **Emoji reactions** on approval comments: 👍 approved, 👎 denied, 👀 seen
- **Quick Actions section** in issue body with command reference table
- **Configuration** via `comment_settings` in workflow

#### Phase 2: Sub-Issues for Approvals
- **Approval modes**: `comments` (default), `sub_issues`, `hybrid`
- **Sub-issue settings**: title/body templates, labels, protection
- **Per-stage override**: `approval_mode` on individual stages
- **Close protection**: auto-reopen if closed by unauthorized user
- **Parent protection**: prevent parent close until sub-issues done

#### Files Created/Modified:
- `internal/action/sub_issue_handler.go` - Sub-issue creation and close handling
- `internal/action/action.go` - Reaction support, `ProcessSubIssueClose` handler
- `internal/action/pipeline.go` - `GeneratePipelineIssueBodyWithSubIssues()`
- `internal/action/template.go` - `SubIssueInfo` struct in `IssueState`
- `internal/config/types.go` - `ApprovalMode`, `SubIssueSettings`, `CommentSettings`
- `internal/github/issues.go` - `GetIssueByNumber`, `ReopenIssue`
- `internal/github/sub_issues.go` - GitHub Sub-Issues API wrapper

#### Key Types:
```go
// ApprovalMode defines how approvals are collected
type ApprovalMode string
const (
    ApprovalModeComments  ApprovalMode = "comments"
    ApprovalModeSubIssues ApprovalMode = "sub_issues"
    ApprovalModeHybrid    ApprovalMode = "hybrid"
)

// SubIssueSettings configures sub-issue based approval UX
type SubIssueSettings struct {
    TitleTemplate      string
    BodyTemplate       string
    Labels             []string
    AutoCloseRemaining bool
    Protection         *SubIssueProtection
}

// SubIssueProtection configures issue close protection
type SubIssueProtection struct {
    OnlyAssigneeCanClose   bool
    RequireApprovalComment bool
    PreventParentClose     bool
}

// CommentSettings configures enhanced comment UX
type CommentSettings struct {
    ReactToComments    *bool
    ShowQuickActions   *bool
    RequireSlashPrefix bool
}
```

#### Configuration Example:
```yaml
workflows:
  deploy:
    approval_mode: sub_issues
    sub_issue_settings:
      title_template: "⏳ Approve: {{stage}} for {{version}}"  # ✅ when approved
      labels: [approval-stage]
      protection:
        only_assignee_can_close: true
        prevent_parent_close: true
    comment_settings:
      react_to_comments: true
      show_quick_actions: true
    pipeline:
      stages:
        - name: dev
          policy: dev-team
        - name: prod
          policy: prod-team
          approval_mode: sub_issues  # Per-stage override
```

---

## Future Enhancements

### Planned Features
- **Slack/Teams Integration**: Notify channels on approval requests
- **Scheduled Releases**: Time-based release windows
- **Rollback Workflows**: One-click rollback with approval
- **Metrics Dashboard**: Approval cycle time, bottleneck analysis
- **Multi-Repo Releases**: Coordinate releases across repositories

### API Extensions
- Webhook support for external integrations
- REST API for programmatic access
- GraphQL queries for complex status checks

