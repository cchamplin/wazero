// internal/component/compiled_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestCompiledComponentImports(t *testing.T) {
	c := &Component{
		Imports: []Import{
			{Name: "wasi:cli/environment@0.2.0", ExternDesc: ImportExternDesc{Kind: ImportExternDescFunc}},
			{Name: "wasi:io/streams@0.2.0", ExternDesc: ImportExternDesc{Kind: ImportExternDescInstance}},
		},
	}

	cc := NewCompiledComponent(c, nil, nil)
	imports := cc.Imports()

	if len(imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(imports))
	}

	if imports[0].Name != "wasi:cli/environment@0.2.0" {
		t.Errorf("expected import name 'wasi:cli/environment@0.2.0', got %q", imports[0].Name)
	}
}

func TestCompiledComponentExports(t *testing.T) {
	c := &Component{
		Exports: []Export{
			{Name: "run", Kind: ExportKindFunc},
			{Name: "memory", Kind: ExportKindInstance},
		},
	}

	cc := NewCompiledComponent(c, nil, nil)
	exports := cc.Exports()

	if len(exports) != 2 {
		t.Fatalf("expected 2 exports, got %d", len(exports))
	}

	if exports[0].Name != "run" {
		t.Errorf("expected export name 'run', got %q", exports[0].Name)
	}
}

func TestCompiledComponentClose(t *testing.T) {
	cc := NewCompiledComponent(&Component{}, nil, nil)
	err := cc.Close(context.Background())
	if err != nil {
		t.Errorf("unexpected error closing: %v", err)
	}
}

func TestConvertExportKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     ExportKind
		expected api.ComponentExportKind
	}{
		{
			name:     "ExportKindFunc",
			kind:     ExportKindFunc,
			expected: api.ComponentExportKindFunc,
		},
		{
			name:     "ExportKindValue",
			kind:     ExportKindValue,
			expected: api.ComponentExportKindValue,
		},
		{
			name:     "ExportKindType",
			kind:     ExportKindType,
			expected: api.ComponentExportKindType,
		},
		{
			name:     "ExportKindComponent",
			kind:     ExportKindComponent,
			expected: api.ComponentExportKindInstance, // Component maps to Instance
		},
		{
			name:     "ExportKindInstance",
			kind:     ExportKindInstance,
			expected: api.ComponentExportKindInstance,
		},
		{
			name:     "ExportKindTable",
			kind:     ExportKindTable,
			expected: api.ComponentExportKindTable,
		},
		{
			name:     "ExportKindMemory",
			kind:     ExportKindMemory,
			expected: api.ComponentExportKindMemory,
		},
		{
			name:     "ExportKindGlobal",
			kind:     ExportKindGlobal,
			expected: api.ComponentExportKindGlobal,
		},
		{
			name:     "unknown kind defaults to Func",
			kind:     ExportKind(255),
			expected: api.ComponentExportKindFunc,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertExportKind(tt.kind)
			if result != tt.expected {
				t.Errorf("convertExportKind(%d) = %d, want %d", tt.kind, result, tt.expected)
			}
		})
	}
}

func TestConvertImportKind(t *testing.T) {
	tests := []struct {
		name     string
		desc     ImportExternDesc
		expected api.ComponentExportKind
	}{
		{
			name:     "ImportExternDescFunc",
			desc:     ImportExternDesc{Kind: ImportExternDescFunc},
			expected: api.ComponentExportKindFunc,
		},
		{
			name:     "ImportExternDescValue",
			desc:     ImportExternDesc{Kind: ImportExternDescValue},
			expected: api.ComponentExportKindValue,
		},
		{
			name:     "ImportExternDescType",
			desc:     ImportExternDesc{Kind: ImportExternDescType},
			expected: api.ComponentExportKindType,
		},
		{
			name:     "ImportExternDescInstance",
			desc:     ImportExternDesc{Kind: ImportExternDescInstance},
			expected: api.ComponentExportKindInstance,
		},
		{
			name:     "ImportExternDescComponent",
			desc:     ImportExternDesc{Kind: ImportExternDescComponent},
			expected: api.ComponentExportKindInstance, // Component maps to Instance
		},
		{
			name:     "ImportExternDescCoreModule",
			desc:     ImportExternDesc{Kind: ImportExternDescCoreModule},
			expected: api.ComponentExportKindInstance, // CoreModule maps to Instance
		},
		{
			name:     "unknown kind defaults to Func",
			desc:     ImportExternDesc{Kind: ImportExternDescKind(255)},
			expected: api.ComponentExportKindFunc,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertImportKind(tt.desc)
			if result != tt.expected {
				t.Errorf("convertImportKind(%d) = %d, want %d", tt.desc.Kind, result, tt.expected)
			}
		})
	}
}

// buildTypesWithFunc creates a ComponentTypes with a single function type:
// (a: u32, b: s32) -> (bool)
// Returns the ComponentTypes and the FuncTypeIdx.
func buildTypesWithFunc(t *testing.T) (*types.ComponentTypes, types.FuncTypeIdx) {
	t.Helper()
	b := types.NewComponentTypesBuilder()
	paramsTup := b.InternTuple([]types.ValType{types.U32, types.S32})
	resultsTup := b.InternTuple([]types.ValType{types.Bool})
	funcIdx := b.InternFunc(false, []string{"a", "b"}, paramsTup, resultsTup)
	ct := b.Finish()
	return ct, funcIdx
}

