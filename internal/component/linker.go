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
	Exports map[string]Definition
}

func (*InstanceDef) definition() {}

// ResourceDef is a resource type definition.
type ResourceDef struct {
	Destructor func(rep uint32)
}

func (*ResourceDef) definition() {}

// Linker resolves component imports and instantiates components.
type Linker struct {
	definitions map[string]Definition
}

// NewLinker creates a new component linker.
func NewLinker() *Linker {
	return &Linker{
		definitions: make(map[string]Definition),
	}
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
	linker    *Linker
	namespace string
	exports   map[string]Definition
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

// Build finalizes the instance definition and registers it with the linker.
func (b *InstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports}
	return nil
}

// Get retrieves a definition by its full key.
func (l *Linker) Get(key string) (Definition, bool) {
	def, ok := l.definitions[key]
	return def, ok
}

// MatchImport finds a definition that satisfies the import name.
// Supports semver-compatible matching per component model spec.
func (l *Linker) MatchImport(importName string) (Definition, error) {
	// Parse import name into namespace/name format
	// e.g., "test:api@1.0.0/fn" -> namespace="test:api@1.0.0", name="fn"
	lastSlash := strings.LastIndex(importName, "/")
	if lastSlash == -1 {
		return nil, fmt.Errorf("import not found: %s", importName)
	}

	namespace := importName[:lastSlash]
	name := importName[lastSlash+1:]

	// Split namespace into base and version
	baseNamespace, reqVersionStr, hasReqVersion := SplitVersion(namespace)
	if !hasReqVersion {
		// No version - try direct match only
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
		if !SemverCompatible(reqVersion, defVersion) {
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
			if int(exp.Idx) < len(i.component.Types) {
				funcType = i.component.Types[exp.Idx].Func
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
		if !SemverCompatible(reqVersion, expVersion) && !SemverCompatible(expVersion, reqVersion) {
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
