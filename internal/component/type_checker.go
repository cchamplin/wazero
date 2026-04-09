// internal/component/type_checker.go
package component

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TypeChecker validates types during component instantiation.
//
// Session 0 compile-fix: the old subtyping walks that traversed
// NamedValType{Name, ValType: ValTypeRef} pairs have been reduced to
// identity checks on the new *types.TypeFunc shape. Session 1 will reintroduce
// proper subtyping against the canonical ComponentTypes table.
type TypeChecker struct {
	component         *Component
	importedResources map[uint32]resourceTypeInfo
}

// resourceTypeInfo tracks a resource type for equality checking.
type resourceTypeInfo struct {
	sourceImport string // Import name that introduced this resource
	localIndex   uint32 // Index within the source
}

// NewTypeChecker creates a type checker for the given component.
func NewTypeChecker(c *Component) *TypeChecker {
	return &TypeChecker{
		component:         c,
		importedResources: make(map[uint32]resourceTypeInfo),
	}
}

// checkFuncType validates that actual function type matches expected.
//
// Session 0 compile-fix: the previous walk over NamedValType{} pairs is
// gone because TypeFunc now stores params/results as interned tuple
// ValTypes referenced via ComponentTypes.Tuples. Compare by shallow
// equality only; full structural subtyping is Session 2 work that will
// use the canonical ComponentTypes identity instead of walking per-field.
func (tc *TypeChecker) checkFuncType(expected, actual *types.TypeFunc) error {
	if expected == nil || actual == nil {
		if expected != actual {
			return fmt.Errorf("function type mismatch: one side is nil")
		}
		return nil
	}
	if expected.Async != actual.Async {
		return fmt.Errorf("async mismatch: expected %v, got %v", expected.Async, actual.Async)
	}
	if expected.Params != actual.Params {
		return fmt.Errorf("params tuple index mismatch: expected %v, got %v",
			expected.Params, actual.Params)
	}
	if expected.Results != actual.Results {
		return fmt.Errorf("results tuple index mismatch: expected %v, got %v",
			expected.Results, actual.Results)
	}
	return nil
}

// checkInstance validates that actual instance matches expected instance type.
// Instance subtyping allows extra exports (width subtyping).
func (tc *TypeChecker) checkInstance(expected *InstanceTypeDef, actual *InstanceDef) error {
	if expected == nil {
		return nil
	}
	if actual == nil {
		return fmt.Errorf("expected instance, got nil")
	}

	// If host didn't provide type info, trust it (similar to checkFuncDefinition)
	if actual.SkipValidation {
		return nil
	}

	// Check each required export exists
	for _, decl := range expected.Declarations {
		if decl.Kind != InstanceDeclKindExport || decl.Export == nil {
			continue
		}

		// Skip type exports (metadata only)
		if decl.Export.Kind == ExportKindType {
			continue
		}

		exportName := decl.Export.Name
		actualExport, ok := actual.Exports[exportName]
		if !ok {
			return fmt.Errorf("missing required export: %s", exportName)
		}

		// Validate export kind matches
		if err := tc.checkExportKind(decl.Export, actualExport); err != nil {
			return fmt.Errorf("export %s: %w", exportName, err)
		}
	}

	return nil
}

// checkExportKind validates that the actual definition matches the expected
// export kind and, where possible, recursively type-checks the export's
// declared type against the actual definition.
//
// Spec: wasmtime matching.rs:32-114 TypeChecker::definition recurses
// into func/instance arms; Explainer.md:920-982 instance subtyping.
func (tc *TypeChecker) checkExportKind(expected *InstanceExport, actual Definition) error {
	switch expected.Kind {
	case ExportKindFunc:
		fd, ok := actual.(*FuncDef)
		if !ok {
			return fmt.Errorf("expected function, got %T", actual)
		}
		// Resolve the export's declared function type and compare against
		// the actual FuncDef.Type. Per Decision 6, host-provided FuncDefs
		// have fd.Type == nil; skip the deep check in that case.
		if fd.Type != nil && tc.component != nil && int(expected.Idx) < len(tc.component.TypeDefs) {
			expTd, _, err := tc.component.ResolveTypeDef(expected.Idx)
			if err == nil && expTd.Kind == TypeDefKindFunc {
				expFuncType := expTd.FuncType(tc.component)
				if err := tc.checkFuncType(expFuncType, fd.Type); err != nil {
					return err
				}
			}
		}
	case ExportKindInstance:
		childInst, ok := actual.(*InstanceDef)
		if !ok {
			return fmt.Errorf("expected instance, got %T", actual)
		}
		// Recursively type-check nested instance exports.
		if tc.component != nil && int(expected.Idx) < len(tc.component.TypeDefs) {
			expTd, _, err := tc.component.ResolveTypeDef(expected.Idx)
			if err == nil && expTd.Kind == TypeDefKindInstance && expTd.Instance != nil {
				if err := tc.checkInstance(expTd.Instance, childInst); err != nil {
					return err
				}
			}
		}
	case ExportKindType:
		// Type exports don't need runtime validation
	case ExportKindComponent:
		if _, ok := actual.(*ComponentDef); !ok {
			return fmt.Errorf("expected component, got %T", actual)
		}
	}
	return nil
}

