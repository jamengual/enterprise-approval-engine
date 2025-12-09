package jira

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Issue represents a Jira issue.
type Issue struct {
	Key    string      `json:"key"`
	ID     string      `json:"id"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields contains the fields of a Jira issue.
type IssueFields struct {
	Summary     string       `json:"summary"`
	Description string       `json:"description,omitempty"`
	Status      *Status      `json:"status,omitempty"`
	IssueType   *IssueType   `json:"issuetype,omitempty"`
	Priority    *Priority    `json:"priority,omitempty"`
	Assignee    *User        `json:"assignee,omitempty"`
	Reporter    *User        `json:"reporter,omitempty"`
	Project     *Project     `json:"project,omitempty"`
	FixVersions []Version    `json:"fixVersions,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
}

// Status represents the status of a Jira issue.
type Status struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Category    *StatusCategory `json:"statusCategory,omitempty"`
}

// StatusCategory represents a status category.
type StatusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// IssueType represents the type of a Jira issue.
type IssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Subtask     bool   `json:"subtask"`
}

// Priority represents the priority of a Jira issue.
type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User represents a Jira user.
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress,omitempty"`
}

// Project represents a Jira project.
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Version represents a Jira version/release.
type Version struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Released    bool   `json:"released,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	ProjectID   int    `json:"projectId,omitempty"`
}

// GetIssue retrieves a single issue by key.
func (c *Client) GetIssue(ctx context.Context, key string) (*Issue, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s", key)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := decodeResponse(resp, &issue); err != nil {
		return nil, fmt.Errorf("failed to get issue %s: %w", key, err)
	}

	return &issue, nil
}

// GetIssues retrieves multiple issues by their keys.
func (c *Client) GetIssues(ctx context.Context, keys []string) ([]Issue, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	// Use JQL to fetch multiple issues at once
	jql := fmt.Sprintf("key in (%s)", strings.Join(keys, ","))
	return c.SearchIssues(ctx, jql)
}

// SearchIssues searches for issues using JQL.
func (c *Client) SearchIssues(ctx context.Context, jql string) ([]Issue, error) {
	type searchRequest struct {
		JQL        string   `json:"jql"`
		MaxResults int      `json:"maxResults"`
		Fields     []string `json:"fields"`
	}

	type searchResponse struct {
		Issues     []Issue `json:"issues"`
		Total      int     `json:"total"`
		MaxResults int     `json:"maxResults"`
	}

	req := searchRequest{
		JQL:        jql,
		MaxResults: 100,
		Fields:     []string{"summary", "status", "issuetype", "priority", "assignee", "reporter", "project", "fixVersions", "labels"},
	}

	resp, err := c.doRequest(ctx, "POST", "/rest/api/3/search", req)
	if err != nil {
		return nil, err
	}

	var result searchResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	return result.Issues, nil
}

// UpdateIssueFixVersions updates the fix versions of an issue.
func (c *Client) UpdateIssueFixVersions(ctx context.Context, key string, versions []Version) error {
	type updateRequest struct {
		Fields struct {
			FixVersions []Version `json:"fixVersions"`
		} `json:"fields"`
	}

	req := updateRequest{}
	req.Fields.FixVersions = versions

	path := fmt.Sprintf("/rest/api/3/issue/%s", key)
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return err
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to update fix versions for %s: %w", key, err)
	}

	return nil
}

// AddFixVersion adds a fix version to an issue (without removing existing ones).
func (c *Client) AddFixVersion(ctx context.Context, key string, version Version) error {
	type updateRequest struct {
		Update struct {
			FixVersions []struct {
				Add Version `json:"add"`
			} `json:"fixVersions"`
		} `json:"update"`
	}

	req := updateRequest{}
	req.Update.FixVersions = []struct {
		Add Version `json:"add"`
	}{{Add: version}}

	path := fmt.Sprintf("/rest/api/3/issue/%s", key)
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return err
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to add fix version to %s: %w", key, err)
	}

	return nil
}

// IssueKeyPattern matches Jira issue keys like PROJ-123, ABC-1, etc.
var IssueKeyPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)

// ExtractIssueKeys extracts all Jira issue keys from a string.
func ExtractIssueKeys(text string) []string {
	matches := IssueKeyPattern.FindAllString(text, -1)

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, key := range matches {
		if !seen[key] {
			seen[key] = true
			unique = append(unique, key)
		}
	}

	return unique
}

// ExtractIssueKeysFromCommits extracts unique Jira issue keys from multiple commit messages.
func ExtractIssueKeysFromCommits(commits []string) []string {
	seen := make(map[string]bool)
	var keys []string

	for _, commit := range commits {
		for _, key := range ExtractIssueKeys(commit) {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}

	return keys
}

// GetIssueURL returns the browse URL for an issue.
func (c *Client) GetIssueURL(key string) string {
	return fmt.Sprintf("%s/browse/%s", c.baseURL, key)
}

// GetStatusEmoji returns an emoji based on the issue status category.
func GetStatusEmoji(issue *Issue) string {
	if issue == nil || issue.Fields.Status == nil || issue.Fields.Status.Category == nil {
		return "❓"
	}

	switch issue.Fields.Status.Category.Key {
	case "done":
		return "✅"
	case "indeterminate": // In Progress
		return "🔄"
	case "new": // To Do
		return "📋"
	default:
		return "❓"
	}
}

// GetTypeEmoji returns an emoji based on the issue type.
func GetTypeEmoji(issue *Issue) string {
	if issue == nil || issue.Fields.IssueType == nil {
		return "📌"
	}

	name := strings.ToLower(issue.Fields.IssueType.Name)
	switch {
	case strings.Contains(name, "bug"):
		return "🐛"
	case strings.Contains(name, "feature") || strings.Contains(name, "story"):
		return "✨"
	case strings.Contains(name, "task"):
		return "📋"
	case strings.Contains(name, "epic"):
		return "🎯"
	case strings.Contains(name, "improvement"):
		return "💡"
	case strings.Contains(name, "security"):
		return "🔒"
	default:
		return "📌"
	}
}
