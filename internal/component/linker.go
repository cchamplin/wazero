// internal/component/linker.go

package component

import (
	"context"
	"fmt"
	"strings"
)

// HostFunc is a host function that can be called from a component.
type HostFunc func(ctx context.Context, args []Val) ([]Val, error)

// Definition is an item that can satisfy a component import.
type Definition interface {
	definition()
}

// FuncDef is a function definition.
type FuncDef struct {
	Type     *FuncType
	Callback HostFunc
}

func (*FuncDef) definition() {}

// InstanceDef is an instance definition with multiple exports.
type InstanceDef struct {
	Exports        map[string]Definition
	SkipValidation bool // When true, type checker trusts this instance without strict export validation
}

func (*InstanceDef) definition() {}

// ResourceDef is a resource type definition.
type ResourceDef struct {
	Destructor func(rep uint32)
}

func (*ResourceDef) definition() {}

// ComponentDef represents a component definition for component imports.
type ComponentDef struct {
	Component *Component
}

func (*ComponentDef) definition() {}

// ImportedValueDef represents a value definition for component value imports.
// Note: This is distinct from ValueDef in component.go which represents
// binary value data from the component format. ImportedValueDef wraps
// a runtime Val for use in the linker.
type ImportedValueDef struct {
	Value Val
}

func (*ImportedValueDef) definition() {}

// TypeDefDef wraps a TypeDef to implement Definition.
// This is used when passing types as arguments to nested component instantiation.
type TypeDefDef struct {
	TypeDef *TypeDef
}

func (*TypeDefDef) definition() {}

// Linker resolves component imports and instantiates components.
type Linker struct {
	definitions    map[string]Definition
	relaxedSemver  bool
}

// NewLinker creates a new component linker.
func NewLinker() *Linker {
	return &Linker{
		definitions: make(map[string]Definition),
	}
}

// SetRelaxedSemverMatching enables or disables relaxed semver matching.
// When enabled, pre-1.0 versions (0.x.y) match any patch version within
// the same minor version (e.g., 0.2.0 matches 0.2.3).
// By default, strict matching is used where available.Patch >= required.Patch.
func (l *Linker) SetRelaxedSemverMatching(relaxed bool) {
	l.relaxedSemver = relaxed
}

// RelaxedSemverMatching returns whether relaxed semver matching is enabled.
func (l *Linker) RelaxedSemverMatching() bool {
	return l.relaxedSemver
}

// DefineFunc adds a host function definition.
func (l *Linker) DefineFunc(namespace, name string, typ *FuncType, fn HostFunc) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &FuncDef{Type: typ, Callback: fn}
	return nil
}

// DefineResource adds a resource type definition with its destructor.
func (l *Linker) DefineResource(namespace, name string, destructor func(rep uint32)) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &ResourceDef{Destructor: destructor}
	return nil
}

// InstanceBuilder builds an instance definition with multiple exports.
type InstanceBuilder struct {
	linker         *Linker
	namespace      string
	exports        map[string]Definition
	skipValidation bool
}

// DefineInstance starts building an instance definition.
func (l *Linker) DefineInstance(namespace string) *InstanceBuilder {
	return &InstanceBuilder{
		linker:    l,
		namespace: namespace,
		exports:   make(map[string]Definition),
	}
}

// Func adds a function export to the instance.
func (b *InstanceBuilder) Func(name string, typ *FuncType, fn HostFunc) *InstanceBuilder {
	b.exports[name] = &FuncDef{Type: typ, Callback: fn}
	return b
}

// Resource adds a resource type definition with its destructor to the instance.
func (b *InstanceBuilder) Resource(name string, destructor func(rep uint32)) *InstanceBuilder {
	b.exports[name] = &ResourceDef{Destructor: destructor}
	return b
}

// FuncNoType adds a function export without explicit type info.
// Useful for host functions that handle dynamic Val arguments.
func (b *InstanceBuilder) FuncNoType(name string, fn HostFunc) *InstanceBuilder {
	b.exports[name] = &FuncDef{Type: nil, Callback: fn}
	return b
}

// SkipValidation disables validation for this instance definition.
// Use this when providing a partial implementation of a WASI interface.
func (b *InstanceBuilder) SkipValidation() *InstanceBuilder {
	b.skipValidation = true
	return b
}

