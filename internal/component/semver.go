// internal/component/semver.go

package component

import (
	"fmt"
	"strconv"
	"strings"
)

// Semver represents a semantic version.
type Semver struct {
	Major uint32
	Minor uint32
	Patch uint32
}

// ParseSemver parses a semver string like "1.2.3".
func ParseSemver(s string) (*Semver, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid semver: %s", s)
	}

	major, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %w", err)
	}

	return &Semver{
		Major: uint32(major),
		Minor: uint32(minor),
		Patch: uint32(patch),
	}, nil
}

// SplitVersion splits a name like "pkg@1.0.0" into base name and version.
func SplitVersion(name string) (baseName, version string, hasVersion bool) {
	idx := strings.LastIndex(name, "@")
	if idx == -1 {
		return name, "", false
	}
	return name[:idx], name[idx+1:], true
}

// SemverCompatible checks if available version satisfies required version.
// For major > 0: same major, available minor.patch >= required minor.patch
// For major == 0: same major.minor, available patch >= required patch
//
// When relaxed is true, pre-1.0 versions (0.x.y) match any patch version
// within the same minor version (e.g., 0.2.0 matches 0.2.3).
func SemverCompatible(required, available *Semver, relaxed bool) bool {
	if required.Major != available.Major {
		return false
	}

	// Pre-1.0: breaking changes allowed in minor bumps
	if required.Major == 0 {
		if required.Minor != available.Minor {
			return false
		}
		// Relaxed mode: any patch matches any other patch in same minor
		if relaxed {
			return true
		}
		// Strict mode: available patch must be >= required patch
		return available.Patch >= required.Patch
	}

	// 1.0+: breaking changes only in major bumps
	if available.Minor > required.Minor {
		return true
	}
	if available.Minor == required.Minor {
		return available.Patch >= required.Patch
	}
	return false
}
