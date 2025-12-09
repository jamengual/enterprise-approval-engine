package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v57/github"
)

// DeploymentState represents the state of a deployment.
type DeploymentState string

const (
	DeploymentStatePending    DeploymentState = "pending"
	DeploymentStateQueued     DeploymentState = "queued"
	DeploymentStateInProgress DeploymentState = "in_progress"
	DeploymentStateSuccess    DeploymentState = "success"
	DeploymentStateFailure    DeploymentState = "failure"
	DeploymentStateError      DeploymentState = "error"
	DeploymentStateInactive   DeploymentState = "inactive"
)

// Deployment represents a GitHub deployment.
type Deployment struct {
	ID          int64  `json:"id"`
	SHA         string `json:"sha"`
	Ref         string `json:"ref"`
	Environment string `json:"environment"`
	Description string `json:"description"`
	Creator     string `json:"creator"`
	URL         string `json:"url"`
}

// DeploymentStatus represents the status of a deployment.
type DeploymentStatus struct {
	ID          int64           `json:"id"`
	State       DeploymentState `json:"state"`
	Description string          `json:"description"`
	LogURL      string          `json:"log_url"`
	Environment string          `json:"environment"`
}

// CreateDeploymentOptions contains options for creating a deployment.
type CreateDeploymentOptions struct {
	Ref              string   // Git ref (branch, tag, SHA)
	Environment      string   // Target environment (e.g., "production", "staging")
	Description      string   // Description of the deployment
	AutoMerge        bool     // Auto-merge the default branch into the ref
	RequiredContexts []string // Required status contexts (empty = no required contexts)
	Payload          string   // JSON payload with extra information
	TransientEnv     bool     // Mark environment as transient (will be destroyed)
	ProductionEnv    bool     // Mark as production environment
}

