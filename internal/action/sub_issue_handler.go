// Package action provides GitHub Action handlers for the IssueOps approval system.
package action

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamengual/enterprise-approval-engine/internal/config"
	"github.com/jamengual/enterprise-approval-engine/internal/github"
)

// SubIssueHandler handles sub-issue creation and processing.
type SubIssueHandler struct {
	client   *github.Client
	config   *config.Config
	workflow *config.Workflow
}

// NewSubIssueHandler creates a new sub-issue handler.
func NewSubIssueHandler(client *github.Client, cfg *config.Config, workflow *config.Workflow) *SubIssueHandler {
	return &SubIssueHandler{
		client:   client,
		config:   cfg,
		workflow: workflow,
	}
}

// CreateSubIssuesForPipeline creates sub-issues for pipeline stages that use sub-issue approval.
// Returns the list of SubIssueInfo for tracking in the parent issue state.
func (h *SubIssueHandler) CreateSubIssuesForPipeline(
	ctx context.Context,
	parentIssueNumber int,
	state *IssueState,
	pipeline *config.PipelineConfig,
) ([]SubIssueInfo, error) {
	var subIssues []SubIssueInfo

	workflowMode := h.workflow.GetApprovalMode()
	settings := h.workflow.SubIssueSettings

	for i, stage := range pipeline.Stages {
		// Skip if this stage doesn't use sub-issues
		if !stage.UsesSubIssue(workflowMode) {
			continue
		}

		// Skip auto-approve stages
		if stage.AutoApprove {
			continue
		}

		// Get approvers for this stage
		approvers := h.getStageApprovers(stage)

		// Create sub-issue title
		title := settings.GetTitleTemplate()
		title = ReplaceTemplateVars(title, map[string]string{
			"stage":        strings.ToUpper(stage.Name),
			"version":      state.Version,
			"workflow":     state.Workflow,
			"environment":  stage.Environment,
			"parent_issue": fmt.Sprintf("%d", parentIssueNumber),
		})

		// Create sub-issue body
		body := settings.GetBodyTemplate()
		body = ReplaceTemplateVars(body, map[string]string{
			"stage":        strings.ToUpper(stage.Name),
			"version":      state.Version,
			"workflow":     state.Workflow,
			"environment":  stage.Environment,
			"parent_issue": fmt.Sprintf("%d", parentIssueNumber),
		})

		// Add stage index to body for tracking
		body += fmt.Sprintf("\n\n<!-- stage-index:%d -->", i)

		// Create the sub-issue
		issue, err := h.client.CreateApprovalSubIssue(
			ctx,
			parentIssueNumber,
			title,
			body,
			settings.GetLabels(),
			approvers,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create sub-issue for stage %s: %w", stage.Name, err)
		}

		// Get the issue ID for sub-issue API (need to fetch it)
		ghIssue, _, err := h.client.GetIssueByNumber(ctx, issue.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to get issue ID for sub-issue #%d: %w", issue.Number, err)
		}

		subIssues = append(subIssues, SubIssueInfo{
			IssueNumber: issue.Number,
			IssueID:     ghIssue.GetID(),
			Stage:       stage.Name,
			Status:      "open",
			Assignees:   approvers,
		})
	}

	return subIssues, nil
}

// getStageApprovers returns the list of approvers for a stage.
func (h *SubIssueHandler) getStageApprovers(stage config.PipelineStage) []string {
	if len(stage.Approvers) > 0 {
		// Filter out team references (can't assign to teams directly)
		var users []string
		for _, a := range stage.Approvers {
			if !config.IsTeam(a) {
				users = append(users, a)
			}
		}
		return users
	}

	if stage.Policy != "" {
		policy, ok := h.config.Policies[stage.Policy]
		if ok {
			// Filter out team references
			var users []string
			for _, a := range policy.Approvers {
				if !config.IsTeam(a) {
					users = append(users, a)
				}
			}
			return users
		}
	}

	return nil
}

