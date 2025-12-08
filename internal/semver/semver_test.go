package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidVersions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"V1.2.3", "V1.2.3"},
		{"0.0.1", "0.0.1"},
		{"10.20.30", "10.20.30"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			v, err := Parse(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, v.String())
		})
	}
}

func TestParse_PrereleaseVersions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3-alpha.1", "1.2.3-alpha.1"},
		{"v1.2.3-beta.2", "v1.2.3-beta.2"},
		{"1.0.0-rc.1", "1.0.0-rc.1"},
		{"2.0.0-alpha", "2.0.0-alpha"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			v, err := Parse(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, v.String())
			assert.True(t, v.IsPrerelease())
		})
	}
}

func TestParse_InvalidVersions(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"a.b.c",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			assert.Error(t, err)
		})
	}
}

func TestVersion_Components(t *testing.T) {
	v, err := Parse("v1.2.3-alpha.1")
	require.NoError(t, err)

	assert.Equal(t, uint64(1), v.Major())
	assert.Equal(t, uint64(2), v.Minor())
	assert.Equal(t, uint64(3), v.Patch())
	assert.Equal(t, "alpha.1", v.Prerelease())
}

func TestVersion_Increment(t *testing.T) {
	v, err := Parse("v1.2.3")
	require.NoError(t, err)

	major := v.IncrementMajor()
	assert.Equal(t, "v2.0.0", major.String())

	minor := v.IncrementMinor()
	assert.Equal(t, "v1.3.0", minor.String())

	patch := v.IncrementPatch()
	assert.Equal(t, "v1.2.4", patch.String())
}

func TestVersion_Compare(t *testing.T) {
	v1, _ := Parse("1.2.3")
	v2, _ := Parse("1.2.4")
	v3, _ := Parse("1.2.3")

	assert.Equal(t, -1, v1.Compare(v2))
	assert.Equal(t, 1, v2.Compare(v1))
	assert.Equal(t, 0, v1.Compare(v3))
}

func TestValidate(t *testing.T) {
	assert.NoError(t, Validate("1.2.3"))
	assert.NoError(t, Validate("v1.2.3"))
	assert.NoError(t, Validate("1.2.3-alpha"))
	assert.Error(t, Validate("invalid"))
	assert.Error(t, Validate(""))
}

func TestValidateWithOptions(t *testing.T) {
	// Allow prerelease
	assert.NoError(t, ValidateWithOptions("1.2.3-alpha", true))

	// Disallow prerelease
	assert.Error(t, ValidateWithOptions("1.2.3-alpha", false))
	assert.NoError(t, ValidateWithOptions("1.2.3", false))
}

func TestFormatTag(t *testing.T) {
	tests := []struct {
		version  string
		prefix   string
		expected string
	}{
		{"1.2.3", "v", "v1.2.3"},
		{"v1.2.3", "v", "v1.2.3"},
		{"1.2.3", "", "v1.2.3"}, // Default prefix
		{"1.2.3", "release-", "release-1.2.3"},
	}

	for _, tc := range tests {
		t.Run(tc.version+"_"+tc.prefix, func(t *testing.T) {
			result, err := FormatTag(tc.version, tc.prefix)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNextVersion(t *testing.T) {
	tests := []struct {
		current       string
		incrementType string
		expected      string
	}{
		{"1.2.3", "patch", "1.2.4"},
		{"1.2.3", "minor", "1.3.0"},
		{"1.2.3", "major", "2.0.0"},
		{"v1.2.3", "patch", "v1.2.4"},
		{"v0.0.1", "minor", "v0.1.0"},
	}

	for _, tc := range tests {
		t.Run(tc.current+"_"+tc.incrementType, func(t *testing.T) {
			result, err := NextVersion(tc.current, tc.incrementType)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNextVersion_InvalidType(t *testing.T) {
	_, err := NextVersion("1.2.3", "invalid")
	assert.ErrorContains(t, err, "unknown increment type")
}

func TestIncrementTypeFromLabels(t *testing.T) {
	majorLabels := []string{"breaking", "major"}
	minorLabels := []string{"feature", "enhancement"}
	patchLabels := []string{"fix", "bugfix"}

	tests := []struct {
		labels   []string
		expected string
	}{
		{[]string{"breaking"}, "major"},
		{[]string{"MAJOR"}, "major"}, // Case insensitive
		{[]string{"feature"}, "minor"},
		{[]string{"enhancement"}, "minor"},
		{[]string{"fix"}, "patch"},
		{[]string{"bugfix"}, "patch"},
		{[]string{}, "patch"}, // Default
		{[]string{"other"}, "patch"},
		{[]string{"breaking", "feature"}, "major"}, // Major wins
		{[]string{"feature", "fix"}, "minor"},      // Minor wins over patch
	}

	for _, tc := range tests {
		result := IncrementTypeFromLabels(tc.labels, majorLabels, minorLabels, patchLabels)
		assert.Equal(t, tc.expected, result, "labels: %v", tc.labels)
	}
}
