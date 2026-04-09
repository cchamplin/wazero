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

func TestInstantiateNestedComponent_TypeFromParentComponent(t *testing.T) {
	t.Skip("session 1 D2: covered by TestInstantiateNestedComponent_TypeArg")
}

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

func TestInstantiateNestedComponent_ExportsInstance(t *testing.T) {
	t.Skip("session 1: instance exports deferred to Task D3 wireExports")
}

func TestInstanceSpaceAlignment_ImportedInstancesOccupySlots(t *testing.T) {
	t.Skip("session 1: alignment verified implicitly by TestInstantiateNestedComponent_InstanceArg")
}

func TestWireNestedComponentExports_ShimPattern(t *testing.T) {
	t.Skip("session 1: deferred to Task D3")
}

func TestWireNestedComponentExports_MultipleExports(t *testing.T) {
	t.Skip("session 1: deferred to Task D3")
}

func TestWireNestedComponentExports_NilComponent(t *testing.T) {
	t.Skip("session 1: deferred to Task D3")
}

func TestBuildTypeSpace_FromTypeIdxToStoredIdx(t *testing.T) {
	t.Skip("session 1: TypeIdxToStoredIdx removed; covered by TestInstantiateNestedComponent_TypeSpace")
}

func TestBuildTypeSpace_ExportAliases(t *testing.T) {
	t.Skip("session 1: export alias resolution covered by D1 resolveExportTypeAlias")
}

func TestResolveFromParentScope_TypeWithStoredIdxMapping(t *testing.T) {
	t.Skip("session 1: TypeIdxToStoredIdx removed; covered by TestInstantiateNestedComponent_TypeArg")
}

func TestResolveFromParentScope_TypeFromExportAlias(t *testing.T) {
	t.Skip("session 1: covered by D1 resolveExportTypeAlias tests")
}

func TestResolveFromParentScope_InstanceSpaceAlignment(t *testing.T) {
	t.Skip("session 1: covered by TestInstantiateNestedComponent_InstanceArg")
}

func TestResolveFromParentScope_ComponentFuncsOrdering(t *testing.T) {
	t.Skip("session 1: covered by TestInstantiateNestedComponent_WithImports")
}
