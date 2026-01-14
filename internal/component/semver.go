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
