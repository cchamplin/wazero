// internal/component/nested_component_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// nestedMockFunction implements api.Function for nested component tests.
type nestedMockFunction struct {
	internalapi.WazeroOnlyType
	callFn func(ctx context.Context, params ...uint64) ([]uint64, error)
}

func (m *nestedMockFunction) Definition() api.FunctionDefinition { return nil }
func (m *nestedMockFunction) Call(ctx context.Context, params ...uint64) ([]uint64, error) {
	return m.callFn(ctx, params...)
}
func (m *nestedMockFunction) CallWithStack(ctx context.Context, stack []uint64) error { return nil }

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

func TestInstantiateNestedComponent_ComponentIdxOutOfRange(t *testing.T) {
	parent := &Instance{}

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 5, // Out of range
		Args:         []ComponentInstantiateArg{},
	}

	parentComponent := &Component{
		Components: []*Component{}, // Empty
	}

	l := &ComponentLinker{}

	ctx := context.Background()
	_, err := l.instantiateNestedComponent(ctx, parent, compInst, parentComponent)
	if err == nil {
		t.Fatal("should fail when component index is out of range")
	}
}

func TestInstantiateNestedComponent_FuncArgNotFound(t *testing.T) {
	child := &Component{
		Imports: []Import{
			{
				Name: "my-func",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	parent := &Instance{
		componentFuncs: map[uint32]ComponentFunc{}, // Empty - no functions
	}

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-func", Sort: SortFunc, Idx: 0}, // Func 0 doesn't exist
		},
	}

	parentComponent := &Component{
		Components: []*Component{child},
	}

	l := &ComponentLinker{}

	ctx := context.Background()
	_, err := l.instantiateNestedComponent(ctx, parent, compInst, parentComponent)
	if err == nil {
		t.Fatal("should fail when function arg not found in parent")
	}
}

func TestInstantiateNestedComponent_InstanceArg(t *testing.T) {
	child := &Component{
		Imports: []Import{
			{
				Name: "my-instance",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
	}

	// Create a parent with an instance in its instance space
	parent := &Instance{}
	providedInstance := &Instance{}
	parent.AddInstanceToSpace(providedInstance)

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-instance", Sort: SortInstance, Idx: 0},
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

func TestInstantiateNestedComponent_TypeArg(t *testing.T) {
	child := &Component{
		Imports: []Import{
			{
				Name: "my-type",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescType,
					TypeIdx: 0,
				},
			},
		},
	}

	// Create a parent with a type in its type space
	parent := &Instance{}
	providedType := &TypeDef{Kind: TypeDefKindFunc}
	parent.AddTypeToSpace(providedType)

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-type", Sort: SortType, Idx: 0},
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
}

func TestInstantiateNestedComponent_ComponentArg(t *testing.T) {
	child := &Component{
		Imports: []Import{
			{
				Name: "my-component",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescComponent,
					TypeIdx: 0,
				},
			},
		},
	}

	// Parent component with a nested component in its component space
	nestedComponent := &Component{}

	parent := &Instance{}
	parent.AddComponentToSpace(nestedComponent)

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-component", Sort: SortComponent, Idx: 0},
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
}

func TestInstantiateNestedComponent_ValueArg(t *testing.T) {
	child := &Component{
		Imports: []Import{
			{
				Name: "my-value",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescValue,
					ValType: &ValTypeRef{IsPrimitive: true, Primitive: 0x7a}, // s32
				},
			},
		},
	}

	// Parent with a value in its value space
	parent := &Instance{}
	parent.AddValue(ValS32(123))

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-value", Sort: SortValue, Idx: 0},
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
}

func TestInstantiateNestedComponent_TypeFromParentComponent(t *testing.T) {
	// Child component that imports a type
	child := &Component{
		Imports: []Import{
			{
				Name: "my-type",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescType,
					TypeIdx: 0,
				},
			},
		},
	}

	// Parent with no types in type space, but parent component has types
	parent := &Instance{}

	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "my-type", Sort: SortType, Idx: 0}, // Type from parentComponent.Types
		},
	}

	parentComponent := &Component{
		Components: []*Component{child},
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
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
}

