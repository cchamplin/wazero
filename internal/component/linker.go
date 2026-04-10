// internal/component/linker.go

package component

import (
	"context"
	"strings"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// HostFunc is the canonical host function callback. It mirrors
// wasmtime's func_new dynamic host path:
//
//	debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:665-675
//	debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/host.rs:619-626
//	debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/host.rs:640-694
//
// fnType is the *types.TypeFunc looked up from the component's import
// declaration at call time and supplied by the runtime. The host has
// no type to declare — the component's import IS the source of truth.
// Most callers ignore fnType and read args directly; complex hosts
// (e.g., generic dispatchers) may inspect it.
//
// Spec: definitions.py:1997 (canon_lift), :2089 (canon_lower) lift
// the args against the component's import type and pass them as
// []types.Val to the host. The host returns []types.Val which the
// runtime lowers back per the same import type.
type HostFunc func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error)

// Definition is an item that can satisfy a component import.
type Definition interface {
	definition()
}

// FuncDef is a function definition.
//
// Type is the component function type. It is populated only by the
// binary decoder for type-bound imports / by nested-component
// construction (nested_component.go:78). For host-provided imports
// registered via the Linker, Type stays nil; the per-instance
// resolved type lives on ComponentFunc.Type (instance.go:121).
//
// Rationale: a *FuncDef stored in Linker.definitions is shared across
// every instantiate of that linker. Writing the resolved type here
// from the type checker would mutate shared state and silently break
// multi-instance scenarios where two components import the same host
// function at differently-typed slots. The shared FuncDef must stay
// immutable after registration.
type FuncDef struct {
	Type     *types.TypeFunc
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
	Value types.Val
}

func (*ImportedValueDef) definition() {}

// TypeDefDef wraps a TypeDef to implement Definition.
// This is used when passing types as arguments to nested component instantiation.
type TypeDefDef struct {
	TypeDef *TypeDef
}

func (*TypeDefDef) definition() {}

// getExactExportedFunc finds an exported function by exact name match.
// If the function was wired by ComponentLinker.Instantiate (stored in i.exports),
// it returns the fully-wired ExportedFunc with coreFunc, memory, etc.
//
// Session 0 compile-fix: the stub path that scans i.component.Exports and
// pulls a *FuncType from c.Types[canon.TypeIdx] depended on the old
// []TypeDef indexed shape. c.Types is now *types.ComponentTypes — the
// canonical bag — and resolving a canon lift to its *types.TypeFunc is
// Session 1 work. Until then we return the fully-wired export when
// available, otherwise a stub with no funcType.
func (i *Instance) getExactExportedFunc(name string) *ExportedFunc {
	if f, ok := i.exports[name]; ok && f != nil {
		return f
	}
	for _, exp := range i.component.Exports {
		if exp.Name == name && exp.Kind == ExportKindFunc {
			return &ExportedFunc{
				name:     name,
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

	// Check the exports map for a fully-wired function from ComponentLinker
	if f, ok := i.exports[bestExport.Name]; ok && f != nil {
		return f
	}

	// Fall back to a stub with no funcType; Session 1 will resolve canon
	// lift to its *types.TypeFunc via the rewritten abi/ package.
	return &ExportedFunc{
		name:     bestExport.Name,
		instance: i,
	}
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
