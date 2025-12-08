// Package semver provides semantic versioning utilities for tag creation.
package semver

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Version wraps a semver version with prefix handling.
type Version struct {
	original string
	prefix   string
	version  *semver.Version
}

// Parse parses a version string, handling optional "v" prefix.
func Parse(version string) (*Version, error) {
	if version == "" {
		return nil, fmt.Errorf("version string is empty")
	}

	original := version
	prefix := ""

	// Handle v prefix
	if strings.HasPrefix(version, "v") || strings.HasPrefix(version, "V") {
		prefix = string(version[0])
		version = version[1:]
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid semver format %q: %w", original, err)
	}

	return &Version{
		original: original,
		prefix:   prefix,
		version:  v,
	}, nil
}

// String returns the version string with its original prefix.
func (v *Version) String() string {
	return v.prefix + v.version.String()
}

// StringWithPrefix returns the version with a specific prefix.
func (v *Version) StringWithPrefix(prefix string) string {
	return prefix + v.version.String()
}

// Major returns the major version number.
func (v *Version) Major() uint64 {
	return v.version.Major()
}

// Minor returns the minor version number.
func (v *Version) Minor() uint64 {
	return v.version.Minor()
}

// Patch returns the patch version number.
func (v *Version) Patch() uint64 {
	return v.version.Patch()
}

// Prerelease returns the prerelease string.
func (v *Version) Prerelease() string {
	return v.version.Prerelease()
}

// IsPrerelease returns true if this is a prerelease version.
func (v *Version) IsPrerelease() bool {
	return v.version.Prerelease() != ""
}

// IncrementMajor returns a new version with major incremented.
func (v *Version) IncrementMajor() *Version {
	inc := v.version.IncMajor()
	return &Version{
		prefix:  v.prefix,
		version: &inc,
	}
}

// IncrementMinor returns a new version with minor incremented.
func (v *Version) IncrementMinor() *Version {
	inc := v.version.IncMinor()
	return &Version{
		prefix:  v.prefix,
		version: &inc,
	}
}

// IncrementPatch returns a new version with patch incremented.
func (v *Version) IncrementPatch() *Version {
	inc := v.version.IncPatch()
	return &Version{
		prefix:  v.prefix,
		version: &inc,
	}
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v *Version) Compare(other *Version) int {
	return v.version.Compare(other.version)
}

// Validate validates a version string without parsing fully.
func Validate(version string) error {
	_, err := Parse(version)
	return err
}

// ValidateWithOptions validates a version with configurable options.
func ValidateWithOptions(version string, allowPrerelease bool) error {
	v, err := Parse(version)
	if err != nil {
		return err
	}

	if !allowPrerelease && v.IsPrerelease() {
		return fmt.Errorf("prerelease versions not allowed: %s", version)
	}

	return nil
}

// FormatTag formats a version as a git tag with the specified prefix.
func FormatTag(version, prefix string) (string, error) {
	v, err := Parse(version)
	if err != nil {
		return "", err
	}

	// Use provided prefix, or default to "v"
	if prefix == "" {
		prefix = "v"
	}

	return v.StringWithPrefix(prefix), nil
}

// NextVersion determines the next version based on increment type.
func NextVersion(current, incrementType string) (string, error) {
	v, err := Parse(current)
	if err != nil {
		return "", err
	}

	var next *Version
	switch strings.ToLower(incrementType) {
	case "major":
		next = v.IncrementMajor()
	case "minor":
		next = v.IncrementMinor()
	case "patch":
		next = v.IncrementPatch()
	default:
		return "", fmt.Errorf("unknown increment type: %s (expected major, minor, or patch)", incrementType)
	}

	return next.String(), nil
}

// IncrementTypeFromLabels determines increment type from issue labels.
func IncrementTypeFromLabels(labels []string, majorLabels, minorLabels, patchLabels []string) string {
	labelSet := make(map[string]bool)
	for _, l := range labels {
		labelSet[strings.ToLower(l)] = true
	}

	// Check major first (highest priority)
	for _, ml := range majorLabels {
		if labelSet[strings.ToLower(ml)] {
			return "major"
		}
	}

	// Then minor
	for _, ml := range minorLabels {
		if labelSet[strings.ToLower(ml)] {
			return "minor"
		}
	}

	// Then patch
	for _, pl := range patchLabels {
		if labelSet[strings.ToLower(pl)] {
			return "patch"
		}
	}

	// Default to patch
	return "patch"
}