// CreateDeployment creates a new deployment.
func (c *Client) CreateDeployment(ctx context.Context, opts CreateDeploymentOptions) (*Deployment, error) {
	// Use empty slice to skip all status checks
	requiredContexts := opts.RequiredContexts
	if requiredContexts == nil {
		requiredContexts = []string{}
	}

	req := &github.DeploymentRequest{
		Ref:              github.String(opts.Ref),
		Environment:      github.String(opts.Environment),
		Description:      github.String(opts.Description),
		AutoMerge:        github.Bool(opts.AutoMerge),
		RequiredContexts: &requiredContexts,
	}

	if opts.Payload != "" {
		req.Payload = opts.Payload
	}

	if opts.TransientEnv {
		req.TransientEnvironment = github.Bool(true)
	}

	if opts.ProductionEnv {
		req.ProductionEnvironment = github.Bool(true)
	}

	deployment, _, err := c.client.Repositories.CreateDeployment(ctx, c.owner, c.repo, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	return &Deployment{
		ID:          deployment.GetID(),
		SHA:         deployment.GetSHA(),
		Ref:         deployment.GetRef(),
		Environment: deployment.GetEnvironment(),
		Description: deployment.GetDescription(),
		Creator:     deployment.GetCreator().GetLogin(),
		URL:         deployment.GetURL(),
	}, nil
}

// CreateDeploymentStatusOptions contains options for creating a deployment status.
type CreateDeploymentStatusOptions struct {
	DeploymentID   int64
	State          DeploymentState
	Description    string
	LogURL         string // URL to deployment logs or approval issue
	Environment    string
	EnvironmentURL string // URL to the deployed environment
	AutoInactive   bool   // Mark previous deployments as inactive
}

// CreateDeploymentStatus creates a status for a deployment.
func (c *Client) CreateDeploymentStatus(ctx context.Context, opts CreateDeploymentStatusOptions) (*DeploymentStatus, error) {
	req := &github.DeploymentStatusRequest{
		State:       github.String(string(opts.State)),
		Description: github.String(opts.Description),
	}

	if opts.LogURL != "" {
		req.LogURL = github.String(opts.LogURL)
	}

	if opts.Environment != "" {
		req.Environment = github.String(opts.Environment)
	}

	if opts.EnvironmentURL != "" {
		req.EnvironmentURL = github.String(opts.EnvironmentURL)
	}

	req.AutoInactive = github.Bool(opts.AutoInactive)

	status, _, err := c.client.Repositories.CreateDeploymentStatus(ctx, c.owner, c.repo, opts.DeploymentID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment status: %w", err)
	}

	return &DeploymentStatus{
		ID:          status.GetID(),
		State:       DeploymentState(status.GetState()),
		Description: status.GetDescription(),
		LogURL:      status.GetLogURL(),
		Environment: status.GetEnvironment(),
	}, nil
}

// GetDeployment retrieves a deployment by ID.
func (c *Client) GetDeployment(ctx context.Context, deploymentID int64) (*Deployment, error) {
	deployment, _, err := c.client.Repositories.GetDeployment(ctx, c.owner, c.repo, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %d: %w", deploymentID, err)
	}

	return &Deployment{
		ID:          deployment.GetID(),
		SHA:         deployment.GetSHA(),
		Ref:         deployment.GetRef(),
		Environment: deployment.GetEnvironment(),
		Description: deployment.GetDescription(),
		Creator:     deployment.GetCreator().GetLogin(),
		URL:         deployment.GetURL(),
	}, nil
}

// ListDeployments lists deployments for the repository.
func (c *Client) ListDeployments(ctx context.Context, environment string, ref string) ([]Deployment, error) {
	opts := &github.DeploymentsListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	if environment != "" {
		opts.Environment = environment
	}
	if ref != "" {
		opts.Ref = ref
	}

	deployments, _, err := c.client.Repositories.ListDeployments(ctx, c.owner, c.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var result []Deployment
	for _, d := range deployments {
		result = append(result, Deployment{
			ID:          d.GetID(),
			SHA:         d.GetSHA(),
			Ref:         d.GetRef(),
			Environment: d.GetEnvironment(),
			Description: d.GetDescription(),
			Creator:     d.GetCreator().GetLogin(),
			URL:         d.GetURL(),
		})
	}

	return result, nil
}

// GetLatestDeploymentStatus gets the latest status for a deployment.
func (c *Client) GetLatestDeploymentStatus(ctx context.Context, deploymentID int64) (*DeploymentStatus, error) {
	statuses, _, err := c.client.Repositories.ListDeploymentStatuses(ctx, c.owner, c.repo, deploymentID, &github.ListOptions{PerPage: 1})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployment statuses: %w", err)
	}

	if len(statuses) == 0 {
		return nil, nil
	}

	s := statuses[0]
	return &DeploymentStatus{
		ID:          s.GetID(),
		State:       DeploymentState(s.GetState()),
		Description: s.GetDescription(),
		LogURL:      s.GetLogURL(),
		Environment: s.GetEnvironment(),
	}, nil
}

// GetEnvironmentDeploymentStatus returns the status of the latest deployment to an environment.
func (c *Client) GetEnvironmentDeploymentStatus(ctx context.Context, environment string) (*Deployment, *DeploymentStatus, error) {
	deployments, err := c.ListDeployments(ctx, environment, "")
	if err != nil {
		return nil, nil, err
	}

	if len(deployments) == 0 {
		return nil, nil, nil
	}

	latest := deployments[0]
	status, err := c.GetLatestDeploymentStatus(ctx, latest.ID)
	if err != nil {
		return &latest, nil, err
	}

	return &latest, status, nil
}

// DeploymentStateEmoji returns an emoji for a deployment state.
func DeploymentStateEmoji(state DeploymentState) string {
	switch state {
	case DeploymentStatePending:
		return "⏳"
	case DeploymentStateQueued:
		return "📋"
	case DeploymentStateInProgress:
		return "🔄"
	case DeploymentStateSuccess:
		return "✅"
	case DeploymentStateFailure:
		return "❌"
	case DeploymentStateError:
		return "⚠️"
	case DeploymentStateInactive:
		return "💤"
	default:
		return "❓"
	}
}
