package action

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/issueops/approvals/internal/approval"
	"github.com/issueops/approvals/internal/config"
)

// PipelineProcessor handles progressive deployment pipelines.
type PipelineProcessor struct {
	handler *Handler
}

// NewPipelineProcessor creates a new pipeline processor.
func NewPipelineProcessor(handler *Handler) *PipelineProcessor {
	return &PipelineProcessor{handler: handler}
}

// ProcessPipelineApproval processes an approval for a pipeline workflow.
// Returns the updated state and whether the pipeline is complete.
func (p *PipelineProcessor) ProcessPipelineApproval(
	ctx context.Context,
	state *IssueState,
	workflow *config.Workflow,
	approver string,
) (*PipelineResult, error) {
	if !workflow.IsPipeline() {
		return nil, fmt.Errorf("workflow is not a pipeline")
	}

	pipeline := workflow.Pipeline
	currentStage := state.CurrentStage

	// Check if pipeline is already complete
	if currentStage >= len(pipeline.Stages) {
		return &PipelineResult{
			Complete:     true,
			StageMessage: "Pipeline already complete",
		}, nil
	}

	stage := pipeline.Stages[currentStage]

	// Record the stage completion
	completion := StageCompletion{
		Stage:      stage.Name,
		ApprovedBy: approver,
		ApprovedAt: time.Now().UTC().Format(time.RFC3339),
	}
	state.StageHistory = append(state.StageHistory, completion)

	// Move to next stage
	state.CurrentStage++

	result := &PipelineResult{
		StageName:     stage.Name,
		StageIndex:    currentStage,
		ApprovedBy:    approver,
		StageMessage:  stage.OnApproved,
		CreateTag:     stage.CreateTag,
		Complete:      state.CurrentStage >= len(pipeline.Stages) || stage.IsFinal,
		NextStage:     "",
		NextApprovers: nil,
	}

	// If not complete, get next stage info
	if !result.Complete && state.CurrentStage < len(pipeline.Stages) {
		nextStage := pipeline.Stages[state.CurrentStage]
		result.NextStage = nextStage.Name
		result.NextApprovers = p.getStageApprovers(nextStage)
	}

	return result, nil
}

// GetCurrentStageRequirement returns the approval requirement for the current pipeline stage.
func (p *PipelineProcessor) GetCurrentStageRequirement(
	state *IssueState,
	workflow *config.Workflow,
) (*config.Requirement, error) {
	if !workflow.IsPipeline() {
		return nil, fmt.Errorf("workflow is not a pipeline")
	}

	pipeline := workflow.Pipeline
	if state.CurrentStage >= len(pipeline.Stages) {
		return nil, fmt.Errorf("pipeline already complete")
	}

	stage := pipeline.Stages[state.CurrentStage]

	// Build requirement from stage config
	req := &config.Requirement{}
	if stage.Policy != "" {
		req.Policy = stage.Policy
	} else if len(stage.Approvers) > 0 {
		req.Approvers = stage.Approvers
	}

	return req, nil
}

// getStageApprovers returns the list of approvers for a stage.
func (p *PipelineProcessor) getStageApprovers(stage config.PipelineStage) []string {
	if len(stage.Approvers) > 0 {
		return stage.Approvers
	}
	if stage.Policy != "" {
		policy, ok := p.handler.config.Policies[stage.Policy]
		if ok {
			return policy.Approvers
		}
	}
	return nil
}

// GeneratePipelineTable generates a markdown table showing pipeline status.
func GeneratePipelineTable(state *IssueState, pipeline *config.PipelineConfig) string {
	if pipeline == nil || len(pipeline.Stages) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("| Stage | Status | Approver | Time |\n")
	sb.WriteString("|-------|--------|----------|------|\n")

	completedMap := make(map[string]StageCompletion)
	for _, c := range state.StageHistory {
		completedMap[c.Stage] = c
	}

	for i, stage := range pipeline.Stages {
		status := "⬜ Pending"
		approver := "-"
		timestamp := "-"

		if completion, ok := completedMap[stage.Name]; ok {
			status = "✅ Deployed"
			approver = "@" + completion.ApprovedBy
			if t, err := time.Parse(time.RFC3339, completion.ApprovedAt); err == nil {
				timestamp = t.Format("Jan 2 15:04")
			}
		} else if i == state.CurrentStage {
			status = "⏳ Awaiting"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			strings.ToUpper(stage.Name), status, approver, timestamp))
	}

	return sb.String()
}

// GeneratePRTable generates a markdown table showing PRs in the release.
func GeneratePRTable(prs []PRInfo) string {
	if len(prs) == 0 {
		return "_No PRs in this release_"
	}

	var sb strings.Builder
	sb.WriteString("| PR | Title | Author |\n")
	sb.WriteString("|----|-------|--------|\n")

	for _, pr := range prs {
		sb.WriteString(fmt.Sprintf("| [#%d](%s) | %s | @%s |\n",
			pr.Number, pr.URL, pr.Title, pr.Author))
	}

	return sb.String()
}

