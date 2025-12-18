// Package approval implements the approval logic engine.
package approval

import (
	"time"

	"github.com/jamengual/enterprise-approval-engine/internal/config"
)

// Status represents the current approval state.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusDenied   Status = "denied"
	StatusTimeout  Status = "timeout"
)

// Approval represents a single approval from a user.
type Approval struct {
	User      string
	Timestamp time.Time
	Comment   string
}

// Denial represents a denial from a user.
type Denial struct {
	User      string
	Timestamp time.Time
	Comment   string
}

// GroupStatus tracks approval progress for a single requirement group.
type GroupStatus struct {
	Name        string         // Policy name or "custom"
	Approvers   []string       // Eligible approvers for this group (flattened)
	RequireAll  bool           // If true, all must approve; otherwise use MinRequired
	MinRequired int            // Minimum approvals needed (if not RequireAll)
	Current     int            // Current approval count
	Approved    []string       // Users who have approved
	Satisfied   bool           // Whether this group's requirement is met
	Sources     []SourceStatus // Per-source status (for advanced "from" format)
	Logic       string         // "and" or "or" - how sources are combined
}

// SourceStatus tracks approval progress for a single source within a group.
type SourceStatus struct {
	Name        string   // Source identifier (team name, user, etc.)
	Approvers   []string // Eligible approvers from this source
	RequireAll  bool     // If true, all must approve
	MinRequired int      // Minimum approvals needed
	Current     int      // Current approval count from this source
	Approved    []string // Users from this source who approved
	Satisfied   bool     // Whether this source's requirement is met
}

// ApprovalResult contains the full approval status.
type ApprovalResult struct {
	Status         Status
	Groups         []GroupStatus
	SatisfiedGroup string     // Name of the group that was satisfied (for OR logic)
	Approvals      []Approval // All approvals received
	Denials        []Denial   // All denials received
	Denier         string     // User who denied (if denied)
}

// Request contains the context for evaluating an approval.
type Request struct {
	Config       *config.Config
	Workflow     *config.Workflow
	IssueNumber  int
	Requestor    string // User who initiated the request
	Comments     []Comment
}

// Comment represents an issue comment for approval parsing.
type Comment struct {
	ID        int64
	User      string
	Body      string
	CreatedAt time.Time
}
