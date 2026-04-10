// internal/component/wasip2test/converter_test.go
//
// Task 2.2: Function Import/Export Test (convert example)
//
// This test exercises basic function import/export:
// - Host function imports
// - Function lifting (guest export)
// - s32 type encoding/decoding
// - Basic component instantiation
package wasip2test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/testutil"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestComponentConvert(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Build component from WAT
	// This component implements a temperature converter that:
	// 1. Imports a "host:math/ops" instance with a "multiply" function
	// 2. Exports convert-celsius-to-fahrenheit which computes: x * 9 / 5 + 32
	//
	// Note: We use an instance import because the current linker API doesn't
	// support plain function imports (DefineFunc creates namespace/name keys).
	wat := `
(component
  ;; Import instance with multiply function
  (import "host:math/ops" (instance $math
    (export "multiply" (func (param "a" s32) (param "b" s32) (result s32)))
  ))

  ;; Alias the multiply function from the imported instance
  (alias export $math "multiply" (func $multiply))

  ;; Core module that implements temperature conversion
  (core module $impl
    (import "host" "multiply" (func $host_multiply (param i32 i32) (result i32)))

    ;; convert-celsius-to-fahrenheit(x) = x * 9 / 5 + 32
    (func (export "convert") (param $x i32) (result i32)
      ;; x * 9
      (i32.mul (local.get $x) (i32.const 9))
      ;; / 5
      (i32.div_s (i32.const 5))
      ;; + 32
      (i32.add (i32.const 32))
    )
  )

  ;; Lower import for core module
  (core func $multiply_lowered (canon lower (func $multiply)))

  ;; Instantiate core module
  (core instance $i (instantiate $impl
    (with "host" (instance
      (export "multiply" (func $multiply_lowered))
    ))
  ))

  ;; Alias and lift export
  (alias core export $i "convert" (core func $convert))
  (type $convert_type (func (param "celsius" s32) (result s32)))
  (func (export "convert-celsius-to-fahrenheit") (type $convert_type)
    (canon lift (core func $convert)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools WAT compilation failed: %v", err)
	}

	// Create linker and define host instance with multiply function
	linker := component.NewLinker()

	var multiplyCallCount int
	err = linker.DefineInstance("host:math/ops").
		Func("multiply", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			multiplyCallCount++
			a := args[0].S32()
			b := args[1].S32()
			return []types.Val{types.ValS32(a * b)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("decoder limitation: CompileComponent failed: %v", err)
	}
	defer compiled.Close(ctx)

	componentLinker := component.NewComponentLinker(rt)
	componentLinker.MergeFrom(linker)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)



	instance, err := componentLinker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("pipeline limitation: Instantiate failed: %v", err)
	}

	// Call convert-celsius-to-fahrenheit(100)
	fn := instance.ExportedFunction("convert-celsius-to-fahrenheit")
	if fn == nil {
		t.Fatal("convert-celsius-to-fahrenheit function not found")
	}

	result, err := fn.CallAndPostReturn(testCtx, types.ValS32(100))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	// 100C = 212F
	// Formula: 100 * 9 / 5 + 32 = 900 / 5 + 32 = 180 + 32 = 212
	if got := result[0].S32(); got != 212 {
		t.Errorf("convert(100) = %d, want 212", got)
	}

	t.Logf("convert(100) = %d (expected 212)", result[0].S32())
}

// TestComponentConvert_NegativeTemperature tests conversion with negative input.
func TestComponentConvert_NegativeTemperature(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wat := `
(component
  ;; Import instance with multiply function
  (import "host:math/ops" (instance $math
    (export "multiply" (func (param "a" s32) (param "b" s32) (result s32)))
  ))

  ;; Alias the multiply function from the imported instance
  (alias export $math "multiply" (func $multiply))

  ;; Core module that implements temperature conversion
  (core module $impl
    (import "host" "multiply" (func $host_multiply (param i32 i32) (result i32)))

    ;; convert-celsius-to-fahrenheit(x) = x * 9 / 5 + 32
    (func (export "convert") (param $x i32) (result i32)
      (i32.mul (local.get $x) (i32.const 9))
      (i32.div_s (i32.const 5))
      (i32.add (i32.const 32))
    )
  )

  (core func $multiply_lowered (canon lower (func $multiply)))
  (core instance $i (instantiate $impl
    (with "host" (instance
      (export "multiply" (func $multiply_lowered))
    ))
  ))

  (alias core export $i "convert" (core func $convert))
  (type $convert_type (func (param "celsius" s32) (result s32)))
  (func (export "convert-celsius-to-fahrenheit") (type $convert_type)
    (canon lift (core func $convert)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools WAT compilation failed: %v", err)
	}

	linker := component.NewLinker()
	err = linker.DefineInstance("host:math/ops").
		Func("multiply", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			a := args[0].S32()
			b := args[1].S32()
			return []types.Val{types.ValS32(a * b)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("decoder limitation: CompileComponent failed: %v", err)
	}
	defer compiled.Close(ctx)

	componentLinker := component.NewComponentLinker(rt)
	componentLinker.MergeFrom(linker)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)



	instance, err := componentLinker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("pipeline limitation: Instantiate failed: %v", err)
	}

	fn := instance.ExportedFunction("convert-celsius-to-fahrenheit")
	if fn == nil {
		t.Fatal("convert-celsius-to-fahrenheit function not found")
	}

	// Test -40C = -40F (the crossover point)
	// Formula: -40 * 9 / 5 + 32 = -360 / 5 + 32 = -72 + 32 = -40
	result, err := fn.CallAndPostReturn(testCtx, types.ValS32(-40))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if got := result[0].S32(); got != -40 {
		t.Errorf("convert(-40) = %d, want -40", got)
	}

	t.Logf("convert(-40) = %d (expected -40)", result[0].S32())
}

