package spectest

import (
	"testing"
)

func TestParseWastFile(t *testing.T) {
	// Parse a simple .wast test file
	commands, err := ParseWastFile("testdata/simple.wast")
	if err != nil {
		t.Fatalf("ParseWastFile: %v", err)
	}
	if len(commands) == 0 {
		t.Error("Expected at least one command")
	}

	// Verify we have the expected commands
	if len(commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(commands))
	}

	// First command should be a module
	if commands[0].Type != "module" {
		t.Errorf("Expected first command type to be 'module', got %q", commands[0].Type)
	}
	if commands[0].Line != 2 {
		t.Errorf("Expected first command line to be 2, got %d", commands[0].Line)
	}

	// Second command should be assert_invalid
	if commands[1].Type != "assert_invalid" {
		t.Errorf("Expected second command type to be 'assert_invalid', got %q", commands[1].Type)
	}
	if commands[1].Text != "unknown func" {
		t.Errorf("Expected second command text to be 'unknown func', got %q", commands[1].Text)
	}
}
