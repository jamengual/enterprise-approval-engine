package action

import (
	"context"
	"fmt"

	"github.com/issueops/approvals/internal/config"
	"github.com/issueops/approvals/internal/github"
)

// ReleaseTracker fetches release candidates based on the configured strategy.
type ReleaseTracker struct {
	client   *github.Client
	strategy config.ReleaseStrategyConfig
	version  string
}

// ReleaseContents holds the PRs and commits for a release.
type ReleaseContents struct {
	PRs     []github.PullRequest
	Commits []github.Commit
	// Metadata about how the release was identified
	StrategyType config.ReleaseStrategyType
	Identifier   string // branch name, label, milestone title, or tag range
}

// NewReleaseTracker creates a new release tracker.
func NewReleaseTracker(client *github.Client, strategy config.ReleaseStrategyConfig, version string) *ReleaseTracker {
	return &ReleaseTracker{
		client:   client,
		strategy: strategy,
		version:  version,
	}
}

// GetReleaseContents fetches the PRs and commits for the release based on strategy.
func (r *ReleaseTracker) GetReleaseContents(ctx context.Context, previousTag string) (*ReleaseContents, error) {
	switch r.strategy.GetType() {
	case config.StrategyBranch:
		return r.getByBranch(ctx)
	case config.StrategyLabel:
		return r.getByLabel(ctx)
	case config.StrategyMilestone:
		return r.getByMilestone(ctx)
	case config.StrategyTag:
		fallthrough
	default:
		return r.getByTag(ctx, previousTag)
	}
}

// getByTag fetches release contents between two tags (default behavior).
func (r *ReleaseTracker) getByTag(ctx context.Context, previousTag string) (*ReleaseContents, error) {
	contents := &ReleaseContents{
		StrategyType: config.StrategyTag,
		Identifier:   fmt.Sprintf("%s...%s", previousTag, r.version),
	}

	if previousTag == "" {
		return contents, nil
	}

	// Get PRs between tags
	prs, err := r.client.GetMergedPRsBetween(ctx, previousTag, "HEAD")
	if err != nil {
		// Don't fail on PR fetch errors - may be permissions
		prs = nil
	}
	contents.PRs = prs

	// Get commits between tags
	commits, err := r.client.CompareCommits(ctx, previousTag, "HEAD")
	if err != nil {
		commits = nil
	}
	contents.Commits = commits

	return contents, nil
}

// getByBranch fetches release contents from a release branch.
func (r *ReleaseTracker) getByBranch(ctx context.Context) (*ReleaseContents, error) {
	branchName := r.strategy.FormatBranchName(r.version)
	baseBranch := r.strategy.GetBaseBranch()

	contents := &ReleaseContents{
		StrategyType: config.StrategyBranch,
		Identifier:   branchName,
	}

	// Check if branch exists
	branch, err := r.client.GetBranch(ctx, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to check branch %q: %w", branchName, err)
	}
	if branch == nil {
		// Branch doesn't exist yet - that's OK, it might be created later
		return contents, nil
	}

	// Get PRs merged to the release branch
	prs, err := r.client.GetPRsMergedToBranch(ctx, branchName)
	if err != nil {
		// Don't fail on PR fetch errors
		prs = nil
	}
	contents.PRs = prs

	// Get commits between base and release branch
	commits, err := r.client.GetCommitsBetweenBranches(ctx, baseBranch, branchName)
	if err != nil {
		commits = nil
	}
	contents.Commits = commits

	return contents, nil
}

// getByLabel fetches release contents by PR label.
func (r *ReleaseTracker) getByLabel(ctx context.Context) (*ReleaseContents, error) {
	labelName := r.strategy.FormatLabelName(r.version)

	contents := &ReleaseContents{
		StrategyType: config.StrategyLabel,
		Identifier:   labelName,
	}

	// Get PRs with the release label
	prs, err := r.client.GetPRsByLabel(ctx, labelName)
	if err != nil {
		// Don't fail on PR fetch errors
		return contents, nil
	}
	contents.PRs = prs

	// For commits, we'd need to aggregate from all the PRs
	// This is more complex, so we'll just use the PR list for now
	// The commit information is available via the PR merge commits

	return contents, nil
}

