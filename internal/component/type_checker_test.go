// internal/component/type_checker_test.go
//
// Restored from Session 0 compile-fix stubs (Task F2). Each test exercises
// the type_checker.go API using the current identity-based comparison model.
//
// Spec: definitions.py:88-101 FuncType identity; Explainer.md:920-982
// instance subtyping; wasmtime matching.rs:32-114 TypeChecker::definition.
package component

import (
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// TestNewTypeChecker verifies that NewTypeChecker returns a non-nil
// TypeChecker with an initialised importedResources map.
//
// Spec: wasmtime matching.rs:18-30 TypeChecker constructor initialises
// empty resource state.
func TestNewTypeChecker(t *testing.T) {
	c := &Component{}
	tc := NewTypeChecker(c)
	if tc == nil {
		t.Fatal("NewTypeChecker returned nil")
	}
	if tc.component != c {
		t.Fatal("TypeChecker.component not set correctly")
	}
	if tc.importedResources == nil {
		t.Fatal("importedResources map not initialised")
	}
}

// TestCheckFuncType_ExactMatch verifies that two TypeFunc values with
// identical Async/Params/Results pass checkFuncType.
//
// Spec: definitions.py:88-101 FuncType identity — identical tuple
// indices mean identical parameter/result shapes.
func TestCheckFuncType_ExactMatch(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32, types.S64})
	resultTuple := b.InternTuple([]types.ValType{types.Bool})
	ct := b.Finish()

	expected := &types.TypeFunc{
		Async:      false,
		ParamNames: []string{"a", "b"},
		Params:     paramTuple,
		Results:    resultTuple,
	}
	actual := &types.TypeFunc{
		Async:      false,
		ParamNames: []string{"a", "b"},
		Params:     paramTuple,
		Results:    resultTuple,
	}

	tc := NewTypeChecker(&Component{Types: ct})
	if err := tc.checkFuncType(expected, actual); err != nil {
		t.Fatalf("exact match should pass, got: %v", err)
	}
}

// TestCheckFuncType_InsufficientParams verifies that differing Params
// tuple indices are rejected. "Insufficient" in the original test name
// referred to the old per-field walk; under identity comparison any
// difference in the Params index is an error.
//
// Spec: definitions.py:88-101 FuncType identity — params tuple index
// must be identical.
func TestCheckFuncType_InsufficientParams(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	t1 := b.InternTuple([]types.ValType{types.S32, types.S64})
	t2 := b.InternTuple([]types.ValType{types.S32}) // fewer elements
	resultTuple := b.InternTuple([]types.ValType{types.Bool})
	ct := b.Finish()

	tc := NewTypeChecker(&Component{Types: ct})

	expected := &types.TypeFunc{Params: t1, Results: resultTuple}
	actual := &types.TypeFunc{Params: t2, Results: resultTuple}

	if err := tc.checkFuncType(expected, actual); err == nil {
		t.Fatal("expected params mismatch error for insufficient params")
	} else if !strings.Contains(err.Error(), "params tuple index mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckFuncType_ResultCountMismatch verifies that differing Results
// tuple indices are rejected.
//
// Spec: definitions.py:88-101 FuncType identity — results tuple index
// must be identical.
func TestCheckFuncType_ResultCountMismatch(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32})
	r1 := b.InternTuple([]types.ValType{types.S32})
	r2 := b.InternTuple([]types.ValType{types.S32, types.S64})
	ct := b.Finish()

	tc := NewTypeChecker(&Component{Types: ct})

	expected := &types.TypeFunc{Params: paramTuple, Results: r1}
	actual := &types.TypeFunc{Params: paramTuple, Results: r2}

	if err := tc.checkFuncType(expected, actual); err == nil {
		t.Fatal("expected results mismatch error")
	} else if !strings.Contains(err.Error(), "results tuple index mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckInstance_ExtraExportsOK verifies that an actual instance with
// more exports than expected passes (width subtyping).
//
// Spec: Explainer.md:920-982 instance subtyping — an instance that
// provides a superset of the expected exports is a valid subtype.
func TestCheckInstance_ExtraExportsOK(t *testing.T) {
	expected := &InstanceTypeDef{
		Declarations: []InstanceDecl{
			{
				Kind: InstanceDeclKindExport,
				Export: &InstanceExport{
					Name: "greet",
					Kind: ExportKindFunc,
					Idx:  0,
				},
			},
		},
	}

	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.String_})
	resultTuple := b.InternTuple([]types.ValType{types.String_})
	funcIdx := b.InternFunc(false, []string{"name"}, paramTuple, resultTuple)
	ct := b.Finish()

	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		NextTypeIdx: 1,
	}

	actual := &InstanceDef{
		Exports: map[string]Definition{
			"greet": &FuncDef{Callback: nil},
			"bonus": &FuncDef{Callback: nil}, // extra export
		},
	}

	tc := NewTypeChecker(c)
	if err := tc.checkInstance(expected, actual); err != nil {
		t.Fatalf("extra exports should be allowed, got: %v", err)
	}
}

