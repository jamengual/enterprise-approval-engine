package action

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jamengual/enterprise-approval-engine/internal/approval"
	"github.com/jamengual/enterprise-approval-engine/internal/config"
	"github.com/jamengual/enterprise-approval-engine/internal/github"
	"github.com/jamengual/enterprise-approval-engine/internal/semver"
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

	// For pipeline workflows, fetch PR/commit tracking data
	var body string
	if workflow.IsPipeline() {
		body, err = h.generatePipelineIssueBody(ctx, &templateData, workflow)
	} else {
		body, err = GenerateIssueBodyFromConfig(templateData, &workflow.Issue)
	}
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

	// Create sub-issues if the workflow uses sub-issue approval mode
	if workflow.IsPipeline() && workflow.UsesSubIssues() {
		subHandler := NewSubIssueHandler(h.client, h.config, workflow)
		subIssues, err := subHandler.CreateSubIssuesForPipeline(ctx, issue.Number, &templateData.State, workflow.Pipeline)
		if err != nil {
			// Log error but don't fail - the parent issue is already created
			_ = h.client.CreateComment(ctx, issue.Number,
				fmt.Sprintf("**Warning:** Failed to create sub-issues for approval stages: %v\n\nPlease use comment-based approval instead.", err))
		} else if len(subIssues) > 0 {
			// Update the parent issue with sub-issue information
			templateData.State.SubIssues = subIssues
			templateData.State.ApprovalMode = string(workflow.GetApprovalMode())

			// Regenerate and update the issue body with sub-issue links
			updatedBody := GeneratePipelineIssueBodyWithSubIssues(&templateData, &templateData.State, workflow.Pipeline, subIssues)
			_ = h.client.UpdateIssueBody(ctx, issue.Number, updatedBody)
		}
	}

	return &RequestOutput{
		IssueNumber: issue.Number,
		IssueURL:    issue.HTMLURL,
	}, nil
}

