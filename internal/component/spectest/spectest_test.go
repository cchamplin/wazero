package spectest

import (
	"testing"
)

func TestParseWastFile(t *testing.T) {
	// Parse the upstream simple.wast test file
	commands, err := ParseWastFile("testdata/wasmtime/simple.wast")
	if err != nil {
		t.Fatalf("ParseWastFile: %v", err)
	}
	if len(commands) == 0 {
		t.Error("Expected at least one command")
	}

	// The upstream simple.wast has 8 commands:
	// 4 module definitions, 2 assert_invalid, 1 module, 1 assert_return
	if len(commands) != 8 {
		t.Errorf("Expected 8 commands, got %d", len(commands))
	}

	// First command should be a module
	if commands[0].Type != "module" {
		t.Errorf("Expected first command type to be 'module', got %q", commands[0].Type)
	}
	if commands[0].Line != 1 {
		t.Errorf("Expected first command line to be 1, got %d", commands[0].Line)
	}

	// Verify we have the expected command types
	expectedTypes := []string{"module", "module", "module", "module", "assert_invalid", "assert_invalid", "module", "assert_return"}
	for i, cmd := range commands {
		if i < len(expectedTypes) && cmd.Type != expectedTypes[i] {
			t.Errorf("Expected command[%d] type to be %q, got %q", i, expectedTypes[i], cmd.Type)
		}
	}
}
