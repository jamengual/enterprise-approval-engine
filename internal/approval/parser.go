package approval

import (
	"regexp"
	"strings"
)

// Default approval keywords
var defaultApprovalKeywords = []string{
	"approve",
	"approved",
	"lgtm",
	"yes",
	"/approve",
}

// Default denial keywords
var defaultDenialKeywords = []string{
	"deny",
	"denied",
	"reject",
	"rejected",
	"no",
	"/deny",
}

// Parser handles parsing of approval/denial comments.
type Parser struct {
	approvalKeywords []string
	denialKeywords   []string
	approvalRegexes  []*regexp.Regexp
	denialRegexes    []*regexp.Regexp
}

// NewParser creates a new comment parser with default keywords.
func NewParser() *Parser {
	return NewParserWithKeywords(nil, nil)
}

// NewParserWithKeywords creates a parser with custom keywords added to defaults.
func NewParserWithKeywords(additionalApproval, additionalDenial []string) *Parser {
	approvalKeywords := append([]string{}, defaultApprovalKeywords...)
	approvalKeywords = append(approvalKeywords, additionalApproval...)

	denialKeywords := append([]string{}, defaultDenialKeywords...)
	denialKeywords = append(denialKeywords, additionalDenial...)

	p := &Parser{
		approvalKeywords: approvalKeywords,
		denialKeywords:   denialKeywords,
	}

	// Pre-compile regexes for each keyword
	for _, kw := range approvalKeywords {
		// Match keyword with optional punctuation at end, case insensitive
		// Must be the entire comment (or start of comment with just whitespace after)
		pattern := `(?i)^\s*` + regexp.QuoteMeta(kw) + `[.!]?\s*$`
		p.approvalRegexes = append(p.approvalRegexes, regexp.MustCompile(pattern))
	}

	for _, kw := range denialKeywords {
		pattern := `(?i)^\s*` + regexp.QuoteMeta(kw) + `[.!]?\s*$`
		p.denialRegexes = append(p.denialRegexes, regexp.MustCompile(pattern))
	}

	return p
}

// ParseResult contains the result of parsing a comment.
type ParseResult struct {
	IsApproval bool
	IsDenial   bool
	Keyword    string
}

// Parse parses a comment body to detect approval or denial.
func (p *Parser) Parse(body string) ParseResult {
	body = strings.TrimSpace(body)

	// Check denial first (denial takes precedence)
	for i, re := range p.denialRegexes {
		if re.MatchString(body) {
			return ParseResult{
				IsDenial: true,
				Keyword:  p.denialKeywords[i],
			}
		}
	}

	// Check approval
	for i, re := range p.approvalRegexes {
		if re.MatchString(body) {
			return ParseResult{
				IsApproval: true,
				Keyword:    p.approvalKeywords[i],
			}
		}
	}

	return ParseResult{}
}

// IsApproval returns true if the comment is an approval.
func (p *Parser) IsApproval(body string) bool {
	return p.Parse(body).IsApproval
}

// IsDenial returns true if the comment is a denial.
func (p *Parser) IsDenial(body string) bool {
	return p.Parse(body).IsDenial
}

// FormatApprovalKeywords returns a formatted string of approval keywords.
func (p *Parser) FormatApprovalKeywords() string {
	return formatKeywords(p.approvalKeywords)
}

// FormatDenialKeywords returns a formatted string of denial keywords.
func (p *Parser) FormatDenialKeywords() string {
	return formatKeywords(p.denialKeywords)
}

func formatKeywords(keywords []string) string {
	quoted := make([]string, len(keywords))
	for i, kw := range keywords {
		quoted[i] = `"` + kw + `"`
	}
	return strings.Join(quoted, ", ")
}
