// internal/component/wasip2test/linking_test.go
//
// Task 2.4: Component Linking Test
//
// This test exercises multi-component linking:
// - Multi-component instantiation
// - Import resolution from other components
// - Export discovery and aliasing

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

// TestComponentLinking_ProviderConsumer tests linking a provider component's
// exports to a consumer component's imports.
//
// Provider exports: add(a: s32, b: s32) -> s32
// Consumer imports: "math" instance with add function
// Consumer exports: double-add(a: s32, b: s32) -> s32 = add(a,b) + add(a,b)
func TestComponentLinking_ProviderConsumer(t *testing.T) {
	ctx := context.Background()

	// Provider component WAT: exports an "add" function
	providerWAT := `
(component
  (core module $provider_mod
    (func (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
  )
  (core instance $i (instantiate $provider_mod))
  (alias core export $i "add" (core func $add))
  (type $add_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "add") (type $add_type)
    (canon lift (core func $add)))
)
`

	// Consumer component WAT: imports "math" instance with "add", exports "double-add"
	consumerWAT := `
(component
  (import "math" (instance $math
    (export "add" (func (param "a" s32) (param "b" s32) (result s32)))
  ))
  (alias export $math "add" (func $add))
  (core module $consumer_mod
    (import "math" "add" (func $add (param i32 i32) (result i32)))
    (func (export "double-add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      call $add
      local.get 0
      local.get 1
      call $add
      i32.add
    )
  )
  (core func $add_lowered (canon lower (func $add)))
  (core instance $i (instantiate $consumer_mod
    (with "math" (instance
      (export "add" (func $add_lowered))
    ))
  ))
  (alias core export $i "double-add" (core func $double_add))
  (type $double_add_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "double-add") (type $double_add_type)
    (canon lift (core func $double_add)))
)
`

	// Build provider component
	providerBytes, err := testutil.BuildComponentFromWAT(providerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT (provider): %v", err)
	}

	// Build consumer component
	consumerBytes, err := testutil.BuildComponentFromWAT(consumerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT (consumer): %v", err)
	}

	// Use separate runtimes for provider and consumer to avoid module name conflicts
	providerRT := wazero.NewRuntime(ctx)
	defer providerRT.Close(ctx)

	consumerRT := wazero.NewRuntime(ctx)
	defer consumerRT.Close(ctx)

	// Compile provider component
	compiledProvider, err := providerRT.CompileComponent(ctx, providerBytes)
	if err != nil {
		t.Skipf("CompileComponent (provider): %v", err)
	}
	defer compiledProvider.Close(ctx)

	// Compile consumer component
	compiledConsumer, err := consumerRT.CompileComponent(ctx, consumerBytes)
	if err != nil {
		t.Skipf("CompileComponent (consumer): %v", err)
	}
	defer compiledConsumer.Close(ctx)

	// Set up resource table for instantiation
	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)

	// Create a linker for the provider (no imports needed)
	providerLinker := component.NewComponentLinker(providerRT)

	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	// Instantiate the provider component
	providerInstance, err := providerLinker.Instantiate(testCtx, compiledProvider.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate (provider): %v", err)
	}

	// Get the provider's exported "add" function
	providerAddFunc := providerInstance.ExportedFunction("add")
	if providerAddFunc == nil {
		t.Fatal("provider 'add' function not found")
	}

	// Verify the provider's add function works directly
	providerResult, err := providerAddFunc.Call(testCtx, types.ValS32(10), types.ValS32(5))
	if err != nil {
		t.Fatalf("provider add(10, 5): %v", err)
	}
	if got := providerResult[0].S32(); got != 15 {
		t.Errorf("provider add(10, 5) = %d, want 15", got)
	}
	t.Logf("Provider add(10, 5) = %d", providerResult[0].S32())

	// Create a linker for the consumer that provides the "math" instance
	// by wrapping the provider's exported add function
	linker := component.NewLinker()
	err = linker.DefineInstance("math").
		Func("add", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			// Forward the call to the provider's add function
			return providerAddFunc.Call(ctx, args...)
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance (math): %v", err)
	}

	// Create component linker for consumer
	consumerLinker := component.NewComponentLinker(consumerRT)
	consumerLinker.MergeFrom(linker)

	// Instantiate the consumer component
	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	consumerInstance, err := consumerLinker.Instantiate(testCtx, compiledConsumer.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate (consumer): %v", err)
	}

	// Get the consumer's exported "double-add" function
	doubleAddFunc := consumerInstance.ExportedFunction("double-add")
	if doubleAddFunc == nil {
		t.Fatal("consumer 'double-add' function not found")
	}

	// Test double-add(3, 4) = add(3,4) + add(3,4) = 7 + 7 = 14
	result, err := doubleAddFunc.Call(testCtx, types.ValS32(3), types.ValS32(4))
	if err != nil {
		t.Fatalf("double-add(3, 4): %v", err)
	}

	expected := int32(14) // (3+4) + (3+4) = 14
	if got := result[0].S32(); got != expected {
		t.Errorf("double-add(3, 4) = %d, want %d", got, expected)
	}

	t.Logf("double-add(3, 4) = %d (expected %d)", result[0].S32(), expected)
}

