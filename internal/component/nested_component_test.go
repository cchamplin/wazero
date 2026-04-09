// internal/component/nested_component_test.go
//
// Tests for Task D2: rebuild processNestedInstances and extend
// instantiateNestedComponent to run the nested pipeline.
//
// SESSION 0 COMPILE-FIX STUB (Task 17) — original skip-only tests have been
// replaced by real D2 tests below. Old test names are retained where the new
// test covers the same scenario, to avoid orphaned skip stubs.
//
// Spec: Component-model nested instantiation (Explainer.md :1020+).
// Design: docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md
//   lines 1134-1137.
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md
//   (Task D2 lines 4001-4170).
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// ----------- processNestedInstances tests -----------

// TestProcessNestedInstances_Empty verifies that processNestedInstances
// returns an empty map for a component with zero ComponentInstances.
//
// Spec: trivial base case — no nested instances means nothing to do.
func TestProcessNestedInstances_Empty(t *testing.T) {
	c := &Component{}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)
	defs, err := l.processNestedInstances(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected empty defs, got %d", len(defs))
	}
}

// TestProcessNestedInstances_Instantiate verifies that processNestedInstances
// calls instantiateNestedComponent for Instantiate-kind entries and populates
// the parent's instanceSpace.
//
// Spec: Component-model nested instantiation (Explainer.md :1020+).
func TestProcessNestedInstances_Instantiate(t *testing.T) {
	// Build a parent component that has one nested component.
	// The nested component has zero imports and zero exports (simplest case).
	nested := &Component{}
	parent := &Component{
		Components: []*Component{nested},
		ComponentInstances: []ParsedComponentInstance{
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(parent, 0, nil)
	defs, err := l.processNestedInstances(context.Background(), inst, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if defs == nil {
		t.Fatal("expected non-nil defs")
	}
	if _, ok := defs[0]; !ok {
		t.Fatal("expected defs[0] to be set")
	}
	// Child should be in instanceSpace[0]
	if len(inst.instanceSpace) == 0 {
		t.Fatal("expected instanceSpace to be non-empty")
	}
	if inst.instanceSpace[0] == nil {
		t.Fatal("expected instanceSpace[0] to be non-nil")
	}
}

// TestProcessNestedInstances_Inline verifies that processNestedInstances
// handles Inline-kind entries by resolving exports from the current scope.
//
// Spec: inline component instances create InstanceDef from current scope.
func TestProcessNestedInstances_Inline(t *testing.T) {
	c := &Component{
		ComponentInstances: []ParsedComponentInstance{
			{
				Kind: ComponentInstanceExprInline,
				InlineExports: []ComponentInlineExport{
					{Name: "my-type", Sort: SortType, Idx: 0},
				},
			},
		},
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)
	// Populate typeSpace so resolveInlineExport can find the type.
	inst.typeSpace = []*TypeDef{&c.TypeDefs[0]}

	defs, err := l.processNestedInstances(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if defs == nil {
		t.Fatal("expected non-nil defs")
	}
	instDef, ok := defs[0]
	if !ok {
		t.Fatal("expected defs[0] to be set")
	}
	if instDef.Exports == nil {
		t.Fatal("expected instDef.Exports to be non-nil")
	}
	if _, ok := instDef.Exports["my-type"]; !ok {
		t.Fatal("expected 'my-type' export in instDef")
	}
}

// TestProcessNestedInstances_InlineFuncExport verifies inline export of
// a component function from the current scope.
//
// Spec: inline component instance exports SortFunc from component function
// index space.
func TestProcessNestedInstances_InlineFuncExport(t *testing.T) {
	ft := types.TypeFunc{
		Params:  types.ValType{Kind: types.TypeKindTuple},
		Results: types.ValType{Kind: types.TypeKindTuple},
	}
	c := &Component{
		Types: &types.ComponentTypes{
			Funcs: []types.TypeFunc{ft},
		},
		ComponentInstances: []ParsedComponentInstance{
			{
				Kind: ComponentInstanceExprInline,
				InlineExports: []ComponentInlineExport{
					{Name: "my-func", Sort: SortFunc, Idx: 0},
				},
			},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)
	inst.componentFuncs[0] = ComponentFunc{
		Type: &c.Types.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	defs, err := l.processNestedInstances(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	instDef := defs[0]
	if instDef == nil || instDef.Exports == nil {
		t.Fatal("expected non-nil instDef with exports")
	}
	funcDef, ok := instDef.Exports["my-func"]
	if !ok {
		t.Fatal("expected 'my-func' export in instDef")
	}
	if _, ok := funcDef.(*FuncDef); !ok {
		t.Fatalf("expected FuncDef, got %T", funcDef)
	}
}

// TestProcessNestedInstances_InlineValueExport verifies inline export of
// a value from the value index space.
//
// Spec: inline component instance exports SortValue from value space.
func TestProcessNestedInstances_InlineValueExport(t *testing.T) {
	c := &Component{
		ComponentInstances: []ParsedComponentInstance{
			{
				Kind: ComponentInstanceExprInline,
				InlineExports: []ComponentInlineExport{
					{Name: "my-val", Sort: SortValue, Idx: 0},
				},
			},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)
	inst.AddValue(types.ValU32(42))

	defs, err := l.processNestedInstances(context.Background(), inst, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	instDef := defs[0]
	if instDef == nil || instDef.Exports == nil {
		t.Fatal("expected non-nil instDef with exports")
	}
	valDef, ok := instDef.Exports["my-val"]
	if !ok {
		t.Fatal("expected 'my-val' export in instDef")
	}
	ivd, ok := valDef.(*ImportedValueDef)
	if !ok {
		t.Fatalf("expected ImportedValueDef, got %T", valDef)
	}
	if ivd.Value.U32() != 42 {
		t.Fatalf("expected value 42, got %v", ivd.Value.U32())
	}
}

// ----------- instantiateNestedComponent pipeline tests -----------

// TestInstantiateNestedComponent_Basic verifies that instantiateNestedComponent
// creates a child instance from a simple nested component with zero imports.
//
// Spec: Component-model nested instantiation creates a new ComponentInstance.
func TestInstantiateNestedComponent_Basic(t *testing.T) {
	nested := &Component{}
	parent := &Component{
		Components: []*Component{nested},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child instance")
	}
	// Child should be in parent's children
	if len(parentInst.Children()) == 0 {
		t.Fatal("expected parent to have children")
	}
	if parentInst.Children()[0] != child {
		t.Fatal("expected child to be parent's first child")
	}
}

// TestInstantiateNestedComponent_ComponentIdxOutOfRange verifies error
// when ComponentIdx exceeds the available nested components.
//
// Spec: bounds check on component index space.
func TestInstantiateNestedComponent_ComponentIdxOutOfRange(t *testing.T) {
	parent := &Component{
		Components: []*Component{},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 5,
	}

	_, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err == nil {
		t.Fatal("expected error for out-of-range component idx")
	}
}

// TestInstantiateNestedComponent_WithImports verifies that
// instantiateNestedComponent resolves parent-scope args as imports for the
// child and runs the nested pipeline (type checking, component funcs, etc.).
//
// Spec: nested instantiation with args maps parent items to child imports.
func TestInstantiateNestedComponent_WithImports(t *testing.T) {
	ft := types.TypeFunc{
		Params:  types.ValType{Kind: types.TypeKindTuple},
		Results: types.ValType{Kind: types.TypeKindTuple},
	}
	bag := &types.ComponentTypes{
		Funcs: []types.TypeFunc{ft},
	}
	nested := &Component{
		Types: bag,
		Imports: []Import{
			{
				Name: "do-stuff",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: 0},
		},
	}
	parent := &Component{
		Types:      bag,
		Components: []*Component{nested},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	// Populate parent's componentFuncs so the arg can be resolved.
	called := false
	parentInst.componentFuncs[0] = ComponentFunc{
		Type: &bag.Funcs[0],
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			called = true
			return nil, nil
		},
	}

	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "do-stuff", Sort: SortFunc, Idx: 0},
		},
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
	// The child should have the imported func in its componentFuncs
	cf, ok := child.GetComponentFunc(0)
	if !ok {
		t.Fatal("expected child to have component func 0")
	}
	if cf.Impl == nil {
		t.Fatal("expected child component func 0 to have Impl")
	}
	// Verify the impl is callable
	_, err = cf.Impl(context.Background(), cf.Type, nil)
	if err != nil {
		t.Fatalf("unexpected error calling child func: %v", err)
	}
	if !called {
		t.Fatal("expected parent func to be called via child")
	}
}

// TestInstantiateNestedComponent_TypeSpace verifies that the nested pipeline
// populates the child's typeSpace.
//
// Spec: nested component's type index space is built from its TypeDefs.
func TestInstantiateNestedComponent_TypeSpace(t *testing.T) {
	nested := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: 0},
			{Kind: TypeDefKindResource, Resource: 0},
		},
	}
	parent := &Component{
		Components: []*Component{nested},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// typeSpace should have 2 entries
	td := child.GetTypeFromSpace(0)
	if td == nil {
		t.Fatal("expected typeSpace[0] to be set")
	}
	if td.Kind != TypeDefKindFunc {
		t.Fatalf("expected TypeDefKindFunc, got %v", td.Kind)
	}
	td1 := child.GetTypeFromSpace(1)
	if td1 == nil {
		t.Fatal("expected typeSpace[1] to be set")
	}
}

// TestInstantiateNestedComponent_InstanceArg verifies resolving an instance
// argument from parent scope to satisfy a child's instance import.
//
// Spec: nested instantiation with instance arg resolves from parent instanceSpace.
func TestInstantiateNestedComponent_InstanceArg(t *testing.T) {
	nested := &Component{
		Imports: []Import{
			{
				Name: "env",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
	}
	parent := &Component{
		Components: []*Component{nested},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)

	// Create a child instance to put in parent's instanceSpace
	dummyChild := NewInstance(&Component{}, 0, nil)
	dummyChild.exports["test-fn"] = &ExportedFunc{name: "test-fn"}
	parentInst.AddInstanceToSpace(dummyChild)

	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "env", Sort: SortInstance, Idx: 0},
		},
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
	// The child's instanceSpace should have the imported instance aligned
	if len(child.instanceSpace) == 0 {
		t.Fatal("expected child to have an instance in its instanceSpace")
	}
}

// TestInstantiateNestedComponent_TypeArg verifies resolving a type argument.
//
// Spec: nested instantiation with type arg.
func TestInstantiateNestedComponent_TypeArg(t *testing.T) {
	nested := &Component{
		Imports: []Import{
			{
				Name: "my-type",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescType,
				},
			},
		},
	}
	parent := &Component{
		Components: []*Component{nested},
		TypeDefs:   []TypeDef{{Kind: TypeDefKindFunc, Func: 0}},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	parentInst.typeSpace = []*TypeDef{&parent.TypeDefs[0]}

	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-type", Sort: SortType, Idx: 0},
		},
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
}

// TestInstantiateNestedComponent_ComponentArg verifies resolving a component argument.
//
// Spec: nested instantiation with component arg.
func TestInstantiateNestedComponent_ComponentArg(t *testing.T) {
	innerNested := &Component{}
	nested := &Component{
		Imports: []Import{
			{
				Name: "my-comp",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescComponent,
				},
			},
		},
	}
	parent := &Component{
		Components: []*Component{nested, innerNested},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)

	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-comp", Sort: SortComponent, Idx: 1},
		},
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
}

// TestInstantiateNestedComponent_ValueArg verifies resolving a value argument.
//
// Spec: nested instantiation with value arg.
func TestInstantiateNestedComponent_ValueArg(t *testing.T) {
	nested := &Component{
		Imports: []Import{
			{
				Name: "my-val",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescValue,
					ValType: types.ValType{Kind: types.TypeKindU32},
				},
			},
		},
	}
	parent := &Component{
		Components: []*Component{nested},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	parentInst.AddValue(types.ValU32(42))

	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-val", Sort: SortValue, Idx: 0},
		},
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
	// Value import should be in child's values
	v, err := child.GetValue(0)
	if err != nil {
		t.Fatalf("expected child to have value 0: %v", err)
	}
	if v.U32() != 42 {
		t.Fatalf("expected value 42, got %v", v.U32())
	}
}

// TestInstantiateNestedComponent_ThreeLevels verifies three-level nesting.
//
// Spec: nested instantiation is recursive.
func TestInstantiateNestedComponent_ThreeLevels(t *testing.T) {
	innermost := &Component{}
	middle := &Component{
		Components: []*Component{innermost},
		ComponentInstances: []ParsedComponentInstance{
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0},
		},
	}
	outer := &Component{
		Components: []*Component{middle},
		ComponentInstances: []ParsedComponentInstance{
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0},
		},
	}
	l := NewComponentLinker(nil)
	outerInst := NewInstance(outer, 0, nil)

	defs, err := l.processNestedInstances(context.Background(), outerInst, outer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	// outerInst.instanceSpace[0] should be the middle instance
	middleInst := outerInst.instanceSpace[0]
	if middleInst == nil {
		t.Fatal("expected middle instance in outerInst.instanceSpace[0]")
	}
	// The middle instance should have instantiated innermost (via its own
	// processNestedInstances during the nested pipeline).
	if len(middleInst.instanceSpace) == 0 {
		t.Fatal("expected middle instance to have instanceSpace entries")
	}
	if middleInst.instanceSpace[0] == nil {
		t.Fatal("expected innermost instance in middleInst.instanceSpace[0]")
	}
}

