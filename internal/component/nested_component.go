// internal/component/nested_component.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// instantiateNestedComponent creates an instance of a nested component.
// This is called when processing ParsedComponentInstance definitions of kind
// Instantiate. It resolves arguments from the parent scope, creates the child
// instance, and runs the nested instantiation pipeline (steps 2-14 from the
// main Instantiate path, adapted for nested use).
//
// Spec: Component-model nested instantiation (Explainer.md :1020+).
// Design: docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md
//   lines 1134-1137.
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md
//   (Task D2).
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

	// Step 1 — Create nested instance via newInstance so the embedded
	// *runtime.ComponentInstance is wired up with the parent's runtime
	// state (Parent pointer, freshly-allocated Table, Destructors,
	// ReentranceTracker, may_leave=true).
	nestedInst := newInstance(nestedComp, l.nextInstanceID(), parent)

	// Propagate the store-wide ResourceStore from the parent so all
	// instances in the same instantiation tree share the same registry.
	if parent.rt.Store != nil {
		nestedInst.rt.Store = parent.rt.Store
		parent.rt.Store.RegisterInstance(nestedInst.rt.ID, nestedInst)
	}

	// Set parent relationship (wrapper-layer child list; rt.Parent is
	// already wired by newInstance via the parent argument).
	parent.AddChild(nestedInst)

	// Step 2 — Bind resource type declarations to runtime identities.
	if err := l.bindResourceTypes(nestedInst, nestedComp); err != nil {
		return nil, fmt.Errorf("nested: bind resource types: %w", err)
	}

	// Step 3 — Build core index spaces from aliases.
	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	l.buildCoreIndexSpaces(nestedComp, funcSpace, memSpace)

	// Step 4 — Resolve and type-check imports from withArgs (not from
	// the linker's global definitions).
	tc := NewTypeChecker(nestedComp)
	resolvedImports := make(map[string]Definition)
	instanceToImport := make(map[uint32]string)
	if err := l.resolveNestedImports(nestedComp, tc, resolvedImports, instanceToImport, withArgs); err != nil {
		return nil, fmt.Errorf("nested: resolve imports: %w", err)
	}

	// Step 5 — Populate value index space from value imports.
	l.populateValueImports(nestedInst, nestedComp, resolvedImports)

	// Step 6 — Align instance index space with instance imports.
	l.alignInstanceImports(nestedInst, nestedComp)

	// Step 7 — Build component function index space from canon.lift
	// declarations + resolved function imports.
	l.buildComponentFuncs(nestedInst, nestedComp, resolvedImports)

	// Step 8 — Build type index space for further nested instantiation.
	l.buildTypeSpace(nestedInst, nestedComp)

	// Step 9 — Process nested component instances (recursive).
	componentInstDefs, err := l.processNestedInstances(ctx, nestedInst, nestedComp)
	if err != nil {
		return nil, fmt.Errorf("nested: nested instances: %w", err)
	}

	_ = instanceToImport

	// Step 10 — Build canon lower / canon resource info maps.
	canonLowers, canonResources := l.buildCanonMaps(nestedComp)

	// Step 11 — Build function alias map for inline instance resolution.
	funcAliases := l.buildFuncAliases(nestedComp)

	// Step 12 — Instantiate core modules using pre-compiled modules stored
	// on the nested Component by CompileComponent's recursive pass.
	if err := l.instantiateCoreModules(ctx, nestedInst, nestedComp, nestedComp.CompiledCoreModules,
		canonLowers, canonResources, funcAliases); err != nil {
		return nil, fmt.Errorf("nested: core modules: %w", err)
	}

	// Step 12.5 — Resolve pending guest destructors and reallocs.
	l.resolvePendingDtors(nestedInst, funcSpace)
	l.resolvePendingReallocs(nestedInst, funcSpace)

	// Step 13 — Execute start function.
	if err := l.executeStartFunction(ctx, nestedInst, nestedComp); err != nil {
		return nil, fmt.Errorf("nested: start function: %w", err)
	}

	// Step 14 — Wire exports.
	// wireExports currently returns an error for non-func export kinds.
	// For nested components with no exports, this is a no-op.
	if err := l.wireExports(nestedInst, nestedComp, componentInstDefs, funcSpace, memSpace); err != nil {
		return nil, fmt.Errorf("nested: wire exports: %w", err)
	}

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
		// bag, and the current implementation threads TypeDef through differently.
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

