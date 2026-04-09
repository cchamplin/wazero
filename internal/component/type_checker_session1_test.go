// internal/component/type_checker_session1_test.go
//
// Session 1 Task F1 tests for checkFuncType, checkInstanceDefinition,
// and the recursive checkInstance/checkExportKind logic.
//
// Spec: definitions.py:88-101 FuncType identity; Explainer.md:920-982
// instance subtyping; wasmtime matching.rs:146-166.
package component

import (
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestCheckFuncTypeIdentityOnly verifies that checkFuncType compares
// Async/Params/Results by identity and ignores ParamNames.
func TestCheckFuncTypeIdentityOnly(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32})
	resultTuple := b.InternTuple([]types.ValType{types.S32})
	ct := b.Finish()

	expected := &types.TypeFunc{
		Async:      false,
		ParamNames: []string{"x"},
		Params:     paramTuple,
		Results:    resultTuple,
	}
	actual := &types.TypeFunc{
		Async:      false,
		ParamNames: []string{"y"}, // different name, same types
		Params:     paramTuple,
		Results:    resultTuple,
	}

	tc := NewTypeChecker(&Component{Types: ct})
	if err := tc.checkFuncType(expected, actual); err != nil {
		t.Fatalf("expected no error for identity match with different ParamNames, got: %v", err)
	}
}

// TestCheckFuncTypeNilMismatch verifies that one nil and one non-nil
// TypeFunc produces an error, while both nil passes.
func TestCheckFuncTypeNilMismatch(t *testing.T) {
	tc := NewTypeChecker(&Component{})

	// Both nil: pass
	if err := tc.checkFuncType(nil, nil); err != nil {
		t.Fatalf("both nil should pass, got: %v", err)
	}

	// One nil, one non-nil: error
	nonNil := &types.TypeFunc{}
	if err := tc.checkFuncType(nonNil, nil); err == nil {
		t.Fatal("expected error when expected is non-nil but actual is nil")
	} else if !strings.Contains(err.Error(), "one side is nil") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if err := tc.checkFuncType(nil, nonNil); err == nil {
		t.Fatal("expected error when expected is nil but actual is non-nil")
	} else if !strings.Contains(err.Error(), "one side is nil") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestCheckInstanceDefinitionRecursivelyTypeChecks verifies that
// checkInstanceDefinition resolves the expected instance type and
// recursively walks its exports: matching, mismatching kind, and
// missing exports.
func TestCheckInstanceDefinitionRecursivelyTypeChecks(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32})
	resultTuple := b.InternTuple([]types.ValType{types.S32})
	funcIdx := b.InternFunc(false, []string{"a"}, paramTuple, resultTuple)
	ct := b.Finish()

	// TypeDefs[0] = func type, TypeDefs[1] = instance type with one func export "run"
	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "run",
								Kind: ExportKindFunc,
								Idx:  0, // refers to TypeDefs[0]
							},
						},
					},
				},
			},
		},
		NextTypeIdx: 2,
	}

	expected := &ImportExternDesc{
		Kind:    ImportExternDescInstance,
		TypeIdx: 1, // TypeDefs[1]
	}

	tc := NewTypeChecker(c)

	t.Run("matching", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"run": &FuncDef{Callback: nil}, // host func, Type==nil, passes Decision 6
			},
		}
		if err := tc.checkInstanceDefinition(expected, actual); err != nil {
			t.Fatalf("expected pass for matching instance, got: %v", err)
		}
	})

	t.Run("missing export", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{},
		}
		if err := tc.checkInstanceDefinition(expected, actual); err == nil {
			t.Fatal("expected error for missing export")
		} else if !strings.Contains(err.Error(), "missing required export: run") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("wrong kind", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"run": &InstanceDef{Exports: map[string]Definition{}},
			},
		}
		if err := tc.checkInstanceDefinition(expected, actual); err == nil {
			t.Fatal("expected error for wrong kind")
		} else if !strings.Contains(err.Error(), "expected function") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not an InstanceDef", func(t *testing.T) {
		actual := &FuncDef{}
		if err := tc.checkInstanceDefinition(expected, actual); err == nil {
			t.Fatal("expected error for non-instance definition")
		} else if !strings.Contains(err.Error(), "expected instance") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("extra exports OK (width subtyping)", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"run":   &FuncDef{Callback: nil},
				"extra": &FuncDef{Callback: nil},
			},
		}
		if err := tc.checkInstanceDefinition(expected, actual); err != nil {
			t.Fatalf("expected pass for extra exports (width subtyping), got: %v", err)
		}
	})
}

