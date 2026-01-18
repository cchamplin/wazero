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
		result := SemverCompatible(tt.required, tt.available, false)
		require.Equal(t, tt.compatible, result,
			"required=%v available=%v", tt.required, tt.available)
	}
}

func TestSemverCompatibleRelaxed(t *testing.T) {
	tests := []struct {
		name       string
		required   *Semver
		available  *Semver
		relaxed    bool
		compatible bool
	}{
		// Strict mode (relaxed=false): 0.x.y requires available.Patch >= required.Patch
		{"strict: 0.2.0 requires 0.2.3 - incompatible", &Semver{0, 2, 3}, &Semver{0, 2, 0}, false, false},
		{"strict: 0.2.0 provides 0.2.3 - compatible", &Semver{0, 2, 0}, &Semver{0, 2, 3}, false, true},
		{"strict: 0.2.3 exact match", &Semver{0, 2, 3}, &Semver{0, 2, 3}, false, true},

		// Relaxed mode (relaxed=true): any 0.x.* matches any other 0.x.*
		{"relaxed: 0.2.0 requires 0.2.3 - compatible", &Semver{0, 2, 3}, &Semver{0, 2, 0}, true, true},
		{"relaxed: 0.2.3 provides 0.2.0 - compatible", &Semver{0, 2, 0}, &Semver{0, 2, 3}, true, true},
		{"relaxed: 0.2.3 exact match", &Semver{0, 2, 3}, &Semver{0, 2, 3}, true, true},

		// Relaxed mode still requires same minor version
		{"relaxed: 0.2.x vs 0.3.x - incompatible", &Semver{0, 2, 0}, &Semver{0, 3, 0}, true, false},
		{"relaxed: 0.3.x vs 0.2.x - incompatible", &Semver{0, 3, 0}, &Semver{0, 2, 0}, true, false},

		// Relaxed mode doesn't affect 1.x+ versions
		{"relaxed: 1.0.0 vs 1.0.1 - compatible", &Semver{1, 0, 0}, &Semver{1, 0, 1}, true, true},
		{"relaxed: 1.0.1 vs 1.0.0 - incompatible", &Semver{1, 0, 1}, &Semver{1, 0, 0}, true, false},
		{"relaxed: 1.1.0 vs 1.0.0 - incompatible", &Semver{1, 1, 0}, &Semver{1, 0, 0}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SemverCompatible(tt.required, tt.available, tt.relaxed)
			require.Equal(t, tt.compatible, result,
				"required=%v available=%v relaxed=%v", tt.required, tt.available, tt.relaxed)
		})
	}
}
