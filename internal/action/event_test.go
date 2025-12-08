package action

import (
	"os"
	"path/filepath"
	"testing"
)

// createTestEventFile creates a temporary event file with the given JSON content.
func createTestEventFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	eventFile := filepath.Join(tmpDir, "event.json")
	if err := os.WriteFile(eventFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test event file: %v", err)
	}
	return eventFile
}

// setEventPath sets GITHUB_EVENT_PATH and returns a cleanup function.
func setEventPath(t *testing.T, path string) {
	t.Helper()
	oldPath := os.Getenv("GITHUB_EVENT_PATH")
	os.Setenv("GITHUB_EVENT_PATH", path)
	t.Cleanup(func() {
		if oldPath == "" {
			os.Unsetenv("GITHUB_EVENT_PATH")
		} else {
			os.Setenv("GITHUB_EVENT_PATH", oldPath)
		}
	})
}

func TestParseGitHubEvent(t *testing.T) {
	eventJSON := `{
		"action": "created",
		"issue": {
			"number": 42,
			"title": "Test Issue",
			"body": "Test body",
			"state": "open",
			"labels": [{"name": "approval-required"}],
			"user": {"login": "testuser"}
		},
		"comment": {
			"id": 12345,
			"body": "/approve",
			"user": {"login": "approver"}
		},
		"repository": {
			"full_name": "owner/repo",
			"owner": {"login": "owner"},
			"name": "repo"
		},
		"sender": {"login": "approver"}
	}`

	eventFile := createTestEventFile(t, eventJSON)
	setEventPath(t, eventFile)

	event, err := ParseGitHubEvent()
	if err != nil {
		t.Fatalf("ParseGitHubEvent failed: %v", err)
	}

	if event.Action != "created" {
		t.Errorf("Expected action 'created', got %q", event.Action)
	}
	if event.Issue == nil {
		t.Fatal("Expected issue to be present")
	}
	if event.Issue.Number != 42 {
		t.Errorf("Expected issue number 42, got %d", event.Issue.Number)
	}
	if event.Issue.Title != "Test Issue" {
		t.Errorf("Expected issue title 'Test Issue', got %q", event.Issue.Title)
	}
	if event.Comment == nil {
		t.Fatal("Expected comment to be present")
	}
	if event.Comment.ID != 12345 {
		t.Errorf("Expected comment ID 12345, got %d", event.Comment.ID)
	}
	if event.Comment.Body != "/approve" {
		t.Errorf("Expected comment body '/approve', got %q", event.Comment.Body)
	}
	if event.Repository.FullName != "owner/repo" {
		t.Errorf("Expected repo 'owner/repo', got %q", event.Repository.FullName)
	}
}

func TestParseGitHubEvent_NoEventPath(t *testing.T) {
	// Clear the environment variable
	oldPath := os.Getenv("GITHUB_EVENT_PATH")
	os.Unsetenv("GITHUB_EVENT_PATH")
	t.Cleanup(func() {
		if oldPath != "" {
			os.Setenv("GITHUB_EVENT_PATH", oldPath)
		}
	})

	_, err := ParseGitHubEvent()
	if err == nil {
		t.Error("Expected error when GITHUB_EVENT_PATH not set")
	}
}

