// internal/component/import_name.go

package component

import (
	"fmt"
	"strings"
)

// ImportNameKind represents the kind of import name.
type ImportNameKind int

const (
	// ImportNameKindPlain is a plain import name like "foo".
	ImportNameKindPlain ImportNameKind = iota
	// ImportNameKindInterface is an interface import like "wasi:io/streams@0.2.0".
	ImportNameKindInterface
	// ImportNameKindLockedDep is a locked dependency like "locked-dep=my-pkg:1.2.3".
	ImportNameKindLockedDep
	// ImportNameKindUnlockedDep is an unlocked dependency like "unlocked-dep=my-pkg:>=1.0.0".
	ImportNameKindUnlockedDep
	// ImportNameKindURL is a URL import like "url=https://example.com".
	ImportNameKindURL
	// ImportNameKindHash is a hash/integrity import like "integrity=sha256:abc123".
	ImportNameKindHash
)

// ImportName represents a parsed import name from the Component Model.
type ImportName struct {
	// Kind indicates which type of import name this is.
	Kind ImportNameKind

	// Name is used for plain names and interface names.
	Name string

	// Namespace is used for interface names (e.g., "wasi:io" in "wasi:io/streams@0.2.0").
	Namespace string

	// Version is used for interface names and locked dependencies.
	Version *Semver

	// DepName is used for locked and unlocked dependencies.
	DepName string

	// VersionRangeStr stores the version range string for unlocked deps.
	// This will be replaced with a proper SemverRange in Task 2.
	VersionRangeStr string

	// URL is used for URL imports.
	URL string

	// Hash is used for integrity/hash imports.
	Hash string
}

// ParseImportName parses an import name string according to the Component Model spec.
// It handles all import name variants:
//   - Plain name: "foo"
//   - Interface: "wasi:io/streams@0.2.0"
//   - Locked dep: "locked-dep=my-pkg:1.2.3"
//   - Unlocked dep: "unlocked-dep=my-pkg:>=1.0.0"
//   - URL: "url=https://..."
//   - Hash: "integrity=sha256:..."
func ParseImportName(s string) (*ImportName, error) {
	// Check for prefixed forms first
	if strings.HasPrefix(s, "locked-dep=") {
		return parseLockedDep(s)
	}
	if strings.HasPrefix(s, "unlocked-dep=") {
		return parseUnlockedDep(s)
	}
	if strings.HasPrefix(s, "url=") {
		return parseURL(s)
	}
	if strings.HasPrefix(s, "integrity=") {
		return parseHash(s)
	}

	// Check for interface name (contains / which separates namespace from interface name)
	if strings.Contains(s, "/") {
		return parseInterface(s)
	}

	// Otherwise it's a plain name
	return &ImportName{
		Kind: ImportNameKindPlain,
		Name: s,
	}, nil
}

// parseInterface parses an interface import name like "wasi:io/streams@0.2.0".
func parseInterface(s string) (*ImportName, error) {
	// Split by "/" to get namespace and interface name
	slashIdx := strings.LastIndex(s, "/")
	if slashIdx == -1 {
		return nil, fmt.Errorf("invalid interface name: missing /: %s", s)
	}

	namespace := s[:slashIdx]
	rest := s[slashIdx+1:]

	// Split off the version if present
	name, versionStr, hasVersion := SplitVersion(rest)

	result := &ImportName{
		Kind:      ImportNameKindInterface,
		Namespace: namespace,
		Name:      name,
	}

	if hasVersion {
		version, err := ParseSemver(versionStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interface version: %w", err)
		}
		result.Version = version
	}

	return result, nil
}

// parseLockedDep parses a locked dependency like "locked-dep=my-pkg:1.2.3".
func parseLockedDep(s string) (*ImportName, error) {
	// Remove the prefix
	value := strings.TrimPrefix(s, "locked-dep=")

	// Split by ":" to get dep name and version
	colonIdx := strings.LastIndex(value, ":")
	if colonIdx == -1 {
		return nil, fmt.Errorf("invalid locked-dep: missing version separator: %s", s)
	}

	depName := value[:colonIdx]
	versionStr := value[colonIdx+1:]

	version, err := ParseSemver(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid locked-dep version: %w", err)
	}

	return &ImportName{
		Kind:    ImportNameKindLockedDep,
		DepName: depName,
		Version: version,
	}, nil
}

// parseUnlockedDep parses an unlocked dependency like "unlocked-dep=my-pkg:>=1.0.0".
func parseUnlockedDep(s string) (*ImportName, error) {
	// Remove the prefix
	value := strings.TrimPrefix(s, "unlocked-dep=")

	// Split by ":" to get dep name and version range
	colonIdx := strings.Index(value, ":")
	if colonIdx == -1 {
		return nil, fmt.Errorf("invalid unlocked-dep: missing version range separator: %s", s)
	}

	depName := value[:colonIdx]
	versionRangeStr := value[colonIdx+1:]

	return &ImportName{
		Kind:            ImportNameKindUnlockedDep,
		DepName:         depName,
		VersionRangeStr: versionRangeStr,
	}, nil
}

// parseURL parses a URL import like "url=https://example.com".
func parseURL(s string) (*ImportName, error) {
	url := strings.TrimPrefix(s, "url=")
	return &ImportName{
		Kind: ImportNameKindURL,
		URL:  url,
	}, nil
}

// parseHash parses a hash/integrity import like "integrity=sha256:abc123".
func parseHash(s string) (*ImportName, error) {
	hash := strings.TrimPrefix(s, "integrity=")
	return &ImportName{
		Kind: ImportNameKindHash,
		Hash: hash,
	}, nil
}