func TestResolveFromParentScope_UnsupportedSort(t *testing.T) {
	parent := &Instance{}
	parentComponent := &Component{}

	arg := ComponentInstantiateArg{
		Name: "test",
		Sort: SortCoreSort, // Core sort is not supported for component instantiation
		Idx:  0,
	}

	l := &ComponentLinker{}

	_, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err == nil {
		t.Fatal("should fail for unsupported sort")
	}
}

func TestInstantiateNestedComponent_ThreeLevels(t *testing.T) {
	// Create grandparent -> parent -> child hierarchy (3 levels)

	// Level 3: Grandchild component (deepest level)
	grandchild := &Component{
		Imports: []Import{
			{
				Name: "grandchild-fn",
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

	// Level 2: Child component that contains grandchild
	child := &Component{
		Imports: []Import{
			{
				Name: "child-fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
		Components: []*Component{grandchild},
	}

	// Level 1: Parent component that contains child
	parentComponent := &Component{
		Components: []*Component{child},
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
	}

	// Root instance (grandparent)
	grandparent := &Instance{
		componentFuncs: map[uint32]ComponentFunc{
			0: {
				Impl: func(ctx context.Context, args []Val) ([]Val, error) {
					return []Val{ValS32(1)}, nil
				},
			},
		},
	}

	l := &ComponentLinker{}
	ctx := context.Background()

	// Instantiate level 2 (child) from grandparent
	childCompInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "child-fn", Sort: SortFunc, Idx: 0},
		},
	}

	childInst, err := l.instantiateNestedComponent(ctx, grandparent, childCompInst, parentComponent)
	if err != nil {
		t.Fatalf("instantiating child (level 2) failed: %v", err)
	}

	if childInst == nil {
		t.Fatal("child instance should not be nil")
	}

	if childInst.Parent() != grandparent {
		t.Error("child's parent should be grandparent")
	}

	// Add a function to child instance for grandchild to import
	childInst.componentFuncs[0] = ComponentFunc{
		Impl: func(ctx context.Context, args []Val) ([]Val, error) {
			return []Val{ValS32(2)}, nil
		},
	}

	// Instantiate level 3 (grandchild) from child
	grandchildCompInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "grandchild-fn", Sort: SortFunc, Idx: 0},
		},
	}

	grandchildInst, err := l.instantiateNestedComponent(ctx, childInst, grandchildCompInst, child)
	if err != nil {
		t.Fatalf("instantiating grandchild (level 3) failed: %v", err)
	}

	if grandchildInst == nil {
		t.Fatal("grandchild instance should not be nil")
	}

	// Verify parent chain: grandchild -> child -> grandparent
	if grandchildInst.Parent() != childInst {
		t.Error("grandchild's parent should be child")
	}

	if grandchildInst.Parent().Parent() != grandparent {
		t.Error("grandchild's grandparent should be grandparent")
	}

	// Verify children relationships
	if len(grandparent.Children()) != 1 {
		t.Errorf("grandparent should have 1 child, got %d", len(grandparent.Children()))
	}

	if len(childInst.Children()) != 1 {
		t.Errorf("child should have 1 child (grandchild), got %d", len(childInst.Children()))
	}

	if len(grandchildInst.Children()) != 0 {
		t.Errorf("grandchild should have 0 children, got %d", len(grandchildInst.Children()))
	}
}