// getByMilestone fetches release contents by milestone.
func (r *ReleaseTracker) getByMilestone(ctx context.Context) (*ReleaseContents, error) {
	milestoneName := r.strategy.FormatMilestoneName(r.version)

	contents := &ReleaseContents{
		StrategyType: config.StrategyMilestone,
		Identifier:   milestoneName,
	}

	// Find the milestone
	milestone, err := r.client.GetMilestoneByTitle(ctx, milestoneName)
	if err != nil {
		return nil, fmt.Errorf("failed to find milestone %q: %w", milestoneName, err)
	}
	if milestone == nil {
		// Milestone doesn't exist yet
		return contents, nil
	}

	// Get PRs in the milestone
	prs, err := r.client.GetPRsByMilestone(ctx, milestone.Number)
	if err != nil {
		// Don't fail on PR fetch errors
		return contents, nil
	}
	contents.PRs = prs

	return contents, nil
}

// CreateNextReleaseArtifact creates the artifact for the next release version.
// This is called when the final stage (prod) is approved.
func (r *ReleaseTracker) CreateNextReleaseArtifact(ctx context.Context, nextVersion string) error {
	switch r.strategy.GetType() {
	case config.StrategyBranch:
		return r.createNextBranch(ctx, nextVersion)
	case config.StrategyLabel:
		return r.createNextLabel(ctx, nextVersion)
	case config.StrategyMilestone:
		return r.createNextMilestone(ctx, nextVersion)
	case config.StrategyTag:
		// Tags are created by the pipeline itself, nothing to do here
		return nil
	default:
		return nil
	}
}

// createNextBranch creates a new release branch for the next version.
func (r *ReleaseTracker) createNextBranch(ctx context.Context, nextVersion string) error {
	branchName := r.strategy.FormatBranchName(nextVersion)
	baseBranch := r.strategy.GetBaseBranch()

	// Check if branch already exists
	existing, err := r.client.GetBranch(ctx, branchName)
	if err != nil {
		return fmt.Errorf("failed to check existing branch: %w", err)
	}
	if existing != nil {
		return nil // Branch already exists
	}

	_, err = r.client.CreateBranch(ctx, branchName, baseBranch)
	if err != nil {
		return fmt.Errorf("failed to create release branch %q: %w", branchName, err)
	}

	return nil
}

// createNextLabel creates a new release label for the next version.
func (r *ReleaseTracker) createNextLabel(ctx context.Context, nextVersion string) error {
	labelName := r.strategy.FormatLabelName(nextVersion)

	// Create the label (will be no-op if exists)
	err := r.client.CreateLabel(ctx, labelName, "0e8a16", fmt.Sprintf("Release %s", nextVersion))
	if err != nil {
		return fmt.Errorf("failed to create release label %q: %w", labelName, err)
	}

	return nil
}

// createNextMilestone creates a new milestone for the next version.
func (r *ReleaseTracker) createNextMilestone(ctx context.Context, nextVersion string) error {
	milestoneName := r.strategy.FormatMilestoneName(nextVersion)

	// Check if milestone already exists
	existing, err := r.client.GetMilestoneByTitle(ctx, milestoneName)
	if err != nil {
		return fmt.Errorf("failed to check existing milestone: %w", err)
	}
	if existing != nil {
		return nil // Milestone already exists
	}

	_, err = r.client.CreateMilestone(ctx, milestoneName, fmt.Sprintf("Release %s", nextVersion))
	if err != nil {
		return fmt.Errorf("failed to create milestone %q: %w", milestoneName, err)
	}

	return nil
}

// CleanupCurrentRelease handles cleanup after a successful release.
// This includes closing milestones, removing labels, or deleting branches as configured.
func (r *ReleaseTracker) CleanupCurrentRelease(ctx context.Context, prs []github.PullRequest) error {
	switch r.strategy.GetType() {
	case config.StrategyBranch:
		if r.strategy.Branch.DeleteAfterRelease {
			branchName := r.strategy.FormatBranchName(r.version)
			return r.client.DeleteBranch(ctx, branchName)
		}
	case config.StrategyLabel:
		if r.strategy.Label.RemoveAfterRelease {
			labelName := r.strategy.FormatLabelName(r.version)
			for _, pr := range prs {
				_ = r.client.RemoveLabelFromPR(ctx, pr.Number, labelName)
			}
		}
	case config.StrategyMilestone:
		if r.strategy.Milestone.CloseAfterRelease {
			milestoneName := r.strategy.FormatMilestoneName(r.version)
			milestone, err := r.client.GetMilestoneByTitle(ctx, milestoneName)
			if err == nil && milestone != nil {
				return r.client.CloseMilestone(ctx, milestone.Number)
			}
		}
	}
	return nil
}
