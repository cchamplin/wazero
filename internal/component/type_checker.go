// internal/component/type_checker.go
package component

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
