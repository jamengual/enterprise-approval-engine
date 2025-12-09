package approval

import (
	"strings"
	"time"

	"github.com/issueops/approvals/internal/config"
)

// Engine evaluates approval status based on comments and configuration.
type Engine struct {
	parser            *Parser
	allowSelfApproval bool
	teamResolver      TeamResolver
}

// TeamResolver resolves team membership. Can be nil if teams aren't used.
type TeamResolver interface {
	GetTeamMembers(team string) ([]string, error)
}

// NewEngine creates a new approval engine.
func NewEngine(allowSelfApproval bool, teamResolver TeamResolver) *Engine {
	return &Engine{
		parser:            NewParser(),
		allowSelfApproval: allowSelfApproval,
		teamResolver:      teamResolver,
	}
}

// Evaluate checks the approval status for a request.
//
// Approval Logic:
//   - Multiple requirement groups are combined with OR logic
//   - Any ONE group being satisfied = request is approved
//   - Within a group:
//   - If require_all is true: ALL approvers must approve (AND logic)
//   - If min_approvals is set: X of N approvers must approve
//   - Supports mixed teams and individuals in the same group
func (e *Engine) Evaluate(req *Request) (*ApprovalResult, error) {
	result := &ApprovalResult{
		Status: StatusPending,
		Groups: make([]GroupStatus, 0, len(req.Workflow.Require)),
	}

	// Parse all comments for approvals and denials
	for _, comment := range req.Comments {
		parsed := e.parser.Parse(comment.Body)

		if parsed.IsDenial {
			// Check if denier is an eligible approver in any group
			// Note: Requestor can always deny (withdraw) their own request,
			// even when allow_self_approval is false
			if e.isEligibleApprover(req, comment.User) {
				result.Denials = append(result.Denials, Denial{
					User:      comment.User,
					Timestamp: comment.CreatedAt,
					Comment:   comment.Body,
				})
				result.Status = StatusDenied
				result.Denier = comment.User
				return result, nil
			}
		}

		if parsed.IsApproval {
			result.Approvals = append(result.Approvals, Approval{
				User:      comment.User,
				Timestamp: comment.CreatedAt,
				Comment:   comment.Body,
			})
		}
	}

	// Evaluate each requirement group (OR logic between groups)
	for _, requirement := range req.Workflow.Require {
		groupStatus, err := e.evaluateGroup(req, requirement, result.Approvals)
		if err != nil {
			return nil, err
		}
		result.Groups = append(result.Groups, groupStatus)

		// OR logic: if ANY group is satisfied, request is approved
		if groupStatus.Satisfied {
			result.Status = StatusApproved
			result.SatisfiedGroup = groupStatus.Name
			// Continue evaluating other groups for reporting purposes
		}
	}

	return result, nil
}

// evaluateGroup evaluates a single requirement group.
func (e *Engine) evaluateGroup(req *Request, requirement config.Requirement, approvals []Approval) (GroupStatus, error) {
	// Check if using advanced "from" format
	if requirement.Policy != "" {
		policy := req.Config.Policies[requirement.Policy]
		if policy.UsesAdvancedFormat() {
			return e.evaluateAdvancedGroup(req, requirement, policy, approvals)
		}
	}

	// Simple format evaluation
	return e.evaluateSimpleGroup(req, requirement, approvals)
}

// evaluateSimpleGroup evaluates a group with the simple approvers format.
func (e *Engine) evaluateSimpleGroup(req *Request, requirement config.Requirement, approvals []Approval) (GroupStatus, error) {
	approvers, minApprovals, requireAll := req.Config.ResolveRequirement(requirement)

	// Expand teams to individual users
	expandedApprovers, err := e.expandApprovers(approvers)
	if err != nil {
		return GroupStatus{}, err
	}

	// Filter out requestor if self-approval is not allowed
	if !e.allowSelfApproval {
		expandedApprovers = filterUser(expandedApprovers, req.Requestor)
	}

	// Track which approvers have approved
	approvedUsers := make(map[string]bool)
	for _, approval := range approvals {
		if e.isUserInList(approval.User, expandedApprovers) {
			approvedUsers[strings.ToLower(approval.User)] = true
		}
	}

	// Build list of who approved
	var approved []string
	for user := range approvedUsers {
		approved = append(approved, user)
	}

	status := GroupStatus{
		Name:        requirement.Name(),
		Approvers:   expandedApprovers,
		RequireAll:  requireAll,
		MinRequired: minApprovals,
		Current:     len(approvedUsers),
		Approved:    approved,
	}

	// Determine if this group is satisfied
	if requireAll {
		// ALL must approve
		status.Satisfied = len(approvedUsers) == len(expandedApprovers)
	} else {
		// X of N must approve
		status.Satisfied = len(approvedUsers) >= minApprovals
	}

	return status, nil
}

