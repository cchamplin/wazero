# Phase 5: Advanced Import Names

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support all import name variants from the Component Model spec, including locked/unlocked dependencies and version ranges.

**Architecture:** Create import name parser, extend semver to support ranges, update MatchImport to handle all variants.

**Tech Stack:** Go

**Parent Plan:** [00-root.md](./00-root.md)

**Prerequisite:** None (independent phase)

**Gap Analysis Reference:** [Section 3: Import Resolution](../2026-01-20-instantiation-linking-gap-analysis.md#3-import-resolution)

---

## Spec References

Read these before starting:
- `debug-vendored/component-model/design/mvp/Explainer.md` lines 2507-2634 (Import names)
- `debug-vendored/component-model/design/mvp/Linking.md` (Version ranges)

Import name variants:
- Plain name: `"foo"`
- Interface: `"wasi:io/streams@0.2.0"`
- Locked dep: `"locked-dep=<name>:<version>"`
- Unlocked dep: `"unlocked-dep=<name>:<range>"`
- URL: `"url=https://..."`
- Hash: `"integrity=sha256:..."`

---

## Task 1: Create Import Name Parser

**Files:**
- Create: `internal/component/import_name.go`
- Test: `internal/component/import_name_test.go`

### Step 1: Write the failing test for import name parsing

```go
// internal/component/import_name_test.go
package component

import (
	"testing"
)

func TestParseImportName_Interface(t *testing.T) {
	parsed, err := ParseImportName("wasi:io/streams@0.2.0")
	if err != nil {
		t.Fatalf("ParseImportName failed: %v", err)
	}

	if parsed.Kind != ImportNameInterface {
		t.Errorf("expected Interface kind, got %v", parsed.Kind)
	}
	if parsed.Namespace != "wasi:io" {
		t.Errorf("expected namespace 'wasi:io', got '%s'", parsed.Namespace)
	}
	if parsed.Name != "streams" {
		t.Errorf("expected name 'streams', got '%s'", parsed.Name)
	}
	if parsed.Version == nil {
		t.Fatal("version should not be nil")
	}
	if parsed.Version.String() != "0.2.0" {
		t.Errorf("expected version '0.2.0', got '%s'", parsed.Version.String())
	}
}

func TestParseImportName_LockedDep(t *testing.T) {
	parsed, err := ParseImportName("locked-dep=my-pkg:1.2.3")
	if err != nil {
		t.Fatalf("ParseImportName failed: %v", err)
	}

	if parsed.Kind != ImportNameLockedDep {
		t.Errorf("expected LockedDep kind, got %v", parsed.Kind)
	}
	if parsed.DepName != "my-pkg" {
		t.Errorf("expected depName 'my-pkg', got '%s'", parsed.DepName)
	}
	if parsed.Version == nil {
		t.Fatal("version should not be nil")
	}
}

func TestParseImportName_UnlockedDep(t *testing.T) {
	parsed, err := ParseImportName("unlocked-dep=my-pkg:>=1.0.0")
	if err != nil {
		t.Fatalf("ParseImportName failed: %v", err)
	}

	if parsed.Kind != ImportNameUnlockedDep {
		t.Errorf("expected UnlockedDep kind, got %v", parsed.Kind)
	}
	if parsed.DepName != "my-pkg" {
		t.Errorf("expected depName 'my-pkg', got '%s'", parsed.DepName)
	}
	if parsed.VersionRange == nil {
		t.Fatal("versionRange should not be nil")
	}
}

func TestParseImportName_Plain(t *testing.T) {
	parsed, err := ParseImportName("simple-name")
	if err != nil {
		t.Fatalf("ParseImportName failed: %v", err)
	}

	if parsed.Kind != ImportNamePlain {
		t.Errorf("expected Plain kind, got %v", parsed.Kind)
	}
	if parsed.Raw != "simple-name" {
		t.Errorf("expected raw 'simple-name', got '%s'", parsed.Raw)
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestParseImportName -v`

Expected: FAIL with "undefined: ParseImportName"

### Step 3: Write minimal implementation

```go
// internal/component/import_name.go
package component

import (
	"fmt"
	"strings"
)

// ImportNameKind identifies the type of import name.
type ImportNameKind int

const (
	ImportNamePlain ImportNameKind = iota
	ImportNameInterface
	ImportNameLockedDep
	ImportNameUnlockedDep
	ImportNameURL
	ImportNameHash
)

// ParsedImportName is a parsed import name with its components.
type ParsedImportName struct {
	Kind         ImportNameKind
	Raw          string
	Namespace    string        // For interface names
	Name         string        // For interface names
	Version      *Semver       // For locked deps and interface names
	VersionRange *SemverRange  // For unlocked deps
	DepName      string        // For locked/unlocked deps
	URL          string        // For URL imports
	Hash         string        // For hash imports
}

// ParseImportName parses an import name into its components.
func ParseImportName(raw string) (*ParsedImportName, error) {
	// Check for special prefixes
	if strings.HasPrefix(raw, "locked-dep=") {
		return parseLockedDep(raw)
	}
	if strings.HasPrefix(raw, "unlocked-dep=") {
		return parseUnlockedDep(raw)
	}
	if strings.HasPrefix(raw, "url=") {
		return parseURL(raw)
	}
	if strings.HasPrefix(raw, "integrity=") {
		return parseHash(raw)
	}

	// Try to parse as interface name (has namespace:pkg/name@version)
	if strings.Contains(raw, "/") {
		return parseInterfaceName(raw)
	}

	// Plain name
	return &ParsedImportName{
		Kind: ImportNamePlain,
		Raw:  raw,
	}, nil
}

func parseLockedDep(raw string) (*ParsedImportName, error) {
	// Format: locked-dep=<name>:<version>
	rest := strings.TrimPrefix(raw, "locked-dep=")
	colonIdx := strings.LastIndex(rest, ":")
	if colonIdx == -1 {
		return nil, fmt.Errorf("invalid locked-dep format: %s", raw)
	}

	depName := rest[:colonIdx]
	versionStr := rest[colonIdx+1:]

	version, err := ParseSemver(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version in locked-dep: %w", err)
	}

	return &ParsedImportName{
		Kind:    ImportNameLockedDep,
		Raw:     raw,
		DepName: depName,
		Version: version,
	}, nil
}

func parseUnlockedDep(raw string) (*ParsedImportName, error) {
	// Format: unlocked-dep=<name>:<range>
	rest := strings.TrimPrefix(raw, "unlocked-dep=")
	colonIdx := strings.LastIndex(rest, ":")
	if colonIdx == -1 {
		return nil, fmt.Errorf("invalid unlocked-dep format: %s", raw)
	}

	depName := rest[:colonIdx]
	rangeStr := rest[colonIdx+1:]

	versionRange, err := ParseSemverRange(rangeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version range in unlocked-dep: %w", err)
	}

	return &ParsedImportName{
		Kind:         ImportNameUnlockedDep,
		Raw:          raw,
		DepName:      depName,
		VersionRange: versionRange,
	}, nil
}

func parseURL(raw string) (*ParsedImportName, error) {
	url := strings.TrimPrefix(raw, "url=")
	return &ParsedImportName{
		Kind: ImportNameURL,
		Raw:  raw,
		URL:  url,
	}, nil
}

func parseHash(raw string) (*ParsedImportName, error) {
	hash := strings.TrimPrefix(raw, "integrity=")
	return &ParsedImportName{
		Kind: ImportNameHash,
		Raw:  raw,
		Hash: hash,
	}, nil
}

func parseInterfaceName(raw string) (*ParsedImportName, error) {
	// Format: namespace/name@version or namespace/name
	lastSlash := strings.LastIndex(raw, "/")
	if lastSlash == -1 {
		return nil, fmt.Errorf("invalid interface name: %s", raw)
	}

	namespace := raw[:lastSlash]
	nameWithVersion := raw[lastSlash+1:]

	// Check for version
	name, versionStr, hasVersion := SplitVersion(nameWithVersion)
	if !hasVersion {
		// No version - try namespace for version
		baseNS, nsVersion, hasNSVersion := SplitVersion(namespace)
		if hasNSVersion {
			namespace = baseNS
			version, err := ParseSemver(nsVersion)
			if err != nil {
				return nil, fmt.Errorf("invalid version: %w", err)
			}
			return &ParsedImportName{
				Kind:      ImportNameInterface,
				Raw:       raw,
				Namespace: namespace,
				Name:      nameWithVersion,
				Version:   version,
			}, nil
		}

		return &ParsedImportName{
			Kind:      ImportNameInterface,
			Raw:       raw,
			Namespace: namespace,
			Name:      name,
		}, nil
	}

	version, err := ParseSemver(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}

	return &ParsedImportName{
		Kind:      ImportNameInterface,
		Raw:       raw,
		Namespace: namespace,
		Name:      name,
		Version:   version,
	}, nil
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestParseImportName -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/import_name.go internal/component/import_name_test.go
git commit -m "feat(component): add import name parser

Parses all import name variants:
- Plain names
- Interface names with versions
- Locked dependencies
- Unlocked dependencies
- URL and hash imports

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 2: Implement SemverRange

**Files:**
- Modify: `internal/component/semver.go`
- Test: `internal/component/semver_test.go`

### Step 1: Write the failing test for semver ranges

```go
// Add to internal/component/semver_test.go

func TestParseSemverRange_MatchAll(t *testing.T) {
	r, err := ParseSemverRange("*")
	if err != nil {
		t.Fatalf("ParseSemverRange failed: %v", err)
	}
	if !r.MatchAll {
		t.Error("expected MatchAll to be true")
	}

	// Should match any version
	v1, _ := ParseSemver("0.1.0")
	v2, _ := ParseSemver("5.0.0")
	if !r.Matches(v1) {
		t.Error("should match 0.1.0")
	}
	if !r.Matches(v2) {
		t.Error("should match 5.0.0")
	}
}

func TestParseSemverRange_LowerBound(t *testing.T) {
	r, err := ParseSemverRange(">=1.0.0")
	if err != nil {
		t.Fatalf("ParseSemverRange failed: %v", err)
	}

	below, _ := ParseSemver("0.9.0")
	atMin, _ := ParseSemver("1.0.0")
	above, _ := ParseSemver("2.0.0")

	if r.Matches(below) {
		t.Error("should not match 0.9.0")
	}
	if !r.Matches(atMin) {
		t.Error("should match 1.0.0")
	}
	if !r.Matches(above) {
		t.Error("should match 2.0.0")
	}
}

func TestParseSemverRange_UpperBound(t *testing.T) {
	r, err := ParseSemverRange("<2.0.0")
	if err != nil {
		t.Fatalf("ParseSemverRange failed: %v", err)
	}

	below, _ := ParseSemver("1.5.0")
	atMax, _ := ParseSemver("2.0.0")
	above, _ := ParseSemver("2.1.0")

	if !r.Matches(below) {
		t.Error("should match 1.5.0")
	}
	if r.Matches(atMax) {
		t.Error("should not match 2.0.0 (exclusive)")
	}
	if r.Matches(above) {
		t.Error("should not match 2.1.0")
	}
}

func TestParseSemverRange_Combined(t *testing.T) {
	// Combined constraints: {>=1.0.0 <2.0.0}
	r, err := ParseSemverRange("{>=1.0.0 <2.0.0}")
	if err != nil {
		t.Fatalf("ParseSemverRange failed: %v", err)
	}

	below, _ := ParseSemver("0.9.0")
	inRange, _ := ParseSemver("1.5.0")
	atMax, _ := ParseSemver("2.0.0")

	if r.Matches(below) {
		t.Error("should not match 0.9.0")
	}
	if !r.Matches(inRange) {
		t.Error("should match 1.5.0")
	}
	if r.Matches(atMax) {
		t.Error("should not match 2.0.0")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestParseSemverRange -v`

Expected: FAIL with "undefined: ParseSemverRange"

### Step 3: Write minimal implementation

```go
// Add to internal/component/semver.go

// SemverRange represents a range of semantic versions.
type SemverRange struct {
	Min          *Semver
	Max          *Semver
	MinInclusive bool // >= vs >
	MaxInclusive bool // <= vs <
	MatchAll     bool // * matches everything
}

// ParseSemverRange parses a version range expression.
// Supported formats:
//   - "*" - matches all versions
//   - ">=1.0.0" - lower bound (inclusive)
//   - ">1.0.0" - lower bound (exclusive)
//   - "<2.0.0" - upper bound (exclusive)
//   - "<=2.0.0" - upper bound (inclusive)
//   - "{>=1.0.0 <2.0.0}" - combined range
func ParseSemverRange(s string) (*SemverRange, error) {
	s = strings.TrimSpace(s)

	// Match all: *
	if s == "*" {
		return &SemverRange{MatchAll: true}, nil
	}

	// Combined range: {constraints}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		return parseRangeExpression(inner)
	}

	// Single constraint
	return parseSingleConstraint(s)
}

func parseRangeExpression(s string) (*SemverRange, error) {
	// Split on whitespace for multiple constraints
	parts := strings.Fields(s)
	r := &SemverRange{}

	for _, part := range parts {
		constraint, err := parseSingleConstraint(part)
		if err != nil {
			return nil, err
		}

		// Merge constraints
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

	// Check min bound
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

	// Check max bound
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
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestParseSemverRange -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/semver.go internal/component/semver_test.go
git commit -m "feat(component): implement SemverRange for version constraints

Supports:
- Match all (*)
- Lower bounds (>=, >)
- Upper bounds (<=, <)
- Combined ranges ({>=1.0.0 <2.0.0})

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 3: Update MatchImport for Advanced Names

**Files:**
- Modify: `internal/component/linker.go`
- Test: `internal/component/linker_test.go`

### Step 1: Write the failing test for advanced import matching

```go
// Add to internal/component/linker_test.go

func TestLinker_MatchLockedDep(t *testing.T) {
	l := NewLinker()

	// Define the dependency
	err := l.DefineInstance("my-pkg@1.2.3").
		Func("fn", nil, func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	if err != nil {
		t.Fatalf("DefineInstance failed: %v", err)
	}

	// Match locked-dep import
	def, err := l.MatchImport("locked-dep=my-pkg:1.2.3")
	if err != nil {
		t.Fatalf("MatchImport failed: %v", err)
	}
	if def == nil {
		t.Error("should find locked-dep")
	}
}

func TestLinker_MatchUnlockedDep(t *testing.T) {
	l := NewLinker()

	// Define multiple versions
	for _, ver := range []string{"1.0.0", "1.5.0", "2.0.0"} {
		err := l.DefineInstance("my-pkg@" + ver).
			Func("fn", nil, func(ctx context.Context, args []Val) ([]Val, error) {
				return nil, nil
			}).
			Build()
		if err != nil {
			t.Fatalf("DefineInstance failed: %v", err)
		}
	}

	// Match unlocked-dep with range
	def, err := l.MatchImport("unlocked-dep=my-pkg:{>=1.0.0 <2.0.0}")
	if err != nil {
		t.Fatalf("MatchImport failed: %v", err)
	}
	if def == nil {
		t.Error("should find unlocked-dep")
	}
	// Should get highest in range (1.5.0)
}

func TestLinker_MatchAll(t *testing.T) {
	l := NewLinker()

	// Define some versions
	for _, ver := range []string{"0.1.0", "1.0.0", "3.0.0"} {
		err := l.DefineInstance("pkg@" + ver).
			Func("fn", nil, func(ctx context.Context, args []Val) ([]Val, error) {
				return nil, nil
			}).
			Build()
		if err != nil {
			t.Fatalf("DefineInstance failed: %v", err)
		}
	}

	// Match all versions
	def, err := l.MatchImport("unlocked-dep=pkg:*")
	if err != nil {
		t.Fatalf("MatchImport failed: %v", err)
	}
	if def == nil {
		t.Error("should find highest version")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run "TestLinker_Match(Locked|Unlocked|All)" -v`

Expected: FAIL

### Step 3: Update MatchImport implementation

```go
// Modify MatchImport in internal/component/linker.go

func (l *Linker) MatchImport(importName string) (Definition, error) {
	// Parse the import name
	parsed, err := ParseImportName(importName)
	if err != nil {
		// Fall back to existing behavior for unparseable names
		return l.matchSimpleImport(importName)
	}

	switch parsed.Kind {
	case ImportNameLockedDep:
		return l.matchLockedDep(parsed)
	case ImportNameUnlockedDep:
		return l.matchUnlockedDep(parsed)
	case ImportNameURL:
		return nil, fmt.Errorf("URL imports not supported: %s", parsed.URL)
	case ImportNameHash:
		return nil, fmt.Errorf("hash imports not supported: %s", parsed.Hash)
	case ImportNamePlain:
		return l.matchSimpleImport(importName)
	case ImportNameInterface:
		return l.matchInterfaceImport(parsed)
	default:
		return l.matchSimpleImport(importName)
	}
}

func (l *Linker) matchLockedDep(parsed *ParsedImportName) (Definition, error) {
	// Look for exact match: depName@version
	key := fmt.Sprintf("%s@%s", parsed.DepName, parsed.Version.String())
	if def, ok := l.definitions[key]; ok {
		return def, nil
	}

	// Try without slash (for top-level definitions)
	for defName, def := range l.definitions {
		baseName, verStr, hasVer := SplitVersion(defName)
		if !hasVer {
			continue
		}
		if baseName != parsed.DepName {
			continue
		}
		ver, err := ParseSemver(verStr)
		if err != nil {
			continue
		}
		if ver.String() == parsed.Version.String() {
			return def, nil
		}
	}

	return nil, fmt.Errorf("locked-dep not found: %s@%s", parsed.DepName, parsed.Version.String())
}

func (l *Linker) matchUnlockedDep(parsed *ParsedImportName) (Definition, error) {
	var bestDef Definition
	var bestVersion *Semver

	for defName, def := range l.definitions {
		baseName, verStr, hasVer := SplitVersion(defName)
		if !hasVer {
			continue
		}
		if baseName != parsed.DepName {
			continue
		}

		ver, err := ParseSemver(verStr)
		if err != nil {
			continue
		}

		if !parsed.VersionRange.Matches(ver) {
			continue
		}

		// Select highest matching version
		if bestVersion == nil || semverGreater(ver, bestVersion) {
			bestDef = def
			bestVersion = ver
		}
	}

	if bestDef == nil {
		return nil, fmt.Errorf("no version matches range for: %s", parsed.DepName)
	}

	return bestDef, nil
}

func (l *Linker) matchInterfaceImport(parsed *ParsedImportName) (Definition, error) {
	// Reconstruct the original matching logic but with parsed components
	if parsed.Version == nil {
		// No version - try exact match
		key := fmt.Sprintf("%s/%s", parsed.Namespace, parsed.Name)
		if def, ok := l.definitions[key]; ok {
			return def, nil
		}
		return nil, fmt.Errorf("import not found: %s", parsed.Raw)
	}

	// With version - find best compatible match
	var bestDef Definition
	var bestVersion *Semver

	for defName, def := range l.definitions {
		// ... existing semver matching logic
		// Extract namespace and name from defName
		// Compare with parsed.Namespace and parsed.Name
		// Check version compatibility
		// Select highest compatible

		// Simplified: try exact match first
		key := fmt.Sprintf("%s/%s@%s", parsed.Namespace, parsed.Name, parsed.Version.String())
		if defName == key {
			return def, nil
		}

		// Try semver compatible match
		defLastSlash := strings.LastIndex(defName, "/")
		if defLastSlash == -1 {
			continue
		}
		defNS := defName[:defLastSlash]
		defNameWithVer := defName[defLastSlash+1:]

		defName2, defVerStr, hasDefVer := SplitVersion(defNameWithVer)
		if defName2 != parsed.Name {
			continue
		}

		defNSBase, _, _ := SplitVersion(defNS)
		if defNSBase != parsed.Namespace {
			continue
		}

		if !hasDefVer {
			continue
		}

		defVer, err := ParseSemver(defVerStr)
		if err != nil {
			continue
		}

		if !SemverCompatible(parsed.Version, defVer, l.relaxedSemver) {
			continue
		}

		if bestVersion == nil || semverGreater(defVer, bestVersion) {
			bestDef = def
			bestVersion = defVer
		}
	}

	if bestDef != nil {
		return bestDef, nil
	}

	return nil, fmt.Errorf("no compatible definition for: %s", parsed.Raw)
}

func (l *Linker) matchSimpleImport(importName string) (Definition, error) {
	// Direct lookup
	if def, ok := l.definitions[importName]; ok {
		return def, nil
	}

	// Existing matching logic...
	// (keep the existing MatchImport body for backward compatibility)
	// ... rest of existing implementation
	return nil, fmt.Errorf("import not found: %s", importName)
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run "TestLinker_Match" -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/linker.go internal/component/linker_test.go
git commit -m "feat(component): update MatchImport for advanced import names

Supports:
- locked-dep=name:version
- unlocked-dep=name:range
- Version ranges (* and {constraints})

URL and hash imports return clear error messages.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 4: Run Phase 5 Regression Tests

**Files:** None (verification only)

### Step 1: Run all import name tests

Run: `CGO_ENABLED=0 go test ./internal/component/... -run "ParseImportName|SemverRange|MatchImport" -v`

Expected: All PASS

### Step 2: Run calculator regression tests

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/add -v`

Expected: PASS

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/subtract -v`

Expected: PASS

### Step 3: Update progress tracker

Edit `docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md`:
- Mark Phase 5 status as `[x] Complete`
- Mark Phase 5 regression as `[x] Verified`

### Step 4: Commit

```bash
git add docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md
git commit -m "docs: mark Phase 5 (Advanced Imports) complete

All import name parsing and matching tests pass.
Calculator add/subtract regression tests pass.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Phase 5 Complete

**Summary of changes:**
- Created import name parser for all variants
- Implemented SemverRange for version constraints
- Updated MatchImport to handle locked-dep, unlocked-dep, and ranges
- URL and hash imports return clear "not supported" errors

**All Phases Complete!**

The instantiation and linking system is now spec-compliant for:
- Type checking (Phase 1)
- Start function execution (Phase 2)
- Nested component instantiation (Phase 3)
- Export instance API access (Phase 4)
- Advanced import name variants (Phase 5)
