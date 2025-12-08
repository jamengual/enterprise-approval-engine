package action

import (
	"encoding/json"
	"fmt"
	"os"
)

// GitHubEvent represents the common structure of GitHub webhook events.
type GitHubEvent struct {
	Action string `json:"action"`
	Issue  *struct {
		Number int      `json:"number"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		State  string   `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"issue"`
	Comment *struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Repository struct {
		FullName string `json:"full_name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

// ParseGitHubEvent reads and parses the GitHub event from GITHUB_EVENT_PATH.
func ParseGitHubEvent() (*GitHubEvent, error) {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return nil, fmt.Errorf("GITHUB_EVENT_PATH environment variable not set")
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read event file: %w", err)
	}

	var event GitHubEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse event JSON: %w", err)
	}

	return &event, nil
}

// GetIssueNumberFromEvent extracts the issue number from the GitHub event.
func GetIssueNumberFromEvent() (int, error) {
	event, err := ParseGitHubEvent()
	if err != nil {
		return 0, err
	}

	if event.Issue == nil {
		return 0, fmt.Errorf("event does not contain issue information")
	}

	return event.Issue.Number, nil
}

// GetCommentFromEvent extracts comment information from the GitHub event.
func GetCommentFromEvent() (id int64, user string, body string, err error) {
	event, err := ParseGitHubEvent()
	if err != nil {
		return 0, "", "", err
	}

	if event.Comment == nil {
		return 0, "", "", fmt.Errorf("event does not contain comment information")
	}

	return event.Comment.ID, event.Comment.User.Login, event.Comment.Body, nil
}

// GetEventAction returns the action type from the GitHub event (e.g., "created", "closed").
func GetEventAction() (string, error) {
	event, err := ParseGitHubEvent()
	if err != nil {
		return "", err
	}

	return event.Action, nil
}

// IsIssueOpsIssue checks if the issue has the approval-required label.
func IsIssueOpsIssue() (bool, error) {
	event, err := ParseGitHubEvent()
	if err != nil {
		return false, err
	}

	if event.Issue == nil {
		return false, nil
	}

	for _, label := range event.Issue.Labels {
		if label.Name == "approval-required" {
			return true, nil
		}
	}

	return false, nil
}