// evaluateAdvancedGroup evaluates a group with the advanced "from" format.
func (e *Engine) evaluateAdvancedGroup(req *Request, requirement config.Requirement, policy config.Policy, approvals []Approval) (GroupStatus, error) {
	defaultLogic := policy.GetLogic()

	status := GroupStatus{
		Name:    requirement.Name(),
		Logic:   defaultLogic,
		Sources: make([]SourceStatus, 0, len(policy.From)),
	}

	var allApprovers []string
	var allApproved []string

	// Evaluate each source
	sourceResults := make([]bool, len(policy.From))
	for i, source := range policy.From {
		sourceStatus, err := e.evaluateSource(req, source, approvals)
		if err != nil {
			return GroupStatus{}, err
		}

		status.Sources = append(status.Sources, sourceStatus)
		allApprovers = append(allApprovers, sourceStatus.Approvers...)
		allApproved = append(allApproved, sourceStatus.Approved...)
		sourceResults[i] = sourceStatus.Satisfied
	}

	// Remove duplicates
	status.Approvers = deduplicateUsers(allApprovers)
	status.Approved = deduplicateUsers(allApproved)
	status.Current = len(status.Approved)

	// Evaluate the expression with per-source "then" logic
	// Uses standard precedence: AND before OR
	status.Satisfied = e.evaluateExpression(policy.From, sourceResults, defaultLogic)

	return status, nil
}

// evaluateExpression evaluates a boolean expression with AND/OR operators.
// Uses standard precedence: AND binds tighter than OR.
// Expression: A then:and B then:or C then:and D
// Parsed as: (A AND B) OR (C AND D)
func (e *Engine) evaluateExpression(sources []config.ApproverSource, results []bool, defaultLogic string) bool {
	if len(sources) == 0 {
		return false
	}
	if len(sources) == 1 {
		return results[0]
	}

	// Build groups of ANDs, then OR between groups
	// Example: A and B or C and D -> [[A,B], [C,D]] -> (A&&B) || (C&&D)
	type andGroup struct {
		indices []int
	}

	groups := []andGroup{{indices: []int{0}}}

	for i := 0; i < len(sources)-1; i++ {
		// Get the logic connector to the next source
		logic := sources[i].Logic
		if logic == "" {
			logic = defaultLogic
		}

		if logic == "or" {
			// Start a new group
			groups = append(groups, andGroup{indices: []int{i + 1}})
		} else {
			// Add to current group (AND)
			groups[len(groups)-1].indices = append(groups[len(groups)-1].indices, i+1)
		}
	}

	// Evaluate: OR between groups, AND within groups
	for _, group := range groups {
		groupSatisfied := true
		for _, idx := range group.indices {
			if !results[idx] {
				groupSatisfied = false
				break
			}
		}
		if groupSatisfied {
			return true // OR logic: any group satisfied = success
		}
	}

	return false
}

