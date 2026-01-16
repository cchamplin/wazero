// internal/component/index_space_test.go
package component

import (
	"testing"
)

func TestCoreFuncIndexSpace(t *testing.T) {
	space := NewCoreFuncIndexSpace()

	// Add alias: component func index 0 = core instance 0, export "add"
	space.AddAlias(0, 0, "add")

	instIdx, exportName, err := space.Resolve(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instIdx != 0 {
		t.Errorf("expected instance 0, got %d", instIdx)
	}
	if exportName != "add" {
		t.Errorf("expected export 'add', got %q", exportName)
	}
}

func TestCoreFuncIndexSpaceNotFound(t *testing.T) {
	space := NewCoreFuncIndexSpace()

	_, _, err := space.Resolve(99)
	if err == nil {
		t.Error("expected error for missing index")
	}
}

func TestCoreMemoryIndexSpace(t *testing.T) {
	space := NewCoreMemoryIndexSpace()

	space.AddAlias(0, 0, "memory")

	instIdx, exportName, err := space.Resolve(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instIdx != 0 || exportName != "memory" {
		t.Errorf("unexpected: inst=%d, name=%q", instIdx, exportName)
	}
}