// Build finalizes the instance definition and registers it with the linker.
// Validation is enabled by default to catch missing required exports.
// Use SkipValidation() to disable for partial implementations.
func (b *InstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports, SkipValidation: b.skipValidation}
	return nil
}

// Get retrieves a definition by its full key.
func (l *Linker) Get(key string) (Definition, bool) {
	def, ok := l.definitions[key]
	return def, ok
}

// MatchImport finds a definition that satisfies the import name.
// Supports semver-compatible matching per component model spec.
// Handles all import name variants:
//   - locked-dep=name:version - Find exact match for the dep name and version
//   - unlocked-dep=name:range - Find highest version matching the range
//   - url=... - Returns "not supported" error
//   - integrity=... - Returns "not supported" error
//   - Plain names and interface names - Existing behavior
func (l *Linker) MatchImport(importName string) (Definition, error) {
	// Parse the import name
	parsed, err := ParseImportName(importName)
	if err != nil {
		// Fall back to existing behavior for unparseable names
		return l.matchLegacyImport(importName)
	}

	switch parsed.Kind {
	case ImportNameKindLockedDep:
		return l.matchLockedDep(parsed)
	case ImportNameKindUnlockedDep:
		return l.matchUnlockedDep(parsed)
	case ImportNameKindURL:
		return nil, fmt.Errorf("URL imports not supported: %s", parsed.URL)
	case ImportNameKindHash:
		return nil, fmt.Errorf("hash imports not supported: %s", parsed.Hash)
	case ImportNameKindPlain:
		return l.matchPlainImport(parsed)
	case ImportNameKindInterface:
		return l.matchInterfaceImport(parsed)
	default:
		return l.matchLegacyImport(importName)
	}
}

// matchLockedDep finds a definition matching "depName@version" exactly.
func (l *Linker) matchLockedDep(parsed *ImportName) (Definition, error) {
	// Construct the key: depName@version
	key := fmt.Sprintf("%s@%d.%d.%d", parsed.DepName, parsed.Version.Major, parsed.Version.Minor, parsed.Version.Patch)

	if def, ok := l.definitions[key]; ok {
		return def, nil
	}

	return nil, fmt.Errorf("no definition found for locked-dep: %s@%d.%d.%d",
		parsed.DepName, parsed.Version.Major, parsed.Version.Minor, parsed.Version.Patch)
}

// matchUnlockedDep finds the highest version matching the range for depName.
func (l *Linker) matchUnlockedDep(parsed *ImportName) (Definition, error) {
	// Parse the version range
	versionRange, err := ParseSemverRange(parsed.VersionRangeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version range in unlocked-dep: %w", err)
	}

	var bestDef Definition
	var bestVersion *Semver

	// Search through all definitions for matching dep name with compatible version
	for defName, def := range l.definitions {
		// Try to extract depName and version from definition key
		baseName, versionStr, hasVersion := SplitVersion(defName)
		if !hasVersion {
			continue
		}

		// Check if base name matches
		if baseName != parsed.DepName {
			continue
		}

		defVersion, err := ParseSemver(versionStr)
		if err != nil {
			continue
		}

		// Check if version matches the range
		if !versionRange.Matches(defVersion) {
			continue
		}

		// Select highest matching version
		if bestVersion == nil || semverGreater(defVersion, bestVersion) {
			bestDef = def
			bestVersion = defVersion
		}
	}

	if bestDef == nil {
		return nil, fmt.Errorf("no definition found for unlocked-dep: %s matching range %s",
			parsed.DepName, parsed.VersionRangeStr)
	}

	return bestDef, nil
}

// matchPlainImport handles direct lookup for plain import names.
func (l *Linker) matchPlainImport(parsed *ImportName) (Definition, error) {
	if def, ok := l.definitions[parsed.Name]; ok {
		return def, nil
	}
	return nil, fmt.Errorf("import not found: %s", parsed.Name)
}

// matchInterfaceImport handles interface imports like "wasi:cli/environment@0.2.0".
func (l *Linker) matchInterfaceImport(parsed *ImportName) (Definition, error) {
	// Reconstruct the full import name for legacy matching
	var importName string
	if parsed.Version != nil {
		importName = fmt.Sprintf("%s/%s@%d.%d.%d", parsed.Namespace, parsed.Name,
			parsed.Version.Major, parsed.Version.Minor, parsed.Version.Patch)
	} else {
		importName = fmt.Sprintf("%s/%s", parsed.Namespace, parsed.Name)
	}

	return l.matchLegacyImport(importName)
}

