package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/issueops/approvals/internal/action"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Get action type from command line or input
	actionType := ""
	if len(os.Args) > 1 {
		actionType = os.Args[1]
	}
	if actionType == "" {
		actionType = action.GetInput("action")
	}

	if actionType == "" {
		return fmt.Errorf("action input is required (request, check, or process-comment)")
	}

	// Get config path
	configPath := action.GetInput("config_path")
	if configPath == "" {
		configPath = action.GetInput("config-path")
	}
	if configPath == "" {
		configPath = ".github/approvals.yml"
	}

	// Get external config repo (optional)
	configRepo := action.GetInput("config_repo")
	if configRepo == "" {
		configRepo = action.GetInput("config-repo")
	}

	// Create handler with options
	handler, err := action.NewHandlerWithOptions(ctx, action.HandlerOptions{
		ConfigPath: configPath,
		ConfigRepo: configRepo,
	})
	if err != nil {
		return err
	}

	switch strings.ToLower(actionType) {
	case "request":
		return handleRequest(ctx, handler)
	case "check":
		return handleCheck(ctx, handler)
	case "process-comment":
		return handleProcessComment(ctx, handler)
	case "close-issue":
		return handleCloseIssue(ctx, handler)
	default:
		return fmt.Errorf("unknown action: %s (expected request, check, process-comment, or close-issue)", actionType)
	}
}

func handleRequest(ctx context.Context, handler *action.Handler) error {
	workflow := action.GetInput("workflow")
	if workflow == "" {
		return fmt.Errorf("workflow input is required for request action")
	}

	input := action.RequestInput{
		Workflow:    workflow,
		Version:     action.GetInput("version"),
		Environment: action.GetInput("environment"),
	}

	output, err := handler.Request(ctx, input)
	if err != nil {
		return err
	}

	fmt.Printf("Created approval issue #%d: %s\n", output.IssueNumber, output.IssueURL)

	return action.SetOutputs(map[string]string{
		"issue_number": fmt.Sprintf("%d", output.IssueNumber),
		"issue_url":    output.IssueURL,
		"status":       "pending",
	})
}

func handleCheck(ctx context.Context, handler *action.Handler) error {
	issueNumber, err := action.GetInputInt("issue_number")
	if err != nil {
		return fmt.Errorf("invalid issue_number: %w", err)
	}
	if issueNumber == 0 {
		issueNumber, err = action.GetInputInt("issue-number")
		if err != nil {
			return fmt.Errorf("invalid issue-number: %w", err)
		}
	}
	if issueNumber == 0 {
		return fmt.Errorf("issue_number input is required for check action")
	}

	timeout, err := action.GetInputDuration("timeout")
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	if timeout == 0 {
		timeout = 72 * time.Hour
	}

	input := action.CheckInput{
		IssueNumber: issueNumber,
		Wait:        action.GetInputBool("wait"),
		Timeout:     timeout,
	}

	output, err := handler.Check(ctx, input)
	if err != nil {
		return err
	}

	fmt.Printf("Approval status: %s\n", output.Status)
	if len(output.Approvers) > 0 {
		fmt.Printf("Approvers: %s\n", strings.Join(output.Approvers, ", "))
	}
	if output.Denier != "" {
		fmt.Printf("Denied by: %s\n", output.Denier)
	}
	if output.SatisfiedGroup != "" {
		fmt.Printf("Satisfied group: %s\n", output.SatisfiedGroup)
	}

	return action.SetOutputs(map[string]string{
		"status":          output.Status,
		"approvers":       strings.Join(output.Approvers, ","),
		"denier":          output.Denier,
		"satisfied_group": output.SatisfiedGroup,
	})
}

func handleProcessComment(ctx context.Context, handler *action.Handler) error {
	// Get issue number from event context
	issueNumber, err := getIssueNumberFromEvent()
	if err != nil {
		return err
	}

	// Get comment details from event
	commentID, commentUser, commentBody, err := getCommentFromEvent()
	if err != nil {
		return err
	}

	input := action.ProcessCommentInput{
		IssueNumber: issueNumber,
		CommentID:   commentID,
		CommentUser: commentUser,
		CommentBody: commentBody,
	}

	output, err := handler.ProcessComment(ctx, input)
	if err != nil {
		return err
	}

	fmt.Printf("Approval status: %s\n", output.Status)
	if len(output.Approvers) > 0 {
		fmt.Printf("Approvers: %s\n", strings.Join(output.Approvers, ", "))
	}
	if output.Denier != "" {
		fmt.Printf("Denied by: %s\n", output.Denier)
	}
	if output.Tag != "" {
		fmt.Printf("Created tag: %s\n", output.Tag)
	}

	return action.SetOutputs(map[string]string{
		"status":          output.Status,
		"approvers":       strings.Join(output.Approvers, ","),
		"denier":          output.Denier,
		"satisfied_group": output.SatisfiedGroup,
		"tag":             output.Tag,
	})
}

func getIssueNumberFromEvent() (int, error) {
	// Try from input first
	issueNumber, err := action.GetInputInt("issue_number")
	if err == nil && issueNumber > 0 {
		return issueNumber, nil
	}

	// Try alternative input name
	issueNumber, err = action.GetInputInt("issue-number")
	if err == nil && issueNumber > 0 {
		return issueNumber, nil
	}

	// Parse from GitHub event
	return action.GetIssueNumberFromEvent()
}

func getCommentFromEvent() (int64, string, string, error) {
	// Parse from GitHub event
	return action.GetCommentFromEvent()
}

func handleCloseIssue(ctx context.Context, handler *action.Handler) error {
	// Get issue number from input or event
	issueNumber, err := getIssueNumberFromEvent()
	if err != nil {
		return fmt.Errorf("failed to get issue number: %w", err)
	}
	if issueNumber == 0 {
		return fmt.Errorf("issue_number input is required for close-issue action")
	}

	// Get the action type from event (closed, reopened, etc.)
	issueAction := action.GetInput("issue_action")
	if issueAction == "" {
		// Try to get from event
		eventAction, err := action.GetEventAction()
		if err == nil && eventAction != "" {
			issueAction = eventAction
		}
	}
	if issueAction == "" {
		issueAction = "closed" // Default to closed
	}

	input := action.CloseIssueInput{
		IssueNumber: issueNumber,
		Action:      issueAction,
	}

	output, err := handler.CloseIssue(ctx, input)
	if err != nil {
		return err
	}

	fmt.Printf("Close issue status: %s\n", output.Status)
	if output.TagDeleted != "" {
		fmt.Printf("Deleted tag: %s\n", output.TagDeleted)
	}

	return action.SetOutputs(map[string]string{
		"status":      output.Status,
		"tag_deleted": output.TagDeleted,
	})
}