func TestInstantiateNestedComponent_ExportsInstance(t *testing.T) {
	// Create a child component that will be instantiated
	child := &Component{
		Imports: []Import{
			{
				Name: "util-fn",
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

	parentComponent := &Component{
		Components: []*Component{child},
	}

	// Component instance definition
	compInst := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "util-fn", Sort: SortFunc, Idx: 0},
		},
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

	// Export the nested instance from the parent
	exportName := "child-instance"
	parent.AddExportedInstance(exportName, nestedInst)

	// Verify the exported instance is accessible
	exported := parent.GetExportedInstance(exportName)
	if exported == nil {
		t.Fatal("exported instance should be retrievable")
	}

	if exported != nestedInst {
		t.Error("exported instance should be the same as the nested instance")
	}

	// Verify parent-child relationship is preserved
	if exported.Parent() != parent {
		t.Error("exported instance parent should be the parent")
	}

	// Verify that adding to instance space also works (separate from exports)
	parent.AddInstanceToSpace(nestedInst)
	fromSpace := parent.GetInstanceFromSpace(0)
	if fromSpace != nestedInst {
		t.Error("instance from space should match nested instance")
	}
}

// TestInstanceSpaceAlignment_ImportedInstancesOccupySlots verifies that imported
// instances occupy slots in the instance index space, so that subsequent component
// instances are at the correct indices. This was a bug where the instance space
// only contained component instances, causing misaligned indices when exports
// referenced component instances by their absolute index.
func TestInstanceSpaceAlignment_ImportedInstancesOccupySlots(t *testing.T) {
	parent := &Instance{}

	// Simulate 5 imported instances occupying slots 0-4 (as nil placeholders)
	for i := 0; i < 5; i++ {
		parent.AddInstanceToSpace(nil)
	}

	// Now add a real component instance — should be at index 5
	nestedInst := &Instance{
		exports: make(map[string]*ExportedFunc),
	}
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

// TestWireNestedComponentExports_ShimPattern verifies that wireNestedComponentExports
// correctly wires a shim component's exports. The shim pattern is used by wasm-tools
// to create interface exports: a shim component imports a canon-lifted function and
// re-exports it under a different name.
func TestWireNestedComponentExports_ShimPattern(t *testing.T) {
	// The shim component:
	//   (import "import-func-process" (func (type 0)))
	//   (export "process" (func 0))
	shimComp := &Component{
		Imports: []Import{
			{
				Name: "import-func-process",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{
				Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}}, // u32
			}},
		},
		Exports: []Export{
			{Name: "process", Kind: ExportKindFunc, Idx: 0},
		},
	}

	// The nested instance (shim) with empty exports to be wired
	nestedInst := &Instance{
		component: shimComp,
		exports:   make(map[string]*ExportedFunc),
	}

	// The component instance definition that instantiates the shim:
	//   (instantiate $shim (with "import-func-process" (func 30)))
	compInstDef := &ComponentInstance{
		Kind:         ComponentInstanceExprInstantiate,
		ComponentIdx: 0,
		Args: []ComponentInstantiateArg{
			{Name: "import-func-process", Sort: SortFunc, Idx: 30},
		},
	}

	// Parent component with func 30 as a canon lift
	parentComp := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{
				Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			}},
		},
		Canonicals: []CanonicalDef{
			{
				Kind:             CanonKindLift,
				ComponentFuncIdx: 30,
				CoreFuncIdx:      91, // core func index
				TypeIdx:          0,
			},
		},
		FuncIdxToCanonical: map[uint32]uint32{
			30: 0, // func 30 -> Canonicals[0]
		},
		Aliases: []Alias{
			{Kind: AliasKindCoreExport, CoreSort: CoreSortFunc, Idx: 91, InstanceIdx: 0, ExportName: "test:repro/handler#process"},
		},
	}

	// Mock core instance with the exported core function
	mockCoreFunc := &nestedMockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			return []uint64{42}, nil
		},
	}
	mockModule := &mockModuleForExport{
		exportedFuncs: map[string]api.Function{
			"test:repro/handler#process": mockCoreFunc,
		},
	}

	// Parent instance with core instances and componentFuncs
	parent := &Instance{
		component:     parentComp,
		coreInstances: []api.Module{mockModule},
		exports:       make(map[string]*ExportedFunc),
		componentFuncs: map[uint32]ComponentFunc{
			30: {
				Type: parentComp.Types[0].Func,
				Impl: nil, // Canon lift - Impl is nil
			},
		},
	}

	// Build index spaces
	funcSpace := NewCoreFuncIndexSpace()
	funcSpace.AddAlias(91, 0, "test:repro/handler#process")
	memSpace := NewCoreMemoryIndexSpace()

	l := &ComponentLinker{}

	err := l.wireNestedComponentExports(parent, parentComp, nestedInst, compInstDef, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireNestedComponentExports failed: %v", err)
	}

	// Verify the "process" export was wired on the shim instance
	processFunc := nestedInst.exports["process"]
	if processFunc == nil {
		t.Fatal("expected 'process' export to be wired on shim instance")
	}

	if processFunc.Name() != "process" {
		t.Errorf("expected export name 'process', got %q", processFunc.Name())
	}

	// Call the wired function and verify it delegates to the core function
	ctx := context.Background()
	results, err := processFunc.Call(ctx)
	if err != nil {
		t.Fatalf("process() call failed: %v", err)
	}
	if len(results) != 1 || results[0].S32() != 42 {
		t.Errorf("process() = %v, want [42]", results)
	}
}