// resolveNestedImports walks the nested component's imports and resolves
// each from the withArgs map (parent-scope resolved definitions) rather
// than from the linker's global definitions. Type checking is still
// performed via tc.CheckDefinition.
//
// This is the nested-instantiation analog of resolveAndCheckImports.
//
// Spec: Component-model import resolution for nested instantiation.
func (l *ComponentLinker) resolveNestedImports(
	c *Component,
	tc *TypeChecker,
	resolvedImports map[string]Definition,
	instanceToImport map[uint32]string,
	withArgs map[string]Definition,
) error {
	var instanceIdx uint32
	for i := range c.Imports {
		imp := &c.Imports[i]
		def, ok := withArgs[imp.Name]
		if !ok {
			return fmt.Errorf("import %q: not provided by parent scope", imp.Name)
		}
		if err := tc.CheckDefinition(&imp.ExternDesc, imp.Name, def); err != nil {
			return fmt.Errorf("import %q: type check: %w", imp.Name, err)
		}
		resolvedImports[imp.Name] = def
		if imp.ExternDesc.Kind == ImportExternDescInstance {
			instanceToImport[instanceIdx] = imp.Name
			instanceIdx++
		}
	}
	return nil
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
func (l *ComponentLinker) resolveExportTypeAlias(parent *Instance, _ *Component, alias *Alias) *TypeDef {
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
		// ResolveTypeDef returns a deferred error for export aliases and
		// cross-scope outer aliases. Fall back to the raw TypeDef slot,
		// but only if it's a concrete type — returning an unresolved
		// TypeDefKindAlias would silently leak into callers that switch
		// on td.Kind without handling the alias variant.
		if int(exportIdx) < len(srcComp.TypeDefs) {
			fallback := &srcComp.TypeDefs[exportIdx]
			if fallback.Kind == TypeDefKindAlias {
				return nil
			}
			return fallback
		}
		return nil
	}
	return td
}

// wireNestedComponentExports stores a real nested child instance in the
// parent's exportedInstances map under the given export name. Callers
// drill into the child's exports via GetExportedInstance(name) →
// ExportedFunction(subname). Currently infallible; error return reserved
// for future validation (e.g., export-type conformance checking).
//
// Spec: Component-model instance exports — an instance export is a namespace
// of sub-exports accessible by drilling through the exported instance.
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/component.rs:847-848 (Export::Instance { exports, .. }).
//
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md
//
//	(Task D3).
func (l *ComponentLinker) wireNestedComponentExports(
	parent *Instance,
	nested *Instance,
	exportName string,
) error {
	parent.AddExportedInstance(exportName, nested)
	return nil
}

// wireInlineInstanceExports creates a synthetic Instance from an InstanceDef
// (produced by processNestedInstances for inline instances) and stores it
// as an exported instance on the parent. Each FuncDef export in the
// InstanceDef becomes an ExportedFunc on the synthetic instance.
//
// Spec: inline component instances re-export items from the current scope.
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md
//
//	(Task D3).
func (l *ComponentLinker) wireInlineInstanceExports(
	parent *Instance,
	instDef *InstanceDef,
	exportName string,
) error {
	// Create a synthetic Instance to hold the inline instance's exports.
	// The empty &Component{} is intentional — synthetic instances exist
	// solely to hold exports and have no meaningful component metadata.
	synth := newInstance(&Component{}, l.nextInstanceID(), parent)
	for name, def := range instDef.Exports {
		switch d := def.(type) {
		case *FuncDef:
			synth.exports[name] = &ExportedFunc{
				name:     name,
				funcType: d.Type,
				impl:     d.Callback,
			}
		// Non-FuncDef exports (InstanceDef, TypeDefDef, ImportedValueDef,
		// ComponentDef) are not converted to ExportedFuncs because they
		// don't have callable semantics at runtime. If downstream code
		// needs to access non-function inline exports, this path should
		// be extended to store them in the appropriate index space on the
		// synthetic instance.
		default:
			// Silently skip non-callable exports.
		}
	}
	parent.AddExportedInstance(exportName, synth)
	return nil
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
					return exportedFn.CallAndPostReturn(ctx, args...)
				},
			}
		}
	}
	return &InstanceDef{Exports: exports}
}