// processSubIssueClose handles the close event for an approval sub-issue (internal implementation).
func (h *SubIssueHandler) ProcessSubIssueClose(
	ctx context.Context,
	input ProcessSubIssueCloseInput,
) (*ProcessSubIssueCloseOutput, error) {
	output := &ProcessSubIssueCloseOutput{}

	// Get the parent issue
	parent, err := h.client.GetParentIssue(ctx, input.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue: %w", err)
	}
	if parent == nil {
		return nil, fmt.Errorf("issue #%d is not a sub-issue", input.IssueNumber)
	}

	output.ParentIssueNumber = parent.GetNumber()

	// Get the parent issue body to parse state
	parentIssue, err := h.client.GetIssue(ctx, parent.GetNumber())
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue details: %w", err)
	}

	// Parse the parent issue state
	state, err := ParseIssueState(parentIssue.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parent issue state: %w", err)
	}

	// Find the sub-issue in state
	var subIssue *SubIssueInfo
	var subIssueIdx int
	for i, si := range state.SubIssues {
		if si.IssueNumber == input.IssueNumber {
			subIssue = &state.SubIssues[i]
			subIssueIdx = i
			break
		}
	}

	if subIssue == nil {
		return nil, fmt.Errorf("sub-issue #%d not found in parent state", input.IssueNumber)
	}

	output.StageName = subIssue.Stage

	// Handle reopen event
	if input.Action == "reopened" {
		subIssue.Status = "open"
		subIssue.ClosedBy = ""
		subIssue.ClosedAt = ""
		output.Status = "reopened"

		// Update parent issue state
		state.SubIssues[subIssueIdx] = *subIssue
		if updatedBody, err := UpdateIssueState(parentIssue.Body, *state); err == nil {
			_ = h.client.UpdateIssueBody(ctx, parent.GetNumber(), updatedBody)
		}

		return output, nil
	}

	// Handle close event
	if input.Action != "closed" {
		return output, nil
	}

	// Check if closer is authorized
	protection := h.workflow.SubIssueSettings.Protection
	if protection != nil && protection.OnlyAssigneeCanClose {
		isAuthorized := false
		for _, assignee := range subIssue.Assignees {
			if strings.EqualFold(assignee, input.ClosedBy) {
				isAuthorized = true
				break
			}
		}

		if !isAuthorized {
			// Reopen the issue
			if err := h.reopenUnauthorizedClose(ctx, input.IssueNumber, input.ClosedBy, subIssue.Assignees); err != nil {
				return nil, fmt.Errorf("failed to reopen unauthorized close: %w", err)
			}
			output.Status = "unauthorized"
			output.Message = fmt.Sprintf("@%s is not authorized to close this issue", input.ClosedBy)
			return output, nil
		}
	}

	// Check if we need an approval comment before close
	if protection != nil && protection.RequireApprovalComment {
		hasApproval, err := h.checkForApprovalComment(ctx, input.IssueNumber, input.ClosedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to check for approval comment: %w", err)
		}

		if !hasApproval {
			// Reopen - no approval comment found
			if err := h.reopenNoApprovalComment(ctx, input.IssueNumber, input.ClosedBy); err != nil {
				return nil, fmt.Errorf("failed to reopen without approval: %w", err)
			}
			output.Status = "unauthorized"
			output.Message = "Please comment 'approve' or 'deny' before closing"
			return output, nil
		}
	}

	// Check if this is a denial (look for deny comment)
	isDenial, err := h.checkForDenialComment(ctx, input.IssueNumber, input.ClosedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to check for denial: %w", err)
	}

	// Update sub-issue status
	if isDenial {
		subIssue.Status = "denied"
		output.Status = "denied"
	} else {
		subIssue.Status = "approved"
		output.Status = "approved"
	}
	subIssue.ClosedBy = input.ClosedBy
	subIssue.ClosedAt = time.Now().UTC().Format(time.RFC3339)
	state.SubIssues[subIssueIdx] = *subIssue

	// Record stage completion
	if output.Status == "approved" {
		state.StageHistory = append(state.StageHistory, StageCompletion{
			Stage:      subIssue.Stage,
			ApprovedBy: input.ClosedBy,
			ApprovedAt: subIssue.ClosedAt,
		})
		state.CurrentStage++
	}

	// Check if pipeline is complete
	allApproved := true
	for _, si := range state.SubIssues {
		if si.Status == "open" {
			allApproved = false
			output.NextStage = si.Stage
			break
		}
		if si.Status == "denied" {
			// Pipeline denied
			output.PipelineComplete = true
			break
		}
	}

	if allApproved {
		output.PipelineComplete = true
	}

	// Update parent issue state
	if updatedBody, err := UpdateIssueState(parentIssue.Body, *state); err == nil {
		_ = h.client.UpdateIssueBody(ctx, parent.GetNumber(), updatedBody)
	}

	// If denied and auto_close_remaining is set, close other sub-issues
	if isDenial && h.workflow.SubIssueSettings.AutoCloseRemaining {
		h.closeRemainingSubIssues(ctx, state, input.IssueNumber)
	}

	return output, nil
}

