// internal/component/type_checker_test.go
package component

import (
	"context"
	"testing"
)

func TestNewTypeChecker(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
	}

	tc := NewTypeChecker(c)

	if tc == nil {
		t.Fatal("NewTypeChecker returned nil")
	}
	if tc.component != c {
		t.Error("component not set correctly")
	}
	if tc.importedResources == nil {
		t.Error("importedResources map not initialized")
	}
}

func TestCheckFuncType_ExactMatch(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Params: []NamedValType{
						{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
						{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
					},
					Results: []NamedValType{
						{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
					},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	// Exact match should pass
	actual := &FuncType{
		Params: []NamedValType{
			{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
			{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		},
	}

	err := tc.checkFuncType(c.Types[0].Func, actual)
	if err != nil {
		t.Errorf("exact match should pass: %v", err)
	}
}

func TestCheckFuncType_InsufficientParams(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Params: []NamedValType{
						{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
						{Name: "b", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	// Fewer params should fail (contravariance means actual needs at least as many)
	actual := &FuncType{
		Params: []NamedValType{
			{Name: "a", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		},
	}

	err := tc.checkFuncType(c.Types[0].Func, actual)
	if err == nil {
		t.Error("insufficient params should fail")
	}
}

func TestCheckFuncType_ResultCountMismatch(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Results: []NamedValType{
						{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
					},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	// Wrong result count should fail
	actual := &FuncType{
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		},
	}

	err := tc.checkFuncType(c.Types[0].Func, actual)
	if err == nil {
		t.Error("result count mismatch should fail")
	}
}

func TestCheckInstance_ExtraExportsOK(t *testing.T) {
	// Instance type expects one export
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "required-fn",
								Kind: ExportKindFunc,
								Idx:  0,
							},
						},
					},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	// Actual has more exports - should pass (width subtyping)
	actual := &InstanceDef{
		Exports: map[string]Definition{
			"required-fn": &FuncDef{Callback: func(ctx context.Context, args []Val) ([]Val, error) { return nil, nil }},
			"extra-fn":    &FuncDef{Callback: func(ctx context.Context, args []Val) ([]Val, error) { return nil, nil }},
		},
	}

	err := tc.checkInstance(c.Types[0].Instance, actual)
	if err != nil {
		t.Errorf("extra exports should be OK: %v", err)
	}
}

func TestCheckInstance_MissingExport(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "required-fn",
								Kind: ExportKindFunc,
								Idx:  0,
							},
						},
					},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	// Actual missing required export - should fail
	actual := &InstanceDef{
		Exports: map[string]Definition{
			"wrong-name": &FuncDef{},
		},
	}

	err := tc.checkInstance(c.Types[0].Instance, actual)
	if err == nil {
		t.Error("missing required export should fail")
	}
}

func TestCheckResource_FirstOccurrence(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindResource},
		},
	}

	tc := NewTypeChecker(c)

	// First occurrence should be recorded
	actual := &ResourceDef{Destructor: func(rep uint32) {}}

	err := tc.checkResource(0, "wasi:test/res", actual)
	if err != nil {
		t.Errorf("first resource occurrence should pass: %v", err)
	}

	// Verify it was recorded
	if _, ok := tc.importedResources[0]; !ok {
		t.Error("resource should be recorded in importedResources")
	}
}

func TestCheckResource_SameResourceTwice(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindResource},
		},
	}

	tc := NewTypeChecker(c)

	actual := &ResourceDef{Destructor: func(rep uint32) {}}

	// First occurrence
	err := tc.checkResource(0, "wasi:test/res", actual)
	if err != nil {
		t.Fatalf("first occurrence failed: %v", err)
	}

	// Same resource from same import - should pass
	err = tc.checkResource(0, "wasi:test/res", actual)
	if err != nil {
		t.Errorf("same resource should pass: %v", err)
	}
}

func TestCheckResource_DifferentResource(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindResource},
		},
	}

	tc := NewTypeChecker(c)

	actual1 := &ResourceDef{Destructor: func(rep uint32) {}}
	actual2 := &ResourceDef{Destructor: func(rep uint32) {}}

	// First occurrence from import A
	err := tc.checkResource(0, "wasi:test/res-a", actual1)
	if err != nil {
		t.Fatalf("first occurrence failed: %v", err)
	}

	// Same index but different import - should fail
	err = tc.checkResource(0, "wasi:test/res-b", actual2)
	if err == nil {
		t.Error("different resource at same index should fail")
	}
}

func TestCheckResource_NilResource(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindResource},
		},
	}

	tc := NewTypeChecker(c)

	// Nil resource should fail
	err := tc.checkResource(0, "wasi:test/res", nil)
	if err == nil {
		t.Error("nil resource should fail")
	}
}

func TestCheckDefinition_Func(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Params:  []NamedValType{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
					Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	expected := &ImportExternDesc{
		Kind:    ImportExternDescFunc,
		TypeIdx: 0,
	}

	actual := &FuncDef{
		Type: &FuncType{
			Params:  []NamedValType{{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
			Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
		},
	}

	err := tc.CheckDefinition(expected, "test/fn", actual)
	if err != nil {
		t.Errorf("matching func should pass: %v", err)
	}
}

func TestCheckDefinition_Instance(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind:   InstanceDeclKindExport,
							Export: &InstanceExport{Name: "fn", Kind: ExportKindFunc},
						},
					},
				},
			},
		},
	}

	tc := NewTypeChecker(c)

	expected := &ImportExternDesc{
		Kind:    ImportExternDescInstance,
		TypeIdx: 0,
	}

	actual := &InstanceDef{
		Exports: map[string]Definition{
			"fn": &FuncDef{},
		},
	}

	err := tc.CheckDefinition(expected, "test/inst", actual)
	if err != nil {
		t.Errorf("matching instance should pass: %v", err)
	}
}

func TestCheckDefinition_WrongKind(t *testing.T) {
	c := &Component{
		Types: []TypeDef{
			{Kind: TypeDefKindFunc, Func: &FuncType{}},
		},
	}

	tc := NewTypeChecker(c)

	expected := &ImportExternDesc{
		Kind:    ImportExternDescFunc,
		TypeIdx: 0,
	}

	// Provide instance instead of func
	actual := &InstanceDef{}

	err := tc.CheckDefinition(expected, "test/fn", actual)
	if err == nil {
		t.Error("wrong definition kind should fail")
	}
}
