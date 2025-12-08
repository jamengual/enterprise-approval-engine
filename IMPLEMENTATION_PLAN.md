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
