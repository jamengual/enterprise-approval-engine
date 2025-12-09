package action

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/issueops/approvals/internal/approval"
	"github.com/issueops/approvals/internal/config"
	"github.com/issueops/approvals/internal/github"
	"github.com/issueops/approvals/internal/semver"
)

// Handler handles action execution.
type Handler struct {
	client *github.Client
	config *config.Config
}

// HandlerOptions configures how the handler loads configuration.
type HandlerOptions struct {
	ConfigPath string
	ConfigRepo string // Optional: owner/repo for external config (e.g., "myorg/.github")
}

// NewHandler creates a new action handler.
func NewHandler(ctx context.Context, configPath string) (*Handler, error) {
	return NewHandlerWithOptions(ctx, HandlerOptions{ConfigPath: configPath})
}

// NewHandlerWithOptions creates a new action handler with additional options.
func NewHandlerWithOptions(ctx context.Context, opts HandlerOptions) (*Handler, error) {
	client, err := github.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	var cfg *config.Config
	if opts.ConfigRepo != "" {
		// Use external config with fallback
		// Get current repo name from environment
		repoName := ""
		if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
			parts := strings.Split(repo, "/")
			if len(parts) == 2 {
				repoName = parts[1]
			}
		}

		// Create fetch function that uses GitHub client
		fetchFunc := func(repo, path string) ([]byte, error) {
			return client.GetFileContentsFromRepo(ctx, repo, path)
		}

		cfg, _, err = config.LoadWithFallback(opts.ConfigRepo, repoName, opts.ConfigPath, fetchFunc)
	} else {
		// Use local config only
		cfg, err = config.Load(opts.ConfigPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &Handler{
		client: client,
		config: cfg,
	}, nil
}

// RequestInput contains inputs for the request action.
type RequestInput struct {
	Workflow    string
	Version     string
	Environment string
}

// RequestOutput contains outputs from the request action.
type RequestOutput struct {
	IssueNumber int
	IssueURL    string
}

// Request creates an approval request issue.
func (h *Handler) Request(ctx context.Context, input RequestInput) (*RequestOutput, error) {
	workflow, err := h.config.GetWorkflow(input.Workflow)
	if err != nil {
		return nil, err
	}

	// Validate version if provided
	if input.Version != "" && h.config.Semver.Validate {
		if err := semver.ValidateWithOptions(input.Version, h.config.Semver.AllowPrerelease); err != nil {
			return nil, fmt.Errorf("invalid version: %w", err)
		}
	}

	// Get context from environment
	requestor := os.Getenv("GITHUB_ACTOR")
	runID := os.Getenv("GITHUB_RUN_ID")
	serverURL := os.Getenv("GITHUB_SERVER_URL")
	if serverURL == "" {
		serverURL = "https://github.com"
	}
	repoFullName := os.Getenv("GITHUB_REPOSITORY")
	repoURL := fmt.Sprintf("%s/%s", serverURL, repoFullName)

	runURL := ""
	if runID != "" {
		runURL = fmt.Sprintf("%s/actions/runs/%s", repoURL, runID)
	}

	commitSHA := os.Getenv("GITHUB_SHA")
	commitURL := ""
	if commitSHA != "" {
		commitURL = fmt.Sprintf("%s/commit/%s", repoURL, commitSHA)
	}
	branch := os.Getenv("GITHUB_REF_NAME")

	// Build issue title
	title := workflow.Issue.Title
	if title == "" {
		title = fmt.Sprintf("Approval Required: %s", input.Workflow)
		if input.Version != "" {
			title = fmt.Sprintf("Approval Required: %s %s", input.Workflow, input.Version)
		}
	}
	title = ReplaceTemplateVars(title, map[string]string{
		"version":     input.Version,
		"environment": input.Environment,
		"workflow":    input.Workflow,
	})

	// Build template data
	groups := BuildGroupTemplateData(h.config, workflow, nil)
	templateData := TemplateData{
		Title:       title,
		Description: workflow.Description,
		Version:     input.Version,
		Requestor:   requestor,
		Environment: input.Environment,
		RunURL:      runURL,
		RepoURL:     repoURL,
		CommitSHA:   commitSHA,
		CommitURL:   commitURL,
		Branch:      branch,
		Groups:      groups,
		Vars:        make(map[string]string),
		State: IssueState{
			Workflow:  input.Workflow,
			Version:   input.Version,
			Requestor: requestor,
			RunID:     runID,
		},
	}

	body, err := GenerateIssueBodyFromConfig(templateData, &workflow.Issue)
	if err != nil {
		return nil, err
	}

	// Collect labels
	labels := append([]string{}, h.config.Defaults.IssueLabels...)
	labels = append(labels, workflow.Issue.Labels...)

	// Collect assignees if configured
	var assignees []string
	if workflow.Issue.AssigneesFromPolicy {
		for _, req := range workflow.Require {
			approvers, _, _ := h.config.ResolveRequirement(req)
			for _, a := range approvers {
				if !config.IsTeam(a) {
					assignees = append(assignees, a)
				}
			}
		}
	}

	// Create the issue
	issue, err := h.client.CreateIssue(ctx, github.CreateIssueOptions{
		Title:     title,
		Body:      body,
		Labels:    labels,
		Assignees: assignees,
	})
	if err != nil {
		return nil, err
	}

	return &RequestOutput{
		IssueNumber: issue.Number,
		IssueURL:    issue.HTMLURL,
	}, nil
}

// CheckInput contains inputs for the check action.
type CheckInput struct {
	IssueNumber int
	Wait        bool
	Timeout     time.Duration
}

// CheckOutput contains outputs from the check action.
type CheckOutput struct {
	Status         string
	Approvers      []string
	Denier         string
	SatisfiedGroup string
}

// Check checks the approval status of an issue.
func (h *Handler) Check(ctx context.Context, input CheckInput) (*CheckOutput, error) {
	// Get the issue
	issue, err := h.client.GetIssue(ctx, input.IssueNumber)
	if err != nil {
		return nil, err
	}

	// Parse state from issue body
	state, err := ParseIssueState(issue.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue state: %w", err)
	}

	workflow, err := h.config.GetWorkflow(state.Workflow)
	if err != nil {
		return nil, err
	}

	// Create team resolver that uses the GitHub client
	teamResolver := &githubTeamResolver{client: h.client, ctx: ctx}

	// Create approval engine
	engine := approval.NewEngine(h.config.Defaults.AllowSelfApproval, teamResolver)

	// Build request
	comments, err := h.client.ListComments(ctx, input.IssueNumber)
	if err != nil {
		return nil, err
	}

	req := &approval.Request{
		Config:      h.config,
		Workflow:    workflow,
		IssueNumber: input.IssueNumber,
		Requestor:   state.Requestor,
		Comments:    convertComments(comments),
	}

	// Evaluate
	result, err := engine.Evaluate(req)
	if err != nil {
		return nil, err
	}

	return &CheckOutput{
		Status:         string(result.Status),
		Approvers:      extractApprovers(result.Approvals),
		Denier:         result.Denier,
		SatisfiedGroup: result.SatisfiedGroup,
	}, nil
}

// ProcessCommentInput contains inputs for the process-comment action.
type ProcessCommentInput struct {
	IssueNumber int
	CommentID   int64
	CommentUser string
	CommentBody string
}

// ProcessCommentOutput contains outputs from the process-comment action.
type ProcessCommentOutput struct {
	Status         string
	Approvers      []string
	Denier         string
	SatisfiedGroup string
	Tag            string
}

// ProcessComment processes an approval/denial comment.
func (h *Handler) ProcessComment(ctx context.Context, input ProcessCommentInput) (*ProcessCommentOutput, error) {
	// Get the issue
	issue, err := h.client.GetIssue(ctx, input.IssueNumber)
	if err != nil {
		return nil, err
	}

	// Parse state from issue body
	state, err := ParseIssueState(issue.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue state: %w", err)
	}

	workflow, err := h.config.GetWorkflow(state.Workflow)
	if err != nil {
		return nil, err
	}

	// Create team resolver
	teamResolver := &githubTeamResolver{client: h.client, ctx: ctx}

	// Create approval engine
	engine := approval.NewEngine(h.config.Defaults.AllowSelfApproval, teamResolver)

	// Get all comments
	comments, err := h.client.ListComments(ctx, input.IssueNumber)
	if err != nil {
		return nil, err
	}

	req := &approval.Request{
		Config:      h.config,
		Workflow:    workflow,
		IssueNumber: input.IssueNumber,
		Requestor:   state.Requestor,
		Comments:    convertComments(comments),
	}

	// Evaluate
	result, err := engine.Evaluate(req)
	if err != nil {
		return nil, err
	}

	output := &ProcessCommentOutput{
		Status:         string(result.Status),
		Approvers:      extractApprovers(result.Approvals),
		Denier:         result.Denier,
		SatisfiedGroup: result.SatisfiedGroup,
	}

	// Handle approval
	if result.Status == approval.StatusApproved {
		// Post approval comment
		if workflow.OnApproved.Comment != "" {
			comment := ReplaceTemplateVars(workflow.OnApproved.Comment, map[string]string{
				"version":       state.Version,
				"satisfied_group": result.SatisfiedGroup,
			})
			_ = h.client.CreateComment(ctx, input.IssueNumber, comment)
		}

		// Create tag if configured
		tagging := workflow.OnApproved.Tagging
		shouldTag := workflow.OnApproved.CreateTag || tagging.IsEnabled()

		if shouldTag {
			version := state.Version

			// If no version provided and auto_increment is set, calculate next version
			if version == "" && tagging.GetAutoIncrement() != "" {
				// Use env_prefix when looking up latest tag for proper env-specific versioning
				prefix := tagging.EnvPrefix + tagging.GetPrefix()
				latestTag, err := h.client.GetLatestTagWithPrefix(ctx, prefix)
				if err != nil {
					return nil, fmt.Errorf("failed to get latest tag: %w", err)
				}

				if latestTag == "" {
					// No existing tags, use start_version
					version = tagging.GetStartVersion()
				} else {
					// Increment from latest tag (strip env prefix first)
					tagVersion := latestTag
					if tagging.EnvPrefix != "" && strings.HasPrefix(latestTag, tagging.EnvPrefix) {
						tagVersion = latestTag[len(tagging.EnvPrefix):]
					}
					version, err = semver.NextVersion(tagVersion, tagging.GetAutoIncrement())
					if err != nil {
						return nil, fmt.Errorf("failed to calculate next version: %w", err)
					}
				}
			} else if version == "" {
				// No version and no auto-increment - use start_version
				version = tagging.GetStartVersion()
			}

			// Format the tag name
			tagName := tagging.FormatTag(version)

			// Check if tag already exists
			exists, err := h.client.TagExists(ctx, tagName)
			if err != nil {
				return nil, fmt.Errorf("failed to check tag existence: %w", err)
			}
			if exists {
				return nil, fmt.Errorf("tag %s already exists", tagName)
			}

			_, err = h.client.CreateTag(ctx, github.CreateTagOptions{
				Name:    tagName,
				Message: fmt.Sprintf("Release %s - approved via IssueOps", tagName),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create tag: %w", err)
			}
			output.Tag = tagName

			// Store tag in issue state for potential deletion on close
			state.Tag = tagName
			state.ApprovedAt = time.Now().UTC().Format(time.RFC3339)
			if updatedBody, err := UpdateIssueState(issue.Body, *state); err == nil {
				_ = h.client.UpdateIssueBody(ctx, input.IssueNumber, updatedBody)
			}
		}

		// Close issue if configured
		if workflow.OnApproved.CloseIssue {
			_ = h.client.CloseIssue(ctx, input.IssueNumber)
		}
	}

	// Handle denial
	if result.Status == approval.StatusDenied {
		// Post denial comment
		if workflow.OnDenied.Comment != "" {
			comment := ReplaceTemplateVars(workflow.OnDenied.Comment, map[string]string{
				"denier": result.Denier,
			})
			_ = h.client.CreateComment(ctx, input.IssueNumber, comment)
		}

		// Close issue if configured
		if workflow.OnDenied.CloseIssue {
			_ = h.client.CloseIssue(ctx, input.IssueNumber)
		}
	}

	return output, nil
}

// githubTeamResolver implements approval.TeamResolver using the GitHub client.
type githubTeamResolver struct {
	client *github.Client
	ctx    context.Context
}

func (r *githubTeamResolver) GetTeamMembers(team string) ([]string, error) {
	members, err := r.client.GetTeamMembers(r.ctx, team)
	if err != nil {
		return nil, err
	}
	var logins []string
	for _, m := range members {
		logins = append(logins, m.Login)
	}
	return logins, nil
}

func convertComments(comments []github.IssueComment) []approval.Comment {
	result := make([]approval.Comment, len(comments))
	for i, c := range comments {
		result[i] = approval.Comment{
			ID:   c.ID,
			User: c.User,
			Body: c.Body,
		}
	}
	return result
}

func extractApprovers(approvals []approval.Approval) []string {
	seen := make(map[string]bool)
	var result []string
	for _, a := range approvals {
		if !seen[strings.ToLower(a.User)] {
			seen[strings.ToLower(a.User)] = true
			result = append(result, a.User)
		}
	}
	return result
}

// SetOutput writes an output to the GitHub Actions output file.
func SetOutput(name, value string) error {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		fmt.Printf("::set-output name=%s::%s\n", name, value)
		return nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s=%s\n", name, value)
	return err
}

// SetOutputs writes multiple outputs to the GitHub Actions output file.
func SetOutputs(outputs map[string]string) error {
	for name, value := range outputs {
		if err := SetOutput(name, value); err != nil {
			return err
		}
	}
	return nil
}

// GetInput gets an action input from environment variables.
func GetInput(name string) string {
	// Try INPUT_NAME format first
	envName := "INPUT_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if value := os.Getenv(envName); value != "" {
		return value
	}
	return ""
}

// GetInputInt gets an integer input.
func GetInputInt(name string) (int, error) {
	value := GetInput(name)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

// GetInputBool gets a boolean input.
func GetInputBool(name string) bool {
	value := strings.ToLower(GetInput(name))
	return value == "true" || value == "yes" || value == "1"
}

// GetInputDuration gets a duration input.
func GetInputDuration(name string) (time.Duration, error) {
	value := GetInput(name)
	if value == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}

// CloseIssueInput contains inputs for the close-issue action.
type CloseIssueInput struct {
	IssueNumber int
	Action      string // "closed" or "reopened"
}

// CloseIssueOutput contains outputs from the close-issue action.
type CloseIssueOutput struct {
	TagDeleted string // Tag that was deleted, if any
	Status     string // Result status
}

// CloseIssue handles the closing of an approval issue.
// This is triggered by the 'issues' event with action 'closed'.
// If on_closed.delete_tag is true and a tag was created, it will be deleted.
func (h *Handler) CloseIssue(ctx context.Context, input CloseIssueInput) (*CloseIssueOutput, error) {
	output := &CloseIssueOutput{
		Status: "processed",
	}

	// Only process close events
	if input.Action != "closed" {
		output.Status = "skipped"
		return output, nil
	}

	// Get the issue
	issue, err := h.client.GetIssue(ctx, input.IssueNumber)
	if err != nil {
		return nil, err
	}

	// Parse state from issue body
	state, err := ParseIssueState(issue.Body)
	if err != nil {
		// Issue wasn't created by us, skip
		output.Status = "skipped"
		return output, nil
	}

	workflow, err := h.config.GetWorkflow(state.Workflow)
	if err != nil {
		return nil, err
	}

	// Check if we should delete the tag
	if workflow.OnClosed.DeleteTag && state.Tag != "" {
		// Only delete if the issue was closed without proper approval
		// (i.e., state.ApprovedAt is empty means it wasn't approved before closing)
		// Some teams want to delete tags even if approved, so we check for any tag
		err := h.client.DeleteTag(ctx, state.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to delete tag %s: %w", state.Tag, err)
		}
		output.TagDeleted = state.Tag
		output.Status = "tag_deleted"

		// Post comment if configured
		if workflow.OnClosed.Comment != "" {
			comment := ReplaceTemplateVars(workflow.OnClosed.Comment, map[string]string{
				"tag":     state.Tag,
				"version": state.Version,
			})
			_ = h.client.CreateComment(ctx, input.IssueNumber, comment)
		}
	} else if workflow.OnClosed.Comment != "" {
		// Just post the close comment if configured
		comment := ReplaceTemplateVars(workflow.OnClosed.Comment, map[string]string{
			"tag":     state.Tag,
			"version": state.Version,
		})
		_ = h.client.CreateComment(ctx, input.IssueNumber, comment)
	}

	return output, nil
}