// TestCheckInstanceDefinitionThreeLevelNesting tests recursive instance
// type checking through 3 nested levels:
//
//	expected instance type (level 0) exports "child" → instance type (level 1) exports "leaf" → func type
//
// Spec: wasmtime matching.rs:146-166 (instance recurses into definition()).
func TestCheckInstanceDefinitionThreeLevelNesting(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.U8})
	resultTuple := b.InternTuple([]types.ValType{types.U8})
	funcIdx := b.InternFunc(false, nil, paramTuple, resultTuple)
	ct := b.Finish()

	// TypeDefs[0] = func type
	// TypeDefs[1] = level 1 instance type with export "leaf" → func (TypeDefs[0])
	// TypeDefs[2] = level 0 instance type with export "child" → instance (TypeDefs[1])
	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "leaf",
								Kind: ExportKindFunc,
								Idx:  0,
							},
						},
					},
				},
			},
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "child",
								Kind: ExportKindInstance,
								Idx:  1, // TypeDefs[1]
							},
						},
					},
				},
			},
		},
		NextTypeIdx: 3,
	}

	expected := &ImportExternDesc{
		Kind:    ImportExternDescInstance,
		TypeIdx: 2, // TypeDefs[2]
	}

	tc := NewTypeChecker(c)

	t.Run("3-level match", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"child": &InstanceDef{
					Exports: map[string]Definition{
						"leaf": &FuncDef{Callback: nil},
					},
				},
			},
		}
		if err := tc.checkInstanceDefinition(expected, actual); err != nil {
			t.Fatalf("expected 3-level nesting to pass, got: %v", err)
		}
	})

	t.Run("missing leaf", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"child": &InstanceDef{
					Exports: map[string]Definition{},
				},
			},
		}
		err := tc.checkInstanceDefinition(expected, actual)
		if err == nil {
			t.Fatal("expected error for missing leaf export in nested instance")
		}
		if !strings.Contains(err.Error(), "missing required export: leaf") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("wrong kind at level 1", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"child": &FuncDef{}, // should be InstanceDef
			},
		}
		err := tc.checkInstanceDefinition(expected, actual)
		if err == nil {
			t.Fatal("expected error for wrong kind at child level")
		}
		if !strings.Contains(err.Error(), "expected instance") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("skip validation on nested instance", func(t *testing.T) {
		actual := &InstanceDef{
			Exports: map[string]Definition{
				"child": &InstanceDef{
					SkipValidation: true,
					Exports:        map[string]Definition{}, // missing "leaf" but skip
				},
			},
		}
		if err := tc.checkInstanceDefinition(expected, actual); err != nil {
			t.Fatalf("expected skip-validation to bypass nested checks, got: %v", err)
		}
	})
}

// TestCheckFuncTypeParamsMismatch verifies that differing Params
// tuple indices produce an error.
func TestCheckFuncTypeParamsMismatch(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	t1 := b.InternTuple([]types.ValType{types.S32})
	t2 := b.InternTuple([]types.ValType{types.S64})
	resultTuple := b.InternTuple([]types.ValType{types.S32})
	ct := b.Finish()

	tc := NewTypeChecker(&Component{Types: ct})

	expected := &types.TypeFunc{Params: t1, Results: resultTuple}
	actual := &types.TypeFunc{Params: t2, Results: resultTuple}

	if err := tc.checkFuncType(expected, actual); err == nil {
		t.Fatal("expected params mismatch error")
	} else if !strings.Contains(err.Error(), "params tuple index mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckFuncTypeAsyncMismatch verifies async flag mismatch is caught.
func TestCheckFuncTypeAsyncMismatch(t *testing.T) {
	tc := NewTypeChecker(&Component{})

	expected := &types.TypeFunc{Async: true}
	actual := &types.TypeFunc{Async: false}

	if err := tc.checkFuncType(expected, actual); err == nil {
		t.Fatal("expected async mismatch error")
	} else if !strings.Contains(err.Error(), "async mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}
