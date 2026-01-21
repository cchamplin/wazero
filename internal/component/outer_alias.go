// internal/component/outer_alias.go
package component

import "fmt"

// ResolveOuterAlias resolves an outer alias to its target definition.
// Outer aliases use de Bruijn indexing: (depth, index).
// Only immutable items (types, modules, components) can be outer-aliased.
//
// Per the Component Model spec, outer aliases allow nested components to
// reference definitions from enclosing scopes. The OuterCount field
// specifies how many scopes to traverse up (1 = parent, 2 = grandparent, etc.),
// and OuterIndex specifies the index within that scope's index space.
func ResolveOuterAlias(inst *Instance, alias *Alias) (interface{}, error) {
	if alias.Kind != AliasKindOuter {
		return nil, fmt.Errorf("not an outer alias: kind is %s", alias.Kind)
	}

	// Navigate up the parent chain using de Bruijn indexing
	target := inst.GetAncestor(alias.OuterCount)
	if target == nil {
		return nil, fmt.Errorf("outer alias depth %d exceeds nesting level", alias.OuterCount)
	}

	// Resolve based on sort
	switch alias.Sort {
	case SortType:
		typeDef := target.GetTypeFromSpace(alias.OuterIndex)
		if typeDef == nil {
			return nil, fmt.Errorf("type index %d not found at depth %d",
				alias.OuterIndex, alias.OuterCount)
		}
		return typeDef, nil

	case SortComponent:
		comp := target.GetComponentFromSpace(alias.OuterIndex)
		if comp == nil {
			return nil, fmt.Errorf("component index %d not found at depth %d",
				alias.OuterIndex, alias.OuterCount)
		}
		return comp, nil

	case SortFunc:
		// Functions cannot be outer-aliased (mutable)
		return nil, fmt.Errorf("cannot outer-alias functions (mutable)")

	case SortInstance:
		// Instances cannot be outer-aliased (mutable)
		return nil, fmt.Errorf("cannot outer-alias instances (mutable)")

	case SortValue:
		// Values cannot be outer-aliased (mutable)
		return nil, fmt.Errorf("cannot outer-alias values (mutable)")

	default:
		return nil, fmt.Errorf("unsupported sort for outer alias: %s", alias.Sort)
	}
}
