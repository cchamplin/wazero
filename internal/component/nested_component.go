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
		// Type from parent's type space or component's types
		typeDef := parent.GetTypeFromSpace(arg.Idx)
		if typeDef != nil {
			return &TypeDefDef{TypeDef: typeDef}, nil
		}
		// Fall back to parent component's types
		if int(arg.Idx) < len(parentComponent.Types) {
			return &TypeDefDef{TypeDef: &parentComponent.Types[arg.Idx]}, nil
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
