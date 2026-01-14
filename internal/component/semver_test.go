// internal/component/semver_test.go

package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input    string
		expected *Semver
	}{
		{"1.0.0", &Semver{Major: 1, Minor: 0, Patch: 0}},
		{"0.2.0", &Semver{Major: 0, Minor: 2, Patch: 0}},
		{"2.1.3", &Semver{Major: 2, Minor: 1, Patch: 3}},
	}

	for _, tt := range tests {
		result, err := ParseSemver(tt.input)
		require.NoError(t, err)
		require.Equal(t, tt.expected.Major, result.Major)
		require.Equal(t, tt.expected.Minor, result.Minor)
		require.Equal(t, tt.expected.Patch, result.Patch)
	}
}

func TestParseSemver_Invalid(t *testing.T) {
	tests := []string{
		"invalid",
		"1.0",
		"1",
		"",
	}

	for _, input := range tests {
		_, err := ParseSemver(input)
		require.Error(t, err)
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		input      string
		baseName   string
		version    string
		hasVersion bool
	}{
		{"wasi:cli/env@0.2.0", "wasi:cli/env", "0.2.0", true},
		{"test-api", "test-api", "", false},
		{"pkg@1.0.0", "pkg", "1.0.0", true},
	}

	for _, tt := range tests {
		base, ver, hasVer := SplitVersion(tt.input)
		require.Equal(t, tt.baseName, base)
		require.Equal(t, tt.version, ver)
		require.Equal(t, tt.hasVersion, hasVer)
	}
}

func TestSemverCompatible(t *testing.T) {
	tests := []struct {
		required   *Semver
		available  *Semver
		compatible bool
	}{
		// Same version is compatible
		{&Semver{1, 0, 0}, &Semver{1, 0, 0}, true},
		// Newer patch is compatible
		{&Semver{1, 0, 0}, &Semver{1, 0, 1}, true},
		// Newer minor is compatible
		{&Semver{1, 0, 0}, &Semver{1, 1, 0}, true},
		// Different major is not compatible
		{&Semver{1, 0, 0}, &Semver{2, 0, 0}, false},
		// Older version is not compatible
		{&Semver{1, 1, 0}, &Semver{1, 0, 0}, false},
		// Pre-1.0 versions: same minor required
		{&Semver{0, 2, 0}, &Semver{0, 2, 1}, true},
		{&Semver{0, 2, 0}, &Semver{0, 3, 0}, false},
	}

	for _, tt := range tests {
		result := SemverCompatible(tt.required, tt.available)
		require.Equal(t, tt.compatible, result,
			"required=%v available=%v", tt.required, tt.available)
	}
}