// checkResource validates resource type equality.
// Resources use exact equality - no subtyping allowed.
// The first occurrence is recorded; subsequent must match.
func (tc *TypeChecker) checkResource(typeIdx uint32, importName string, actual *ResourceDef) error {
	if actual == nil {
		return fmt.Errorf("expected resource, got nil")
	}

	existing, seen := tc.importedResources[typeIdx]
	if seen {
		// Second occurrence - must be same resource
		if existing.sourceImport != importName {
			return fmt.Errorf("resource type mismatch: index %d was %s, now %s",
				typeIdx, existing.sourceImport, importName)
		}
		return nil
	}

	// First occurrence - record it
	tc.importedResources[typeIdx] = resourceTypeInfo{
		sourceImport: importName,
		localIndex:   typeIdx,
	}

	return nil
}

// CheckDefinition validates that actual definition matches expected import type.
// This is the main entry point for type checking during instantiation.
func (tc *TypeChecker) CheckDefinition(expected *ImportExternDesc, importName string, actual Definition) error {
	switch expected.Kind {
	case ImportExternDescFunc:
		return tc.checkFuncDefinition(expected, actual)

	case ImportExternDescInstance:
		return tc.checkInstanceDefinition(expected, actual)

	case ImportExternDescType:
		// Type imports are handled by type substitution, not runtime validation
		return nil

	case ImportExternDescComponent:
		// Component imports need ComponentDef
		if _, ok := actual.(*ComponentDef); !ok {
			return fmt.Errorf("expected component, got %T", actual)
		}
		return nil

	case ImportExternDescValue:
		// Value imports need ImportedValueDef
		if _, ok := actual.(*ImportedValueDef); !ok {
			return fmt.Errorf("expected value, got %T", actual)
		}
		return nil

	default:
		return fmt.Errorf("unknown import kind: %d", expected.Kind)
	}
}

// checkFuncDefinition validates a function import.
//
// Session 1 Decision 6 / Task C3: under wasmtime's func_new
// dynamic-host model, the host has no type to declare at registration.
// The component's import declaration IS the canonical type; we resolve
// expected.TypeIdx here to validate its shape (kind + async bit) but
// do NOT mutate the shared actual.FuncDef.Type field. Mutating the
// shared *FuncDef stored in Linker.definitions would race across
// multi-instance scenarios where the same host import is bound by
// components with differently-typed declarations.
//
// fd.Type is no longer written here. The per-instance resolved type
// will be stored on ComponentFunc.Type (instance.go:121) when the
// lift/lower path populates inst.componentFuncs for function imports.
// Until then, validation is the only job of this function.
//
// Spec: wasmtime func/host.rs:619-626 DynamicHostFn::typecheck only
// validates the async bit at link time; the rest of the type-checking
// happens at lift/lower time against cx.types[ty] (host.rs:640-694).
func (tc *TypeChecker) checkFuncDefinition(expected *ImportExternDesc, actual Definition) error {
	fd, ok := actual.(*FuncDef)
	if !ok {
		return fmt.Errorf("expected function, got %T", actual)
	}
	if tc.component == nil {
		return nil
	}
	expectedTypeDef, _, err := tc.component.ResolveTypeDef(expected.TypeIdx)
	if err != nil {
		return fmt.Errorf("checkFuncDefinition: resolve expected type: %w", err)
	}
	if expectedTypeDef.Kind != TypeDefKindFunc {
		return fmt.Errorf("checkFuncDefinition: expected TypeDefKindFunc, got %v", expectedTypeDef.Kind)
	}
	expectedFuncType := expectedTypeDef.FuncType(tc.component)

	// Async bit MUST match (matches wasmtime DynamicHostFn::typecheck).
	// fd.Type is only non-nil on the nested-component path
	// (nested_component.go:78 builds &FuncDef{Type: fn.Type, ...} from
	// the parent's already-bound function). The linker path
	// (linker.go, component_linker.go) constructs FuncDef with nil
	// Type, so this guard is a no-op for host-provided imports and
	// only fires for nested-instance re-imports.
	if fd.Type != nil && fd.Type.Async != expectedFuncType.Async {
		return fmt.Errorf("checkFuncDefinition: async mismatch (expected %v, host %v)",
			expectedFuncType.Async, fd.Type.Async)
	}
	return nil
}

// checkInstanceDefinition validates an instance import.
//
// Session 1 Task F1: resolves the expected instance type from the
// component's type index space and recursively walks its declared
// exports via checkInstance, type-checking func and instance exports.
//
// Spec: Explainer.md:920-982 instance subtyping; wasmtime
// matching.rs:146-166 TypeChecker::instance walks
// TypeComponentInstance.exports and recurses into definition().
func (tc *TypeChecker) checkInstanceDefinition(expected *ImportExternDesc, actual Definition) error {
	instDef, ok := actual.(*InstanceDef)
	if !ok {
		return fmt.Errorf("expected instance, got %T", actual)
	}
	// Resolve the expected instance type from the component's type space.
	if tc.component == nil || int(expected.TypeIdx) >= len(tc.component.TypeDefs) {
		return nil // No type info to check
	}
	expectedTd, _, err := tc.component.ResolveTypeDef(expected.TypeIdx)
	if err != nil {
		return fmt.Errorf("checkInstanceDefinition: resolve type: %w", err)
	}
	if expectedTd.Kind != TypeDefKindInstance || expectedTd.Instance == nil {
		return nil // Not an instance type — skip
	}
	return tc.checkInstance(expectedTd.Instance, instDef)
}