func TestImportFuncTypeResolution(t *testing.T) {
	ct, funcIdx := buildTypesWithFunc(t)
	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		Imports: []Import{
			{
				Name: "my:pkg/iface",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0, // points to TypeDefs[0]
				},
			},
		},
	}
	cc := NewCompiledComponent(c, nil, nil)
	imports := cc.Imports()

	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	imp := imports[0]
	if imp.Kind != api.ComponentExportKindFunc {
		t.Fatalf("expected func kind, got %d", imp.Kind)
	}
	if imp.FuncType == nil {
		t.Fatal("expected FuncType to be non-nil for function import")
	}
	if imp.FuncType.NumParams() != 2 {
		t.Errorf("NumParams() = %d, want 2", imp.FuncType.NumParams())
	}
	if imp.FuncType.NumResults() != 1 {
		t.Errorf("NumResults() = %d, want 1", imp.FuncType.NumResults())
	}
	params := imp.FuncType.Params()
	if params[0].Name != "a" {
		t.Errorf("Params()[0].Name = %q, want %q", params[0].Name, "a")
	}
	if params[1].Name != "b" {
		t.Errorf("Params()[1].Name = %q, want %q", params[1].Name, "b")
	}
	results := imp.FuncType.Results()
	if results[0].Kind != types.TypeKindBool {
		t.Errorf("Results()[0].Kind = %v, want TypeKindBool", results[0].Kind)
	}
}

func TestImportFuncTypeNilForNonFunc(t *testing.T) {
	c := &Component{
		Imports: []Import{
			{
				Name: "wasi:io/streams@0.2.0",
				ExternDesc: ImportExternDesc{
					Kind: ImportExternDescInstance,
				},
			},
		},
	}
	cc := NewCompiledComponent(c, nil, nil)
	imports := cc.Imports()

	if imports[0].FuncType != nil {
		t.Error("expected FuncType to be nil for instance import")
	}
}

func TestExportFuncTypeViaCanonicalLift(t *testing.T) {
	ct, funcIdx := buildTypesWithFunc(t)
	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		Canonicals: []CanonicalDef{
			{
				Kind:             CanonKindLift,
				ComponentFuncIdx: 0,
				TypeIdx:          0, // points to TypeDefs[0]
			},
		},
		FuncIdxToCanonical: map[uint32]uint32{
			0: 0, // func index 0 -> canonical index 0
		},
		Exports: []Export{
			{
				Name: "add",
				Kind: ExportKindFunc,
				Idx:  0, // component function index 0
			},
		},
	}
	cc := NewCompiledComponent(c, nil, nil)
	exports := cc.Exports()

	if len(exports) != 1 {
		t.Fatalf("expected 1 export, got %d", len(exports))
	}
	exp := exports[0]
	if exp.Kind != api.ComponentExportKindFunc {
		t.Fatalf("expected func kind, got %d", exp.Kind)
	}
	if exp.FuncType == nil {
		t.Fatal("expected FuncType to be non-nil for function export")
	}
	if exp.FuncType.NumParams() != 2 {
		t.Errorf("NumParams() = %d, want 2", exp.FuncType.NumParams())
	}
	if exp.FuncType.NumResults() != 1 {
		t.Errorf("NumResults() = %d, want 1", exp.FuncType.NumResults())
	}
}

func TestExportFuncTypeViaTypeIdx(t *testing.T) {
	ct, funcIdx := buildTypesWithFunc(t)
	typeIdx := uint32(0)
	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: funcIdx},
		},
		Exports: []Export{
			{
				Name:    "add",
				Kind:    ExportKindFunc,
				Idx:     0,
				TypeIdx: &typeIdx,
			},
		},
	}
	cc := NewCompiledComponent(c, nil, nil)
	exports := cc.Exports()

	if exports[0].FuncType == nil {
		t.Fatal("expected FuncType to be non-nil when export has TypeIdx")
	}
	if exports[0].FuncType.NumParams() != 2 {
		t.Errorf("NumParams() = %d, want 2", exports[0].FuncType.NumParams())
	}
}

func TestExportFuncTypeNilForNonFunc(t *testing.T) {
	c := &Component{
		Exports: []Export{
			{
				Name: "my-instance",
				Kind: ExportKindInstance,
			},
		},
	}
	cc := NewCompiledComponent(c, nil, nil)
	exports := cc.Exports()

	if exports[0].FuncType != nil {
		t.Error("expected FuncType to be nil for instance export")
	}
}

func TestImportFuncTypeThroughAlias(t *testing.T) {
	ct, funcIdx := buildTypesWithFunc(t)
	c := &Component{
		Types: ct,
		TypeDefs: []TypeDef{
			// typeidx 0: concrete func type
			{Kind: TypeDefKindFunc, Func: funcIdx},
			// typeidx 1: alias -> typeidx 0
			{
				Kind: TypeDefKindAlias,
				Alias: &AliasTarget{
					IsExport:   false,
					OuterCount: 0,
					OuterIndex: 0,
				},
			},
		},
		Imports: []Import{
			{
				Name: "aliased-func",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 1, // points to alias -> func type
				},
			},
		},
	}
	cc := NewCompiledComponent(c, nil, nil)
	imports := cc.Imports()

	if imports[0].FuncType == nil {
		t.Fatal("expected FuncType to be non-nil after resolving alias")
	}
	if imports[0].FuncType.NumParams() != 2 {
		t.Errorf("NumParams() = %d, want 2", imports[0].FuncType.NumParams())
	}
}
