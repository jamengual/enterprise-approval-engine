package jira

import (
	"context"
	"fmt"
)

// GetProjectVersions retrieves all versions for a project.
func (c *Client) GetProjectVersions(ctx context.Context, projectKey string) ([]Version, error) {
	path := fmt.Sprintf("/rest/api/3/project/%s/versions", projectKey)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var versions []Version
	if err := decodeResponse(resp, &versions); err != nil {
		return nil, fmt.Errorf("failed to get versions for project %s: %w", projectKey, err)
	}

	return versions, nil
}

// CreateVersion creates a new version in a project.
func (c *Client) CreateVersion(ctx context.Context, projectKey string, version Version) (*Version, error) {
	// First, get the project to obtain the project ID
	project, err := c.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
	}

	type createVersionRequest struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		ProjectID   int    `json:"projectId"`
		Released    bool   `json:"released"`
		ReleaseDate string `json:"releaseDate,omitempty"`
	}

	req := createVersionRequest{
		Name:        version.Name,
		Description: version.Description,
		ProjectID:   project.ID,
		Released:    version.Released,
		ReleaseDate: version.ReleaseDate,
	}

	resp, err := c.doRequest(ctx, "POST", "/rest/api/3/version", req)
	if err != nil {
		return nil, err
	}

	var created Version
	if err := decodeResponse(resp, &created); err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}

	return &created, nil
}

// GetOrCreateVersion gets an existing version by name or creates it if it doesn't exist.
func (c *Client) GetOrCreateVersion(ctx context.Context, projectKey, versionName string) (*Version, error) {
	versions, err := c.GetProjectVersions(ctx, projectKey)
	if err != nil {
		return nil, err
	}

	// Look for existing version
	for i := range versions {
		if versions[i].Name == versionName {
			return &versions[i], nil
		}
	}

	// Create new version
	return c.CreateVersion(ctx, projectKey, Version{
		Name:     versionName,
		Released: false,
	})
}

// ReleaseVersion marks a version as released.
func (c *Client) ReleaseVersion(ctx context.Context, versionID string, releaseDate string) error {
	type updateRequest struct {
		Released    bool   `json:"released"`
		ReleaseDate string `json:"releaseDate,omitempty"`
	}

	req := updateRequest{
		Released:    true,
		ReleaseDate: releaseDate,
	}

	path := fmt.Sprintf("/rest/api/3/version/%s", versionID)
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return err
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to release version %s: %w", versionID, err)
	}

	return nil
}

// ProjectInfo contains basic project information.
type ProjectInfo struct {
	ID   int    `json:"id,string"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// GetProject retrieves project information.
func (c *Client) GetProject(ctx context.Context, projectKey string) (*ProjectInfo, error) {
	path := fmt.Sprintf("/rest/api/3/project/%s", projectKey)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var project ProjectInfo
	if err := decodeResponse(resp, &project); err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
	}

	return &project, nil
}

// SetFixVersionForIssues sets the fix version for multiple issues.
func (c *Client) SetFixVersionForIssues(ctx context.Context, issueKeys []string, version Version) error {
	for _, key := range issueKeys {
		if err := c.AddFixVersion(ctx, key, version); err != nil {
			// Log but continue with other issues
			fmt.Printf("Warning: failed to set fix version for %s: %v\n", key, err)
		}
	}
	return nil
}
