// internal/component/nested_component.go

package component

import (
	"context"
	"fmt"
	"github.com/tetratelabs/wazero/api"
)

// instantiateNestedComponent creates an instance of a nested component.
// This is called when processing ComponentInstance definitions of kind Instantiate.
// It resolves arguments from the parent scope and establishes the parent/child relationship.
func (l *ComponentLinker) instantiateNestedComponent(
	ctx context.Context,
	parent *Instance,
	compInst *ComponentInstance,
	parentComponent *Component,
) (*Instance, error) {
	// Get the nested component definition
	if compInst.ComponentIdx >= uint32(len(parentComponent.Components)) {
		return nil, fmt.Errorf("component index %d out of range (have %d)", compInst.ComponentIdx, len(parentComponent.Components))
	}
	nestedComp := parentComponent.Components[compInst.ComponentIdx]

	// Build with arguments from parent scope
	withArgs := make(map[string]Definition)
	for _, arg := range compInst.Args {
		def, err := l.resolveFromParentScope(parent, parentComponent, arg)
		if err != nil {
			return nil, fmt.Errorf("arg %q: %w", arg.Name, err)
		}
		withArgs[arg.Name] = def
	}

	// Create nested instance
	nestedInst := &Instance{
		component:      nestedComp,
		coreInstances:  make([]api.Module, 0),
		exports:        make(map[string]*ExportedFunc),
		resourceTable:  NewResourceTable(),
		componentFuncs: make(map[uint32]ComponentFunc),
	}

	// Set parent relationship
	parent.AddChild(nestedInst)

	// Store resolved arguments for later use during full instantiation
	// For now, the basic instance is returned
	// Full type checking and instantiation logic would validate imports against withArgs

	return nestedInst, nil
}

// resolveFromParentScope resolves an instantiation argument from parent scope.
// This maps component-level items (functions, types, instances, etc.) from the
// parent's index spaces to Definition objects that can satisfy the child's imports.
func (l *ComponentLinker) resolveFromParentScope(
	parent *Instance,
	parentComponent *Component,
	arg ComponentInstantiateArg,
) (Definition, error) {
	switch arg.Sort {
	case SortFunc:
		// Function from parent's component function space
		fn, ok := parent.componentFuncs[arg.Idx]
		if !ok {
			return nil, fmt.Errorf("func %d not found in parent", arg.Idx)
		}
		return &FuncDef{Type: fn.Type, Callback: fn.Impl}, nil

	case SortInstance:
		// Instance from parent's instance space
		inst := parent.GetInstanceFromSpace(arg.Idx)
		if inst == nil {
			return nil, fmt.Errorf("instance %d not found in parent", arg.Idx)
		}
		return instanceToDefinition(inst), nil

	case SortType:
		// Type from parent's type space (populated for nested instances with
		// outer aliases) or from the parent component's type definitions.
		typeDef := parent.GetTypeFromSpace(arg.Idx)
		if typeDef != nil {
			return &TypeDefDef{TypeDef: typeDef}, nil
		}
		// Look up from the component's type index space. The type index space
		// can be larger than the Types array because type aliases (export and
		// outer) consume indices without adding to Types. Use TypeIdxToStoredIdx
		// to map from type index space to the compact Types array.
		if storedIdx, ok := parentComponent.TypeIdxToStoredIdx[arg.Idx]; ok {
			if int(storedIdx) < len(parentComponent.Types) {
				return &TypeDefDef{TypeDef: &parentComponent.Types[storedIdx]}, nil
			}
		}
		// Direct index fallback for backward compatibility (when TypeIdxToStoredIdx
		// is not populated or type indices align with Types array).
		if int(arg.Idx) < len(parentComponent.Types) {
			return &TypeDefDef{TypeDef: &parentComponent.Types[arg.Idx]}, nil
		}
		// Check if this type index comes from a type alias (export or outer).
		// Type aliases consume type index space entries during decoding but
		// don't add to TypeIdxToStoredIdx. Resolve them by tracing back to
		// the actual type definition through the alias chain.
		resolved := l.resolveTypeAlias(parent, parentComponent, arg.Idx)
		if resolved != nil {
			return &TypeDefDef{TypeDef: resolved}, nil
		}
		return nil, fmt.Errorf("type %d not found in parent", arg.Idx)

	case SortComponent:
		// First try parent's component space
		comp := parent.GetComponentFromSpace(arg.Idx)
		if comp != nil {
			return &ComponentDef{Component: comp}, nil
		}
		// Fall back to parent component's nested components
		if int(arg.Idx) < len(parentComponent.Components) {
			return &ComponentDef{Component: parentComponent.Components[arg.Idx]}, nil
		}
		return nil, fmt.Errorf("component %d not found in parent", arg.Idx)

	case SortValue:
		val, err := parent.GetValue(arg.Idx)
		if err != nil {
			return nil, fmt.Errorf("value %d: %w", arg.Idx, err)
		}
		return &ImportedValueDef{Value: val}, nil

	default:
		return nil, fmt.Errorf("unsupported sort for component instantiation: %s", arg.Sort)
	}
}

