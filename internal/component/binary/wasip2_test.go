// internal/component/binary/wasip2_test.go
package binary

import (
	"os"
	"testing"
)

func TestParseAddWasm(t *testing.T) {
	data, err := os.ReadFile("../wasip2test/plugins/add.wasm")
	if err != nil {
		t.Skip("add.wasm not found")
	}

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("failed to parse add.wasm: %v", err)
	}

	// Verify expected structure - WASI P2 components include adapter modules
	if len(c.CoreModules) < 1 {
		t.Errorf("expected at least 1 core module, got %d", len(c.CoreModules))
	}
	t.Logf("Found %d core modules", len(c.CoreModules))

	// add.wasm imports 10 WASI interfaces
	if len(c.Imports) < 5 {
		t.Errorf("expected at least 5 imports, got %d", len(c.Imports))
	}
	t.Logf("Found %d imports", len(c.Imports))

	// add.wasm has 2 exports (get-plugin-name and evaluate)
	if len(c.Exports) != 2 {
		t.Errorf("expected 2 exports, got %d", len(c.Exports))
	}
	t.Logf("Found %d exports", len(c.Exports))

	// Check some import names to verify parsing
	expectedImports := []string{
		"wasi:cli/environment@0.2.3",
		"wasi:cli/exit@0.2.3",
		"wasi:io/error@0.2.3",
	}
	for _, expected := range expectedImports {
		found := false
		for _, imp := range c.Imports {
			if imp.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected import: %s", expected)
		}
	}

	// Check export names
	foundGetPluginName := false
	foundEvaluate := false
	for _, exp := range c.Exports {
		t.Logf("Export: %s", exp.Name)
		if exp.Name == "get-plugin-name" {
			foundGetPluginName = true
		}
		if exp.Name == "evaluate" {
			foundEvaluate = true
		}
	}
	if !foundGetPluginName {
		t.Error("missing export 'get-plugin-name'")
	}
	if !foundEvaluate {
		t.Error("missing export 'evaluate'")
	}
}

func TestParseSubtractWasm(t *testing.T) {
	data, err := os.ReadFile("../wasip2test/plugins/subtract.wasm")
	if err != nil {
		t.Skip("subtract.wasm not found")
	}

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("failed to parse subtract.wasm: %v", err)
	}

	// subtract.wasm is simpler - verify basic structure
	if len(c.CoreModules) < 1 {
		t.Errorf("expected at least 1 core module, got %d", len(c.CoreModules))
	}

	if len(c.Exports) < 2 {
		t.Errorf("expected at least 2 exports, got %d", len(c.Exports))
	}
}