// matchLegacyImport is the original MatchImport logic for backward compatibility.
func (l *Linker) matchLegacyImport(importName string) (Definition, error) {
	// Parse import name into namespace/name format
	// e.g., "test:api@1.0.0/fn" -> namespace="test:api@1.0.0", name="fn"
	// or "wasi:cli/environment@0.2.0" -> this is an instance import
	lastSlash := strings.LastIndex(importName, "/")
	if lastSlash == -1 {
		return nil, fmt.Errorf("import not found: %s", importName)
	}

	namespace := importName[:lastSlash]
	name := importName[lastSlash+1:]

	// Split namespace into base and version
	baseNamespace, reqVersionStr, hasReqVersion := SplitVersion(namespace)
	if !hasReqVersion {
		// No version in namespace - check if version is in the name part
		// This handles instance imports like "wasi:cli/environment@0.2.0"
		baseName, nameVersionStr, hasNameVersion := SplitVersion(name)
		if hasNameVersion {
			// This is an instance import with version in the name
			reqVersion, err := ParseSemver(nameVersionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid version in import: %w", err)
			}

			// Construct the base key without version
			baseKey := namespace + "/" + baseName

			// Find best compatible match for instance imports
			var bestDef Definition
			var bestVersion *Semver

			for defName, def := range l.definitions {
				// Extract base key and version from definition
				defLastSlash := strings.LastIndex(defName, "/")
				if defLastSlash == -1 {
					continue
				}
				defNamespace := defName[:defLastSlash]
				defName2 := defName[defLastSlash+1:]

				// Check if namespace matches
				if defNamespace != namespace {
					continue
				}

				// Check if base name matches (without version)
				defBaseName, defVersionStr, hasDefVersion := SplitVersion(defName2)
				if defBaseName != baseName {
					continue
				}
				if !hasDefVersion {
					continue
				}

				defVersion, err := ParseSemver(defVersionStr)
				if err != nil {
					continue
				}

				// Check semver compatibility
				if !SemverCompatible(reqVersion, defVersion, l.relaxedSemver) {
					continue
				}

				// Select highest compatible version
				if bestVersion == nil || semverGreater(defVersion, bestVersion) {
					bestDef = def
					bestVersion = defVersion
				}
			}

			if bestDef != nil {
				return bestDef, nil
			}

			return nil, fmt.Errorf("no compatible definition for: %s (base: %s)", importName, baseKey)
		}

		// No version anywhere - try direct match only
		if def, ok := l.definitions[importName]; ok {
			return def, nil
		}
		return nil, fmt.Errorf("import not found: %s", importName)
	}

	reqVersion, err := ParseSemver(reqVersionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version in import: %w", err)
	}

	// Find best compatible match
	var bestDef Definition
	var bestVersion *Semver

	for defName, def := range l.definitions {
		// Check if it's the same function name
		defLastSlash := strings.LastIndex(defName, "/")
		if defLastSlash == -1 {
			continue
		}
		defNamespace := defName[:defLastSlash]
		defFuncName := defName[defLastSlash+1:]

		if defFuncName != name {
			continue
		}

		// Check namespace compatibility
		defBase, defVersionStr, hasDefVersion := SplitVersion(defNamespace)
		if defBase != baseNamespace {
			continue
		}
		if !hasDefVersion {
			continue
		}

		defVersion, err := ParseSemver(defVersionStr)
		if err != nil {
			continue
		}

		// Check semver compatibility
		if !SemverCompatible(reqVersion, defVersion, l.relaxedSemver) {
			continue
		}

		// Select highest compatible version
		if bestVersion == nil || semverGreater(defVersion, bestVersion) {
			bestDef = def
			bestVersion = defVersion
		}
	}

	if bestDef == nil {
		return nil, fmt.Errorf("no compatible definition for: %s", importName)
	}

	return bestDef, nil
}

func semverGreater(a, b *Semver) bool {
	if a.Major != b.Major {
		return a.Major > b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor > b.Minor
	}
	return a.Patch > b.Patch
}

