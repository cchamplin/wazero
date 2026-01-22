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

// WastTestSuite represents a parsed .wast file with its commands and wasm binaries
type WastTestSuite struct {
	Commands []Command
	WasmDir  string // Directory containing generated wasm binaries
	cleanup  func() // Cleanup function to remove temp directory
}

// Close removes the temporary directory containing wasm binaries
func (s *WastTestSuite) Close() error {
	if s.cleanup != nil {
		s.cleanup()
	}
	return nil
}

// GetWasmBytes reads the wasm binary for a command by filename
func (s *WastTestSuite) GetWasmBytes(filename string) ([]byte, error) {
	if filename == "" {
		return nil, fmt.Errorf("no filename specified for command")
	}
	return os.ReadFile(filepath.Join(s.WasmDir, filename))
}

// ParseWastFileWithBinaries parses a .wast file and keeps the wasm binaries available
// Caller must call Close() on the returned WastTestSuite to clean up temp files
func ParseWastFileWithBinaries(path string) (*WastTestSuite, error) {
	// Create temp dir for wasm-tools output
	tempDir, err := os.MkdirTemp("", "wast-*")
	if err != nil {
		return nil, err
	}

	// Run wasm-tools json-from-wast
	jsonPath := filepath.Join(tempDir, "output.json")
	cmd := exec.Command("wasm-tools", "json-from-wast", path, "-o", jsonPath, "--wasm-dir", tempDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("wasm-tools json-from-wast %s: %v: %s", path, err, output)
	}

	// Parse JSON output
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	var result wastJSON
	if err := json.Unmarshal(data, &result); err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	return &WastTestSuite{
		Commands: result.Commands,
		WasmDir:  tempDir,
		cleanup:  func() { os.RemoveAll(tempDir) },
	}, nil
}