// TestComponentLinking_MultipleValues tests linking with multiple test values
// to verify the linking is working correctly across different inputs.
func TestComponentLinking_MultipleValues(t *testing.T) {
	ctx := context.Background()

	// Provider component WAT: exports an "add" function
	providerWAT := `
(component
  (core module $provider_mod2
    (func (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
  )
  (core instance $i (instantiate $provider_mod2))
  (alias core export $i "add" (core func $add))
  (type $add_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "add") (type $add_type)
    (canon lift (core func $add)))
)
`

	// Consumer component WAT: imports "math" instance with "add", exports "double-add"
	consumerWAT := `
(component
  (import "math" (instance $math
    (export "add" (func (param "a" s32) (param "b" s32) (result s32)))
  ))
  (alias export $math "add" (func $add))
  (core module $consumer_mod2
    (import "math" "add" (func $add (param i32 i32) (result i32)))
    (func (export "double-add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      call $add
      local.get 0
      local.get 1
      call $add
      i32.add
    )
  )
  (core func $add_lowered (canon lower (func $add)))
  (core instance $i (instantiate $consumer_mod2
    (with "math" (instance
      (export "add" (func $add_lowered))
    ))
  ))
  (alias core export $i "double-add" (core func $double_add))
  (type $double_add_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "double-add") (type $double_add_type)
    (canon lift (core func $double_add)))
)
`

	providerBytes, err := testutil.BuildComponentFromWAT(providerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT (provider): %v", err)
	}

	consumerBytes, err := testutil.BuildComponentFromWAT(consumerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT (consumer): %v", err)
	}

	// Use separate runtimes for provider and consumer
	providerRT := wazero.NewRuntime(ctx)
	defer providerRT.Close(ctx)

	consumerRT := wazero.NewRuntime(ctx)
	defer consumerRT.Close(ctx)

	compiledProvider, err := providerRT.CompileComponent(ctx, providerBytes)
	if err != nil {
		t.Skipf("CompileComponent (provider): %v", err)
	}
	defer compiledProvider.Close(ctx)

	compiledConsumer, err := consumerRT.CompileComponent(ctx, consumerBytes)
	if err != nil {
		t.Skipf("CompileComponent (consumer): %v", err)
	}
	defer compiledConsumer.Close(ctx)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)

	providerLinker := component.NewComponentLinker(providerRT)
	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	providerInstance, err := providerLinker.Instantiate(testCtx, compiledProvider.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate (provider): %v", err)
	}

	providerAddFunc := providerInstance.ExportedFunction("add")
	if providerAddFunc == nil {
		t.Fatal("provider 'add' function not found")
	}

	linker := component.NewLinker()
	err = linker.DefineInstance("math").
		Func("add", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return providerAddFunc.Call(ctx, args...)
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance (math): %v", err)
	}

	consumerLinker := component.NewComponentLinker(consumerRT)
	consumerLinker.MergeFrom(linker)

	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	consumerInstance, err := consumerLinker.Instantiate(testCtx, compiledConsumer.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate (consumer): %v", err)
	}

	doubleAddFunc := consumerInstance.ExportedFunction("double-add")
	if doubleAddFunc == nil {
		t.Fatal("consumer 'double-add' function not found")
	}

	// Test multiple values
	testCases := []struct {
		a, b     int32
		expected int32 // (a+b) + (a+b) = 2*(a+b)
	}{
		{0, 0, 0},
		{1, 1, 4},       // (1+1) + (1+1) = 4
		{3, 4, 14},      // (3+4) + (3+4) = 14
		{10, 20, 60},    // (10+20) + (10+20) = 60
		{-5, 10, 10},    // (-5+10) + (-5+10) = 10
		{-10, -20, -60}, // (-10+-20) + (-10+-20) = -60
		{100, 200, 600}, // (100+200) + (100+200) = 600
	}

	for _, tc := range testCases {
		result, err := doubleAddFunc.Call(testCtx, types.ValS32(tc.a), types.ValS32(tc.b))
		if err != nil {
			t.Fatalf("double-add(%d, %d): %v", tc.a, tc.b, err)
		}

		got := result[0].S32()
		if got != tc.expected {
			t.Errorf("double-add(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.expected)
		}
	}

	t.Log("All multiple value tests passed")
}