// Instantiate creates an instance of a component with resolved imports.
func (l *Linker) Instantiate(ctx context.Context, c *Component) (*Instance, error) {
	inst := &Instance{
		component: c,
		exports:   make(map[string]*ExportedFunc),
	}

	// Resolve imports
	for _, imp := range c.Imports {
		_, err := l.MatchImport(imp.Name)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", imp.Name, err)
		}
		// Store resolved import for later use during execution
	}

	// Build exports map
	for _, exp := range c.Exports {
		// For now, just track that we have the export
		// Full implementation will wire up actual functions
		inst.exports[exp.Name] = nil
	}

	return inst, nil
}

// getExactExportedFunc finds an exported function by exact name match.
func (i *Instance) getExactExportedFunc(name string) *ExportedFunc {
	for _, exp := range i.component.Exports {
		if exp.Name == name && exp.Kind == ExportKindFunc {
			var funcType *FuncType
			// exp.Idx is the component function index, not the canonical array index.
			// Look up the canonical to get the type.
			if canonIdx, ok := i.component.FuncIdxToCanonical[exp.Idx]; ok {
				if int(canonIdx) < len(i.component.Canonicals) {
					canon := &i.component.Canonicals[canonIdx]
					if int(canon.TypeIdx) < len(i.component.Types) {
						funcType = i.component.Types[canon.TypeIdx].Func
					}
				}
			}
			return &ExportedFunc{
				name:     name,
				funcType: funcType,
				instance: i,
			}
		}
	}
	return nil
}

// GetExportedFunc retrieves an exported function by name.
// Returns nil if not found or if the export is not a function.
// Supports semver-compatible matching for versioned export names.
// When multiple compatible versions exist, returns the highest compatible version.
func (i *Instance) GetExportedFunc(name string) *ExportedFunc {
	// Parse requested name into namespace/name format
	lastSlash := strings.LastIndex(name, "/")
	if lastSlash == -1 {
		// No slash - try exact match only (non-versioned name)
		return i.getExactExportedFunc(name)
	}

	namespace := name[:lastSlash]
	funcName := name[lastSlash+1:]

	// Split namespace into base and version
	baseNamespace, reqVersionStr, hasReqVersion := SplitVersion(namespace)
	if !hasReqVersion {
		// No version in requested name - try exact match only
		return i.getExactExportedFunc(name)
	}

	reqVersion, err := ParseSemver(reqVersionStr)
	if err != nil {
		// Invalid version - try exact match only
		return i.getExactExportedFunc(name)
	}

	// Find best compatible match (highest compatible version)
	var bestExport *Export
	var bestVersion *Semver

	for idx := range i.component.Exports {
		exp := &i.component.Exports[idx]
		if exp.Kind != ExportKindFunc {
			continue
		}

		// Parse export name
		expLastSlash := strings.LastIndex(exp.Name, "/")
		if expLastSlash == -1 {
			continue
		}
		expNamespace := exp.Name[:expLastSlash]
		expFuncName := exp.Name[expLastSlash+1:]

		if expFuncName != funcName {
			continue
		}

		// Check namespace compatibility
		expBase, expVersionStr, hasExpVersion := SplitVersion(expNamespace)
		if expBase != baseNamespace {
			continue
		}
		if !hasExpVersion {
			continue
		}

		expVersion, err := ParseSemver(expVersionStr)
		if err != nil {
			continue
		}

		// For exports, check bidirectional semver compatibility:
		// - Requested old, export new: SemverCompatible(reqVersion, expVersion)
		// - Requested new, export old: SemverCompatible(expVersion, reqVersion)
		// Note: Using strict mode (false) for export matching. Relaxed mode
		// is primarily for import resolution during linking.
		if !SemverCompatible(reqVersion, expVersion, false) && !SemverCompatible(expVersion, reqVersion, false) {
			continue
		}

		// Select highest compatible version
		if bestVersion == nil || semverGreater(expVersion, bestVersion) {
			bestExport = exp
			bestVersion = expVersion
		}
	}

	if bestExport == nil {
		return nil
	}

	var funcType *FuncType
	if int(bestExport.Idx) < len(i.component.Types) {
		funcType = i.component.Types[bestExport.Idx].Func
	}
	return &ExportedFunc{
		name:     bestExport.Name,
		funcType: funcType,
		instance: i,
	}
}

// Type returns the function's type.
func (f *ExportedFunc) Type() *FuncType {
	return f.funcType
}
