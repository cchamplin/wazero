// internal/component/type_checker_test.go
package component

import (
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