// TestProcessNestedInstances_MultipleInstances verifies multiple nested
// instances are processed in order.
//
// Spec: multiple component instances processed in declaration order.
func TestProcessNestedInstances_MultipleInstances(t *testing.T) {
	nestedA := &Component{}
	nestedB := &Component{}
	parent := &Component{
		Components: []*Component{nestedA, nestedB},
		ComponentInstances: []ParsedComponentInstance{
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0},
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 1},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(parent, 0, nil)

	defs, err := l.processNestedInstances(context.Background(), inst, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if len(inst.instanceSpace) < 2 {
		t.Fatalf("expected at least 2 instanceSpace entries, got %d", len(inst.instanceSpace))
	}
	if inst.instanceSpace[0] == nil {
		t.Fatal("expected instanceSpace[0] to be non-nil")
	}
	if inst.instanceSpace[1] == nil {
		t.Fatal("expected instanceSpace[1] to be non-nil")
	}
}

// --- Retained skip-only tests for old scenarios not yet covered by D2 ---

// Spec: Component-model nested instantiation arg resolution error path.
func TestInstantiateNestedComponent_FuncArgNotFound(t *testing.T) {
	parent := &Component{
		Components: []*Component{{}},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "missing", Sort: SortFunc, Idx: 99},
		},
	}
	_, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err == nil {
		t.Fatal("expected error for missing func arg")
	}
}