// TestWireNestedComponentExports_MultipleExports verifies that multiple function
// exports from a shim component are all wired correctly.
func TestWireNestedComponentExports_MultipleExports(t *testing.T) {
	shimComp := &Component{
		Imports: []Import{
			{Name: "import-fn-a", ExternDesc: ImportExternDesc{Kind: ImportExternDescFunc, TypeIdx: 0}},
			{Name: "import-fn-b", ExternDesc: ImportExternDesc{Kind: ImportExternDescFunc, TypeIdx: 0}},
		},
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{
				Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			}},
		},
		Exports: []Export{
			{Name: "alpha", Kind: ExportKindFunc, Idx: 0},
			{Name: "beta", Kind: ExportKindFunc, Idx: 1},
		},
	}

	nestedInst := &Instance{
		component: shimComp,
		exports:   make(map[string]*ExportedFunc),
	}

	compInstDef := &ComponentInstance{
		Kind: ComponentInstanceExprInstantiate,
		Args: []ComponentInstantiateArg{
			{Name: "import-fn-a", Sort: SortFunc, Idx: 10},
			{Name: "import-fn-b", Sort: SortFunc, Idx: 11},
		},
	}

	parentComp := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{
				Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			}},
		},
		Canonicals: []CanonicalDef{
			{Kind: CanonKindLift, ComponentFuncIdx: 10, CoreFuncIdx: 50, TypeIdx: 0},
			{Kind: CanonKindLift, ComponentFuncIdx: 11, CoreFuncIdx: 51, TypeIdx: 0},
		},
		FuncIdxToCanonical: map[uint32]uint32{10: 0, 11: 1},
		Aliases: []Alias{
			{Kind: AliasKindCoreExport, CoreSort: CoreSortFunc, Idx: 50, InstanceIdx: 0, ExportName: "fn-a"},
			{Kind: AliasKindCoreExport, CoreSort: CoreSortFunc, Idx: 51, InstanceIdx: 0, ExportName: "fn-b"},
		},
	}

	mockModule := &mockModuleForExport{
		exportedFuncs: map[string]api.Function{
			"fn-a": &nestedMockFunction{callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) { return []uint64{10}, nil }},
			"fn-b": &nestedMockFunction{callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) { return []uint64{20}, nil }},
		},
	}

	parent := &Instance{
		component:     parentComp,
		coreInstances: []api.Module{mockModule},
		exports:       make(map[string]*ExportedFunc),
		componentFuncs: map[uint32]ComponentFunc{
			10: {Type: parentComp.Types[0].Func},
			11: {Type: parentComp.Types[0].Func},
		},
	}

	funcSpace := NewCoreFuncIndexSpace()
	funcSpace.AddAlias(50, 0, "fn-a")
	funcSpace.AddAlias(51, 0, "fn-b")
	memSpace := NewCoreMemoryIndexSpace()

	l := &ComponentLinker{}

	err := l.wireNestedComponentExports(parent, parentComp, nestedInst, compInstDef, funcSpace, memSpace)
	if err != nil {
		t.Fatalf("wireNestedComponentExports failed: %v", err)
	}

	// Verify both exports were wired
	if nestedInst.exports["alpha"] == nil {
		t.Fatal("expected 'alpha' export")
	}
	if nestedInst.exports["beta"] == nil {
		t.Fatal("expected 'beta' export")
	}

	ctx := context.Background()

	results, err := nestedInst.exports["alpha"].Call(ctx)
	if err != nil {
		t.Fatalf("alpha() failed: %v", err)
	}
	if results[0].S32() != 10 {
		t.Errorf("alpha() = %d, want 10", results[0].S32())
	}

	results, err = nestedInst.exports["beta"].Call(ctx)
	if err != nil {
		t.Fatalf("beta() failed: %v", err)
	}
	if results[0].S32() != 20 {
		t.Errorf("beta() = %d, want 20", results[0].S32())
	}
}