// TestCheckInstance_MissingExport verifies that checkInstance returns an
// error when a required export is absent.
//
// Spec: Explainer.md:920-982 instance subtyping — missing a declared
// export violates the subtyping relation.
func TestCheckInstance_MissingExport(t *testing.T) {
	expected := &InstanceTypeDef{
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
	}

	c := &Component{
		TypeDefs:    []TypeDef{{Kind: TypeDefKindFunc}},
		NextTypeIdx: 1,
	}

	actual := &InstanceDef{
		Exports: map[string]Definition{}, // missing "required-fn"
	}

	tc := NewTypeChecker(c)
	if err := tc.checkInstance(expected, actual); err == nil {
		t.Fatal("expected error for missing export")
	} else if !strings.Contains(err.Error(), "missing required export: required-fn") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckResource_FirstOccurrence verifies that checkResource records
// a resource on first encounter without error.
//
// Spec: wasmtime matching.rs:175-194 resource_drop equality check —
// first occurrence is recorded, no prior to conflict with.
func TestCheckResource_FirstOccurrence(t *testing.T) {
	tc := NewTypeChecker(&Component{})
	rd := &ResourceDef{Destructor: func(rep uint32) {}}

	if err := tc.checkResource(0, "my-resource", rd); err != nil {
		t.Fatalf("first occurrence should succeed, got: %v", err)
	}
	info, ok := tc.importedResources[0]
	if !ok {
		t.Fatal("resource not recorded after first occurrence")
	}
	if info.sourceImport != "my-resource" {
		t.Fatalf("sourceImport = %q, want %q", info.sourceImport, "my-resource")
	}
}

// TestCheckResource_SameResourceTwice verifies that presenting the same
// resource (same typeIdx and importName) a second time succeeds.
//
// Spec: wasmtime matching.rs:175-194 resource equality — second
// occurrence of identical (typeIdx, importName) pair passes.
func TestCheckResource_SameResourceTwice(t *testing.T) {
	tc := NewTypeChecker(&Component{})
	rd := &ResourceDef{Destructor: func(rep uint32) {}}

	if err := tc.checkResource(5, "stream", rd); err != nil {
		t.Fatalf("first occurrence: %v", err)
	}
	if err := tc.checkResource(5, "stream", rd); err != nil {
		t.Fatalf("second identical occurrence should succeed, got: %v", err)
	}
}

// TestCheckResource_DifferentResource verifies that presenting a
// different importName for an already-seen typeIdx returns an error.
//
// Spec: wasmtime matching.rs:175-194 resource equality — a type index
// bound to one resource cannot be re-bound to a different one.
func TestCheckResource_DifferentResource(t *testing.T) {
	tc := NewTypeChecker(&Component{})
	rd := &ResourceDef{Destructor: func(rep uint32) {}}

	if err := tc.checkResource(7, "original", rd); err != nil {
		t.Fatalf("first occurrence: %v", err)
	}
	if err := tc.checkResource(7, "different", rd); err == nil {
		t.Fatal("expected error for conflicting resource import name")
	} else if !strings.Contains(err.Error(), "resource type mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckResource_NilResource verifies that checkResource returns an
// error when the actual ResourceDef is nil.
//
// Spec: wasmtime matching.rs:175-194 — a nil resource cannot satisfy
// a resource import.
func TestCheckResource_NilResource(t *testing.T) {
	tc := NewTypeChecker(&Component{})

	if err := tc.checkResource(0, "some-resource", nil); err == nil {
		t.Fatal("expected error for nil resource")
	} else if !strings.Contains(err.Error(), "expected resource, got nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckDefinition_Func verifies that CheckDefinition dispatches to
// checkFuncDefinition for ImportExternDescFunc and validates correctly
// when the expected type resolves to a TypeDefKindFunc.
//
// Spec: wasmtime matching.rs:32-114 TypeChecker::definition — func
// arm resolves expected type and validates async bit.
func TestCheckDefinition_Func(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32})
	resultTuple := b.InternTuple([]types.ValType{types.S32})
	funcIdx := b.InternFunc(false, []string{"x"}, paramTuple, resultTuple)
	ct := b.Finish()

	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		NextTypeIdx: 1,
	}

	expected := &ImportExternDesc{
		Kind:    ImportExternDescFunc,
		TypeIdx: 0,
	}

	tc := NewTypeChecker(c)

	// Host-provided FuncDef with nil Type (Decision 6 dynamic-host model)
	actual := &FuncDef{Callback: nil}
	if err := tc.CheckDefinition(expected, "my-func", actual); err != nil {
		t.Fatalf("func definition with nil Type should pass, got: %v", err)
	}

	// FuncDef with matching Type (nested-component path)
	actualTyped := &FuncDef{
		Type: &ct.Funcs[funcIdx],
	}
	if err := tc.CheckDefinition(expected, "my-func", actualTyped); err != nil {
		t.Fatalf("func definition with matching Type should pass, got: %v", err)
	}
}

// TestCheckDefinition_Instance verifies that CheckDefinition dispatches
// to checkInstanceDefinition for ImportExternDescInstance and validates
// the declared exports.
//
// Spec: Explainer.md:920-982 instance subtyping; wasmtime
// matching.rs:146-166 TypeChecker::instance.
func TestCheckDefinition_Instance(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.U8})
	resultTuple := b.InternTuple([]types.ValType{types.U8})
	funcIdx := b.InternFunc(false, nil, paramTuple, resultTuple)
	ct := b.Finish()

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
								Name: "process",
								Kind: ExportKindFunc,
								Idx:  0,
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
		TypeIdx: 1,
	}

	tc := NewTypeChecker(c)

	actual := &InstanceDef{
		Exports: map[string]Definition{
			"process": &FuncDef{Callback: nil},
		},
	}
	if err := tc.CheckDefinition(expected, "my-instance", actual); err != nil {
		t.Fatalf("matching instance definition should pass, got: %v", err)
	}
}