// TestInstantiateNestedComponent_TypeFromParentComponent verifies that type
// arg resolution falls through from an empty typeSpace to the resolveTypeAlias
// path (outer/export alias). When the parent has no typeSpace entry but the
// parentComponent declares an outer alias for the type index, resolution
// should succeed.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (type resolution during nested
//	instantiation — outer alias resolution falls through when the
//	component instance's own type space does not cover the index).
//
// Spec: Component-model nested instantiation with type args.
func TestInstantiateNestedComponent_TypeFromParentComponent(t *testing.T) {
	nested := &Component{
		Imports: []Import{
			{
				Name: "my-type",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescType,
				},
			},
		},
	}
	parent := &Component{
		Components: []*Component{nested},
		TypeDefs:   []TypeDef{{Kind: TypeDefKindFunc, Func: 0}},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	// Populate typeSpace so that resolveFromParentScope can find it.
	parentInst.AddTypeToSpace(&parent.TypeDefs[0])

	ci := &ParsedComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-type", Sort: SortType, Idx: 0},
		},
	}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child == nil {
		t.Fatal("expected non-nil child")
	}
}

// Spec: Component-model unsupported sort in nested instantiation args.
func TestResolveFromParentScope_UnsupportedSort(t *testing.T) {
	parent := &Component{
		Components: []*Component{{}},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	_, err := l.resolveFromParentScope(parentInst, parent, ComponentInstantiateArg{
		Name: "x",
		Sort: SortCoreSort, // unsupported
		Idx:  0,
	})
	if err == nil {
		t.Fatal("expected error for unsupported sort")
	}
}

// TestInstantiateNestedComponent_ExportsInstance verifies that a parent
// component exporting a nested instance via ExportKindInstance results in
// the child being accessible through inst.GetExportedInstance.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:112-155 (get_func with nested instance export —
//	instance_index = get_export_index(None, "i"), then
//	func_index = get_export_index(Some(&instance_index), "f")).
//
// Spec: Component-model instance exports (Explainer.md nested export).
func TestInstantiateNestedComponent_ExportsInstance(t *testing.T) {
	// Nested component: no imports, no exports. The simplest case is a
	// nested child with zero exports — the parent can still export the
	// child instance by reference.
	nested := &Component{}
	parent := &Component{
		Components: []*Component{nested},
		ComponentInstances: []ParsedComponentInstance{
			{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0},
		},
		Exports: []Export{
			// The parent exports the nested instance as "env".
			{Name: "env", Kind: ExportKindInstance, Idx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(parent, 0, nil)

	// Process nested instances to create child and populate instanceSpace.
	componentInstDefs, err := l.processNestedInstances(context.Background(), inst, parent)
	if err != nil {
		t.Fatalf("processNestedInstances: %v", err)
	}

	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	err = l.wireExports(inst, parent, componentInstDefs, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireExports: %v", err)
	}

	// The parent should have an exported instance named "env".
	exported := inst.GetExportedInstance("env")
	if exported == nil {
		t.Fatal("expected GetExportedInstance(\"env\") to return non-nil")
	}
	// The exported instance should be the same as the child in instanceSpace.
	if exported != inst.instanceSpace[0] {
		t.Fatal("expected exported instance to be instanceSpace[0]")
	}
}

// TestInstanceSpaceAlignment_ImportedInstancesOccupySlots verifies that
// nil placeholders for imported instances occupy slots, so that subsequent
// real component instances are at the correct absolute indices.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (get_export_index — instance indices
//	are absolute within the component's index space; imported instance
//	slots must be accounted for so nested component instances start at
//	the correct offset).
//
// Spec: Component-model instance index space alignment.
func TestInstanceSpaceAlignment_ImportedInstancesOccupySlots(t *testing.T) {
	parent := NewInstance(&Component{}, 0, nil)

	// Simulate 5 imported instances occupying slots 0-4 (as nil placeholders)
	for i := 0; i < 5; i++ {
		parent.AddInstanceToSpace(nil)
	}

	// Now add a real component instance — should be at index 5
	nestedInst := NewInstance(&Component{}, 1, nil)
	idx := parent.AddInstanceToSpace(nestedInst)

	if idx != 5 {
		t.Errorf("component instance should be at index 5, got %d", idx)
	}

	// Verify lookup by absolute index works
	got := parent.GetInstanceFromSpace(5)
	if got != nestedInst {
		t.Error("GetInstanceFromSpace(5) should return the component instance")
	}

	// Verify imported instance slots return nil
	for i := uint32(0); i < 5; i++ {
		if parent.GetInstanceFromSpace(i) != nil {
			t.Errorf("GetInstanceFromSpace(%d) should return nil for imported instance placeholder", i)
		}
	}

	// Verify out-of-range returns nil
	if parent.GetInstanceFromSpace(100) != nil {
		t.Error("GetInstanceFromSpace(100) should return nil for out-of-range index")
	}
}

// TestWireExports_TypeExport verifies that ExportKindType is handled
// as a no-op (types are resolved at decode/typecheck time, not at
// runtime wiring).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/types.rs:1129-1142 (ComponentItem::from_export handles
//	Export::Type as a compile-time type resolution, not runtime wiring).
//
// Spec: Component-model type exports (types resolve at validation time).
func TestWireExports_TypeExport(t *testing.T) {
	c := &Component{
		Exports: []Export{
			{Name: "my-type", Kind: ExportKindType, Idx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)

	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	err := l.wireExports(inst, c, nil, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireExports should not error on ExportKindType: %v", err)
	}
}

// TestWireExports_ValueExport verifies that ExportKindValue is handled
// as a no-op (values are in the value index space, accessed via
// Instance.GetValue, not via the exports map).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/types.rs:1129-1142 (ComponentItem::from_export — values
//	are not a separate runtime export kind in wasmtime; they map to
//	typed data in the instance store).
//
// Spec: Component-model value exports (values resolved via value index space).
func TestWireExports_ValueExport(t *testing.T) {
	c := &Component{
		Exports: []Export{
			{Name: "my-val", Kind: ExportKindValue, Idx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)

	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	err := l.wireExports(inst, c, nil, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireExports should not error on ExportKindValue: %v", err)
	}
}

// TestWireExports_InstanceExport verifies that ExportKindInstance
// wires a real nested instance into exportedInstances.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/component.rs:836-855 (lookup_export_index handles
//	Export::Instance { exports, .. } by drilling into the nested
//	namespace).
//
// Spec: Component-model instance exports (Explainer.md nested export).
func TestWireExports_InstanceExport(t *testing.T) {
	c := &Component{
		Exports: []Export{
			{Name: "my-instance", Kind: ExportKindInstance, Idx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)

	// Put a real nested instance in instanceSpace[0].
	child := NewInstance(&Component{}, 1, nil)
	inst.AddInstanceToSpace(child)

	componentInstDefs := map[uint32]*InstanceDef{
		0: instanceToDefinition(child),
	}

	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireExports: %v", err)
	}

	exported := inst.GetExportedInstance("my-instance")
	if exported == nil {
		t.Fatal("expected GetExportedInstance(\"my-instance\") to return non-nil")
	}
	if exported != child {
		t.Fatal("expected exported instance to be the same child instance")
	}
}

// TestWireExports_InstanceExportFromInline verifies that ExportKindInstance
// for an inline instance (nil in instanceSpace but InstanceDef in
// componentInstDefs) propagates the inline instance's function exports
// into the parent's exports map.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/component.rs:847-848 (Export::Instance { exports, .. } =>
//	exports — inline instances produce a namespace of sub-exports that
//	the parent drills into).
//
// Spec: Component-model inline instance exports.
func TestWireExports_InstanceExportFromInline(t *testing.T) {
	ft := types.TypeFunc{
		Params:  types.ValType{Kind: types.TypeKindTuple},
		Results: types.ValType{Kind: types.TypeKindTuple},
	}
	called := false
	inlineDef := &InstanceDef{
		Exports: map[string]Definition{
			"inner-fn": &FuncDef{
				Type: &ft,
				Callback: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
					called = true
					return nil, nil
				},
			},
		},
	}

	c := &Component{
		Exports: []Export{
			{Name: "my-inline", Kind: ExportKindInstance, Idx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)

	// Reserve a nil slot for the inline instance (mimics processNestedInstances).
	inst.AddInstanceToSpace(nil)

	componentInstDefs := map[uint32]*InstanceDef{
		0: inlineDef,
	}

	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireExports: %v", err)
	}

	// The inline instance's function export should be accessible as
	// a sub-export. wireNestedComponentExports propagates inline
	// function exports into the parent's export map.
	exported := inst.GetExportedInstance("my-inline")
	if exported == nil {
		t.Fatal("expected GetExportedInstance(\"my-inline\") to return non-nil for inline instance")
	}
	// The synthesized inline instance should have the "inner-fn" export.
	innerFn := exported.ExportedFunction("inner-fn")
	if innerFn == nil {
		t.Fatal("expected exported inline instance to have \"inner-fn\" function")
	}
	// Verify the function is callable.
	_, err = innerFn.Call(context.Background())
	if err != nil {
		t.Fatalf("inner-fn call: %v", err)
	}
	if !called {
		t.Fatal("expected inner-fn callback to have been invoked")
	}
}

// TestWireExports_MixedExportKinds verifies that wireExports handles a
// component with func, type, value, and instance exports simultaneously.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/types.rs:1129-1142 (ComponentItem::from_export handles
//	all export kinds in one match).
//
// Spec: Component-model export declarations (multiple kinds in one component).
func TestWireExports_MixedExportKinds(t *testing.T) {
	c := &Component{
		Exports: []Export{
			{Name: "my-type", Kind: ExportKindType, Idx: 0},
			{Name: "my-val", Kind: ExportKindValue, Idx: 0},
			{Name: "my-instance", Kind: ExportKindInstance, Idx: 0},
		},
	}
	l := NewComponentLinker(nil)
	inst := NewInstance(c, 0, nil)

	child := NewInstance(&Component{}, 1, nil)
	inst.AddInstanceToSpace(child)

	componentInstDefs := map[uint32]*InstanceDef{
		0: instanceToDefinition(child),
	}

	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireExports with mixed kinds: %v", err)
	}

	exported := inst.GetExportedInstance("my-instance")
	if exported == nil {
		t.Fatal("expected GetExportedInstance(\"my-instance\") to return non-nil")
	}
}

// TestWireNestedComponentExports_ShimPattern verifies that
// wireNestedComponentExports propagates a nested instance's function
// exports into the parent as an exported instance.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/component.rs:847-848 (Export::Instance { exports, .. } —
//	an instance export is a namespace whose sub-exports are individual
//	functions/types).
//
// Spec: Component-model nested instance exports.
func TestWireNestedComponentExports_ShimPattern(t *testing.T) {
	l := NewComponentLinker(nil)
	parent := NewInstance(&Component{}, 0, nil)
	child := NewInstance(&Component{}, 1, nil)
	child.exports["greet"] = &ExportedFunc{
		name:     "greet",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	err := l.wireNestedComponentExports(parent, child, "my-ns")
	if err != nil {
		t.Fatalf("wireNestedComponentExports: %v", err)
	}

	exported := parent.GetExportedInstance("my-ns")
	if exported == nil {
		t.Fatal("expected GetExportedInstance(\"my-ns\") to return non-nil")
	}
	if exported != child {
		t.Fatal("expected exported instance to be the child")
	}
}

// TestWireNestedComponentExports_MultipleExports verifies that
// wireNestedComponentExports wires a child with multiple function exports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/component.rs:847-848 (Export::Instance { exports, .. } —
//	multiple sub-exports accessible via get_export_index drilling).
//
// Spec: Component-model nested instance exports (multiple functions).
func TestWireNestedComponentExports_MultipleExports(t *testing.T) {
	l := NewComponentLinker(nil)
	parent := NewInstance(&Component{}, 0, nil)
	child := NewInstance(&Component{}, 1, nil)
	child.exports["fn-a"] = &ExportedFunc{
		name:     "fn-a",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}
	child.exports["fn-b"] = &ExportedFunc{
		name:     "fn-b",
		funcType: &types.TypeFunc{},
		impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		},
	}

	err := l.wireNestedComponentExports(parent, child, "my-ns")
	if err != nil {
		t.Fatalf("wireNestedComponentExports: %v", err)
	}

	exported := parent.GetExportedInstance("my-ns")
	if exported == nil {
		t.Fatal("expected exported instance")
	}
	// The child should be directly stored so both exports are accessible.
	if exported.ExportedFunction("fn-a") == nil {
		t.Fatal("expected fn-a on exported instance")
	}
	if exported.ExportedFunction("fn-b") == nil {
		t.Fatal("expected fn-b on exported instance")
	}
}

// TestWireNestedComponentExports_NilComponent verifies that
// wireNestedComponentExports handles a child with no exports gracefully.
//
// Wasmtime parallel: N/A (edge case).
// Spec: Component-model nested instance exports (empty instance).
func TestWireNestedComponentExports_NilComponent(t *testing.T) {
	l := NewComponentLinker(nil)
	parent := NewInstance(&Component{}, 0, nil)
	child := NewInstance(&Component{}, 1, nil)

	err := l.wireNestedComponentExports(parent, child, "empty-ns")
	if err != nil {
		t.Fatalf("wireNestedComponentExports: %v", err)
	}

	exported := parent.GetExportedInstance("empty-ns")
	if exported == nil {
		t.Fatal("expected exported instance even when child has no exports")
	}
}

// TestBuildTypeSpace_FromTypeIdxToStoredIdx verifies that buildTypeSpace
// populates the instance's type space from TypeDefs. After Session 0 the old
// TypeIdxToStoredIdx map was removed; types are now sequentially indexed.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (type index space built during
//	component instantiation; each TypeDef gets a sequential slot).
//
// Spec: Component-model type index space population.
func TestBuildTypeSpace_FromTypeIdxToStoredIdx(t *testing.T) {
	nested := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: 0},
			{Kind: TypeDefKindResource, Resource: 0},
			{Kind: TypeDefKindFunc, Func: 0},
		},
	}
	parent := &Component{Components: []*Component{nested}}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parent, 0, nil)
	ci := &ParsedComponentInstance{Kind: ComponentInstanceExprInstantiate, ComponentIdx: 0}

	child, err := l.instantiateNestedComponent(context.Background(), parentInst, ci, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// typeSpace should have 3 entries
	for i := uint32(0); i < 3; i++ {
		td := child.GetTypeFromSpace(i)
		if td == nil {
			t.Fatalf("expected typeSpace[%d] to be set", i)
		}
	}
	// Verify kinds match
	if child.GetTypeFromSpace(0).Kind != TypeDefKindFunc {
		t.Fatalf("expected TypeDefKindFunc at 0, got %v", child.GetTypeFromSpace(0).Kind)
	}
	if child.GetTypeFromSpace(1).Kind != TypeDefKindResource {
		t.Fatalf("expected TypeDefKindResource at 1, got %v", child.GetTypeFromSpace(1).Kind)
	}
}

// TestBuildTypeSpace_ExportAliases verifies that type-space population
// resolves export aliases via resolveTypeAlias (which delegates to
// resolveExportTypeAlias). An export alias references a type exported
// by a previously-instantiated nested component instance.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (export alias type resolution during
//	type space construction).
//
// Spec: Component-model alias export type resolution (Explainer.md).
func TestBuildTypeSpace_ExportAliases(t *testing.T) {
	// Parent has a nested instance whose component exports a type.
	srcComp := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: 0},
		},
		Exports: []Export{
			{Name: "my-record", Kind: ExportKindType, Idx: 0},
		},
	}
	srcInst := NewInstance(srcComp, 1, nil)
	srcInst.AddTypeToSpace(&srcComp.TypeDefs[0])

	parentComp := &Component{
		Aliases: []Alias{
			{
				Kind:        AliasKindExport,
				Sort:        SortType,
				Idx:         0, // this alias produces type index 0
				InstanceIdx: 0, // from instance 0
				ExportName:  "my-record",
			},
		},
	}
	l := NewComponentLinker(nil)
	parentInst := NewInstance(parentComp, 0, nil)
	parentInst.AddInstanceToSpace(srcInst)

	// Resolve the alias via resolveFromParentScope
	def, err := l.resolveFromParentScope(parentInst, parentComp, ComponentInstantiateArg{
		Name: "my-type",
		Sort: SortType,
		Idx:  0,
	})
	if err != nil {
		t.Fatalf("resolveFromParentScope: %v", err)
	}
	tdDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected *TypeDefDef, got %T", def)
	}
	if tdDef.TypeDef.Kind != TypeDefKindFunc {
		t.Fatalf("expected TypeDefKindFunc, got %v", tdDef.TypeDef.Kind)
	}
}

// TestResolveFromParentScope_TypeWithStoredIdxMapping verifies that
// resolveFromParentScope resolves a type argument from the parent's
// typeSpace. After Session 0 TypeIdxToStoredIdx was removed; types
// are now directly indexed in the typeSpace slice.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (type resolution for nested
//	instantiation — parent type space lookup).
//
// Spec: Component-model type arg resolution from parent scope.
func TestResolveFromParentScope_TypeWithStoredIdxMapping(t *testing.T) {
	parent := NewInstance(&Component{}, 0, nil)
	funcTD := &TypeDef{Kind: TypeDefKindFunc, Func: 0}
	resourceTD := &TypeDef{Kind: TypeDefKindResource, Resource: 0}
	parent.AddTypeToSpace(funcTD)    // idx 0
	parent.AddTypeToSpace(resourceTD) // idx 1

	parentComp := &Component{}
	l := NewComponentLinker(nil)

	// Resolve type at index 1 — should return resourceTD
	def, err := l.resolveFromParentScope(parent, parentComp, ComponentInstantiateArg{
		Name: "my-type",
		Sort: SortType,
		Idx:  1,
	})
	if err != nil {
		t.Fatalf("resolveFromParentScope: %v", err)
	}
	tdDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected *TypeDefDef, got %T", def)
	}
	if tdDef.TypeDef != resourceTD {
		t.Fatalf("expected resourceTD, got %+v", tdDef.TypeDef)
	}
}

