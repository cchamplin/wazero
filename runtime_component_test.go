// runtime_component_test.go
package wazero

import (
	"context"
	"testing"
)

func TestRuntimeCompileComponent(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx)
	defer rt.Close(ctx)

	// Minimal component binary
	componentBinary := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x0d, 0x00,             // version
		0x01, 0x00,             // layer (component)
	}

	compiled, err := rt.CompileComponent(ctx, componentBinary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer compiled.Close(ctx)

	if compiled == nil {
		t.Fatal("expected non-nil compiled component")
	}
}

func TestRuntimeCompileComponentNotComponent(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx)
	defer rt.Close(ctx)

	// Core module binary (not a component)
	moduleBinary := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x01, 0x00, 0x00, 0x00, // version (core module)
	}

	_, err := rt.CompileComponent(ctx, moduleBinary)
	if err == nil {
		t.Fatal("expected error for non-component binary")
	}
}

func TestRuntimeNewComponentLinker(t *testing.T) {
	ctx := context.Background()
	rt := NewRuntime(ctx)
	defer rt.Close(ctx)

	linker := rt.NewComponentLinker()
	if linker == nil {
		t.Fatal("expected non-nil linker")
	}
}
