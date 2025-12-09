package jira

import (
	"reflect"
	"testing"
)

func TestExtractIssueKeys(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single issue key",
			text: "Fixed PROJ-123",
			want: []string{"PROJ-123"},
		},
		{
			name: "multiple issue keys",
			text: "Fixed PROJ-123 and PROJ-456",
			want: []string{"PROJ-123", "PROJ-456"},
		},
		{
			name: "issue key in brackets",
			text: "[PROJ-123] Fix bug",
			want: []string{"PROJ-123"},
		},
		{
			name: "issue key with colon",
			text: "PROJ-123: Fix the thing",
			want: []string{"PROJ-123"},
		},
		{
			name: "multiple projects",
			text: "PROJ-1 ABC-2 XYZ-999",
			want: []string{"PROJ-1", "ABC-2", "XYZ-999"},
		},
		{
			name: "duplicate keys",
			text: "PROJ-123 related to PROJ-123",
			want: []string{"PROJ-123"},
		},
		{
			name: "no issue keys",
			text: "Just a regular commit message",
			want: nil,
		},
		{
			name: "lowercase not matched",
			text: "proj-123 not valid",
			want: nil,
		},
		{
			name: "single letter project not matched",
			text: "P-123 not valid",
			want: nil,
		},
		{
			name: "issue key with numbers in project",
			text: "ABC2-123 is valid",
			want: []string{"ABC2-123"},
		},
		{
			name: "commit message format",
			text: "feat(auth): PROJ-123 add login feature",
			want: []string{"PROJ-123"},
		},
		{
			name: "multiple lines",
			text: "PROJ-1 first line\nPROJ-2 second line\nPROJ-3 third line",
			want: []string{"PROJ-1", "PROJ-2", "PROJ-3"},
		},
		{
			name: "empty string",
			text: "",
			want: nil,
		},
		{
			name: "issue key at start",
			text: "PROJ-123",
			want: []string{"PROJ-123"},
		},
		{
			name: "issue key at end",
			text: "See PROJ-123",
			want: []string{"PROJ-123"},
		},
		{
			name: "issue key with long number",
			text: "PROJ-99999",
			want: []string{"PROJ-99999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractIssueKeys(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractIssueKeys(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestExtractIssueKeysFromCommits(t *testing.T) {
	tests := []struct {
		name    string
		commits []string
		want    []string
	}{
		{
			name: "single commit with single key",
			commits: []string{
				"PROJ-123 fix bug",
			},
			want: []string{"PROJ-123"},
		},
		{
			name: "multiple commits with unique keys",
			commits: []string{
				"PROJ-1 first commit",
				"PROJ-2 second commit",
				"PROJ-3 third commit",
			},
			want: []string{"PROJ-1", "PROJ-2", "PROJ-3"},
		},
		{
			name: "multiple commits with duplicate keys",
			commits: []string{
				"PROJ-1 first commit",
				"PROJ-1 another commit for same issue",
				"PROJ-2 different issue",
			},
			want: []string{"PROJ-1", "PROJ-2"},
		},
		{
			name: "commits with no keys",
			commits: []string{
				"just a regular commit",
				"another commit without jira",
			},
			want: nil,
		},
		{
			name: "mixed commits",
			commits: []string{
				"PROJ-1 has key",
				"no key here",
				"ABC-99 different project",
			},
			want: []string{"PROJ-1", "ABC-99"},
		},
		{
			name:    "empty commits slice",
			commits: []string{},
			want:    nil,
		},
		{
			name:    "nil commits",
			commits: nil,
			want:    nil,
		},
		{
			name: "commit with multiple keys",
			commits: []string{
				"PROJ-1 PROJ-2 relates to both",
			},
			want: []string{"PROJ-1", "PROJ-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractIssueKeysFromCommits(tt.commits)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractIssueKeysFromCommits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		name  string
		issue *Issue
		want  string
	}{
		{
			name:  "nil issue",
			issue: nil,
			want:  "❓",
		},
		{
			name: "nil status",
			issue: &Issue{
				Fields: IssueFields{},
			},
			want: "❓",
		},
		{
			name: "nil category",
			issue: &Issue{
				Fields: IssueFields{
					Status: &Status{
						Name: "Done",
					},
				},
			},
			want: "❓",
		},
		{
			name: "done status",
			issue: &Issue{
				Fields: IssueFields{
					Status: &Status{
						Category: &StatusCategory{Key: "done"},
					},
				},
			},
			want: "✅",
		},
		{
			name: "in progress status",
			issue: &Issue{
				Fields: IssueFields{
					Status: &Status{
						Category: &StatusCategory{Key: "indeterminate"},
					},
				},
			},
			want: "🔄",
		},
		{
			name: "new/todo status",
			issue: &Issue{
				Fields: IssueFields{
					Status: &Status{
						Category: &StatusCategory{Key: "new"},
					},
				},
			},
			want: "📋",
		},
		{
			name: "unknown status",
			issue: &Issue{
				Fields: IssueFields{
					Status: &Status{
						Category: &StatusCategory{Key: "unknown"},
					},
				},
			},
			want: "❓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStatusEmoji(tt.issue)
			if got != tt.want {
				t.Errorf("GetStatusEmoji() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetTypeEmoji(t *testing.T) {
	tests := []struct {
		name  string
		issue *Issue
		want  string
	}{
		{
			name:  "nil issue",
			issue: nil,
			want:  "📌",
		},
		{
			name: "nil issue type",
			issue: &Issue{
				Fields: IssueFields{},
			},
			want: "📌",
		},
		{
			name: "bug type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Bug"},
				},
			},
			want: "🐛",
		},
		{
			name: "bug type lowercase",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "bug"},
				},
			},
			want: "🐛",
		},
		{
			name: "feature type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Feature"},
				},
			},
			want: "✨",
		},
		{
			name: "story type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Story"},
				},
			},
			want: "✨",
		},
		{
			name: "user story type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "User Story"},
				},
			},
			want: "✨",
		},
		{
			name: "task type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Task"},
				},
			},
			want: "📋",
		},
		{
			name: "sub-task type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Sub-task"},
				},
			},
			want: "📋",
		},
		{
			name: "epic type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Epic"},
				},
			},
			want: "🎯",
		},
		{
			name: "improvement type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Improvement"},
				},
			},
			want: "💡",
		},
		{
			name: "security type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Security Issue"},
				},
			},
			want: "🔒",
		},
		{
			name: "unknown type",
			issue: &Issue{
				Fields: IssueFields{
					IssueType: &IssueType{Name: "Custom Type"},
				},
			},
			want: "📌",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTypeEmoji(tt.issue)
			if got != tt.want {
				t.Errorf("GetTypeEmoji() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueKeyPattern(t *testing.T) {
	validKeys := []string{
		"PROJ-1",
		"PROJ-123",
		"AB-1",
		"ABC-99999",
		"A2B-1",
		"PROJECT123-456",
	}

	invalidKeys := []string{
		"proj-123",     // lowercase
		"P-123",        // single letter project
		"PROJ",         // no number
		"123-PROJ",     // reversed
		"PROJ123",      // no dash
		"-123",         // no project
		"PROJ-",        // no number after dash
		"PROJ-abc",     // letters instead of numbers
	}

	for _, key := range validKeys {
		t.Run("valid_"+key, func(t *testing.T) {
			if !IssueKeyPattern.MatchString(key) {
				t.Errorf("IssueKeyPattern should match %q", key)
			}
		})
	}

	for _, key := range invalidKeys {
		t.Run("invalid_"+key, func(t *testing.T) {
			// For invalid patterns, the full string should not match exactly
			match := IssueKeyPattern.FindString(key)
			if match == key {
				t.Errorf("IssueKeyPattern should not match %q exactly", key)
			}
		})
	}
}
