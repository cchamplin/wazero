package testutil

import (
	"fmt"
	"os"
	"os/exec"
)

// BuildComponentFromWAT compiles WAT text to WASM binary using wasm-tools
func BuildComponentFromWAT(wat string) ([]byte, error) {
	// Write WAT to temp file
	watFile, err := os.CreateTemp("", "component-*.wat")
	if err != nil {
		return nil, err
	}
	defer os.Remove(watFile.Name())

	if _, err := watFile.WriteString(wat); err != nil {
		watFile.Close()
		return nil, err
	}
	watFile.Close()

	// Create output file
	wasmFile, err := os.CreateTemp("", "component-*.wasm")
	if err != nil {
		return nil, err
	}
	wasmFile.Close()
	defer os.Remove(wasmFile.Name())

	// Run wasm-tools parse
	cmd := exec.Command("wasm-tools", "parse", watFile.Name(), "-o", wasmFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("wasm-tools parse: %v: %s", err, output)
	}

	return os.ReadFile(wasmFile.Name())
}

// MustBuildComponentFromWAT is like BuildComponentFromWAT but panics on error
func MustBuildComponentFromWAT(wat string) []byte {
	data, err := BuildComponentFromWAT(wat)
	if err != nil {
		panic(err)
	}
	return data
}