// reopenUnauthorizedClose reopens a sub-issue that was closed by unauthorized user.
func (h *SubIssueHandler) reopenUnauthorizedClose(
	ctx context.Context,
	issueNumber int,
	closedBy string,
	assignees []string,
) error {
	// Reopen the issue
	if err := h.client.ReopenIssue(ctx, issueNumber); err != nil {
		return err
	}

	// Post warning comment
	var approverList string
	for _, a := range assignees {
		approverList += fmt.Sprintf("- @%s\n", a)
	}

	comment := fmt.Sprintf(`**Unauthorized Close Attempt**

@%s attempted to close this issue but is not an authorized approver.

**Authorized approvers for this stage:**
%s
This issue has been automatically reopened.

---
*If you believe this is an error, please contact a repository administrator.*`, closedBy, approverList)

	return h.client.CreateComment(ctx, issueNumber, comment)
}

// reopenNoApprovalComment reopens a sub-issue closed without approval comment.
func (h *SubIssueHandler) reopenNoApprovalComment(
	ctx context.Context,
	issueNumber int,
	closedBy string,
) error {
	// Reopen the issue
	if err := h.client.ReopenIssue(ctx, issueNumber); err != nil {
		return err
	}

	comment := fmt.Sprintf(`**Approval Comment Required**

@%s, please comment ` + "`approve`" + ` or ` + "`deny`" + ` before closing this issue.

This issue has been automatically reopened.`, closedBy)

	return h.client.CreateComment(ctx, issueNumber, comment)
}

// checkForApprovalComment checks if there's an approval comment from the closer.
func (h *SubIssueHandler) checkForApprovalComment(
	ctx context.Context,
	issueNumber int,
	user string,
) (bool, error) {
	comments, err := h.client.ListComments(ctx, issueNumber)
	if err != nil {
		return false, err
	}

	for _, c := range comments {
		if !strings.EqualFold(c.User, user) {
			continue
		}

		body := strings.ToLower(strings.TrimSpace(c.Body))
		if body == "approve" || body == "approved" || body == "lgtm" || body == "yes" ||
			body == "/approve" || body == "deny" || body == "denied" || body == "reject" ||
			body == "rejected" || body == "no" || body == "/deny" {
			return true, nil
		}
	}

	return false, nil
}

// checkForDenialComment checks if the last action comment was a denial.
func (h *SubIssueHandler) checkForDenialComment(
	ctx context.Context,
	issueNumber int,
	user string,
) (bool, error) {
	comments, err := h.client.ListComments(ctx, issueNumber)
	if err != nil {
		return false, err
	}

	// Check the most recent comment from the user
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		if !strings.EqualFold(c.User, user) {
			continue
		}

		body := strings.ToLower(strings.TrimSpace(c.Body))
		if body == "deny" || body == "denied" || body == "reject" ||
			body == "rejected" || body == "no" || body == "/deny" {
			return true, nil
		}
		if body == "approve" || body == "approved" || body == "lgtm" ||
			body == "yes" || body == "/approve" {
			return false, nil
		}
	}

	// Default to approval if closed without explicit action
	return false, nil
}

// closeRemainingSubIssues closes any open sub-issues when one is denied.
func (h *SubIssueHandler) closeRemainingSubIssues(
	ctx context.Context,
	state *IssueState,
	excludeNumber int,
) {
	for _, si := range state.SubIssues {
		if si.IssueNumber == excludeNumber || si.Status != "open" {
			continue
		}

		// Close the sub-issue
		_ = h.client.CloseIssue(ctx, si.IssueNumber)

		// Add comment explaining why
		comment := "This approval sub-issue was automatically closed because another stage was denied."
		_ = h.client.CreateComment(ctx, si.IssueNumber, comment)
	}
}

// ValidateParentIssueClose checks if a parent issue can be closed.
// Returns true if close is allowed, false if it should be reopened.
func (h *SubIssueHandler) ValidateParentIssueClose(
	ctx context.Context,
	parentIssueNumber int,
	closedBy string,
) (bool, string) {
	protection := h.workflow.SubIssueSettings
	if protection == nil || protection.Protection == nil || !protection.Protection.PreventParentClose {
		return true, ""
	}

	// Get the parent issue
	parentIssue, err := h.client.GetIssue(ctx, parentIssueNumber)
	if err != nil {
		return true, "" // Allow if we can't check
	}

	// Parse state
	state, err := ParseIssueState(parentIssue.Body)
	if err != nil {
		return true, ""
	}

	// Check if all sub-issues are closed
	for _, si := range state.SubIssues {
		if si.Status == "open" {
			return false, fmt.Sprintf("Cannot close: sub-issue #%d (%s) is still open", si.IssueNumber, si.Stage)
		}
	}

	return true, ""
}
