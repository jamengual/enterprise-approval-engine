package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v57/github"
)

// Commit represents a Git commit.
type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	URL     string `json:"url"`
}

// CompareCommits compares two refs and returns the commits between them.
// base is the older ref, head is the newer ref.
func (c *Client) CompareCommits(ctx context.Context, base, head string) ([]Commit, error) {
	comparison, _, err := c.client.Repositories.CompareCommits(ctx, c.owner, c.repo, base, head, &github.ListOptions{PerPage: 250})
	if err != nil {
		return nil, fmt.Errorf("failed to compare commits between %s and %s: %w", base, head, err)
	}

	var commits []Commit
	for _, c := range comparison.Commits {
		author := ""
		if c.Author != nil {
			author = c.Author.GetLogin()
		} else if c.Commit != nil && c.Commit.Author != nil {
			author = c.Commit.Author.GetName()
		}

		date := ""
		if c.Commit != nil && c.Commit.Author != nil && c.Commit.Author.Date != nil {
			date = c.Commit.Author.Date.String()
		}

		message := ""
		if c.Commit != nil {
			message = c.Commit.GetMessage()
		}

		commits = append(commits, Commit{
			SHA:     c.GetSHA(),
			Message: message,
			Author:  author,
			Date:    date,
			URL:     c.GetHTMLURL(),
		})
	}

	return commits, nil
}

// GetCommitsBetweenTags gets all commits between two tags.
func (c *Client) GetCommitsBetweenTags(ctx context.Context, oldTag, newTag string) ([]Commit, error) {
	return c.CompareCommits(ctx, oldTag, newTag)
}

// GetCommitsSinceTag gets all commits since a tag on a branch.
func (c *Client) GetCommitsSinceTag(ctx context.Context, tag, branch string) ([]Commit, error) {
	if branch == "" {
		branch = "HEAD"
	}
	return c.CompareCommits(ctx, tag, branch)
}

// GetCommitMessages extracts just the commit messages from a list of commits.
func GetCommitMessages(commits []Commit) []string {
	messages := make([]string, len(commits))
	for i, c := range commits {
		messages[i] = c.Message
	}
	return messages
}

// ListTags lists tags in the repository, ordered by creation date (newest first).
func (c *Client) ListTags(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	tags, _, err := c.client.Repositories.ListTags(ctx, c.owner, c.repo, &github.ListOptions{PerPage: limit})
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	var names []string
	for _, tag := range tags {
		names = append(names, tag.GetName())
	}

	return names, nil
}

// GetPreviousTag returns the tag before the given tag.
func (c *Client) GetPreviousTag(ctx context.Context, currentTag string) (string, error) {
	tags, err := c.ListTags(ctx, 50)
	if err != nil {
		return "", err
	}

	foundCurrent := false
	for _, tag := range tags {
		if foundCurrent {
			return tag, nil
		}
		if tag == currentTag {
			foundCurrent = true
		}
	}

	if foundCurrent {
		// Current tag was found but there's no previous tag
		return "", nil
	}

	return "", fmt.Errorf("tag %s not found", currentTag)
}
