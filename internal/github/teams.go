package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
)

// TeamMember represents a member of a GitHub team.
type TeamMember struct {
	Login string
}

// GetTeamMembers retrieves all members of a team.
// The team can be specified as "team-slug" or "org/team-slug".
func (c *Client) GetTeamMembers(ctx context.Context, team string) ([]TeamMember, error) {
	org, slug := parseTeamRef(team, c.owner)

	// GitHub replaces periods in team names with hyphens
	slug = strings.ReplaceAll(slug, ".", "-")

	var allMembers []TeamMember
	opts := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		members, resp, err := c.client.Teams.ListTeamMembersBySlug(ctx, org, slug, opts)
		if err != nil {
			if IsNotFound(err) {
				return nil, fmt.Errorf("team %s/%s not found", org, slug)
			}
			if IsForbidden(err) {
				return nil, fmt.Errorf("insufficient permissions to list team %s/%s members (requires Organization Members read permission)", org, slug)
			}
			return nil, fmt.Errorf("failed to list team members for %s/%s: %w", org, slug, err)
		}

		for _, member := range members {
			allMembers = append(allMembers, TeamMember{
				Login: member.GetLogin(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allMembers, nil
}

// IsUserInTeam checks if a user is a member of a team.
func (c *Client) IsUserInTeam(ctx context.Context, team, user string) (bool, error) {
	members, err := c.GetTeamMembers(ctx, team)
	if err != nil {
		return false, err
	}

	userLower := strings.ToLower(user)
	for _, member := range members {
		if strings.ToLower(member.Login) == userLower {
			return true, nil
		}
	}

	return false, nil
}

// ExpandTeamToUsers converts team references to individual user logins.
// For non-team references (plain usernames), returns them as-is.
func (c *Client) ExpandTeamToUsers(ctx context.Context, approvers []string) ([]string, error) {
	var expanded []string
	seen := make(map[string]bool)

	for _, approver := range approvers {
		if isTeamRef(approver) {
			teamSlug := approver[5:] // Remove "team:" prefix
			members, err := c.GetTeamMembers(ctx, teamSlug)
			if err != nil {
				return nil, err
			}
			for _, member := range members {
				if !seen[strings.ToLower(member.Login)] {
					seen[strings.ToLower(member.Login)] = true
					expanded = append(expanded, member.Login)
				}
			}
		} else {
			if !seen[strings.ToLower(approver)] {
				seen[strings.ToLower(approver)] = true
				expanded = append(expanded, approver)
			}
		}
	}

	return expanded, nil
}

// parseTeamRef parses a team reference into org and slug.
// Supports "team-slug" (uses default org) or "org/team-slug".
func parseTeamRef(ref, defaultOrg string) (org, slug string) {
	if strings.Contains(ref, "/") {
		parts := strings.SplitN(ref, "/", 2)
		return parts[0], parts[1]
	}
	return defaultOrg, ref
}

// isTeamRef returns true if the approver is a team reference.
func isTeamRef(approver string) bool {
	return len(approver) > 5 && approver[:5] == "team:"
}