// TestCheckDefinition_WrongKind verifies that CheckDefinition returns
// an error when the actual Definition's Go type does not match the
// expected import kind.
//
// Spec: wasmtime matching.rs:32-114 TypeChecker::definition — kind
// mismatch is an immediate error.
func TestCheckDefinition_WrongKind(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32})
	resultTuple := b.InternTuple([]types.ValType{types.S32})
	funcIdx := b.InternFunc(false, nil, paramTuple, resultTuple)
	ct := b.Finish()

	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		NextTypeIdx: 1,
	}

	expected := &ImportExternDesc{
		Kind:    ImportExternDescFunc,
		TypeIdx: 0,
	}

	tc := NewTypeChecker(c)

	// Provide an InstanceDef where a FuncDef is expected
	actual := &InstanceDef{Exports: map[string]Definition{}}
	if err := tc.CheckDefinition(expected, "wrong-kind", actual); err == nil {
		t.Fatal("expected error for wrong definition kind")
	} else if !strings.Contains(err.Error(), "expected function") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDefinitionTypes verifies that each concrete Definition type
// implements the Definition interface and that CheckDefinition
// dispatches correctly for component, value, and type import kinds.
//
// Spec: wasmtime matching.rs:32-114 TypeChecker::definition — each
// arm validates the actual definition's Go type.
func TestDefinitionTypes(t *testing.T) {
	tc := NewTypeChecker(&Component{})

	t.Run("ComponentDef", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescComponent}
		actual := &ComponentDef{Component: &Component{}}
		if err := tc.CheckDefinition(expected, "comp", actual); err != nil {
			t.Fatalf("ComponentDef should satisfy component import, got: %v", err)
		}
	})

	t.Run("ComponentDef wrong type", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescComponent}
		actual := &FuncDef{}
		if err := tc.CheckDefinition(expected, "comp", actual); err == nil {
			t.Fatal("expected error for FuncDef as component import")
		} else if !strings.Contains(err.Error(), "expected component") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ImportedValueDef", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescValue}
		actual := &ImportedValueDef{Value: types.Val{}}
		if err := tc.CheckDefinition(expected, "val", actual); err != nil {
			t.Fatalf("ImportedValueDef should satisfy value import, got: %v", err)
		}
	})

	t.Run("ImportedValueDef wrong type", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescValue}
		actual := &FuncDef{}
		if err := tc.CheckDefinition(expected, "val", actual); err == nil {
			t.Fatal("expected error for FuncDef as value import")
		} else if !strings.Contains(err.Error(), "expected value") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Type import passthrough", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescType}
		// Type imports are handled by type substitution; any actual passes
		actual := &FuncDef{}
		if err := tc.CheckDefinition(expected, "ty", actual); err != nil {
			t.Fatalf("type import should always pass, got: %v", err)
		}
	})
}

