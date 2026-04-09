// internal/component/nested_component.go

package component

import (
	"context"
	"fmt"

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

	// Create nested instance via newInstance so the embedded
	// *runtime.ComponentInstance is wired up with the parent's runtime
	// state (Parent pointer, freshly-allocated Table, Destructors,
	// ReentranceTracker, may_leave=true).
	//
	// Session 1 Task B4: replaces the former struct-literal allocation
	// that directly set the deleted `table` field.
	//
	// Session 1 followup (Task C1+): nested instance IDs should be
	// assigned monotonically by the linker so LookupResourceType's
	// findInstance walk has a unique ID per runtime instance. Until
	// the linker tracks an allocator, newly-created nested instances
	// share id 0 — callers must not rely on ID uniqueness yet.
	nestedInst := newInstance(nestedComp, 0, parent)

	// Set parent relationship (wrapper-layer child list; rt.Parent is
	// already wired by newInstance via the parent argument).
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
// Given an Alias with Kind == AliasKindExport and Sort == SortType, this:
//  1. Looks up the source instance from parent.instanceSpace[alias.InstanceIdx].
//  2. Finds a matching export in the source instance's component (Name + ExportKindType).
//  3. Resolves the TypeDef at that export's type index via Component.ResolveTypeDef
//     for alias-chain safety.
//
// Returns nil if the source instance is unreachable, no matching type export exists,
// or alias-chain resolution fails.
func (l *ComponentLinker) resolveExportTypeAlias(parent *Instance, c *Component, alias *Alias) *TypeDef {
	// Step 1: Look up the source instance.
	if int(alias.InstanceIdx) >= len(parent.instanceSpace) {
		return nil
	}
	srcInst := parent.instanceSpace[alias.InstanceIdx]
	if srcInst == nil || srcInst.component == nil {
		return nil
	}

	// Step 2: Find the matching type export in the source instance's component.
	srcComp := srcInst.component
	var exportIdx uint32
	found := false
	for i := range srcComp.Exports {
		exp := &srcComp.Exports[i]
		if exp.Name == alias.ExportName && exp.Kind == ExportKindType {
			exportIdx = exp.Idx
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	// Step 3: Resolve the TypeDef at the export's type index.
	// ResolveTypeDef walks alias chains within the source component.
	// It returns a deferred error for export aliases and cross-scope
	// outer aliases; in that case we fall back to returning the raw
	// TypeDef at the index (best-effort resolution).
	td, _, err := srcComp.ResolveTypeDef(exportIdx)
	if err != nil {
		// Fallback: return the raw TypeDef if the index is valid.
		if int(exportIdx) < len(srcComp.TypeDefs) {
			return &srcComp.TypeDefs[exportIdx]
		}
		return nil
	}
	return td
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
				Callback: func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
					return exportedFn.Call(ctx, args...)
				},
			}
		}
	}
	return &InstanceDef{Exports: exports}
}