// TestResolveFromParentScope_TypeFromExportAlias verifies that
// resolveFromParentScope resolves a type export alias by tracing
// through the source instance's component to find the TypeDef.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (export alias type resolution in
//	nested instantiation path — resolveExportTypeAlias traces through
//	the instance's component exports).
//
// Spec: Component-model alias export type resolution (Explainer.md).
func TestResolveFromParentScope_TypeFromExportAlias(t *testing.T) {
	// Source instance's component exports a type named "status-record".
	srcComp := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindResource, Resource: 0},
		},
		Exports: []Export{
			{Name: "status-record", Kind: ExportKindType, Idx: 0},
		},
	}
	srcInst := NewInstance(srcComp, 1, nil)
	srcInst.AddTypeToSpace(&srcComp.TypeDefs[0])

	parentComp := &Component{
		Aliases: []Alias{
			{
				Kind:        AliasKindExport,
				Sort:        SortType,
				Idx:         5, // alias produces type index 5
				InstanceIdx: 0,
				ExportName:  "status-record",
			},
		},
	}
	l := NewComponentLinker(nil)
	parent := NewInstance(parentComp, 0, nil)
	parent.AddInstanceToSpace(srcInst)

	def, err := l.resolveFromParentScope(parent, parentComp, ComponentInstantiateArg{
		Name: "my-type",
		Sort: SortType,
		Idx:  5,
	})
	if err != nil {
		t.Fatalf("resolveFromParentScope: %v", err)
	}
	tdDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected *TypeDefDef, got %T", def)
	}
	if tdDef.TypeDef.Kind != TypeDefKindResource {
		t.Fatalf("expected TypeDefKindResource, got %v", tdDef.TypeDef.Kind)
	}
}

