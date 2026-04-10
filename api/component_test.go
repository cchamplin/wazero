// api/component_test.go
package api_test

import (
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestComponentInterfaceExists(t *testing.T) {
	// Verify that the interface types exist and are usable
	var _ api.CompiledComponent
	var _ api.Component
	var _ api.ComponentFunc
	var _ api.ComponentLinker
	var _ api.ComponentImport
	var _ api.ComponentExport
}

func TestComponentFuncType(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	paramsTup := b.InternTuple([]types.ValType{types.U32, types.String_})
	resultsTup := b.InternTuple([]types.ValType{types.Bool})
	funcIdx := b.InternFunc(false, []string{"count", "label"}, paramsTup, resultsTup)
	ct := b.Finish()

	ft := api.NewComponentFuncType(&ct.Funcs[funcIdx], ct)
	if ft == nil {
		t.Fatal("NewComponentFuncType returned nil")
	}
	if ft.NumParams() != 2 {
		t.Errorf("NumParams() = %d, want 2", ft.NumParams())
	}
	if ft.NumResults() != 1 {
		t.Errorf("NumResults() = %d, want 1", ft.NumResults())
	}

	params := ft.Params()
	if len(params) != 2 {
		t.Fatalf("len(Params()) = %d, want 2", len(params))
	}
	if params[0].Name != "count" || params[0].Kind != types.TypeKindU32 {
		t.Errorf("Params()[0] = {%q, %v}, want {count, U32}", params[0].Name, params[0].Kind)
	}
	if params[1].Name != "label" || params[1].Kind != types.TypeKindString {
		t.Errorf("Params()[1] = {%q, %v}, want {label, String}", params[1].Name, params[1].Kind)
	}

	results := ft.Results()
	if len(results) != 1 {
		t.Fatalf("len(Results()) = %d, want 1", len(results))
	}
	if results[0].Kind != types.TypeKindBool {
		t.Errorf("Results()[0].Kind = %v, want Bool", results[0].Kind)
	}
	if results[0].Name != "" {
		t.Errorf("Results()[0].Name = %q, want empty", results[0].Name)
	}
}

func TestComponentImportFuncTypeField(t *testing.T) {
	// Verify that ComponentImport can hold a FuncType
	imp := api.ComponentImport{
		Name:     "test",
		Kind:     api.ComponentExportKindFunc,
		FuncType: nil, // non-function or unresolved
	}
	if imp.FuncType != nil {
		t.Error("expected nil FuncType")
	}
}

func TestComponentExportFuncTypeField(t *testing.T) {
	// Verify that ComponentExport can hold a FuncType
	exp := api.ComponentExport{
		Name:     "test",
		Kind:     api.ComponentExportKindInstance,
		FuncType: nil,
	}
	if exp.FuncType != nil {
		t.Error("expected nil FuncType")
	}
}
