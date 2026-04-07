package wasip2test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestCalculatorPlugins(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Create a basic linker and register WASI P2 interfaces
	wasiLinker := component.NewLinker()
	if err := wasip2.Instantiate(wasiLinker); err != nil {
		t.Fatalf("wasip2.Instantiate: %v", err)
	}

	plugins := []struct {
		name                  string
		pluginName            string // expected return value from get-plugin-name
		file                  string
		expected              int32
		relaxedSemverMatching bool
	}{
		{"add", "add", "plugins/add.wasm", 31, true},                 // 28 + 3 (Rust, requires WASI, uses relaxed semver)
		{"subtract", "subtract", "plugins/subtract.wasm", 25, false}, // 28 - 3 (C, no WASI)
		// multi.wasm (Go)
		{"multi", "Simple-Go-Multi", "plugins/multi.wasm", 84, true}, // 28 * 3 (Go, uses relaxed semver, uses wit-bidgen and wasm-tools adapter)
		{"div", "Simple-Go-Div", "plugins/div.wasm", 9, true},        // 28 / 3 (Go, uses relaxed semver, uses wit-bindgen-go and tinygo wasip2 support)
	}

	for _, p := range plugins {
		t.Run(p.name, func(t *testing.T) {

			// Load component binary
			wasmBytes, err := os.ReadFile(filepath.Join(".", p.file))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			// Compile component
			compiled, err := rt.CompileComponent(ctx, wasmBytes)
			if err != nil {
				t.Fatalf("CompileComponent: %v", err)
			}
			defer compiled.Close(ctx)

			// Create component linker with runtime integration
			linker := component.NewComponentLinker(rt)

			// Enable relaxed semver matching if needed for this plugin
			if p.relaxedSemverMatching {
				linker.SetRelaxedSemverMatching(true)
			}

			// Merge WASI definitions into the component linker
			linker.MergeFrom(wasiLinker)

			// Set up WASI context with config and resource table
			// This is required for plugins that use WASI (especially Go plugins with P1->P2 adapter)
			var stdout, stderr bytes.Buffer
			wasiConfig := wasip2.NewConfig().
				WithStdout(&stdout).
				WithStderr(&stderr).
				WithArgs([]string{"test"}).
				WithEnviron([]string{})
			resourceTable := runtime.NewResourceTable()
			testCtx := wasip2.WithConfig(ctx, wasiConfig)
			testCtx = component.WithResourceTable(testCtx, resourceTable)

			// Instantiate the component
			instance, err := linker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}

			// Test get-plugin-name
			nameFunc := instance.ExportedFunction("get-plugin-name")
			if nameFunc == nil {
				t.Fatal("get-plugin-name function not found")
			}
			nameResult, err := nameFunc.Call(testCtx)
			if err != nil {
				t.Fatalf("get-plugin-name: %v", err)
			}
			if got := nameResult[0].StringVal(); got != p.pluginName {
				t.Errorf("name = %q, want %q", got, p.pluginName)
			}

			// Test evaluate(28, 3)
			evalFunc := instance.ExportedFunction("evaluate")
			if evalFunc == nil {
				t.Fatal("evaluate function not found")
			}
			start := time.Now()
			evalResult, err := evalFunc.Call(testCtx,
				types.ValS32(28),
				types.ValS32(3),
			)
			t.Logf("First call took %v", time.Since(start))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if got := evalResult[0].S32(); got != p.expected {
				t.Errorf("evaluate(28,3) = %d, want %d", got, p.expected)
			}

			// Call again with incremented first argument
			start = time.Now()
			evalResult, err = evalFunc.Call(testCtx,
				types.ValS32(29),
				types.ValS32(3),
			)
			t.Logf("Second call took %v", time.Since(start))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			// For add: 29+3=32, for subtract: 29-3=26, for multi: 29*3=87
			var expectedSecond int32
			switch p.name {
			case "add":
				expectedSecond = 32
			case "subtract":
				expectedSecond = 26
			case "multi":
				expectedSecond = 87
			case "div":
				expectedSecond = 9 // integer division
			}
			if got := evalResult[0].S32(); got != expectedSecond {
				t.Errorf("evaluate(29,3) = %d, want %d", got, expectedSecond)
			}

		})

	}
}
