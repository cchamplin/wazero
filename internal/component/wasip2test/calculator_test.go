package wasip2test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
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
		file                  string
		expected              int32
		relaxedSemverMatching bool
	}{
		{"add", "plugins/add.wasm", 31, true},            // 28 + 3 (Rust, requires WASI, uses relaxed semver)
		{"subtract", "plugins/subtract.wasm", 25, false}, // 28 - 3 (C, no WASI)
		{"multi", "plugins/multi.wasm", 25, true},        // 28 * 3 (Go, no WASI)
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

			// Instantiate the component
			instance, err := linker.Instantiate(ctx, compiled.(*component.CompiledComponent))
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}

			// Test get-plugin-name
			nameFunc := instance.ExportedFunction("get-plugin-name")
			if nameFunc == nil {
				t.Fatal("get-plugin-name function not found")
			}
			nameResult, err := nameFunc.Call(ctx)
			if err != nil {
				t.Fatalf("get-plugin-name: %v", err)
			}
			if got := nameResult[0].StringVal(); got != p.name {
				t.Errorf("name = %q, want %q", got, p.name)
			}

			// Test evaluate(2, 3)
			evalFunc := instance.ExportedFunction("evaluate")
			if evalFunc == nil {
				t.Fatal("evaluate function not found")
			}
			start := time.Now()
			evalResult, err := evalFunc.Call(ctx,
				component.ValS32(28),
				component.ValS32(3),
			)
			t.Logf("First call took %v", time.Since(start))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if got := evalResult[0].S32(); got != p.expected {
				t.Errorf("evaluate(2,3) = %d, want %d", got, p.expected)
			}

			// Call again
			start = time.Now()
			evalResult, err = evalFunc.Call(ctx,
				component.ValS32(28+1),
				component.ValS32(3),
			)
			t.Logf("Second call took %v", time.Since(start))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if got := evalResult[0].S32(); got != p.expected+1 {
				t.Errorf("evaluate(2,3) = %d, want %d", got, p.expected)
			}

		})

	}
}
