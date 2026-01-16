// internal/component/compiled_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
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