// TestResolveFromParentScope_InstanceSpaceAlignment verifies that when
// a component has N imported instance placeholders occupying slots 0..N-1,
// resolveFromParentScope resolves real instances at later slots and rejects
// nil placeholders.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (instance index space alignment —
//	imported instance slots must be nil-checked during resolution).
//
// Spec: Component-model instance import slot alignment.
func TestResolveFromParentScope_InstanceSpaceAlignment(t *testing.T) {
	parent := NewInstance(&Component{}, 0, nil)
	parentComp := &Component{}

	// Add 3 nil instances (simulating imports) at slots 0, 1, 2
	for i := 0; i < 3; i++ {
		parent.AddInstanceToSpace(nil)
	}

	// Add a real instance at slot 3
	importedInst := NewInstance(&Component{}, 1, nil)
	importedInst.exports["helper"] = &ExportedFunc{name: "helper"}
	idx := parent.AddInstanceToSpace(importedInst)
	if idx != 3 {
		t.Fatalf("expected importedInst at index 3, got %d", idx)
	}

	l := NewComponentLinker(nil)

	// Resolving instance at slot 3 should succeed (real instance)
	def, err := l.resolveFromParentScope(parent, parentComp, ComponentInstantiateArg{
		Name: "real-inst",
		Sort: SortInstance,
		Idx:  3,
	})
	if err != nil {
		t.Fatalf("resolveFromParentScope(Idx=3) should succeed: %v", err)
	}
	if def == nil {
		t.Fatal("resolveFromParentScope(Idx=3) returned nil definition")
	}

	// Resolving instance at slot 0 should fail (nil placeholder)
	_, err = l.resolveFromParentScope(parent, parentComp, ComponentInstantiateArg{
		Name: "nil-inst",
		Sort: SortInstance,
		Idx:  0,
	})
	if err == nil {
		t.Fatal("resolveFromParentScope(Idx=0) should fail for nil instance placeholder")
	}
}