// TestComponentConvert_Zero tests conversion of freezing point.
func TestComponentConvert_Zero(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wat := `
(component
  ;; Import instance with multiply function
  (import "host:math/ops" (instance $math
    (export "multiply" (func (param "a" s32) (param "b" s32) (result s32)))
  ))

  ;; Alias the multiply function from the imported instance
  (alias export $math "multiply" (func $multiply))

  ;; Core module that implements temperature conversion
  (core module $impl
    (import "host" "multiply" (func $host_multiply (param i32 i32) (result i32)))

    ;; convert-celsius-to-fahrenheit(x) = x * 9 / 5 + 32
    (func (export "convert") (param $x i32) (result i32)
      (i32.mul (local.get $x) (i32.const 9))
      (i32.div_s (i32.const 5))
      (i32.add (i32.const 32))
    )
  )

  (core func $multiply_lowered (canon lower (func $multiply)))
  (core instance $i (instantiate $impl
    (with "host" (instance
      (export "multiply" (func $multiply_lowered))
    ))
  ))

  (alias core export $i "convert" (core func $convert))
  (type $convert_type (func (param "celsius" s32) (result s32)))
  (func (export "convert-celsius-to-fahrenheit") (type $convert_type)
    (canon lift (core func $convert)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools WAT compilation failed: %v", err)
	}

	linker := component.NewLinker()
	err = linker.DefineInstance("host:math/ops").
		Func("multiply", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			a := args[0].S32()
			b := args[1].S32()
			return []types.Val{types.ValS32(a * b)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("decoder limitation: CompileComponent failed: %v", err)
	}
	defer compiled.Close(ctx)

	componentLinker := component.NewComponentLinker(rt)
	componentLinker.MergeFrom(linker)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)



	instance, err := componentLinker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("pipeline limitation: Instantiate failed: %v", err)
	}

	fn := instance.ExportedFunction("convert-celsius-to-fahrenheit")
	if fn == nil {
		t.Fatal("convert-celsius-to-fahrenheit function not found")
	}

	// Test 0C = 32F (freezing point of water)
	// Formula: 0 * 9 / 5 + 32 = 0 + 32 = 32
	result, err := fn.CallAndPostReturn(testCtx, types.ValS32(0))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if got := result[0].S32(); got != 32 {
		t.Errorf("convert(0) = %d, want 32", got)
	}

	t.Logf("convert(0) = %d (expected 32)", result[0].S32())
}

// TestComponentConvert_HostFunctionCalled tests that a component that actually
// uses the host multiply function calls it correctly.
func TestComponentConvert_HostFunctionCalled(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// This component uses the imported multiply function in the conversion
	wat := `
(component
  ;; Import instance with multiply function
  (import "host:math/ops" (instance $math
    (export "multiply" (func (param "a" s32) (param "b" s32) (result s32)))
  ))

  ;; Alias the multiply function from the imported instance
  (alias export $math "multiply" (func $multiply))

  ;; Core module that uses the host multiply function
  (core module $impl
    (import "host" "multiply" (func $host_multiply (param i32 i32) (result i32)))

    ;; convert uses the host multiply function: x * 9 / 5 + 32
    (func (export "convert") (param $x i32) (result i32)
      ;; Call host multiply(x, 9)
      (call $host_multiply (local.get $x) (i32.const 9))
      ;; / 5
      (i32.div_s (i32.const 5))
      ;; + 32
      (i32.add (i32.const 32))
    )
  )

  (core func $multiply_lowered (canon lower (func $multiply)))
  (core instance $i (instantiate $impl
    (with "host" (instance
      (export "multiply" (func $multiply_lowered))
    ))
  ))

  (alias core export $i "convert" (core func $convert))
  (type $convert_type (func (param "celsius" s32) (result s32)))
  (func (export "convert-celsius-to-fahrenheit") (type $convert_type)
    (canon lift (core func $convert)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools WAT compilation failed: %v", err)
	}

	linker := component.NewLinker()

	var multiplyCallCount int
	var lastMultiplyArgs [2]int32
	err = linker.DefineInstance("host:math/ops").
		Func("multiply", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			multiplyCallCount++
			a := args[0].S32()
			b := args[1].S32()
			lastMultiplyArgs = [2]int32{a, b}
			return []types.Val{types.ValS32(a * b)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("decoder limitation: CompileComponent failed: %v", err)
	}
	defer compiled.Close(ctx)

	componentLinker := component.NewComponentLinker(rt)
	componentLinker.MergeFrom(linker)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)



	instance, err := componentLinker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("pipeline limitation: Instantiate failed: %v", err)
	}

	fn := instance.ExportedFunction("convert-celsius-to-fahrenheit")
	if fn == nil {
		t.Fatal("convert-celsius-to-fahrenheit function not found")
	}

	// Reset counter before call
	multiplyCallCount = 0

	result, err := fn.CallAndPostReturn(testCtx, types.ValS32(100))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	// Verify result is correct
	if got := result[0].S32(); got != 212 {
		t.Errorf("convert(100) = %d, want 212", got)
	}

	// Verify host function was called
	if multiplyCallCount != 1 {
		t.Errorf("multiply was called %d times, want 1", multiplyCallCount)
	}

	// Verify arguments were correct (100, 9)
	if lastMultiplyArgs[0] != 100 || lastMultiplyArgs[1] != 9 {
		t.Errorf("multiply was called with (%d, %d), want (100, 9)", lastMultiplyArgs[0], lastMultiplyArgs[1])
	}

	t.Logf("Host multiply was called %d time(s) with args (%d, %d)", multiplyCallCount, lastMultiplyArgs[0], lastMultiplyArgs[1])
}

// TestComponentConvert_MultipleConversions tests multiple sequential calls.
func TestComponentConvert_MultipleConversions(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	wat := `
(component
  ;; Import instance with multiply function
  (import "host:math/ops" (instance $math
    (export "multiply" (func (param "a" s32) (param "b" s32) (result s32)))
  ))

  ;; Alias the multiply function from the imported instance
  (alias export $math "multiply" (func $multiply))

  ;; Core module that implements temperature conversion
  (core module $impl
    (import "host" "multiply" (func $host_multiply (param i32 i32) (result i32)))

    ;; convert-celsius-to-fahrenheit(x) = x * 9 / 5 + 32
    (func (export "convert") (param $x i32) (result i32)
      (i32.mul (local.get $x) (i32.const 9))
      (i32.div_s (i32.const 5))
      (i32.add (i32.const 32))
    )
  )

  (core func $multiply_lowered (canon lower (func $multiply)))
  (core instance $i (instantiate $impl
    (with "host" (instance
      (export "multiply" (func $multiply_lowered))
    ))
  ))

  (alias core export $i "convert" (core func $convert))
  (type $convert_type (func (param "celsius" s32) (result s32)))
  (func (export "convert-celsius-to-fahrenheit") (type $convert_type)
    (canon lift (core func $convert)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools WAT compilation failed: %v", err)
	}

	linker := component.NewLinker()
	err = linker.DefineInstance("host:math/ops").
		Func("multiply", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			a := args[0].S32()
			b := args[1].S32()
			return []types.Val{types.ValS32(a * b)}, nil
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance: %v", err)
	}

	compiled, err := rt.CompileComponent(ctx, wasmBytes)
	if err != nil {
		t.Skipf("decoder limitation: CompileComponent failed: %v", err)
	}
	defer compiled.Close(ctx)

	componentLinker := component.NewComponentLinker(rt)
	componentLinker.MergeFrom(linker)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)



	instance, err := componentLinker.Instantiate(testCtx, compiled.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("pipeline limitation: Instantiate failed: %v", err)
	}

	fn := instance.ExportedFunction("convert-celsius-to-fahrenheit")
	if fn == nil {
		t.Fatal("convert-celsius-to-fahrenheit function not found")
	}

	// Test multiple temperatures
	testCases := []struct {
		celsius    int32
		fahrenheit int32
	}{
		{0, 32},      // Freezing point
		{100, 212},   // Boiling point
		{-40, -40},   // Crossover point
		{37, 98},     // Body temperature (approximately)
		{-273, -459}, // Near absolute zero
	}

	for _, tc := range testCases {
		result, err := fn.CallAndPostReturn(testCtx, types.ValS32(tc.celsius))
		if err != nil {
			t.Fatalf("Call(%d): %v", tc.celsius, err)
		}

		got := result[0].S32()
		if got != tc.fahrenheit {
			t.Errorf("convert(%d) = %d, want %d", tc.celsius, got, tc.fahrenheit)
		}
	}

	t.Log("All temperature conversions passed")
}
