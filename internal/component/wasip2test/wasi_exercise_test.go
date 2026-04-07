package wasip2test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
)

// wasiExerciseExports lists the test functions exported by the wasi-exercise
// components. Each function exercises a different WASI P2 host implementation
// and returns "ok" on success or a descriptive error string.
var wasiExerciseExports = []struct {
	name string
	area string
}{
	{"test-fs-set-size", "filesystem"},
	{"test-fs-metadata-hash", "filesystem"},
	{"test-fs-is-same-object", "filesystem"},
}

// runWasiExercise loads the named .wasm component file from testdata/, sets
// up a WASI P2 environment with a temporary preopened directory, instantiates
// the component, and calls each of its test exports. Each export must return
// "ok" — any other string is reported as a test failure.
//
// If the .wasm file does not exist (because the build script has not been
// run, or the build tools are unavailable), the test is skipped rather than
// failed. This keeps CI green when only the host code has been modified.
func runWasiExercise(t *testing.T, wasmFile string) {
	t.Helper()
	ctx := context.Background()

	wasmPath := filepath.Join("testdata", wasmFile)
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Skipf("wasm file %s not found (run build_wasi_exercise.sh first): %v", wasmPath, err)
		return
	}

	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Compile the component
	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	defer compiled.Close(ctx)

	// Set up WASI P2 in a separate linker, then merge into the component linker.
	wasiLinker := component.NewLinker()
	if err := wasip2.Instantiate(wasiLinker); err != nil {
		t.Fatalf("wasip2.Instantiate: %v", err)
	}

	linker := component.NewComponentLinker(rt)
	linker.SetRelaxedSemverMatching(true)
	linker.MergeFrom(wasiLinker)

	// Create a per-test temp directory and expose it to the guest as the
	// preopened root. The Rust and Go components use get-directories()[0]
	// as the working directory for their file operations.
	tmpDir := t.TempDir()
	wasiConfig := wasip2.NewConfig().
		WithPreopen("/", tmpDir).
		WithArgs([]string{"wasi-exercise"}).
		WithEnviron([]string{})

	resourceTable := runtime.NewResourceTable()
	testCtx := wasip2.WithConfig(ctx, wasiConfig)
	testCtx = component.WithResourceTable(testCtx, resourceTable)

	instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	for _, tc := range wasiExerciseExports {
		t.Run(tc.name, func(t *testing.T) {
			fn := instance.ExportedFunction(tc.name)
			if fn == nil {
				t.Skipf("function %s not exported by component", tc.name)
				return
			}

			result, err := fn.Call(testCtx)
			if err != nil {
				t.Fatalf("[%s] %s call: %v", tc.area, tc.name, err)
			}
			if len(result) != 1 {
				t.Fatalf("[%s] %s: expected 1 result, got %d", tc.area, tc.name, len(result))
			}

			resultStr := result[0].StringVal()
			if resultStr != "ok" {
				t.Fatalf("[%s] %s returned: %q", tc.area, tc.name, resultStr)
			}
		})
	}
}

// TestWasiExercise_Rust loads the Rust-built wasi-exercise component and
// runs each of its filesystem test functions through the wazero WASI P2
// host implementation.
func TestWasiExercise_Rust(t *testing.T) {
	runWasiExercise(t, "wasi-exercise-rust.wasm")
}

// TestWasiExercise_Go loads the Go-built wasi-exercise component and runs
// each of its filesystem test functions through the wazero WASI P2 host
// implementation.
func TestWasiExercise_Go(t *testing.T) {
	runWasiExercise(t, "wasi-exercise-go.wasm")
}
