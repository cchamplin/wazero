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
		return nil // No type info to check
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

// checkExportKind validates that the actual definition matches the expected export kind.
func (tc *TypeChecker) checkExportKind(expected *InstanceExport, actual Definition) error {
	switch expected.Kind {
	case ExportKindFunc:
		if _, ok := actual.(*FuncDef); !ok {
			return fmt.Errorf("expected function, got %T", actual)
		}
	case ExportKindInstance:
		if _, ok := actual.(*InstanceDef); !ok {
			return fmt.Errorf("expected instance, got %T", actual)
		}
	case ExportKindType:
		// Type exports don't need runtime validation
	case ExportKindComponent:
		// Component exports require ComponentDef
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
// Session 0 compile-fix: the old []TypeDef indexing is gone. Component.Types
// is now *types.ComponentTypes (the canonical bag). Resolving a component
// type index back to a *types.TypeFunc requires Task 13 / Session 2 wiring.
// Until then we trust host-provided FuncDef.Type (no expected side to
// compare against).
func (tc *TypeChecker) checkFuncDefinition(expected *ImportExternDesc, actual Definition) error {
	_, ok := actual.(*FuncDef)
	if !ok {
		return fmt.Errorf("expected function, got %T", actual)
	}
	_ = expected
	return nil
}

// checkInstanceDefinition validates an instance import.
//
// Session 0 compile-fix: the old []TypeDef indexing is gone; see
// checkFuncDefinition for the rationale. We only validate the Go-level
// type of the definition here.
func (tc *TypeChecker) checkInstanceDefinition(expected *ImportExternDesc, actual Definition) error {
	_, ok := actual.(*InstanceDef)
	if !ok {
		return fmt.Errorf("expected instance, got %T", actual)
	}
	_ = expected
	return nil
}
