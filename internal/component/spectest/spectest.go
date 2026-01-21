package spectest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Command represents a single test directive from a .wast file
type Command struct {
	Type       string          `json:"type"`
	Line       int             `json:"line"`
	Filename   string          `json:"filename,omitempty"`
	Action     *Action         `json:"action,omitempty"`
	Expected   []Value         `json:"expected,omitempty"`
	Text       string          `json:"text,omitempty"`
	Module     json.RawMessage `json:"module,omitempty"`
	ModuleType string          `json:"module_type,omitempty"`
	Name       string          `json:"name,omitempty"`
	As         string          `json:"as,omitempty"`
}

// Action represents a test action to perform (invoke, get, etc.)
type Action struct {
	Type   string  `json:"type"`             // Action type: "invoke", "get"
	Module string  `json:"module,omitempty"` // Target module name
	Field  string  `json:"field,omitempty"`  // Function or export name
	Args   []Value `json:"args,omitempty"`   // Arguments for invocation
}

// Value represents a typed value used in test assertions and invocations.
type Value struct {
	Type  string `json:"type"`            // Value type (e.g., "i32", "f64")
	Value string `json:"value,omitempty"` // String representation of the value
}

// wastJSON is the top-level structure of wasm-tools json-from-wast output
type wastJSON struct {
	SourceFilename string    `json:"source_filename"`
	Commands       []Command `json:"commands"`
}

// ParseWastFile uses wasm-tools to convert .wast to JSON and parses the result
func ParseWastFile(path string) ([]Command, error) {
	// Create temp dir for wasm-tools output
	tempDir, err := os.MkdirTemp("", "wast-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// Run wasm-tools json-from-wast
	jsonPath := filepath.Join(tempDir, "output.json")
	cmd := exec.Command("wasm-tools", "json-from-wast", path, "-o", jsonPath, "--wasm-dir", tempDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("wasm-tools json-from-wast %s: %v: %s", path, err, output)
	}

	// Parse JSON output
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var result wastJSON
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Commands, nil
}
