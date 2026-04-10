// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: error-message quality tests verify that
// lift/lower operations produce clear, descriptive error messages
// when given deliberately malformed inputs. This is a wazero
// engineering invariant — the spec does not prescribe error message
// content, but good diagnostics are essential for user experience.
package conformance

import (
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestErrorMessages exercises error-message clarity across lift/lower
// operations with deliberately malformed inputs.
//
// No counterpart (justified): wazero error-message-quality engineering
// invariant. The spec mandates traps but does not prescribe error
// message content.
func TestErrorMessages(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: LiftHeap for a u32 with nil memory produces an error
	// that mentions "memory".
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. The spec traps at definitions.py:1947-1948
	// when memory is inaccessible; we assert that the Go error message
	// is helpful.
	t.Run("NilMemoryHeapLift", func(t *testing.T) {
		liftCtx := &abi.LiftContext{}
		_, err := abi.LiftHeap(liftCtx, types.U32, 0)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "memory") || strings.Contains(errMsg, "Memory"),
			"error should mention memory, got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 2: LiftFlat on a list with nil memory produces an error
	// that mentions "memory".
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. Spec: definitions.py:1947-1948
	// trap_if(ptr + elem_size > len(memory)).
	t.Run("NilMemoryListLift", func(t *testing.T) {
		b := newBuilder()
		listU32 := b.InternList(types.U32)
		ct := b.Finish()

		liftCtx := &abi.LiftContext{
			Types: ct,
			Opts:  &abi.Options{StringEncoding: types.StringEncodingUTF8},
		}
		// ptr=0, len=5 — non-empty list, nil memory
		iter := abi.NewFlatIter([]uint64{0, 5})
		_, err := abi.LiftFlat(liftCtx, listU32, iter)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "memory") || strings.Contains(errMsg, "Memory"),
			"error should mention memory, got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 3: LiftFlat with an invalid variant discriminant produces
	// an error that mentions "discriminant".
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. Spec: definitions.py:1478-1504 variant
	// lift path traps on invalid discriminant.
	t.Run("InvalidVariantDiscriminant", func(t *testing.T) {
		b := newBuilder()
		varType := b.InternVariant([]types.VariantCase{
			{Name: "alpha", HasPayload: false},
			{Name: "beta", HasPayload: false},
		})
		ct := b.Finish()

		liftCtx := &abi.LiftContext{Types: ct}
		// discriminant=99 is out of range for a 2-case variant
		iter := abi.NewFlatIter([]uint64{99})
		_, err := abi.LiftFlat(liftCtx, varType, iter)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "discriminant"),
			"error should mention 'discriminant', got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 4: LiftFlat with an invalid char value produces an error
	// that mentions "Unicode" or "scalar".
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. Spec: definitions.py:1191 char lift
	// traps on invalid Unicode scalar value.
	t.Run("InvalidCharValue", func(t *testing.T) {
		// U+D800 is a surrogate, not a valid Unicode scalar value
		iter := abi.NewFlatIter([]uint64{0xD800})
		_, err := abi.LiftFlat(nil, types.Char, iter)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "Unicode") || strings.Contains(errMsg, "scalar") || strings.Contains(errMsg, "char"),
			"error should mention char/Unicode/scalar, got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 5: LowerFlat with a missing record field produces an error
	// that names the missing field.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. Spec: definitions.py:1326-1333 record
	// lower path iterates all fields.
	t.Run("MissingRecordField", func(t *testing.T) {
		b := newBuilder()
		recType := b.InternRecord([]types.RecordField{
			{Name: "alpha", Type: types.U32},
			{Name: "beta", Type: types.U32},
		})
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		// Only provide "alpha", omit "beta"
		val := types.ValRecord(map[string]types.Val{
			"alpha": types.ValU32(1),
		})
		_, err := abi.LowerFlat(lowerCtx, recType, val)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "beta"),
			"error should mention the missing field 'beta', got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 6: LowerFlat with an unknown variant case produces an error
	// that names the unknown case.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. Spec: definitions.py:1478-1504 variant
	// lower path checks the case name.
	t.Run("UnknownVariantCase", func(t *testing.T) {
		b := newBuilder()
		varType := b.InternVariant([]types.VariantCase{
			{Name: "red", HasPayload: false},
			{Name: "blue", HasPayload: false},
		})
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		val := types.ValVariant("green", nil)
		_, err := abi.LowerFlat(lowerCtx, varType, val)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "green"),
			"error should mention the unknown case 'green', got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 7: LiftFlat with an invalid enum discriminant produces an
	// error that mentions "enum" or "discriminant".
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant. Spec: definitions.py enum lift traps on
	// out-of-range discriminant.
	t.Run("InvalidEnumDiscriminant", func(t *testing.T) {
		b := newBuilder()
		enumType := b.InternEnum([]string{"a", "b", "c"})
		ct := b.Finish()

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter([]uint64{5}) // out of range for 3-case enum
		_, err := abi.LiftFlat(liftCtx, enumType, iter)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "enum") || strings.Contains(errMsg, "discriminant"),
			"error should mention 'enum' or 'discriminant', got: %s", errMsg)
	})

	// ------------------------------------------------------------------
	// Case 8: LowerFlat with an unknown enum case names the case.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): wazero error-message-quality
	// engineering invariant.
	t.Run("UnknownEnumCase", func(t *testing.T) {
		b := newBuilder()
		enumType := b.InternEnum([]string{"x", "y"})
		ct := b.Finish()

		lowerCtx := &abi.LowerContext{Types: ct}
		val := types.ValEnum("z")
		_, err := abi.LowerFlat(lowerCtx, enumType, val)
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t, strings.Contains(errMsg, "z"),
			"error should mention the unknown case 'z', got: %s", errMsg)
	})
}
