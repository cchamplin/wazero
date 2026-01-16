// internal/component/compiled_test.go
package component

import (
	"context"
	"testing"
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
