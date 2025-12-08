package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v57/github"
)

// Tag represents a Git tag.
type Tag struct {
	Name   string
	SHA    string
	Tagger string
}

// CreateTagOptions contains options for creating a tag.
type CreateTagOptions struct {
	Name    string // Tag name (e.g., "v1.2.3")
	SHA     string // Commit SHA to tag (empty = use default branch HEAD)
	Message string // Tag message
}

// CreateTag creates a new annotated tag.
func (c *Client) CreateTag(ctx context.Context, opts CreateTagOptions) (*Tag, error) {
	// If no SHA provided, get the default branch HEAD
	sha := opts.SHA
	if sha == "" {
		repo, _, err := c.client.Repositories.Get(ctx, c.owner, c.repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository: %w", err)
		}

		defaultBranch := repo.GetDefaultBranch()
		ref, _, err := c.client.Git.GetRef(ctx, c.owner, c.repo, "refs/heads/"+defaultBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get default branch ref: %w", err)
		}
		sha = ref.GetObject().GetSHA()
	}

	// Create the tag object
	message := opts.Message
	if message == "" {
		message = fmt.Sprintf("Release %s", opts.Name)
	}

	now := time.Now()
	tagObj := &github.Tag{
		Tag:     &opts.Name,
		Message: &message,
		Object: &github.GitObject{
			Type: github.String("commit"),
			SHA:  &sha,
		},
		Tagger: &github.CommitAuthor{
			Name:  github.String("github-actions[bot]"),
			Email: github.String("github-actions[bot]@users.noreply.github.com"),
			Date:  &github.Timestamp{Time: now},
		},
	}

	createdTag, _, err := c.client.Git.CreateTag(ctx, c.owner, c.repo, tagObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag object: %w", err)
	}

	// Create the reference for the tag
	refName := "refs/tags/" + opts.Name
	ref := &github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: createdTag.SHA,
		},
	}

	_, _, err = c.client.Git.CreateRef(ctx, c.owner, c.repo, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag reference: %w", err)
	}

	return &Tag{
		Name:   opts.Name,
		SHA:    createdTag.GetSHA(),
		Tagger: "github-actions[bot]",
	}, nil
}

// TagExists checks if a tag already exists.
func (c *Client) TagExists(ctx context.Context, name string) (bool, error) {
	refName := "refs/tags/" + name
	_, _, err := c.client.Git.GetRef(ctx, c.owner, c.repo, refName)
	if err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check tag existence: %w", err)
	}
	return true, nil
}

// GetLatestTag returns the latest semver tag.
func (c *Client) GetLatestTag(ctx context.Context) (string, error) {
	return c.GetLatestTagWithPrefix(ctx, "")
}

// GetLatestTagWithPrefix returns the latest tag matching a specific prefix.
// This is useful for environment-specific tags like "dev-v1.0.0", "staging-v1.0.0".
func (c *Client) GetLatestTagWithPrefix(ctx context.Context, prefix string) (string, error) {
	opts := &github.ListOptions{PerPage: 100}

	for {
		tags, resp, err := c.client.Repositories.ListTags(ctx, c.owner, c.repo, opts)
		if err != nil {
			return "", fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			name := tag.GetName()
			if prefix == "" || len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				return name, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return "", nil
}

// DeleteTag deletes a tag by name.
func (c *Client) DeleteTag(ctx context.Context, name string) error {
	refName := "refs/tags/" + name
	_, err := c.client.Git.DeleteRef(ctx, c.owner, c.repo, refName)
	if err != nil {
		if IsNotFound(err) {
			// Tag doesn't exist, consider it a success
			return nil
		}
		return fmt.Errorf("failed to delete tag %s: %w", name, err)
	}
	return nil
}