// resolveTypeAlias resolves a type index that came from an alias (export or outer)
// back to an actual TypeDef. Type aliases consume type index space entries during
// binary decoding but don't add to TypeIdxToStoredIdx.
func (l *ComponentLinker) resolveTypeAlias(parent *Instance, c *Component, typeIdx uint32) *TypeDef {
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Idx != typeIdx || alias.Sort != SortType {
			continue
		}

		switch alias.Kind {
		case AliasKindExport:
			// Export alias: references a type exported by a component instance.
			// Trace through the instance's import type to find the actual TypeDef.
			return l.resolveExportTypeAlias(parent, c, alias)

		case AliasKindOuter:
			// Outer alias: references a type from an enclosing scope.
			resolved, err := ResolveOuterAlias(parent, alias)
			if err == nil {
				if td, ok := resolved.(*TypeDef); ok {
					return td
				}
			}
		}
	}
	return nil
}

// resolveExportTypeAlias resolves a type export alias by tracing through the
// source instance's type definition to find the actual TypeDef for the exported type.
func (l *ComponentLinker) resolveExportTypeAlias(parent *Instance, c *Component, alias *Alias) *TypeDef {
	// Find which import created this instance by walking instance imports
	// in order (each instance import occupies one slot in the instance space).
	var importDesc *Import
	instCount := uint32(0)
	for i := range c.Imports {
		imp := &c.Imports[i]
		if imp.ExternDesc.Kind == ImportExternDescInstance {
			if instCount == alias.InstanceIdx {
				importDesc = imp
				break
			}
			instCount++
		}
	}
	if importDesc == nil {
		return nil
	}

	// Look up the instance type definition from the import's TypeIdx
	instTypeIdx := importDesc.ExternDesc.TypeIdx
	var instTypeDef *InstanceTypeDef

	// First check TypeIdxToStoredIdx for the instance type
	if storedIdx, ok := c.TypeIdxToStoredIdx[instTypeIdx]; ok {
		if int(storedIdx) < len(c.Types) && c.Types[storedIdx].Kind == TypeDefKindInstance {
			instTypeDef = c.Types[storedIdx].Instance
		}
	}
	// Also try direct index (for components where type indices align with Types array)
	if instTypeDef == nil && int(instTypeIdx) < len(c.Types) && c.Types[instTypeIdx].Kind == TypeDefKindInstance {
		instTypeDef = c.Types[instTypeIdx].Instance
	}
	if instTypeDef == nil {
		return nil
	}

	// Build the local type index for the instance type and find the export
	localTypes := buildLocalTypeIndex(instTypeDef, c)
	for _, decl := range instTypeDef.Declarations {
		if decl.Kind == InstanceDeclKindExport && decl.Export != nil {
			if decl.Export.Name == alias.ExportName && decl.Export.Kind == ExportKindType {
				if td, ok := localTypes[decl.Export.Idx]; ok {
					// Set SourceLocalTypes so the resolver can correctly resolve
					// nested type references using the instance type's local scope.
					if td.SourceLocalTypes == nil {
						td.SourceLocalTypes = localTypes
					}
					return td
				}
			}
		}
	}
	return nil
}

// buildTypeSpace populates the instance's type index space from the component's
// type definitions and type aliases. This must be called before processing nested
// component instances, since resolveFromParentScope needs the type space populated.
func (l *ComponentLinker) buildTypeSpace(inst *Instance, c *Component) {
	// Pre-populate from type section entries via TypeIdxToStoredIdx
	for typeIdx, storedIdx := range c.TypeIdxToStoredIdx {
		if int(storedIdx) < len(c.Types) {
			// Ensure typeSpace is large enough
			for uint32(len(inst.typeSpace)) <= typeIdx {
				inst.typeSpace = append(inst.typeSpace, nil)
			}
			inst.typeSpace[typeIdx] = &c.Types[storedIdx]
		}
	}

	// Process type aliases (export and outer) to fill remaining slots
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Sort != SortType {
			continue
		}

		// Ensure typeSpace is large enough
		for uint32(len(inst.typeSpace)) <= alias.Idx {
			inst.typeSpace = append(inst.typeSpace, nil)
		}
		// Skip if already populated (shouldn't happen, but be safe)
		if inst.typeSpace[alias.Idx] != nil {
			continue
		}

		switch alias.Kind {
		case AliasKindExport:
			resolved := l.resolveExportTypeAlias(inst, c, alias)
			if resolved != nil {
				inst.typeSpace[alias.Idx] = resolved
			}
		case AliasKindOuter:
			resolved, err := ResolveOuterAlias(inst, alias)
			if err == nil {
				if td, ok := resolved.(*TypeDef); ok {
					inst.typeSpace[alias.Idx] = td
				}
			}
		}
	}
}

// instanceToDefinition converts an Instance to an InstanceDef.
// This allows an existing instance to be provided as an import argument
// to a nested component.
func instanceToDefinition(inst *Instance) *InstanceDef {
	exports := make(map[string]Definition)
	for name, fn := range inst.exports {
		if fn != nil {
			// Capture fn in closure to avoid loop variable issue
			exportedFn := fn
			exports[name] = &FuncDef{
				Type: fn.funcType,
				// Wrap ExportedFunc.Call to match HostFunc signature
				// ExportedFunc.Call uses variadic params, HostFunc uses slice
				Callback: func(ctx context.Context, args []Val) ([]Val, error) {
					return exportedFn.Call(ctx, args...)
				},
			}
		}
	}
	return &InstanceDef{Exports: exports}
}
