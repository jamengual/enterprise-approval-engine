package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v57/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddSubIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/repos/owner/repo/issues/1/sub_issues", r.URL.Path)
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

		var req SubIssueRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, int64(2), req.SubIssueID)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
	require.NoError(t, err)

	// Override the base URL to point to our test server
	client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

	err = client.AddSubIssue(context.Background(), 1, 2)
	assert.NoError(t, err)
}

func TestListSubIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/repos/owner/repo/issues/1/sub_issues", r.URL.Path)

		subIssues := []*github.Issue{
			{
				Number: github.Int(2),
				Title:  github.String("Approve: DEV"),
				State:  github.String("open"),
			},
			{
				Number: github.Int(3),
				Title:  github.String("Approve: QA"),
				State:  github.String("closed"),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subIssues)
	}))
	defer server.Close()

	client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
	require.NoError(t, err)

	client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

	issues, err := client.ListSubIssues(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.Equal(t, 2, *issues[0].Number)
	assert.Equal(t, "Approve: DEV", *issues[0].Title)
}

func TestGetParentIssue(t *testing.T) {
	t.Run("has parent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/repos/owner/repo/issues/2/parent", r.URL.Path)

			parent := &github.Issue{
				Number: github.Int(1),
				Title:  github.String("Deploy v1.2.0"),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(parent)
		}))
		defer server.Close()

		client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
		require.NoError(t, err)

		client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

		parent, err := client.GetParentIssue(context.Background(), 2)
		require.NoError(t, err)
		assert.NotNil(t, parent)
		assert.Equal(t, 1, *parent.Number)
	})

	t.Run("no parent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
		require.NoError(t, err)

		client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

		parent, err := client.GetParentIssue(context.Background(), 2)
		require.NoError(t, err)
		assert.Nil(t, parent)
	})
}

func TestRemoveSubIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/repos/owner/repo/issues/1/sub_issue", r.URL.Path)

		var req SubIssueRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, int64(2), req.SubIssueID)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
	require.NoError(t, err)

	client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

	err = client.RemoveSubIssue(context.Background(), 1, 2)
	assert.NoError(t, err)
}

func TestIsSubIssue(t *testing.T) {
	t.Run("is sub-issue", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parent := &github.Issue{Number: github.Int(1)}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(parent)
		}))
		defer server.Close()

		client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
		require.NoError(t, err)

		client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

		isSub, err := client.IsSubIssue(context.Background(), 2)
		require.NoError(t, err)
		assert.True(t, isSub)
	})

	t.Run("not sub-issue", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client, err := NewClientWithToken(context.Background(), "test-token", "owner", "repo")
		require.NoError(t, err)

		client.client.BaseURL, _ = client.client.BaseURL.Parse(server.URL + "/")

		isSub, err := client.IsSubIssue(context.Background(), 2)
		require.NoError(t, err)
		assert.False(t, isSub)
	})
}