// TestWireNestedComponentExports_NilComponent verifies that wireNestedComponentExports
// handles a nil nested component gracefully.
func TestWireNestedComponentExports_NilComponent(t *testing.T) {
	nestedInst := &Instance{
		component: nil, // No component
		exports:   make(map[string]*ExportedFunc),
	}

	l := &ComponentLinker{}
	err := l.wireNestedComponentExports(nil, nil, nestedInst, &ComponentInstance{}, nil, nil)
	if err != nil {
		t.Fatalf("should not error on nil component: %v", err)
	}
}

// TestBuildTypeSpace_FromTypeIdxToStoredIdx verifies that buildTypeSpace correctly
// populates the instance's type space from a component with sparse type indices
// (where aliases consume some indices).
func TestBuildTypeSpace_FromTypeIdxToStoredIdx(t *testing.T) {
	// Component has 3 type section entries but aliases consumed indices 1, 3, 4.
	// TypeIdxToStoredIdx maps: 0->0, 2->1, 5->2
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			}},
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{{Name: "y", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0,
			2: 1,
			5: 2,
		},
		NextTypeIdx: 6,
	}

	inst := &Instance{}
	l := &ComponentLinker{}
	l.buildTypeSpace(inst, c)

	// Type 0 should be a Func type
	td0 := inst.GetTypeFromSpace(0)
	if td0 == nil {
		t.Fatal("type 0 should be populated")
	}
	if td0.Kind != TypeDefKindFunc {
		t.Errorf("type 0 should be Func, got %v", td0.Kind)
	}

	// Type 2 should be a record with field "x"
	td2 := inst.GetTypeFromSpace(2)
	if td2 == nil {
		t.Fatal("type 2 should be populated")
	}
	if td2.Record == nil || len(td2.Record.Fields) != 1 || td2.Record.Fields[0].Name != "x" {
		t.Errorf("type 2 should be record with field 'x', got %+v", td2)
	}

	// Type 5 should be a record with field "y"
	td5 := inst.GetTypeFromSpace(5)
	if td5 == nil {
		t.Fatal("type 5 should be populated")
	}
	if td5.Record == nil || len(td5.Record.Fields) != 1 || td5.Record.Fields[0].Name != "y" {
		t.Errorf("type 5 should be record with field 'y', got %+v", td5)
	}

	// Indices 1, 3, 4 are alias slots and should be nil
	for _, idx := range []uint32{1, 3, 4} {
		if inst.GetTypeFromSpace(idx) != nil {
			t.Errorf("type %d should be nil (alias slot), but was populated", idx)
		}
	}
}