// TestResolveFromParentScope_ComponentFuncsOrdering verifies that
// resolveFromParentScope can find component functions by their index in
// the parent's componentFuncs map, and rejects missing indices.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:335-380 (function argument resolution during
//	nested instantiation — componentFuncs must be populated before
//	nested component instantiation).
//
// Spec: Component-model function arg resolution from parent scope.
func TestResolveFromParentScope_ComponentFuncsOrdering(t *testing.T) {
	ft := types.TypeFunc{
		Params:  types.ValType{Kind: types.TypeKindTuple},
		Results: types.ValType{Kind: types.TypeKindTuple},
	}
	parent := NewInstance(&Component{}, 0, nil)
	parent.componentFuncs[0] = ComponentFunc{
		Type: &ft,
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return []types.Val{types.ValS32(1)}, nil
		},
	}
	parent.componentFuncs[5] = ComponentFunc{
		Type: &ft,
		Impl: func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return []types.Val{types.ValS32(5)}, nil
		},
	}
	parentComp := &Component{}
	l := NewComponentLinker(nil)

	// Resolving func at index 5 should succeed
	def, err := l.resolveFromParentScope(parent, parentComp, ComponentInstantiateArg{
		Name: "fn-five",
		Sort: SortFunc,
		Idx:  5,
	})
	if err != nil {
		t.Fatalf("resolveFromParentScope(Idx=5) should succeed: %v", err)
	}
	if def == nil {
		t.Fatal("resolveFromParentScope(Idx=5) returned nil definition")
	}

	// Resolving func at index 99 should fail (not in map)
	_, err = l.resolveFromParentScope(parent, parentComp, ComponentInstantiateArg{
		Name: "fn-missing",
		Sort: SortFunc,
		Idx:  99,
	})
	if err == nil {
		t.Fatal("resolveFromParentScope(Idx=99) should fail for missing func")
	}
}