// evaluateSource evaluates a single source within an advanced policy.
func (e *Engine) evaluateSource(req *Request, source config.ApproverSource, approvals []Approval) (SourceStatus, error) {
	approvers := source.GetApprovers()
	minApprovals := source.GetMinApprovals()
	requireAll := source.GetRequireAll()

	// Expand teams to individual users
	expandedApprovers, err := e.expandApprovers(approvers)
	if err != nil {
		return SourceStatus{}, err
	}

	// Filter out requestor if self-approval is not allowed
	if !e.allowSelfApproval {
		expandedApprovers = filterUser(expandedApprovers, req.Requestor)
	}

	// Track which approvers have approved
	approvedUsers := make(map[string]bool)
	for _, approval := range approvals {
		if e.isUserInList(approval.User, expandedApprovers) {
			approvedUsers[strings.ToLower(approval.User)] = true
		}
	}

	// Build list of who approved
	var approved []string
	for user := range approvedUsers {
		approved = append(approved, user)
	}

	// Build source name
	sourceName := source.Team
	if sourceName == "" && source.User != "" {
		sourceName = source.User
	}
	if sourceName == "" && len(source.Users) > 0 {
		sourceName = "users"
	}

	status := SourceStatus{
		Name:        sourceName,
		Approvers:   expandedApprovers,
		RequireAll:  requireAll,
		MinRequired: minApprovals,
		Current:     len(approvedUsers),
		Approved:    approved,
	}

	// Determine if this source is satisfied
	if requireAll {
		status.Satisfied = len(approvedUsers) == len(expandedApprovers)
	} else {
		status.Satisfied = len(approvedUsers) >= minApprovals
	}

	return status, nil
}

// deduplicateUsers removes duplicate usernames (case-insensitive).
func deduplicateUsers(users []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, u := range users {
		lower := strings.ToLower(u)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, u)
		}
	}
	return result
}

// expandApprovers expands team references to individual users.
func (e *Engine) expandApprovers(approvers []string) ([]string, error) {
	var expanded []string
	seen := make(map[string]bool)

	for _, approver := range approvers {
		if config.IsTeam(approver) {
			if e.teamResolver == nil {
				// No team resolver, keep as-is (will never match)
				if !seen[strings.ToLower(approver)] {
					seen[strings.ToLower(approver)] = true
					expanded = append(expanded, approver)
				}
				continue
			}

			teamSlug := config.ParseTeam(approver)
			members, err := e.teamResolver.GetTeamMembers(teamSlug)
			if err != nil {
				return nil, err
			}
			for _, member := range members {
				if !seen[strings.ToLower(member)] {
					seen[strings.ToLower(member)] = true
					expanded = append(expanded, member)
				}
			}
		} else {
			if !seen[strings.ToLower(approver)] {
				seen[strings.ToLower(approver)] = true
				expanded = append(expanded, approver)
			}
		}
	}

	return expanded, nil
}

// isEligibleApprover checks if a user is eligible to approve in any group.
func (e *Engine) isEligibleApprover(req *Request, user string) bool {
	for _, requirement := range req.Workflow.Require {
		approvers, _, _ := req.Config.ResolveRequirement(requirement)
		expanded, err := e.expandApprovers(approvers)
		if err != nil {
			continue
		}
		if e.isUserInList(user, expanded) {
			return true
		}
	}
	return false
}

// isUserInList checks if a user is in a list (case-insensitive).
func (e *Engine) isUserInList(user string, list []string) bool {
	userLower := strings.ToLower(user)
	for _, u := range list {
		if strings.ToLower(u) == userLower {
			return true
		}
	}
	return false
}

// filterUser removes a user from a list (case-insensitive).
func filterUser(users []string, exclude string) []string {
	excludeLower := strings.ToLower(exclude)
	var filtered []string
	for _, u := range users {
		if strings.ToLower(u) != excludeLower {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// WaitForApproval polls until the request is approved, denied, or times out.
func (e *Engine) WaitForApproval(req *Request, timeout time.Duration, pollInterval time.Duration, getComments func() ([]Comment, error)) (*ApprovalResult, error) {
	deadline := time.Now().Add(timeout)

	for {
		comments, err := getComments()
		if err != nil {
			return nil, err
		}
		req.Comments = comments

		result, err := e.Evaluate(req)
		if err != nil {
			return nil, err
		}

		if result.Status != StatusPending {
			return result, nil
		}

		if time.Now().After(deadline) {
			result.Status = StatusTimeout
			return result, nil
		}

		time.Sleep(pollInterval)
	}
}