// TestCheckValType_RecordWidthSubtyping tests the identity-based
// comparison model. The original test name referenced record width
// subtyping (the old NamedValType walk); under the current model,
// identity comparison means the same tuple index is the same type.
// Different tuple indices (even if structurally identical) are
// distinct.
//
// Spec: definitions.py:88-101 FuncType identity — tuple indices are
// interned; same elements yield same index (builder dedup).
func TestCheckValType_RecordWidthSubtyping(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	// Intern two tuples with identical elements — the builder deduplicates
	// them to the same ValType index.
	t1 := b.InternTuple([]types.ValType{types.S32, types.String_})
	t2 := b.InternTuple([]types.ValType{types.S32, types.String_})
	resultTuple := b.InternTuple([]types.ValType{types.Bool})
	ct := b.Finish()

	if t1 != t2 {
		t.Fatalf("builder should dedup identical tuples: t1=%v, t2=%v", t1, t2)
	}

	tc := NewTypeChecker(&Component{Types: ct})

	expected := &types.TypeFunc{Params: t1, Results: resultTuple}
	actual := &types.TypeFunc{Params: t2, Results: resultTuple}

	// Same interned index → passes identity check
	if err := tc.checkFuncType(expected, actual); err != nil {
		t.Fatalf("identical interned tuples should match, got: %v", err)
	}

	// Build a fresh builder to get a genuinely different (wider) tuple
	b2 := types.NewComponentTypesBuilder()
	wider := b2.InternTuple([]types.ValType{types.S32, types.String_, types.U8})
	narrower := b2.InternTuple([]types.ValType{types.S32, types.String_})
	resultTuple2 := b2.InternTuple([]types.ValType{types.Bool})
	ct2 := b2.Finish()

	tc2 := NewTypeChecker(&Component{Types: ct2})
	exp2 := &types.TypeFunc{Params: narrower, Results: resultTuple2}
	act2 := &types.TypeFunc{Params: wider, Results: resultTuple2}

	if err := tc2.checkFuncType(exp2, act2); err == nil {
		t.Fatal("structurally wider tuple should NOT match narrower under identity comparison")
	} else if !strings.Contains(err.Error(), "params tuple index mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckFuncType_ExtraParams verifies that a TypeFunc with more
// parameters (larger tuple) than expected is rejected. Under identity
// comparison, any difference in Params index is an error regardless
// of direction.
//
// Spec: definitions.py:88-101 FuncType identity — params tuple index
// must be identical; extra parameters imply a different tuple index.
func TestCheckFuncType_ExtraParams(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	t1 := b.InternTuple([]types.ValType{types.S32})
	t2 := b.InternTuple([]types.ValType{types.S32, types.S64, types.U8}) // more elements
	resultTuple := b.InternTuple([]types.ValType{types.Bool})
	ct := b.Finish()

	tc := NewTypeChecker(&Component{Types: ct})

	expected := &types.TypeFunc{Params: t1, Results: resultTuple}
	actual := &types.TypeFunc{Params: t2, Results: resultTuple}

	if err := tc.checkFuncType(expected, actual); err == nil {
		t.Fatal("expected params mismatch error for extra params")
	} else if !strings.Contains(err.Error(), "params tuple index mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckDefinition_NilActual verifies that CheckDefinition returns
// a meaningful error when the actual Definition is an untyped nil
// interface or a wrong-typed definition for the expected import kind.
//
// Spec: wasmtime matching.rs:32-114 TypeChecker::definition — a nil
// or wrong-typed definition cannot satisfy any import.
func TestCheckDefinition_NilActual(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramTuple := b.InternTuple([]types.ValType{types.S32})
	resultTuple := b.InternTuple([]types.ValType{types.S32})
	funcIdx := b.InternFunc(false, nil, paramTuple, resultTuple)
	ct := b.Finish()

	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		NextTypeIdx: 1,
	}

	tc := NewTypeChecker(c)

	// Use an untyped nil interface — type assertion .(*FuncDef) yields ok=false.
	t.Run("untyped nil for func import", func(t *testing.T) {
		expected := &ImportExternDesc{
			Kind:    ImportExternDescFunc,
			TypeIdx: 0,
		}
		if err := tc.CheckDefinition(expected, "f", nil); err == nil {
			t.Fatal("expected error for nil definition on func import")
		} else if !strings.Contains(err.Error(), "expected function") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("untyped nil for instance import", func(t *testing.T) {
		expected := &ImportExternDesc{
			Kind:    ImportExternDescInstance,
			TypeIdx: 0,
		}
		if err := tc.CheckDefinition(expected, "i", nil); err == nil {
			t.Fatal("expected error for nil definition on instance import")
		} else if !strings.Contains(err.Error(), "expected instance") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("untyped nil for component import", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescComponent}
		if err := tc.CheckDefinition(expected, "c", nil); err == nil {
			t.Fatal("expected error for nil definition on component import")
		} else if !strings.Contains(err.Error(), "expected component") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("untyped nil for value import", func(t *testing.T) {
		expected := &ImportExternDesc{Kind: ImportExternDescValue}
		if err := tc.CheckDefinition(expected, "v", nil); err == nil {
			t.Fatal("expected error for nil definition on value import")
		} else if !strings.Contains(err.Error(), "expected value") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