// TestBuildTypeSpace_ExportAliases verifies that buildTypeSpace resolves export
// aliases to find the actual type definition through the instance type's declarations.
func TestBuildTypeSpace_ExportAliases(t *testing.T) {
	// The record type that will be exported by the instance type
	recordType := &TypeDef{
		Kind: TypeDefKindDefined,
		Record: &RecordTypeDef{
			Fields: []RecordField{{Name: "val", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
		},
	}

	// Instance type with a type declaration and an export referencing it
	instTypeDef := &InstanceTypeDef{
		Declarations: []InstanceDecl{
			{Kind: InstanceDeclKindType, Type: recordType},
			{Kind: InstanceDeclKindExport, Export: &InstanceExport{
				Name: "my-record",
				Kind: ExportKindType,
				Idx:  0, // references the first type decl above
			}},
		},
	}

	c := &Component{
		Types: []TypeDef{
			// Type 0: instance type that has the record export
			{Kind: TypeDefKindInstance, Instance: instTypeDef},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0, // instance type at index 0
		},
		Imports: []Import{
			{
				Name: "my-inst",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0, // references Types[0]
				},
			},
		},
		Aliases: []Alias{
			{
				Kind:        AliasKindExport,
				Sort:        SortType,
				Idx:         1, // this alias produces type index 1
				InstanceIdx: 0, // from instance 0
				ExportName:  "my-record",
			},
		},
	}

	inst := &Instance{}
	l := &ComponentLinker{}
	l.buildTypeSpace(inst, c)

	// Type 1 should be resolved to the record with field "val"
	td1 := inst.GetTypeFromSpace(1)
	if td1 == nil {
		t.Fatal("type 1 should be resolved from export alias")
	}
	if td1.Record == nil || len(td1.Record.Fields) != 1 || td1.Record.Fields[0].Name != "val" {
		t.Errorf("type 1 should be record with field 'val', got %+v", td1)
	}
}

// TestResolveFromParentScope_TypeWithStoredIdxMapping verifies that resolveFromParentScope
// correctly resolves a type argument using the TypeIdxToStoredIdx mapping when the
// type index doesn't directly correspond to the Types array index.
func TestResolveFromParentScope_TypeWithStoredIdxMapping(t *testing.T) {
	parentComponent := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
			{Kind: TypeDefKindDefined, Record: &RecordTypeDef{
				Fields: []RecordField{{Name: "data", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			}},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0:  0,
			10: 1, // type index 10 maps to stored index 1
		},
	}

	parent := &Instance{}

	l := &ComponentLinker{}
	arg := ComponentInstantiateArg{
		Name: "my-type",
		Sort: SortType,
		Idx:  10, // requesting type index 10
	}

	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope failed: %v", err)
	}

	tdDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected *TypeDefDef, got %T", def)
	}

	if tdDef.TypeDef.Record == nil || len(tdDef.TypeDef.Record.Fields) != 1 || tdDef.TypeDef.Record.Fields[0].Name != "data" {
		t.Errorf("expected record with field 'data', got %+v", tdDef.TypeDef)
	}
}

// TestResolveFromParentScope_TypeFromExportAlias verifies that resolveFromParentScope
// correctly resolves a type argument that comes from an export alias, tracing through
// the instance type's declarations to find the actual type.
func TestResolveFromParentScope_TypeFromExportAlias(t *testing.T) {
	// The record type that will be exported
	statusRecord := &TypeDef{
		Kind: TypeDefKindDefined,
		Record: &RecordTypeDef{
			Fields: []RecordField{{Name: "status", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
		},
	}

	instTypeDef := &InstanceTypeDef{
		Declarations: []InstanceDecl{
			{Kind: InstanceDeclKindType, Type: statusRecord},
			{Kind: InstanceDeclKindExport, Export: &InstanceExport{
				Name: "status-record",
				Kind: ExportKindType,
				Idx:  0,
			}},
		},
	}

	parentComponent := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindInstance, Instance: instTypeDef},
		},
		TypeIdxToStoredIdx: map[uint32]uint32{
			0: 0, // instance type at index 0
		},
		Imports: []Import{
			{
				Name: "some-inst",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
		Aliases: []Alias{
			{
				Kind:        AliasKindExport,
				Sort:        SortType,
				Idx:         5, // this alias produces type index 5
				InstanceIdx: 0,
				ExportName:  "status-record",
			},
		},
	}

	parent := &Instance{}

	l := &ComponentLinker{}
	arg := ComponentInstantiateArg{
		Name: "my-type",
		Sort: SortType,
		Idx:  5, // requesting the alias-produced type index
	}

	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope failed: %v", err)
	}

	tdDef, ok := def.(*TypeDefDef)
	if !ok {
		t.Fatalf("expected *TypeDefDef, got %T", def)
	}

	if tdDef.TypeDef.Record == nil || len(tdDef.TypeDef.Record.Fields) != 1 || tdDef.TypeDef.Record.Fields[0].Name != "status" {
		t.Errorf("expected record with field 'status', got %+v", tdDef.TypeDef)
	}
}