func TestParseGitHubEvent_InvalidJSON(t *testing.T) {
	eventFile := createTestEventFile(t, "not valid json")
	setEventPath(t, eventFile)

	_, err := ParseGitHubEvent()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseGitHubEvent_FileNotFound(t *testing.T) {
	setEventPath(t, "/nonexistent/path/event.json")

	_, err := ParseGitHubEvent()
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestGetIssueNumberFromEvent(t *testing.T) {
	eventJSON := `{
		"action": "opened",
		"issue": {
			"number": 123,
			"title": "Test",
			"body": "Body",
			"state": "open",
			"labels": [],
			"user": {"login": "user"}
		},
		"repository": {
			"full_name": "owner/repo",
			"owner": {"login": "owner"},
			"name": "repo"
		},
		"sender": {"login": "user"}
	}`

	eventFile := createTestEventFile(t, eventJSON)
	setEventPath(t, eventFile)

	num, err := GetIssueNumberFromEvent()
	if err != nil {
		t.Fatalf("GetIssueNumberFromEvent failed: %v", err)
	}
	if num != 123 {
		t.Errorf("Expected issue number 123, got %d", num)
	}
}

func TestGetIssueNumberFromEvent_NoIssue(t *testing.T) {
	// Event without issue (e.g., push event)
	eventJSON := `{
		"action": "push",
		"repository": {
			"full_name": "owner/repo",
			"owner": {"login": "owner"},
			"name": "repo"
		},
		"sender": {"login": "user"}
	}`

	eventFile := createTestEventFile(t, eventJSON)
	setEventPath(t, eventFile)

	_, err := GetIssueNumberFromEvent()
	if err == nil {
		t.Error("Expected error when issue not present")
	}
}

func TestGetCommentFromEvent(t *testing.T) {
	eventJSON := `{
		"action": "created",
		"issue": {
			"number": 42,
			"title": "Test",
			"body": "Body",
			"state": "open",
			"labels": [],
			"user": {"login": "user"}
		},
		"comment": {
			"id": 98765,
			"body": "/approve looks good",
			"user": {"login": "reviewer"}
		},
		"repository": {
			"full_name": "owner/repo",
			"owner": {"login": "owner"},
			"name": "repo"
		},
		"sender": {"login": "reviewer"}
	}`

	eventFile := createTestEventFile(t, eventJSON)
	setEventPath(t, eventFile)

	id, user, body, err := GetCommentFromEvent()
	if err != nil {
		t.Fatalf("GetCommentFromEvent failed: %v", err)
	}
	if id != 98765 {
		t.Errorf("Expected comment ID 98765, got %d", id)
	}
	if user != "reviewer" {
		t.Errorf("Expected user 'reviewer', got %q", user)
	}
	if body != "/approve looks good" {
		t.Errorf("Expected body '/approve looks good', got %q", body)
	}
}

func TestGetCommentFromEvent_NoComment(t *testing.T) {
	// Issue opened event (no comment)
	eventJSON := `{
		"action": "opened",
		"issue": {
			"number": 42,
			"title": "Test",
			"body": "Body",
			"state": "open",
			"labels": [],
			"user": {"login": "user"}
		},
		"repository": {
			"full_name": "owner/repo",
			"owner": {"login": "owner"},
			"name": "repo"
		},
		"sender": {"login": "user"}
	}`

	eventFile := createTestEventFile(t, eventJSON)
	setEventPath(t, eventFile)

	_, _, _, err := GetCommentFromEvent()
	if err == nil {
		t.Error("Expected error when comment not present")
	}
}

func TestGetEventAction(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		expected string
	}{
		{"created", "created", "created"},
		{"closed", "closed", "closed"},
		{"opened", "opened", "opened"},
		{"edited", "edited", "edited"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventJSON := `{
				"action": "` + tt.action + `",
				"repository": {
					"full_name": "owner/repo",
					"owner": {"login": "owner"},
					"name": "repo"
				},
				"sender": {"login": "user"}
			}`

			eventFile := createTestEventFile(t, eventJSON)
			setEventPath(t, eventFile)

			action, err := GetEventAction()
			if err != nil {
				t.Fatalf("GetEventAction failed: %v", err)
			}
			if action != tt.expected {
				t.Errorf("Expected action %q, got %q", tt.expected, action)
			}
		})
	}
}

func TestIsIssueOpsIssue(t *testing.T) {
	tests := []struct {
		name     string
		labels   string
		expected bool
	}{
		{
			name:     "with approval-required label",
			labels:   `[{"name": "approval-required"}, {"name": "other"}]`,
			expected: true,
		},
		{
			name:     "without approval-required label",
			labels:   `[{"name": "bug"}, {"name": "enhancement"}]`,
			expected: false,
		},
		{
			name:     "empty labels",
			labels:   `[]`,
			expected: false,
		},
		{
			name:     "only approval-required",
			labels:   `[{"name": "approval-required"}]`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventJSON := `{
				"action": "opened",
				"issue": {
					"number": 42,
					"title": "Test",
					"body": "Body",
					"state": "open",
					"labels": ` + tt.labels + `,
					"user": {"login": "user"}
				},
				"repository": {
					"full_name": "owner/repo",
					"owner": {"login": "owner"},
					"name": "repo"
				},
				"sender": {"login": "user"}
			}`

			eventFile := createTestEventFile(t, eventJSON)
			setEventPath(t, eventFile)

			result, err := IsIssueOpsIssue()
			if err != nil {
				t.Fatalf("IsIssueOpsIssue failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsIssueOpsIssue_NoIssue(t *testing.T) {
	// Event without issue
	eventJSON := `{
		"action": "push",
		"repository": {
			"full_name": "owner/repo",
			"owner": {"login": "owner"},
			"name": "repo"
		},
		"sender": {"login": "user"}
	}`

	eventFile := createTestEventFile(t, eventJSON)
	setEventPath(t, eventFile)

	result, err := IsIssueOpsIssue()
	if err != nil {
		t.Fatalf("IsIssueOpsIssue failed: %v", err)
	}
	// Should return false when no issue present (not an error)
	if result != false {
		t.Error("Expected false when no issue present")
	}
}