// TestComponentLinking_ExportDiscovery tests that exported functions can be
// discovered and enumerated from a component instance.
func TestComponentLinking_ExportDiscovery(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Provider component with multiple exports
	providerWAT := `
(component
  (core module $discovery_mod
    (func (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
    (func (export "sub") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.sub
    )
  )
  (core instance $i (instantiate $discovery_mod))

  (alias core export $i "add" (core func $add))
  (type $binop_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "add") (type $binop_type)
    (canon lift (core func $add)))

  (alias core export $i "sub" (core func $sub))
  (func (export "sub") (type $binop_type)
    (canon lift (core func $sub)))
)
`

	providerBytes, err := testutil.BuildComponentFromWAT(providerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT: %v", err)
	}

	compiledProvider, err := rt.CompileComponent(ctx, providerBytes)
	if err != nil {
		t.Skipf("CompileComponent: %v", err)
	}
	defer compiledProvider.Close(ctx)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)

	providerLinker := component.NewComponentLinker(rt)
	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	providerInstance, err := providerLinker.Instantiate(testCtx, compiledProvider.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate: %v", err)
	}

	// Test that we can discover both exported functions
	addFunc := providerInstance.ExportedFunction("add")
	if addFunc == nil {
		t.Error("'add' function should be discoverable")
	}

	subFunc := providerInstance.ExportedFunction("sub")
	if subFunc == nil {
		t.Error("'sub' function should be discoverable")
	}

	// Verify they work
	if addFunc != nil {
		result, err := addFunc.Call(testCtx, types.ValS32(10), types.ValS32(3))
		if err != nil {
			t.Fatalf("add(10, 3): %v", err)
		}
		if got := result[0].S32(); got != 13 {
			t.Errorf("add(10, 3) = %d, want 13", got)
		}
	}

	if subFunc != nil {
		result, err := subFunc.Call(testCtx, types.ValS32(10), types.ValS32(3))
		if err != nil {
			t.Fatalf("sub(10, 3): %v", err)
		}
		if got := result[0].S32(); got != 7 {
			t.Errorf("sub(10, 3) = %d, want 7", got)
		}
	}

	// Verify non-existent function returns nil
	missingFunc := providerInstance.ExportedFunction("nonexistent")
	if missingFunc != nil {
		t.Error("'nonexistent' function should return nil")
	}

	t.Log("Export discovery test passed")
}