// TestResolveFromParentScope_InstanceSpaceAlignment verifies that when a component
// has N instance imports occupying slots 0..N-1, nested component instances start
// at slot N, and resolveFromParentScope correctly resolves (or rejects) by slot index.
func TestResolveFromParentScope_InstanceSpaceAlignment(t *testing.T) {
	parent := &Instance{}
	parentComponent := &Component{}

	// Add 3 nil instances (simulating imports) at slots 0, 1, 2
	for i := 0; i < 3; i++ {
		parent.AddInstanceToSpace(nil)
	}

	// Add a real instance at slot 3
	importedInst := &Instance{
		exports: map[string]*ExportedFunc{
			"helper": {name: "helper"},
		},
	}
	idx := parent.AddInstanceToSpace(importedInst)
	if idx != 3 {
		t.Fatalf("expected importedInst at index 3, got %d", idx)
	}

	l := &ComponentLinker{}

	// Resolving instance at slot 3 should succeed (real instance)
	arg := ComponentInstantiateArg{Name: "real-inst", Sort: SortInstance, Idx: 3}
	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope(Idx=3) should succeed: %v", err)
	}
	if def == nil {
		t.Fatal("resolveFromParentScope(Idx=3) returned nil definition")
	}

	// Resolving instance at slot 0 should fail (nil placeholder)
	arg = ComponentInstantiateArg{Name: "nil-inst", Sort: SortInstance, Idx: 0}
	_, err = l.resolveFromParentScope(parent, parentComponent, arg)
	if err == nil {
		t.Fatal("resolveFromParentScope(Idx=0) should fail for nil instance placeholder")
	}
}

// TestResolveFromParentScope_ComponentFuncsOrdering verifies that buildComponentFuncs
// must run before nested component instantiation, so that resolveFromParentScope can
// find component functions by their index in the parent's componentFuncs map.
func TestResolveFromParentScope_ComponentFuncsOrdering(t *testing.T) {
	parent := &Instance{
		componentFuncs: map[uint32]ComponentFunc{
			0: {
				Impl: func(ctx context.Context, args []Val) ([]Val, error) {
					return []Val{ValS32(1)}, nil
				},
			},
			5: {
				Impl: func(ctx context.Context, args []Val) ([]Val, error) {
					return []Val{ValS32(5)}, nil
				},
			},
		},
	}
	parentComponent := &Component{}

	l := &ComponentLinker{}

	// Resolving func at index 5 should succeed
	arg := ComponentInstantiateArg{Name: "fn-five", Sort: SortFunc, Idx: 5}
	def, err := l.resolveFromParentScope(parent, parentComponent, arg)
	if err != nil {
		t.Fatalf("resolveFromParentScope(Idx=5) should succeed: %v", err)
	}
	if def == nil {
		t.Fatal("resolveFromParentScope(Idx=5) returned nil definition")
	}

	// Resolving func at index 99 should fail (not in map)
	arg = ComponentInstantiateArg{Name: "fn-missing", Sort: SortFunc, Idx: 99}
	_, err = l.resolveFromParentScope(parent, parentComponent, arg)
	if err == nil {
		t.Fatal("resolveFromParentScope(Idx=99) should fail for missing func")
	}
}

// mockModuleForExport implements api.Module minimally for nested component tests.
type mockModuleForExport struct {
	internalapi.WazeroOnlyType
	exportedFuncs map[string]api.Function
}

func (m *mockModuleForExport) ExportedFunction(name string) api.Function {
	return m.exportedFuncs[name]
}

// Stub implementations for api.Module interface
func (m *mockModuleForExport) Name() string                         { return "mock" }
func (m *mockModuleForExport) Memory() api.Memory                   { return nil }
func (m *mockModuleForExport) ExportedMemory(name string) api.Memory { return nil }
func (m *mockModuleForExport) ExportedFunctionDefinitions() map[string]api.FunctionDefinition {
	return nil
}
func (m *mockModuleForExport) ExportedGlobal(name string) api.Global               { return nil }
func (m *mockModuleForExport) ExportedMemoryDefinitions() map[string]api.MemoryDefinition {
	return nil
}
func (m *mockModuleForExport) CloseWithExitCode(ctx context.Context, exitCode uint32) error {
	return nil
}
func (m *mockModuleForExport) Close(ctx context.Context) error    { return nil }
func (m *mockModuleForExport) IsClosed() bool                     { return false }
func (m *mockModuleForExport) NumericCustomSections() []api.CustomSection { return nil }
func (m *mockModuleForExport) CustomSections() []api.CustomSection { return nil }
func (m *mockModuleForExport) String() string                       { return "mock" }
