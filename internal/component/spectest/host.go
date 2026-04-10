// Package spectest provides the component model spectest host fixture.
//
// This is the wazero equivalent of wasmtime's link_component_spectest()
// (debug-vendored/wasmtime/crates/wast/src/spectest.rs:91-223).
// It pre-populates a ComponentLinker with the host functions, resources,
// and instances that component model spec tests (.wast files) expect.
package spectest

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// resourceState tracks shared state for resource1 destructors.
// The spec tests query this via [static]resource1.drops() and
// [static]resource1.last-drop().
type resourceState struct {
	drops    atomic.Uint32
	lastDrop atomic.Uint32
}

// LinkComponentSpectest populates a ComponentLinker with the host fixture
// required by the component model spec tests. This mirrors wasmtime's
// link_component_spectest() function.
//
// Root-level functions:
//   - host-echo-u32(u32) -> u32
//   - host-return-two() -> u32
//
// Instance "host" exports:
//   - return-three() -> u32
//   - return-hi() -> string
//   - nested instance "nested" with return-four() -> u32
//   - resource1, resource2, resource1-again
//   - resource1 constructor, static methods, and instance methods
func LinkComponentSpectest(linker api.ComponentLinker) error {
	// --- Root-level functions ---

	if err := linker.DefineFunc("", "host-echo-u32", hostEchoU32); err != nil {
		return fmt.Errorf("define host-echo-u32: %w", err)
	}
	if err := linker.DefineFunc("", "host-return-two", hostReturnTwo); err != nil {
		return fmt.Errorf("define host-return-two: %w", err)
	}

	// --- Instance "host" ---

	state := &resourceState{}

	builder := linker.DefineInstance("host")

	// Functions
	builder.Func("return-three", hostReturnThree)
	builder.Func("return-hi", hostReturnHi)

	// Resources
	builder.Resource("resource1", func(rep uint32) {
		state.drops.Add(1)
		state.lastDrop.Store(rep)
	})
	builder.Resource("resource2", func(rep uint32) {
		// no-op destructor
	})
	builder.Resource("resource1-again", func(rep uint32) {
		panic("resource1-again destructor should not be called")
	})

	// Resource1 constructor: (u32) -> own<resource1>
	builder.Func("[constructor]resource1", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		rep := args[0].U32()
		return []types.Val{types.ValOwn(rep)}, nil
	})

	// [static]resource1.assert: (own<resource1>, u32) -> ()
	builder.Func("[static]resource1.assert", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		resourceRep := args[0].Own()
		expectedRep := args[1].U32()
		if resourceRep != expectedRep {
			return nil, fmt.Errorf("resource1.assert: rep %d != expected %d", resourceRep, expectedRep)
		}
		return nil, nil
	})

	// [static]resource1.last-drop: () -> u32
	builder.Func("[static]resource1.last-drop", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValU32(state.lastDrop.Load())}, nil
	})

	// [static]resource1.drops: () -> u32
	builder.Func("[static]resource1.drops", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValU32(state.drops.Load())}, nil
	})

	// [method]resource1.simple: (borrow<resource1>, u32) -> ()
	builder.Func("[method]resource1.simple", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		borrowRep := args[0].Borrow()
		expectedRep := args[1].U32()
		if borrowRep != expectedRep {
			return nil, fmt.Errorf("resource1.simple: borrow rep %d != expected %d", borrowRep, expectedRep)
		}
		return nil, nil
	})

	// [method]resource1.take-borrow: (borrow<resource1>, borrow<resource1>) -> ()
	builder.Func("[method]resource1.take-borrow", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		// Both args should be borrows. Validation is that they don't panic.
		_ = args[0].Borrow()
		_ = args[1].Borrow()
		return nil, nil
	})

	// [method]resource1.take-own: (borrow<resource1>, own<resource1>) -> ()
	builder.Func("[method]resource1.take-own", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		// First arg is a borrow, second is an own.
		_ = args[0].Borrow()
		_ = args[1].Own()
		return nil, nil
	})

	// Nested instance "nested" with return-four() -> u32
	nested := builder.Instance("nested")
	nested.Func("return-four", hostReturnFour)
	if err := nested.Build(); err != nil {
		return fmt.Errorf("build nested instance: %w", err)
	}

	// Skip validation: the host fixture is intentionally incomplete.
	// Missing: async functions (never-return, return-two-slowly, echo-slowly,
	// [method]resource1.never-return) which require the async proposal;
	// and "simple-module" (a core wasm module with global g=100 and func
	// f()->101) which requires core module embedding support not yet
	// available on ComponentInstanceBuilder.
	builder.SkipValidation()

	if err := builder.Build(); err != nil {
		return fmt.Errorf("build host instance: %w", err)
	}

	return nil
}

// --- Host function implementations ---

// hostEchoU32 echoes a u32 argument back: (u32) -> u32
func hostEchoU32(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValU32(args[0].U32())}, nil
}

// hostReturnTwo returns the constant 2: () -> u32
func hostReturnTwo(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValU32(2)}, nil
}

// hostReturnThree returns the constant 3: () -> u32
func hostReturnThree(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValU32(3)}, nil
}

// hostReturnFour returns the constant 4: () -> u32
func hostReturnFour(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValU32(4)}, nil
}

// hostReturnHi returns the string "hi": () -> string
func hostReturnHi(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	return []types.Val{types.ValString("hi")}, nil
}