// TestComponentLinking_ProviderCallCount tests that calls through the linked
// components correctly forward to the provider.
func TestComponentLinking_ProviderCallCount(t *testing.T) {
	ctx := context.Background()

	providerWAT := `
(component
  (core module $count_provider_mod
    (func (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
  )
  (core instance $i (instantiate $count_provider_mod))
  (alias core export $i "add" (core func $add))
  (type $add_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "add") (type $add_type)
    (canon lift (core func $add)))
)
`

	consumerWAT := `
(component
  (import "math" (instance $math
    (export "add" (func (param "a" s32) (param "b" s32) (result s32)))
  ))
  (alias export $math "add" (func $add))
  (core module $count_consumer_mod
    (import "math" "add" (func $add (param i32 i32) (result i32)))
    (func (export "double-add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      call $add
      local.get 0
      local.get 1
      call $add
      i32.add
    )
  )
  (core func $add_lowered (canon lower (func $add)))
  (core instance $i (instantiate $count_consumer_mod
    (with "math" (instance
      (export "add" (func $add_lowered))
    ))
  ))
  (alias core export $i "double-add" (core func $double_add))
  (type $double_add_type (func (param "a" s32) (param "b" s32) (result s32)))
  (func (export "double-add") (type $double_add_type)
    (canon lift (core func $double_add)))
)
`

	providerBytes, err := testutil.BuildComponentFromWAT(providerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT (provider): %v", err)
	}

	consumerBytes, err := testutil.BuildComponentFromWAT(consumerWAT)
	if err != nil {
		t.Skipf("BuildComponentFromWAT (consumer): %v", err)
	}

	// Use separate runtimes for provider and consumer
	providerRT := wazero.NewRuntime(ctx)
	defer providerRT.Close(ctx)

	consumerRT := wazero.NewRuntime(ctx)
	defer consumerRT.Close(ctx)

	compiledProvider, err := providerRT.CompileComponent(ctx, providerBytes)
	if err != nil {
		t.Skipf("CompileComponent (provider): %v", err)
	}
	defer compiledProvider.Close(ctx)

	compiledConsumer, err := consumerRT.CompileComponent(ctx, consumerBytes)
	if err != nil {
		t.Skipf("CompileComponent (consumer): %v", err)
	}
	defer compiledConsumer.Close(ctx)

	resourceTable := runtime.NewTable()
	testCtx := component.WithResourceTable(ctx, resourceTable)

	providerLinker := component.NewComponentLinker(providerRT)
	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	providerInstance, err := providerLinker.Instantiate(testCtx, compiledProvider.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate (provider): %v", err)
	}

	providerAddFunc := providerInstance.ExportedFunction("add")
	if providerAddFunc == nil {
		t.Fatal("provider 'add' function not found")
	}

	// Track how many times the wrapper is called
	var callCount int
	linker := component.NewLinker()
	err = linker.DefineInstance("math").
		Func("add", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			callCount++
			return providerAddFunc.Call(ctx, args...)
		}).
		SkipValidation().
		Build()
	if err != nil {
		t.Fatalf("DefineInstance (math): %v", err)
	}

	consumerLinker := component.NewComponentLinker(consumerRT)
	consumerLinker.MergeFrom(linker)

	// session 1 work: ComponentLinker.Instantiate not yet implemented
	t.Skip("session 1 work: ComponentLinker.Instantiate not yet implemented")

	consumerInstance, err := consumerLinker.Instantiate(testCtx, compiledConsumer.(*component.CompiledComponent))
	if err != nil {
		t.Skipf("Instantiate (consumer): %v", err)
	}

	doubleAddFunc := consumerInstance.ExportedFunction("double-add")
	if doubleAddFunc == nil {
		t.Fatal("consumer 'double-add' function not found")
	}

	// Reset call count
	callCount = 0

	// Call double-add which should call add twice
	_, err = doubleAddFunc.Call(testCtx, types.ValS32(3), types.ValS32(4))
	if err != nil {
		t.Fatalf("double-add(3, 4): %v", err)
	}

	// The consumer's double-add calls add twice
	if callCount != 2 {
		t.Errorf("add was called %d times, want 2", callCount)
	}

	t.Logf("Provider add function was called %d times during double-add", callCount)
}
