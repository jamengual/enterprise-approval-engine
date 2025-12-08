package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultTimeout is the default approval timeout if not specified.
const DefaultTimeout = 72 * time.Hour

// Load reads and parses an approvals.yml configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return Parse(data)
}

// Parse parses YAML data into a Config struct.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	return &cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d (expected 1)", c.Version)
	}

	if len(c.Policies) == 0 {
		return fmt.Errorf("at least one policy must be defined")
	}

	if len(c.Workflows) == 0 {
		return fmt.Errorf("at least one workflow must be defined")
	}

	// Validate policies
	for name, policy := range c.Policies {
		if err := validatePolicy(name, policy); err != nil {
			return err
		}
	}

	// Validate workflows
	for name, workflow := range c.Workflows {
		if err := c.validateWorkflow(name, workflow); err != nil {
			return err
		}
	}

	return nil
}

func validatePolicy(name string, policy Policy) error {
	// Check if using advanced "from" format or simple "approvers" format
	hasFrom := len(policy.From) > 0
	hasApprovers := len(policy.Approvers) > 0

	if !hasFrom && !hasApprovers {
		return fmt.Errorf("policy %q must have either 'approvers' or 'from' defined", name)
	}

	if hasFrom && hasApprovers {
		return fmt.Errorf("policy %q cannot use both 'approvers' and 'from' - choose one format", name)
	}

	// Validate simple format
	if hasApprovers {
		for _, approver := range policy.Approvers {
			if approver == "" {
				return fmt.Errorf("policy %q has empty approver", name)
			}
		}

		if policy.MinApprovals < 0 {
			return fmt.Errorf("policy %q min_approvals cannot be negative", name)
		}

		if policy.MinApprovals > len(policy.Approvers) && !hasTeamApprover(policy.Approvers) {
			return fmt.Errorf("policy %q min_approvals (%d) exceeds approver count (%d)",
				name, policy.MinApprovals, len(policy.Approvers))
		}
	}

	// Validate advanced "from" format
	if hasFrom {
		for i, source := range policy.From {
			if err := validateApproverSource(name, i, source); err != nil {
				return err
			}
		}

		// Validate logic field
		if policy.Logic != "" && policy.Logic != "and" && policy.Logic != "or" {
			return fmt.Errorf("policy %q has invalid logic %q (must be 'and' or 'or')", name, policy.Logic)
		}
	}

	return nil
}

func validateApproverSource(policyName string, index int, source ApproverSource) error {
	hasTeam := source.Team != ""
	hasUser := source.User != ""
	hasUsers := len(source.Users) > 0

	count := 0
	if hasTeam {
		count++
	}
	if hasUser {
		count++
	}
	if hasUsers {
		count++
	}

	if count == 0 {
		return fmt.Errorf("policy %q source %d must specify 'team', 'user', or 'users'", policyName, index)
	}

	if count > 1 {
		return fmt.Errorf("policy %q source %d cannot mix 'team', 'user', and 'users' - use one per source", policyName, index)
	}

	if source.MinApprovals < 0 {
		return fmt.Errorf("policy %q source %d min_approvals cannot be negative", policyName, index)
	}

	return nil
}

func (c *Config) validateWorkflow(name string, workflow Workflow) error {
	if len(workflow.Require) == 0 {
		return fmt.Errorf("workflow %q must have at least one requirement", name)
	}

	for i, req := range workflow.Require {
		if err := c.validateRequirement(name, i, req); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) validateRequirement(workflowName string, index int, req Requirement) error {
	hasPolicy := req.Policy != ""
	hasApprovers := len(req.Approvers) > 0

	if !hasPolicy && !hasApprovers {
		return fmt.Errorf("workflow %q requirement %d must specify policy or approvers",
			workflowName, index)
	}

	if hasPolicy && hasApprovers {
		return fmt.Errorf("workflow %q requirement %d cannot specify both policy and approvers",
			workflowName, index)
	}

	if hasPolicy {
		if _, ok := c.Policies[req.Policy]; !ok {
			return fmt.Errorf("workflow %q requirement %d references undefined policy %q",
				workflowName, index, req.Policy)
		}
	}

	if req.MinApprovals < 0 {
		return fmt.Errorf("workflow %q requirement %d min_approvals cannot be negative",
			workflowName, index)
	}

	return nil
}

func (c *Config) applyDefaults() {
	// Apply default timeout
	if c.Defaults.Timeout.Duration == 0 {
		c.Defaults.Timeout.Duration = DefaultTimeout
	}

	// Apply default semver settings
	if c.Semver.Prefix == "" {
		c.Semver.Prefix = "v"
	}
	if c.Semver.Strategy == "" {
		c.Semver.Strategy = "input"
	}
}

// GetWorkflow returns a workflow by name.
func (c *Config) GetWorkflow(name string) (*Workflow, error) {
	workflow, ok := c.Workflows[name]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", name)
	}
	return &workflow, nil
}

// GetPolicy returns a policy by name.
func (c *Config) GetPolicy(name string) (*Policy, error) {
	policy, ok := c.Policies[name]
	if !ok {
		return nil, fmt.Errorf("policy %q not found", name)
	}
	return &policy, nil
}

// ResolveRequirement resolves a requirement to its effective approvers and threshold.
func (c *Config) ResolveRequirement(req Requirement) (approvers []string, minApprovals int, requireAll bool) {
	if req.Policy != "" {
		policy := c.Policies[req.Policy]
		approvers = policy.Approvers
		minApprovals = policy.MinApprovals
		requireAll = policy.RequireAll
	} else {
		approvers = req.Approvers
	}

	// Requirement can override policy settings
	if req.MinApprovals > 0 {
		minApprovals = req.MinApprovals
	}
	if req.RequireAll {
		requireAll = true
	}

	// If neither min_approvals nor require_all is set, default to require_all
	if minApprovals == 0 && !requireAll {
		requireAll = true
	}

	return approvers, minApprovals, requireAll
}

func hasTeamApprover(approvers []string) bool {
	for _, a := range approvers {
		if IsTeam(a) {
			return true
		}
	}
	return false
}