// generatePipelineIssueBody creates an issue body for pipeline workflows with PR/commit tracking.
func (h *Handler) generatePipelineIssueBody(ctx context.Context, data *TemplateData, workflow *config.Workflow) (string, error) {
	pipeline := workflow.Pipeline

	// Initialize pipeline state
	var stageNames []string
	for _, stage := range pipeline.Stages {
		stageNames = append(stageNames, stage.Name)
	}
	data.State.Pipeline = stageNames
	data.State.CurrentStage = 0

	// Auto-advance through initial stages marked with auto_approve: true
	processor := NewPipelineProcessor(h)
	autoApproved := processor.ProcessInitialAutoApproveStages(&data.State, pipeline)

	// Store auto-approved stages for display/logging
	data.State.AutoApprovedStages = autoApproved

	// Store the release strategy type for display
	data.State.ReleaseStrategy = string(pipeline.ReleaseStrategy.GetType())

	// Fetch PR and commit data based on release strategy
	if pipeline.TrackPRs || pipeline.TrackCommits {
		// Use the ReleaseTracker to get content based on strategy
		tracker := NewReleaseTracker(h.client, pipeline.ReleaseStrategy, data.Version)

		// For tag strategy, we need the previous tag
		previousTag := ""
		if pipeline.ReleaseStrategy.GetType() == config.StrategyTag || pipeline.ReleaseStrategy.GetType() == "" {
			if data.Version != "" {
				prevTag, err := h.client.GetPreviousTag(ctx, data.Version)
				if err == nil && prevTag != "" {
					previousTag = prevTag
				}
			}
			// If no previous tag from version, try to get latest tag
			if previousTag == "" {
				tags, err := h.client.ListTags(ctx, 1)
				if err == nil && len(tags) > 0 {
					previousTag = tags[0]
				}
			}
		}
		data.State.PreviousTag = previousTag
		data.PreviousVersion = previousTag

		// Get release contents using the configured strategy
		contents, err := tracker.GetReleaseContents(ctx, previousTag)
		if err == nil && contents != nil {
			// Convert PRs to state format
			if pipeline.TrackPRs {
				for _, pr := range contents.PRs {
					data.State.PRs = append(data.State.PRs, PRInfo{
						Number: pr.Number,
						Title:  pr.Title,
						Author: pr.Author,
						URL:    pr.URL,
					})
				}
			}

			// Convert commits to state format
			if pipeline.TrackCommits {
				for _, c := range contents.Commits {
					data.State.Commits = append(data.State.Commits, CommitInfo{
						SHA:     c.SHA,
						Message: c.Message,
						Author:  c.Author,
						URL:     c.URL,
					})
				}
				data.CommitsCount = len(contents.Commits)
			}

			// Store release identifier for display
			data.State.ReleaseIdentifier = contents.Identifier
		}
	}

	// Generate the pipeline-specific issue body
	return GeneratePipelineIssueBody(data, &data.State, pipeline), nil
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

// ReactionType defines the type of reaction to add to a comment.
type ReactionType string

const (
	ReactionEyes     ReactionType = "eyes"     // 👀 - processing/seen
	ReactionApproved ReactionType = "+1"       // 👍 - approved
	ReactionDenied   ReactionType = "-1"       // 👎 - denied
	ReactionConfused ReactionType = "confused" // 😕 - not authorized
	ReactionRocket   ReactionType = "rocket"   // 🚀 - deployed
)

// addReactionToComment adds an emoji reaction to a comment if configured.
func (h *Handler) addReactionToComment(ctx context.Context, commentID int64, reaction ReactionType, settings *config.CommentSettings) {
	if settings.ShouldReactToComments() {
		_ = h.client.AddReaction(ctx, commentID, string(reaction))
	}
}

// addCommentReaction adds an appropriate reaction based on the approval result.
func (h *Handler) addCommentReaction(ctx context.Context, commentID int64, result *approval.ApprovalResult, settings *config.CommentSettings) {
	if !settings.ShouldReactToComments() {
		return
	}

	switch result.Status {
	case approval.StatusApproved:
		_ = h.client.AddReaction(ctx, commentID, string(ReactionApproved))
	case approval.StatusDenied:
		_ = h.client.AddReaction(ctx, commentID, string(ReactionDenied))
	case approval.StatusPending:
		// For pending, add eyes emoji to indicate the comment was seen
		// but only if it contained an approval/denial keyword from an unauthorized user
		_ = h.client.AddReaction(ctx, commentID, string(ReactionEyes))
	}
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

	// Check if this is a pipeline workflow
	if workflow.IsPipeline() {
		return h.processPipelineComment(ctx, input, issue, state, workflow)
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

	// Add emoji reaction to the comment based on result
	h.addCommentReaction(ctx, input.CommentID, result, workflow.CommentSettings)

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

// processPipelineComment handles approval comments for pipeline workflows.
func (h *Handler) processPipelineComment(
	ctx context.Context,
	input ProcessCommentInput,
	issue *github.Issue,
	state *IssueState,
	workflow *config.Workflow,
) (*ProcessCommentOutput, error) {
	pipeline := workflow.Pipeline

	// Check if pipeline is already complete
	if state.CurrentStage >= len(pipeline.Stages) {
		return &ProcessCommentOutput{
			Status: "approved",
		}, nil
	}

	// Create pipeline processor
	processor := NewPipelineProcessor(h)

	// Get all comments and evaluate current stage
	comments, err := h.client.ListComments(ctx, input.IssueNumber)
	if err != nil {
		return nil, err
	}

	result, err := processor.EvaluatePipelineStage(ctx, state, workflow, convertComments(comments))
	if err != nil {
		return nil, err
	}

	output := &ProcessCommentOutput{
		Status:    string(result.Status),
		Approvers: extractApprovers(result.Approvals),
		Denier:    result.Denier,
	}

	// Add emoji reaction to the comment based on result
	h.addCommentReaction(ctx, input.CommentID, result, workflow.CommentSettings)

	// If current stage is approved, advance the pipeline
	if result.Status == approval.StatusApproved {
		// Get the latest approver from the comments
		latestApprover := input.CommentUser

		// Process the stage approval
		pipelineResult, err := processor.ProcessPipelineApproval(ctx, state, workflow, latestApprover)
		if err != nil {
			return nil, err
		}

		// Post stage completion comment
		if pipelineResult.StageMessage != "" {
			_ = h.client.CreateComment(ctx, input.IssueNumber, pipelineResult.StageMessage)
		}

		// Create tag if this stage requires it
		if pipelineResult.CreateTag && state.Version != "" {
			tagName := state.Version
			exists, err := h.client.TagExists(ctx, tagName)
			if err == nil && !exists {
				_, tagErr := h.client.CreateTag(ctx, github.CreateTagOptions{
					Name:    tagName,
					Message: fmt.Sprintf("Release %s - approved via IssueOps pipeline", tagName),
				})
				if tagErr == nil {
					output.Tag = tagName
					state.Tag = tagName
				}
			}
		}

		// Update issue body with new state and progress table
		updatedBody := regeneratePipelineIssueBody(issue.Body, state, pipeline)
		if updatedBody != "" {
			_ = h.client.UpdateIssueBody(ctx, input.IssueNumber, updatedBody)
		}

		// Check if pipeline is complete
		if pipelineResult.Complete {
			output.Status = "approved"

			// Handle release strategy cleanup and auto-creation
			if pipeline.ReleaseStrategy.GetType() != "" && pipeline.ReleaseStrategy.GetType() != config.StrategyTag {
				tracker := NewReleaseTracker(h.client, pipeline.ReleaseStrategy, state.Version)

				// Cleanup current release (close milestone, remove labels, delete branch)
				var prs []github.PullRequest
				for _, pr := range state.PRs {
					prs = append(prs, github.PullRequest{
						Number: pr.Number,
						Title:  pr.Title,
						Author: pr.Author,
						URL:    pr.URL,
					})
				}
				_ = tracker.CleanupCurrentRelease(ctx, prs)

				// Auto-create next release artifact if configured
				if pipeline.ReleaseStrategy.IsAutoCreateEnabled() {
					nextVersion := calculateNextVersion(state.Version, pipeline.ReleaseStrategy.GetNextVersionStrategy())
					if nextVersion != "" {
						if err := tracker.CreateNextReleaseArtifact(ctx, nextVersion); err == nil {
							// Post comment about next release creation
							comment := pipeline.ReleaseStrategy.AutoCreate.Comment
							if comment == "" {
								comment = fmt.Sprintf("🚀 **Next release prepared:** %s\n\n", nextVersion)
								switch pipeline.ReleaseStrategy.GetType() {
								case config.StrategyBranch:
									comment += fmt.Sprintf("Created release branch: `%s`", pipeline.ReleaseStrategy.FormatBranchName(nextVersion))
								case config.StrategyLabel:
									comment += fmt.Sprintf("Created release label: `%s`", pipeline.ReleaseStrategy.FormatLabelName(nextVersion))
								case config.StrategyMilestone:
									comment += fmt.Sprintf("Created milestone: `%s`", pipeline.ReleaseStrategy.FormatMilestoneName(nextVersion))
								}
							}
							_ = h.client.CreateComment(ctx, input.IssueNumber, comment)

							// Optionally create a new approval issue for next release
							if pipeline.ReleaseStrategy.AutoCreate.CreateIssue {
								// Create new request for next version
								_, _ = h.Request(ctx, RequestInput{
									Workflow: state.Workflow,
									Version:  nextVersion,
								})
							}
						}
					}
				}
			}

			// Post final completion comment
			if workflow.OnApproved.Comment != "" {
				comment := ReplaceTemplateVars(workflow.OnApproved.Comment, map[string]string{
					"version": state.Version,
				})
				_ = h.client.CreateComment(ctx, input.IssueNumber, comment)
			}

			// Close issue if configured
			if workflow.OnApproved.CloseIssue {
				_ = h.client.CloseIssue(ctx, input.IssueNumber)
			}
		} else {
			// Pipeline continues - notify about next stage
			output.Status = "pending"
			output.SatisfiedGroup = pipelineResult.StageName

			if pipelineResult.NextStage != "" {
				nextStageComment := fmt.Sprintf("⏳ **Next stage:** %s\n\n**Awaiting approval from:** %s",
					strings.ToUpper(pipelineResult.NextStage),
					formatApproversList(pipelineResult.NextApprovers))
				_ = h.client.CreateComment(ctx, input.IssueNumber, nextStageComment)
			}
		}
	}

	// Handle denial
	if result.Status == approval.StatusDenied {
		if workflow.OnDenied.Comment != "" {
			comment := ReplaceTemplateVars(workflow.OnDenied.Comment, map[string]string{
				"denier": result.Denier,
			})
			_ = h.client.CreateComment(ctx, input.IssueNumber, comment)
		}

		if workflow.OnDenied.CloseIssue {
			_ = h.client.CloseIssue(ctx, input.IssueNumber)
		}
	}

	return output, nil
}

// regeneratePipelineIssueBody regenerates the full issue body with updated pipeline state.
func regeneratePipelineIssueBody(originalBody string, state *IssueState, pipeline *config.PipelineConfig) string {
	// Extract metadata from original body to preserve it
	// Build a minimal template data from state
	data := &TemplateData{
		Version:     state.Version,
		Requestor:   state.Requestor,
		Description: extractDescription(originalBody),
		Branch:      extractBranch(originalBody),
		CommitSHA:   extractCommitSHA(originalBody),
		CommitURL:   extractCommitURL(originalBody),
		State:       *state,
	}

	// Generate complete new body
	return GeneratePipelineIssueBody(data, state, pipeline)
}

// extractDescription extracts the description from the original body.
func extractDescription(body string) string {
	// Look for content after the title line
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Next non-empty line is description
			if i+1 < len(lines) && i+2 < len(lines) {
				desc := strings.TrimSpace(lines[i+2])
				if desc != "" && !strings.HasPrefix(desc, "#") && !strings.HasPrefix(desc, "|") {
					return desc
				}
			}
		}
	}
	return ""
}

// extractBranch extracts the branch from the original body.
func extractBranch(body string) string {
	if idx := strings.Index(body, "**Branch:** `"); idx != -1 {
		start := idx + len("**Branch:** `")
		end := strings.Index(body[start:], "`")
		if end != -1 {
			return body[start : start+end]
		}
	}
	return ""
}

// extractCommitSHA extracts the commit SHA from the original body.
func extractCommitSHA(body string) string {
	if idx := strings.Index(body, "**Commit:** ["); idx != -1 {
		start := idx + len("**Commit:** [")
		end := strings.Index(body[start:], "]")
		if end != -1 {
			return body[start : start+end]
		}
	}
	return ""
}

// extractCommitURL extracts the commit URL from the original body.
func extractCommitURL(body string) string {
	if idx := strings.Index(body, "**Commit:** ["); idx != -1 {
		urlStart := strings.Index(body[idx:], "](")
		if urlStart != -1 {
			urlStart += idx + 2
			urlEnd := strings.Index(body[urlStart:], ")")
			if urlEnd != -1 {
				return body[urlStart : urlStart+urlEnd]
			}
		}
	}
	return ""
}

// formatApproversList formats a list of approvers for display.
func formatApproversList(approvers []string) string {
	if len(approvers) == 0 {
		return "_configured approvers_"
	}

	var formatted []string
	for _, a := range approvers {
		if config.IsTeam(a) {
			formatted = append(formatted, fmt.Sprintf("@%s (team)", config.ParseTeam(a)))
		} else {
			formatted = append(formatted, "@"+a)
		}
	}
	return strings.Join(formatted, ", ")
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

// ProcessSubIssueCloseInput contains inputs for processing a sub-issue close event.
type ProcessSubIssueCloseInput struct {
	IssueNumber int
	ClosedBy    string
	Action      string // "closed" or "reopened"
}

// ProcessSubIssueCloseOutput contains outputs from processing a sub-issue close.
type ProcessSubIssueCloseOutput struct {
	ParentIssueNumber int
	StageName         string
	Status            string // "approved", "denied", "reopened", "unauthorized"
	PipelineComplete  bool
	NextStage         string
	Message           string
}

// ProcessSubIssueClose handles the close event for an approval sub-issue.
func (h *Handler) ProcessSubIssueClose(ctx context.Context, input ProcessSubIssueCloseInput) (*ProcessSubIssueCloseOutput, error) {
	// First, check if this issue is a sub-issue
	isSub, err := h.client.IsSubIssue(ctx, input.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to check if issue is sub-issue: %w", err)
	}
	if !isSub {
		// Not a sub-issue, skip processing
		return &ProcessSubIssueCloseOutput{
			Status: "skipped",
		}, nil
	}

	// Get parent issue to determine workflow
	parent, err := h.client.GetParentIssue(ctx, input.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue: %w", err)
	}
	if parent == nil {
		return &ProcessSubIssueCloseOutput{
			Status: "skipped",
		}, nil
	}

	// Get parent issue details
	parentIssue, err := h.client.GetIssue(ctx, parent.GetNumber())
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue details: %w", err)
	}

	// Parse state from parent issue
	state, err := ParseIssueState(parentIssue.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parent issue state: %w", err)
	}

	// Get workflow configuration
	workflow, err := h.config.GetWorkflow(state.Workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Create sub-issue handler and process the close
	subHandler := NewSubIssueHandler(h.client, h.config, workflow)
	result, err := subHandler.ProcessSubIssueClose(ctx, ProcessSubIssueCloseInput{
		IssueNumber: input.IssueNumber,
		ClosedBy:    input.ClosedBy,
		Action:      input.Action,
	})
	if err != nil {
		return nil, err
	}

	output := &ProcessSubIssueCloseOutput{
		ParentIssueNumber: result.ParentIssueNumber,
		StageName:         result.StageName,
		Status:            result.Status,
		PipelineComplete:  result.PipelineComplete,
		NextStage:         result.NextStage,
		Message:           result.Message,
	}

	// If pipeline is complete, handle workflow completion
	if result.PipelineComplete && result.Status == "approved" {
		// Post final completion comment
		if workflow.OnApproved.Comment != "" {
			comment := ReplaceTemplateVars(workflow.OnApproved.Comment, map[string]string{
				"version": state.Version,
			})
			_ = h.client.CreateComment(ctx, parent.GetNumber(), comment)
		}

		// Create tag if configured
		if workflow.OnApproved.CreateTag && state.Version != "" {
			tagName := state.Version
			exists, err := h.client.TagExists(ctx, tagName)
			if err == nil && !exists {
				_, _ = h.client.CreateTag(ctx, github.CreateTagOptions{
					Name:    tagName,
					Message: fmt.Sprintf("Release %s - approved via IssueOps sub-issues", tagName),
				})
			}
		}

		// Close parent issue if configured
		if workflow.OnApproved.CloseIssue {
			_ = h.client.CloseIssue(ctx, parent.GetNumber())
		}
	}

	return output, nil
}

// calculateNextVersion calculates the next version based on the current version and strategy.
func calculateNextVersion(currentVersion, strategy string) string {
	if currentVersion == "" {
		return ""
	}

	// Use the semver package for proper version incrementing
	nextVersion, err := semver.NextVersion(currentVersion, strategy)
	if err != nil {
		return ""
	}
	return nextVersion
}
