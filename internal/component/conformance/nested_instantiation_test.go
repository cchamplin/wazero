// Package conformance contains conformance tests for the Component Model implementation.
// This file tests WAT-based nested component instantiation patterns.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/binary"
	"github.com/tetratelabs/wazero/internal/component/testutil"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestNestedInstantiation_SimpleExport tests a component with a core module
// that exports a function, canon lift wrapping it, and a world-level export.
// Verifies the basic instantiation-to-call path works.
func TestNestedInstantiation_SimpleExport(t *testing.T) {
	wat := `
(component
    (core module $m
        (func (export "get-val") (result i32)
            i32.const 42
        )
    )
    (core instance $i (instantiate $m))
    (alias core export $i "get-val" (core func $f))
    (type $ft (func (result u32)))
    (func (export "get-val") (type $ft)
        (canon lift (core func $f)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools not available or WAT invalid: %v", err)
	}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(wasmBytes)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	getVal := inst.ExportedFunction("get-val")
	require.NotNil(t, getVal, "expected 'get-val' export")

	results, err := getVal.Call(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint32(42), results[0].U32())
}

// TestNestedInstantiation_RecordReturn tests a component with a core function
// that returns a record via retptr (flat count 2 > MAX_FLAT_RESULTS=1).
// The record has two s32 fields (x=10, y=20).
func TestNestedInstantiation_RecordReturn(t *testing.T) {
	wat := `
(component
    (core module $m
        (memory (export "memory") 1)
        (func $realloc (export "cabi_realloc")
            (param i32 i32 i32 i32) (result i32)
            i32.const 1024
        )
        (func (export "get-pair") (param i32)
            local.get 0
            i32.const 10
            i32.store
            local.get 0
            i32.const 20
            i32.store offset=4
        )
    )
    (core instance $i (instantiate $m))
    (alias core export $i "get-pair" (core func $f))
    (alias core export $i "memory" (core memory $mem))
    (alias core export $i "cabi_realloc" (core func $realloc))
    (type $pair (record (field "x" s32) (field "y" s32)))
    (type $ft (func (result $pair)))
    (func (export "get-pair") (type $ft)
        (canon lift (core func $f)
            (memory $mem)
            (realloc $realloc)))
)
`

	wasmBytes, err := testutil.BuildComponentFromWAT(wat)
	if err != nil {
		t.Skipf("wasm-tools not available or WAT invalid: %v", err)
	}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	c, err := binary.DecodeComponent(wasmBytes)
	require.NoError(t, err)

	inst, err := component.Instantiate(ctx, component.NewRuntimeInstantiator(rt), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	getPair := inst.ExportedFunction("get-pair")
	require.NotNil(t, getPair, "expected 'get-pair' export")

	results, err := getPair.Call(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	// The result should be a record with fields "x" and "y"
	xVal, ok := results[0].RecordField("x")
	require.True(t, ok, "expected record field 'x'")
	require.Equal(t, int32(10), xVal.S32())

	yVal, ok := results[0].RecordField("y")
	require.True(t, ok, "expected record field 'y'")
	require.Equal(t, int32(20), yVal.S32())
}
