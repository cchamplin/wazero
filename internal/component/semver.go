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

// SemverRange represents a range of semantic versions.
type SemverRange struct {
	Min          *Semver
	Max          *Semver
	MinInclusive bool // >= vs >
	MaxInclusive bool // <= vs <
	MatchAll     bool // * matches everything
}

// ParseSemverRange parses a version range expression.
func ParseSemverRange(s string) (*SemverRange, error) {
	s = strings.TrimSpace(s)

	if s == "*" {
		return &SemverRange{MatchAll: true}, nil
	}

	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		return parseRangeExpression(inner)
	}

	return parseSingleConstraint(s)
}

func parseRangeExpression(s string) (*SemverRange, error) {
	parts := strings.Fields(s)
	r := &SemverRange{}

	for _, part := range parts {
		constraint, err := parseSingleConstraint(part)
		if err != nil {
			return nil, err
		}

		if constraint.Min != nil {
			r.Min = constraint.Min
			r.MinInclusive = constraint.MinInclusive
		}
		if constraint.Max != nil {
			r.Max = constraint.Max
			r.MaxInclusive = constraint.MaxInclusive
		}
	}

	return r, nil
}

func parseSingleConstraint(s string) (*SemverRange, error) {
	r := &SemverRange{}

	if strings.HasPrefix(s, ">=") {
		ver, err := ParseSemver(strings.TrimPrefix(s, ">="))
		if err != nil {
			return nil, err
		}
		r.Min = ver
		r.MinInclusive = true
	} else if strings.HasPrefix(s, ">") {
		ver, err := ParseSemver(strings.TrimPrefix(s, ">"))
		if err != nil {
			return nil, err
		}
		r.Min = ver
		r.MinInclusive = false
	} else if strings.HasPrefix(s, "<=") {
		ver, err := ParseSemver(strings.TrimPrefix(s, "<="))
		if err != nil {
			return nil, err
		}
		r.Max = ver
		r.MaxInclusive = true
	} else if strings.HasPrefix(s, "<") {
		ver, err := ParseSemver(strings.TrimPrefix(s, "<"))
		if err != nil {
			return nil, err
		}
		r.Max = ver
		r.MaxInclusive = false
	} else {
		// Exact version match
		ver, err := ParseSemver(s)
		if err != nil {
			return nil, err
		}
		r.Min = ver
		r.Max = ver
		r.MinInclusive = true
		r.MaxInclusive = true
	}

	return r, nil
}

// Matches checks if a version satisfies this range.
func (r *SemverRange) Matches(v *Semver) bool {
	if r.MatchAll {
		return true
	}

	if r.Min != nil {
		cmp := compareSemver(v, r.Min)
		if r.MinInclusive {
			if cmp < 0 {
				return false
			}
		} else {
			if cmp <= 0 {
				return false
			}
		}
	}

	if r.Max != nil {
		cmp := compareSemver(v, r.Max)
		if r.MaxInclusive {
			if cmp > 0 {
				return false
			}
		} else {
			if cmp >= 0 {
				return false
			}
		}
	}

	return true
}

// compareSemver compares two semver versions.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareSemver(a, b *Semver) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return 0
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
