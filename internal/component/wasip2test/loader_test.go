package wasip2test

import (
	"testing"
)

func TestLoadWasiTestComponent(t *testing.T) {
	// This will be used to load pre-compiled test components
	// For now, verify the loader interface exists
	loader := NewWasiTestLoader("plugins")

	components, err := loader.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Should find at least the existing calculator plugins
	if len(components) == 0 {
		t.Error("Expected to find test components")
	}

	// Verify we found the expected plugins
	expectedPlugins := []string{"add.wasm", "subtract.wasm", "multi.wasm", "div.wasm"}
	if len(components) < len(expectedPlugins) {
		t.Errorf("Expected at least %d components, got %d", len(expectedPlugins), len(components))
	}
}

func TestWasiTestLoaderLoad(t *testing.T) {
	loader := NewWasiTestLoader("plugins")

	// Test loading with explicit .wasm extension
	data, err := loader.Load("add.wasm")
	if err != nil {
		t.Fatalf("Load(add.wasm): %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty wasm data")
	}

	// Test loading without .wasm extension (should auto-append)
	data, err = loader.Load("subtract")
	if err != nil {
		t.Fatalf("Load(subtract): %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty wasm data")
	}

	// Test loading non-existent file
	_, err = loader.Load("nonexistent")
	if err == nil {
		t.Error("Expected error loading non-existent file")
	}
}
