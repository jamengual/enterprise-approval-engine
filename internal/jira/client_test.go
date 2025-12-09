package jira

import (
	"net/http"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name            string
		cfg             ClientConfig
		wantErr         bool
		errMsg          string
		wantAPICalls    bool // whether client should be able to make API calls
	}{
		{
			name: "full config with API access",
			cfg: ClientConfig{
				BaseURL:  "https://example.atlassian.net",
				Email:    "user@example.com",
				APIToken: "token123",
			},
			wantErr:      false,
			wantAPICalls: true,
		},
		{
			name: "links-only mode (just base URL)",
			cfg: ClientConfig{
				BaseURL: "https://example.atlassian.net",
			},
			wantErr:      false,
			wantAPICalls: false,
		},
		{
			name: "partial auth (only email) - links-only mode",
			cfg: ClientConfig{
				BaseURL: "https://example.atlassian.net",
				Email:   "user@example.com",
			},
			wantErr:      false,
			wantAPICalls: false,
		},
		{
			name: "partial auth (only token) - links-only mode",
			cfg: ClientConfig{
				BaseURL:  "https://example.atlassian.net",
				APIToken: "token123",
			},
			wantErr:      false,
			wantAPICalls: false,
		},
		{
			name: "missing base URL",
			cfg: ClientConfig{
				Email:    "user@example.com",
				APIToken: "token123",
			},
			wantErr: true,
			errMsg:  "jira base URL is required",
		},
		{
			name:    "all fields empty",
			cfg:     ClientConfig{},
			wantErr: true,
			errMsg:  "jira base URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("NewClient() error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewClient() unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("NewClient() returned nil client")
				return
			}

			if client.baseURL != tt.cfg.BaseURL {
				t.Errorf("client.baseURL = %q, want %q", client.baseURL, tt.cfg.BaseURL)
			}

			// Check API access capability
			if got := client.CanMakeAPICalls(); got != tt.wantAPICalls {
				t.Errorf("client.CanMakeAPICalls() = %v, want %v", got, tt.wantAPICalls)
			}

			// IsConfigured should be true for any valid client
			if !client.IsConfigured() {
				t.Error("client.IsConfigured() should be true for valid client")
			}
		})
	}
}

func TestNewLinksOnlyClient(t *testing.T) {
	client, err := NewLinksOnlyClient("https://example.atlassian.net")
	if err != nil {
		t.Fatalf("NewLinksOnlyClient() error = %v", err)
	}

	if !client.IsConfigured() {
		t.Error("links-only client should be configured")
	}

	if client.CanMakeAPICalls() {
		t.Error("links-only client should not be able to make API calls")
	}

	if client.BaseURL() != "https://example.atlassian.net" {
		t.Errorf("BaseURL() = %q, want %q", client.BaseURL(), "https://example.atlassian.net")
	}
}

func TestClient_IsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		client *Client
		want   bool
	}{
		{
			name:   "nil client",
			client: nil,
			want:   false,
		},
		{
			name: "fully configured client",
			client: &Client{
				baseURL:  "https://example.atlassian.net",
				email:    "user@example.com",
				apiToken: "token123",
			},
			want: true,
		},
		{
			name: "links-only client (just base URL)",
			client: &Client{
				baseURL: "https://example.atlassian.net",
			},
			want: true,
		},
		{
			name: "missing base URL",
			client: &Client{
				email:    "user@example.com",
				apiToken: "token123",
			},
			want: false,
		},
		{
			name:   "empty client",
			client: &Client{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.client.IsConfigured()
			if got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_CanMakeAPICalls(t *testing.T) {
	tests := []struct {
		name   string
		client *Client
		want   bool
	}{
		{
			name:   "nil client",
			client: nil,
			want:   false,
		},
		{
			name: "fully configured client with http client",
			client: &Client{
				baseURL:    "https://example.atlassian.net",
				email:      "user@example.com",
				apiToken:   "token123",
				httpClient: &http.Client{},
			},
			want: true,
		},
		{
			name: "links-only client",
			client: &Client{
				baseURL: "https://example.atlassian.net",
			},
			want: false,
		},
		{
			name: "missing http client",
			client: &Client{
				baseURL:  "https://example.atlassian.net",
				email:    "user@example.com",
				apiToken: "token123",
			},
			want: false,
		},
		{
			name: "missing email",
			client: &Client{
				baseURL:    "https://example.atlassian.net",
				apiToken:   "token123",
				httpClient: &http.Client{},
			},
			want: false,
		},
		{
			name: "missing API token",
			client: &Client{
				baseURL:    "https://example.atlassian.net",
				email:      "user@example.com",
				httpClient: &http.Client{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.client.CanMakeAPICalls()
			if got != tt.want {
				t.Errorf("CanMakeAPICalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetIssueURL(t *testing.T) {
	client := &Client{
		baseURL: "https://example.atlassian.net",
	}

	tests := []struct {
		key  string
		want string
	}{
		{
			key:  "PROJ-123",
			want: "https://example.atlassian.net/browse/PROJ-123",
		},
		{
			key:  "AB-1",
			want: "https://example.atlassian.net/browse/AB-1",
		},
		{
			key:  "LONGPROJECT-99999",
			want: "https://example.atlassian.net/browse/LONGPROJECT-99999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := client.GetIssueURL(tt.key)
			if got != tt.want {
				t.Errorf("GetIssueURL(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