// GenerateCommitList generates a markdown list of commits.
func GenerateCommitList(commits []CommitInfo) string {
	if len(commits) == 0 {
		return "_No commits_"
	}

	var sb strings.Builder
	for _, c := range commits {
		shortSHA := c.SHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		// Truncate message to first line
		msg := c.Message
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		sb.WriteString(fmt.Sprintf("- [`%s`](%s) %s\n", shortSHA, c.URL, msg))
	}

	return sb.String()
}

// PipelineResult contains the result of processing a pipeline approval.
type PipelineResult struct {
	StageName     string   // Name of the stage that was approved
	StageIndex    int      // Index of the stage
	ApprovedBy    string   // Who approved
	StageMessage  string   // Message to post for this stage
	CreateTag     bool     // Whether to create a tag at this stage
	Complete      bool     // Whether the pipeline is complete
	NextStage     string   // Name of the next stage (if not complete)
	NextApprovers []string // Approvers for the next stage
}

// EvaluatePipelineStage evaluates whether the current user can approve the current stage.
func (p *PipelineProcessor) EvaluatePipelineStage(
	ctx context.Context,
	state *IssueState,
	workflow *config.Workflow,
	comments []approval.Comment,
) (*approval.ApprovalResult, error) {
	if !workflow.IsPipeline() {
		return nil, fmt.Errorf("workflow is not a pipeline")
	}

	pipeline := workflow.Pipeline
	if state.CurrentStage >= len(pipeline.Stages) {
		return &approval.ApprovalResult{
			Status: approval.StatusApproved,
		}, nil
	}

	stage := pipeline.Stages[state.CurrentStage]

	// Build a temporary workflow with just the current stage's requirements
	tempWorkflow := &config.Workflow{
		Require: []config.Requirement{},
	}

	if stage.Policy != "" {
		tempWorkflow.Require = append(tempWorkflow.Require, config.Requirement{
			Policy: stage.Policy,
		})
	} else if len(stage.Approvers) > 0 {
		tempWorkflow.Require = append(tempWorkflow.Require, config.Requirement{
			Approvers: stage.Approvers,
		})
	}

	// Create a request for this stage
	req := &approval.Request{
		Config:    p.handler.config,
		Workflow:  tempWorkflow,
		Requestor: state.Requestor,
		Comments:  comments,
	}

	// Create engine and evaluate
	engine := approval.NewEngine(p.handler.config.Defaults.AllowSelfApproval, nil)
	return engine.Evaluate(req)
}

// GeneratePipelineIssueBody generates the issue body for a pipeline workflow.
func GeneratePipelineIssueBody(data *TemplateData, state *IssueState, pipeline *config.PipelineConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 🚀 Deployment Pipeline: %s\n\n", data.Version))
	sb.WriteString(fmt.Sprintf("%s\n\n", data.Description))

	// Pipeline status table
	sb.WriteString("### Deployment Progress\n\n")
	sb.WriteString(GeneratePipelineTable(state, pipeline))
	sb.WriteString("\n")

	// Current stage info
	if state.CurrentStage < len(pipeline.Stages) {
		stage := pipeline.Stages[state.CurrentStage]
		sb.WriteString(fmt.Sprintf("**Current Stage:** %s\n\n", strings.ToUpper(stage.Name)))
		sb.WriteString("**To approve this stage:** Comment `approve`, `lgtm`, or `yes`\n\n")
	}

	sb.WriteString("---\n\n")

	// Request info
	sb.WriteString("### Request Information\n\n")
	sb.WriteString(fmt.Sprintf("- **Requested by:** @%s\n", data.Requestor))
	sb.WriteString(fmt.Sprintf("- **Version:** `%s`\n", data.Version))
	if data.Branch != "" {
		sb.WriteString(fmt.Sprintf("- **Branch:** `%s`\n", data.Branch))
	}
	if data.CommitSHA != "" {
		shortSHA := data.CommitSHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		sb.WriteString(fmt.Sprintf("- **Commit:** [%s](%s)\n", shortSHA, data.CommitURL))
	}
	sb.WriteString(fmt.Sprintf("- **Requested at:** %s\n", data.CreatedAt))
	sb.WriteString("\n")

	// PRs in release
	if len(state.PRs) > 0 {
		sb.WriteString("### Pull Requests in this Release\n\n")
		sb.WriteString(GeneratePRTable(state.PRs))
		sb.WriteString("\n")
	}

	// Commits in release
	if len(state.Commits) > 0 {
		sb.WriteString("### Commits\n\n")
		sb.WriteString(GenerateCommitList(state.Commits))
		sb.WriteString("\n")
	}

	// Append the hidden state marker (required for ProcessComment to work)
	stateJSON, err := json.Marshal(state)
	if err != nil {
		// Fall back without state if marshal fails
		return sb.String()
	}
	sb.WriteString("\n<!-- issueops-state:")
	sb.WriteString(string(stateJSON))
	sb.WriteString(" -->")

	return sb.String()
}
