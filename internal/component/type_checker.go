// internal/component/type_checker.go
package component

import "fmt"

// TypeChecker validates types during component instantiation.
// It implements the subtyping rules from the Component Model spec.
//
// Key rules:
// - Functions: params contravariant, results covariant
// - Instances: width subtyping (extra exports OK)
// - Resources: exact equality only
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
// Params are contravariant: actual must have at least as many params.
// Results are covariant: actual must match result count exactly.
func (tc *TypeChecker) checkFuncType(expected, actual *FuncType) error {
	if expected == nil || actual == nil {
		return nil // No type info to check
	}

	// Check params (contravariant - actual can have more)
	if len(actual.Params) < len(expected.Params) {
		return fmt.Errorf("insufficient params: expected %d, got %d",
			len(expected.Params), len(actual.Params))
	}

	// Check each expected param has compatible actual param
	for i, ep := range expected.Params {
		ap := actual.Params[i]
		if !tc.valTypeEqual(ep.ValType, ap.ValType) {
			return fmt.Errorf("param %d (%s): type mismatch", i, ep.Name)
		}
	}

	// Check results (covariant - must match count)
	if len(actual.Results) != len(expected.Results) {
		return fmt.Errorf("result count mismatch: expected %d, got %d",
			len(expected.Results), len(actual.Results))
	}

	for i, er := range expected.Results {
		ar := actual.Results[i]
		if !tc.valTypeEqual(ar.ValType, er.ValType) {
			return fmt.Errorf("result %d: type mismatch", i)
		}
	}

	return nil
}

// valTypeEqual checks if two ValTypeRefs are equal.
func (tc *TypeChecker) valTypeEqual(a, b ValTypeRef) bool {
	// Primitives must match exactly
	if a.IsPrimitive && b.IsPrimitive {
		return a.Primitive == b.Primitive
	}

	// Own handles
	if a.IsOwn && b.IsOwn {
		return tc.resourceTypesEqual(a.TypeIdx, b.TypeIdx)
	}

	// Borrow handles
	if a.IsBorrow && b.IsBorrow {
		return tc.resourceTypesEqual(a.TypeIdx, b.TypeIdx)
	}

	// Type indices
	if !a.IsPrimitive && !a.IsOwn && !a.IsBorrow &&
		!b.IsPrimitive && !b.IsOwn && !b.IsBorrow {
		return a.TypeIdx == b.TypeIdx
	}

	return false
}

// resourceTypesEqual checks if two resource type indices refer to the same resource.
func (tc *TypeChecker) resourceTypesEqual(idx1, idx2 uint32) bool {
	r1, ok1 := tc.importedResources[idx1]
	r2, ok2 := tc.importedResources[idx2]
	if ok1 && ok2 {
		return r1 == r2
	}
	return idx1 == idx2 // Same local index
}
