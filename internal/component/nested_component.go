// internal/component/nested_component.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// instantiateNestedComponent creates an instance of a nested component.
// This is called when processing ParsedComponentInstance definitions of kind Instantiate.
// It resolves arguments from the parent scope and establishes the parent/child relationship.
func (l *ComponentLinker) instantiateNestedComponent(
	ctx context.Context,
	parent *Instance,
	compInst *ParsedComponentInstance,
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

	// Create nested instance.
	//
	// Session 0 compile-fix: the old runtime.NewResourceTable() call no
	// longer exists. The unified runtime.Table is allocated eagerly via
	// NewTable. Session 1 folds this into the runtime.ComponentInstance
	// construction path.
	nestedInst := &Instance{
		component:      nestedComp,
		coreInstances:  make([]api.Module, 0),
		exports:        make(map[string]*ExportedFunc),
		table:          runtime.NewTable(),
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
		// outer aliases). Session 0 compile-fix: the old TypeIdxToStoredIdx
		// fallback and direct parentComponent.Types[idx] indexing are gone —
		// Component.Types is now *types.ComponentTypes, the canonical type
		// bag, and the Session 2 rewrite threads TypeDef through differently.
		typeDef := parent.GetTypeFromSpace(arg.Idx)
		if typeDef != nil {
			return &TypeDefDef{TypeDef: typeDef}, nil
		}
		// Fall back to type-alias resolution for export/outer aliases.
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
//
// Session 0 compile-fix: the previous body indexed c.Types as a []TypeDef
// slice and relied on TypeIdxToStoredIdx plus a buildLocalTypeIndex helper
// that walked the old InstanceTypeDef declarations. All of those shapes
// have been reworked by Tasks 2, 12 and will be rebuilt by Task 13's
// binary decoder. Until then this panics with a Session 1 followup-note
// pointer rather than dereference into a partially-migrated type bag.
func (l *ComponentLinker) resolveExportTypeAlias(parent *Instance, c *Component, alias *Alias) *TypeDef {
	_ = parent
	_ = c
	_ = alias
	panic("compile-fix stub: see Session 1 followup note — nested_component.go resolveExportTypeAlias scheduled for Session 1/2 restoration")
}

// buildTypeSpace populates the instance's type index space from the component's
// type definitions and type aliases.
//
// Session 0 compile-fix: the old TypeIdxToStoredIdx + []TypeDef indexing
// path is gone. Only the outer-alias branch still produces useful TypeDef
// pointers today; the top-level section-7 types are threaded through the
// canonical bag (Component.Types *types.ComponentTypes) and will be
// rewired by Task 13's binary decoder rewrite.
func (l *ComponentLinker) buildTypeSpace(inst *Instance, c *Component) {
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Sort != SortType {
			continue
		}

		for uint32(len(inst.typeSpace)) <= alias.Idx {
			inst.typeSpace = append(inst.typeSpace, nil)
		}
		if inst.typeSpace[alias.Idx] != nil {
			continue
		}

		switch alias.Kind {
		case AliasKindExport:
			if resolved := l.resolveExportTypeAlias(inst, c, alias); resolved != nil {
				inst.typeSpace[alias.Idx] = resolved
			}
		case AliasKindOuter:
			if resolved, err := ResolveOuterAlias(inst, alias); err == nil {
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
			// Capture fn in closure to avoid loop variable issue.
			exportedFn := fn
			exports[name] = &FuncDef{
				Type: fn.funcType,
				Callback: func(ctx context.Context, args []types.Val) ([]types.Val, error) {
					return exportedFn.Call(ctx, args...)
				},
			}
		}
	}
	return &InstanceDef{Exports: exports}
}
