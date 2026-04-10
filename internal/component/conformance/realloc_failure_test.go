// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: realloc-failure propagation tests verify that
// when the realloc function returns an error or is absent, the lower
// path propagates the error correctly.
//
// Spec: definitions.py:1891-1903 realloc invocation.
package conformance

import (
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestReallocFailureStringLower verifies that when realloc returns
// an error during string lowering, the error is propagated to the
// caller.
//
// Spec: definitions.py:1891-1903 — realloc is invoked for string
// storage; if it traps, the entire lower operation traps.
func TestReallocFailureStringLower(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	reallocErr := errors.New("out of memory")

	lowerCtx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, reallocErr
		},
	}

	_, _, err := abi.LowerString(lowerCtx, "hello world")
	require.Error(t, err)
}

// TestReallocFailureListLower verifies that when realloc returns
// an error during list lowering, the error is propagated.
//
// Spec: definitions.py:1891-1903 — realloc is invoked for list
// element storage; if it traps, the entire lower operation traps.
func TestReallocFailureListLower(t *testing.T) {
	b := newBuilder()
	listU32 := b.InternList(types.U32)
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	reallocErr := errors.New("allocation limit exceeded")

	lowerCtx := &abi.LowerContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, reallocErr
		},
	}

	elements := []types.Val{types.ValU32(1), types.ValU32(2)}
	val := types.ValList(elements)
	_, err := abi.LowerFlat(lowerCtx, listU32, val)
	require.Error(t, err)
}

// TestReallocNilListLower verifies that lowering a non-empty list
// without a realloc function produces an error.
//
// Spec: definitions.py:1891-1903 — realloc must be present for
// any operation that allocates linear memory.
func TestReallocNilListLower(t *testing.T) {
	b := newBuilder()
	listU32 := b.InternList(types.U32)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{
		Types: ct,
		// No Memory, no Realloc
	}

	elements := []types.Val{types.ValU32(1)}
	val := types.ValList(elements)
	_, err := abi.LowerFlat(lowerCtx, listU32, val)
	require.Error(t, err)
}

// TestReallocReturnsButMemoryWriteFails verifies that when realloc
// succeeds but the subsequent memory write fails (e.g., realloc
// returned a pointer beyond memory bounds), the error is propagated.
//
// Spec: definitions.py:1891-1903 — after realloc returns, the
// lower path writes data into the allocated region; a write failure
// is a trap.
func TestReallocReturnsButMemoryWriteFails(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	lowerCtx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			// Return a pointer past the end of memory
			return uint32(mem.Size()) + 1000, nil
		},
	}

	_, _, err := abi.LowerString(lowerCtx, "hello")
	require.Error(t, err)
}

// TestReallocSuccessAfterFailure verifies that a realloc that fails
// on the first call but succeeds on a second does not corrupt state.
// This tests that the lower path does not partially commit before
// realloc succeeds.
//
// Spec: definitions.py:1891-1903 realloc invocation — the lower
// path must be atomic with respect to realloc success/failure.
func TestReallocSuccessAfterFailure(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	callCount := 0

	lowerCtx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			callCount++
			if callCount == 1 {
				return 0, errors.New("transient failure")
			}
			return 256, nil
		},
	}

	// First call should fail
	_, _, err := abi.LowerString(lowerCtx, "test")
	require.Error(t, err)

	// Second call should succeed (realloc succeeds now)
	ptr, length, err := abi.LowerString(lowerCtx, "test")
	require.NoError(t, err)
	require.Equal(t, uint32(256), ptr)
	require.Equal(t, uint32(4), length)
}

// TestReallocEmptyStringNoCall verifies that lowering an empty
// string does NOT invoke realloc (no allocation needed).
//
// Spec: definitions.py:1891-1903 — zero-length strings require
// no allocation.
func TestReallocEmptyStringNoCall(t *testing.T) {
	called := false
	lowerCtx := &abi.LowerContext{
		Opts: &abi.Options{StringEncoding: types.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			called = true
			return 0, nil
		},
	}

	ptr, length, err := abi.LowerString(lowerCtx, "")
	require.NoError(t, err)
	require.Equal(t, uint32(0), ptr)
	require.Equal(t, uint32(0), length)
	require.False(t, called, "realloc should not be called for empty string")
}
