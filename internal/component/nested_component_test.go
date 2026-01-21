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
