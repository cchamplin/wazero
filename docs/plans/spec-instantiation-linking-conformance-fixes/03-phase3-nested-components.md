# Phase 3: Nested Component Support

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support component-in-component instantiation with proper scope resolution and outer alias handling.

**Architecture:** Add parent/child tracking to Instance, implement recursive component instantiation, support outer aliases for type/module/component references.

**Tech Stack:** Go

**Parent Plan:** [00-root.md](./00-root.md)

**Prerequisite:** Phase 1 (Type Checking) must be complete.

**Gap Analysis Reference:** [Section 2: Component Instantiation](../2026-01-20-instantiation-linking-gap-analysis.md#2-component-instantiation)

---

## Spec References

Read these before starting:
- `debug-vendored/component-model/design/mvp/Explainer.md` lines 343-406 (Outer aliases)
- `debug-vendored/component-model/design/mvp/Binary.md` lines 77-89 (Component instantiation)

Key requirements:
- Component instantiation requires matching all named imports with `with` arguments
- Type substitution occurs immediately when type imports are supplied
- Outer aliases use de Bruijn indexing (component-nesting-depth, index-in-target-space)
- Only immutable definitions can be outer-aliased (types, modules, components)

---

## Task 1: Add Parent/Child Tracking to Instance

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

### Step 1: Write the failing test for parent/child tracking

```go
// Add to internal/component/instance_test.go

func TestInstance_ParentChild(t *testing.T) {
	parent := &Instance{}
	child := &Instance{}

	parent.AddChild(child)

	if child.Parent() != parent {
		t.Error("child.Parent() should return parent")
	}

	children := parent.Children()
	if len(children) != 1 {
		t.Errorf("expected 1 child, got %d", len(children))
	}
	if children[0] != child {
		t.Error("children[0] should be child")
	}
}

func TestInstance_ParentChain(t *testing.T) {
	grandparent := &Instance{}
	parent := &Instance{}
	child := &Instance{}

	grandparent.AddChild(parent)
	parent.AddChild(child)

	// Navigate up the chain
	if child.Parent() != parent {
		t.Error("child.Parent() should be parent")
	}
	if parent.Parent() != grandparent {
		t.Error("parent.Parent() should be grandparent")
	}
	if grandparent.Parent() != nil {
		t.Error("grandparent.Parent() should be nil")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_Parent -v`

Expected: FAIL with "undefined: parent.AddChild"

### Step 3: Write minimal implementation

```go
// Add to internal/component/instance.go

// In the Instance struct, add:
//
// // Nested component support
// parent   *Instance
// children []*Instance

// Parent returns this instance's parent, or nil if top-level.
func (i *Instance) Parent() *Instance {
	return i.parent
}

// Children returns this instance's child instances.
func (i *Instance) Children() []*Instance {
	return i.children
}

// AddChild adds a child instance and sets its parent.
func (i *Instance) AddChild(child *Instance) {
	if i.children == nil {
		i.children = make([]*Instance, 0)
	}
	i.children = append(i.children, child)
	child.parent = i
}

// GetAncestor returns the ancestor at the given depth.
// Depth 0 returns self, depth 1 returns parent, etc.
func (i *Instance) GetAncestor(depth uint32) *Instance {
	current := i
	for d := uint32(0); d < depth && current != nil; d++ {
		current = current.parent
	}
	return current
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_Parent -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "feat(component): add parent/child tracking to Instance

Enables navigation up component hierarchy for outer aliases.
GetAncestor supports de Bruijn indexing.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 2: Add Index Spaces to Instance

**Files:**
- Modify: `internal/component/instance.go`
- Test: `internal/component/instance_test.go`

### Step 1: Write the failing test for index spaces

```go
// Add to internal/component/instance_test.go

func TestInstance_IndexSpaces(t *testing.T) {
	inst := &Instance{}

	// Add items to instance index space
	childInst := &Instance{}
	idx := inst.AddInstanceToSpace(childInst)
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	// Retrieve from space
	retrieved := inst.GetInstanceFromSpace(0)
	if retrieved != childInst {
		t.Error("retrieved instance should match")
	}

	// Add to type index space
	typeDef := &TypeDef{Kind: TypeDefKindFunc}
	typeIdx := inst.AddTypeToSpace(typeDef)
	if typeIdx != 0 {
		t.Errorf("expected type index 0, got %d", typeIdx)
	}

	retrievedType := inst.GetTypeFromSpace(0)
	if retrievedType != typeDef {
		t.Error("retrieved type should match")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_IndexSpaces -v`

Expected: FAIL with "undefined: inst.AddInstanceToSpace"

### Step 3: Write minimal implementation

```go
// Add to internal/component/instance.go

// In the Instance struct, add:
//
// // Index spaces for nested component support
// instanceSpace []*Instance
// typeSpace     []*TypeDef
// moduleSpace   []interface{} // wasm.Module or similar
// componentSpace []*Component

// AddInstanceToSpace adds an instance to the instance index space.
func (i *Instance) AddInstanceToSpace(inst *Instance) uint32 {
	idx := uint32(len(i.instanceSpace))
	i.instanceSpace = append(i.instanceSpace, inst)
	return idx
}

// GetInstanceFromSpace retrieves an instance from the instance index space.
func (i *Instance) GetInstanceFromSpace(idx uint32) *Instance {
	if idx >= uint32(len(i.instanceSpace)) {
		return nil
	}
	return i.instanceSpace[idx]
}

// AddTypeToSpace adds a type definition to the type index space.
func (i *Instance) AddTypeToSpace(t *TypeDef) uint32 {
	idx := uint32(len(i.typeSpace))
	i.typeSpace = append(i.typeSpace, t)
	return idx
}

// GetTypeFromSpace retrieves a type from the type index space.
func (i *Instance) GetTypeFromSpace(idx uint32) *TypeDef {
	if idx >= uint32(len(i.typeSpace)) {
		return nil
	}
	return i.typeSpace[idx]
}

// AddComponentToSpace adds a component to the component index space.
func (i *Instance) AddComponentToSpace(c *Component) uint32 {
	idx := uint32(len(i.componentSpace))
	i.componentSpace = append(i.componentSpace, c)
	return idx
}

// GetComponentFromSpace retrieves a component from the component index space.
func (i *Instance) GetComponentFromSpace(idx uint32) *Component {
	if idx >= uint32(len(i.componentSpace)) {
		return nil
	}
	return i.componentSpace[idx]
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstance_IndexSpaces -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/instance.go internal/component/instance_test.go
git commit -m "feat(component): add index spaces to Instance

Supports instance, type, and component index spaces.
Required for nested component argument resolution.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 3: Implement Outer Alias Resolution

**Files:**
- Create: `internal/component/outer_alias.go`
- Test: `internal/component/outer_alias_test.go`

### Step 1: Write the failing test for outer alias resolution

```go
// internal/component/outer_alias_test.go
package component

import (
	"testing"
)

func TestResolveOuterAlias_Type(t *testing.T) {
	// Create parent with a type
	parent := &Instance{}
	parentType := &TypeDef{Kind: TypeDefKindFunc, Func: &FuncType{}}
	parent.AddTypeToSpace(parentType)

	// Create child
	child := &Instance{}
	parent.AddChild(child)

	// Outer alias: depth=1, index=0 (parent's type at index 0)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 1,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(child, alias)
	if err != nil {
		t.Fatalf("ResolveOuterAlias failed: %v", err)
	}

	resolvedType, ok := resolved.(*TypeDef)
	if !ok {
		t.Fatalf("expected *TypeDef, got %T", resolved)
	}
	if resolvedType != parentType {
		t.Error("resolved type should match parent's type")
	}
}

func TestResolveOuterAlias_TooDeep(t *testing.T) {
	// Create single instance (no parent)
	inst := &Instance{}

	// Try to resolve outer alias with depth > nesting
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 2, // No grandparent exists
		OuterIndex: 0,
	}

	_, err := ResolveOuterAlias(inst, alias)
	if err == nil {
		t.Error("should fail when outer depth exceeds nesting")
	}
}

func TestResolveOuterAlias_Component(t *testing.T) {
	parent := &Instance{}
	nestedComp := &Component{}
	parent.AddComponentToSpace(nestedComp)

	child := &Instance{}
	parent.AddChild(child)

	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortComponent,
		OuterCount: 1,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(child, alias)
	if err != nil {
		t.Fatalf("ResolveOuterAlias failed: %v", err)
	}

	resolvedComp, ok := resolved.(*Component)
	if !ok {
		t.Fatalf("expected *Component, got %T", resolved)
	}
	if resolvedComp != nestedComp {
		t.Error("resolved component should match parent's component")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestResolveOuterAlias -v`

Expected: FAIL with "undefined: ResolveOuterAlias"

### Step 3: Write minimal implementation

```go
// internal/component/outer_alias.go
package component

import "fmt"

// ResolveOuterAlias resolves an outer alias to its target definition.
// Outer aliases use de Bruijn indexing: (depth, index).
// Only immutable items (types, modules, components) can be outer-aliased.
func ResolveOuterAlias(inst *Instance, alias *Alias) (interface{}, error) {
	if alias.Kind != AliasKindOuter {
		return nil, fmt.Errorf("not an outer alias")
	}

	// Navigate up the parent chain
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

	default:
		return nil, fmt.Errorf("unsupported sort for outer alias: %d", alias.Sort)
	}
}
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestResolveOuterAlias -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/outer_alias.go internal/component/outer_alias_test.go
git commit -m "feat(component): implement outer alias resolution

Uses de Bruijn indexing to navigate parent chain.
Only allows aliasing immutable items (types, components).

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 4: Implement Nested Component Instantiation

**Files:**
- Create: `internal/component/nested_component.go`
- Test: `internal/component/nested_component_test.go`

### Step 1: Write the failing test for nested instantiation

```go
// internal/component/nested_component_test.go
package component

import (
	"context"
	"testing"
)

func TestInstantiateNestedComponent_Basic(t *testing.T) {
	// Child component with one import
	child := &Component{
		Imports: []Import{
			{
				Name: "add-fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
	}

	// Parent instance with the function to provide
	parent := &Instance{
		componentFuncs: map[uint32]ComponentFunc{
			0: {
				Impl: func(ctx context.Context, args []Val) ([]Val, error) {
					return []Val{ValS32(42)}, nil
				},
			},
		},
	}

	// Component instance definition
	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "add-fn", Sort: SortFunc, Idx: 0},
		},
	}

	parentComponent := &Component{
		Components: []*Component{child},
	}

	l := &ComponentLinker{}

	ctx := context.Background()
	nestedInst, err := l.instantiateNestedComponent(ctx, parent, compInst, parentComponent)
	if err != nil {
		t.Fatalf("instantiateNestedComponent failed: %v", err)
	}

	if nestedInst == nil {
		t.Fatal("nested instance should not be nil")
	}

	if nestedInst.Parent() != parent {
		t.Error("nested instance parent should be set")
	}
}
```

### Step 2: Run test to verify it fails

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstantiateNestedComponent -v`

Expected: FAIL with "undefined: l.instantiateNestedComponent"

### Step 3: Write minimal implementation

```go
// internal/component/nested_component.go
package component

import (
	"context"
	"fmt"
)

// instantiateNestedComponent creates an instance of a nested component.
func (l *ComponentLinker) instantiateNestedComponent(
	ctx context.Context,
	parent *Instance,
	compInst *ComponentInstance,
	parentComponent *Component,
) (*Instance, error) {
	// Get the nested component definition
	if compInst.ComponentIdx >= uint32(len(parentComponent.Components)) {
		return nil, fmt.Errorf("component index %d out of range", compInst.ComponentIdx)
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

	// Type check the arguments
	typeChecker := NewTypeChecker(nestedComp)
	for _, imp := range nestedComp.Imports {
		def, ok := withArgs[imp.Name]
		if !ok {
			return nil, fmt.Errorf("missing import: %s", imp.Name)
		}
		if err := typeChecker.CheckDefinition(&imp.ExternDesc, imp.Name, def); err != nil {
			return nil, fmt.Errorf("import %q: %w", imp.Name, err)
		}
	}

	// For now, store resolved imports but don't fully instantiate
	// Full instantiation would recurse through core modules, etc.
	_ = withArgs

	return nestedInst, nil
}

// resolveFromParentScope resolves an instantiation argument from parent scope.
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
			return nil, fmt.Errorf("func %d not found", arg.Idx)
		}
		return &FuncDef{Type: fn.Type, Callback: fn.Impl}, nil

	case SortInstance:
		// Instance from parent's instance space
		inst := parent.GetInstanceFromSpace(arg.Idx)
		if inst == nil {
			return nil, fmt.Errorf("instance %d not found", arg.Idx)
		}
		return instanceToDefinition(inst), nil

	case SortType:
		// Type from parent's type space
		typeDef := parent.GetTypeFromSpace(arg.Idx)
		if typeDef == nil {
			// Fall back to component's types
			if int(arg.Idx) < len(parentComponent.Types) {
				return &parentComponent.Types[arg.Idx], nil
			}
			return nil, fmt.Errorf("type %d not found", arg.Idx)
		}
		return typeDef, nil

	case SortComponent:
		// Nested component (for further nesting)
		if int(arg.Idx) < len(parentComponent.Components) {
			return &ComponentDef{Component: parentComponent.Components[arg.Idx]}, nil
		}
		return nil, fmt.Errorf("component %d not found", arg.Idx)

	case SortValue:
		// Value from parent's value space
		val, err := parent.GetValue(arg.Idx)
		if err != nil {
			return nil, fmt.Errorf("value %d: %w", arg.Idx, err)
		}
		return &ValueDef{Value: val}, nil

	default:
		return nil, fmt.Errorf("unsupported sort: %d", arg.Sort)
	}
}

// instanceToDefinition converts an Instance to an InstanceDef for passing to child.
func instanceToDefinition(inst *Instance) *InstanceDef {
	exports := make(map[string]Definition)
	for name, fn := range inst.exports {
		if fn != nil {
			exports[name] = &FuncDef{
				Type:     fn.funcType,
				Callback: fn.Call,
			}
		}
	}
	return &InstanceDef{Exports: exports}
}
```

Note: You'll need to add the import for `api` at the top of the file:
```go
import (
	"github.com/tetratelabs/wazero/api"
)
```

### Step 4: Run test to verify it passes

Run: `CGO_ENABLED=0 go test ./internal/component/... -run TestInstantiateNestedComponent -v`

Expected: PASS

### Step 5: Commit

```bash
git add internal/component/nested_component.go internal/component/nested_component_test.go
git commit -m "feat(component): implement nested component instantiation

Resolves with arguments from parent scope.
Sets up parent/child relationship.
Type checks arguments before instantiation.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 5: Process ComponentInstances in Instantiate

**Files:**
- Modify: `internal/component/component_linker.go`

### Step 1: Identify integration point

Find the comment in `component_linker.go`:
```go
// Component instance definitions also add to the instance index space
```

This is where we currently just count component instances without processing them.

### Step 2: Replace counting with actual instantiation

```go
// Replace the counting loop with:

// Process nested component instances
for i := range c.ComponentInstances {
	compInst := &c.ComponentInstances[i]
	if compInst.Kind == ComponentInstanceExprInstantiate {
		nestedInst, err := l.instantiateNestedComponent(ctx, inst, compInst, c)
		if err != nil {
			return nil, fmt.Errorf("component instance %d: %w", i, err)
		}
		inst.AddInstanceToSpace(nestedInst)
	}
	// Handle inline component instances if needed
}
```

### Step 3: Run tests to verify no regression

Run: `CGO_ENABLED=0 go test ./internal/component/... -v`

Expected: All PASS

### Step 4: Commit

```bash
git add internal/component/component_linker.go
git commit -m "feat(component): process ComponentInstances during Instantiate

Nested components are now actually instantiated.
Results added to instance index space.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Task 6: Run Phase 3 Regression Tests

**Files:** None (verification only)

### Step 1: Run all nested component tests

Run: `CGO_ENABLED=0 go test ./internal/component/... -run "NestedComponent|OuterAlias|Instance_Parent|Instance_IndexSpaces" -v`

Expected: All PASS

### Step 2: Run calculator regression tests

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/add -v`

Expected: PASS

Run: `CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run TestCalculatorPlugins/subtract -v`

Expected: PASS

### Step 3: Update progress tracker

Edit `docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md`:
- Mark Phase 3 status as `[x] Complete`
- Mark Phase 3 regression as `[x] Verified`

### Step 4: Commit

```bash
git add docs/plans/spec-instantiation-linking-conformance-fixes/00-root.md
git commit -m "docs: mark Phase 3 (Nested Components) complete

All nested component tests pass.
Calculator add/subtract regression tests pass.

Refs: docs/plans/spec-instantiation-linking-conformance-fixes"
```

---

## Phase 3 Complete

**Summary of changes:**
- Added parent/child tracking to Instance
- Added index spaces (instance, type, component) to Instance
- Implemented outer alias resolution with de Bruijn indexing
- Implemented nested component instantiation
- Integrated nested component processing into Instantiate

**Next steps:**
- Proceed to Phase 4 (Export Instance API)
- Phase 4 exposes nested instances through the public API
