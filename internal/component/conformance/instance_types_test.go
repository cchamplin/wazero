// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: instance-type roundtrip tests verify that
// composite types (Record, Variant, List, Tuple, Option, Result)
// correctly round-trip through LowerFlat / LiftFlat using the
// ComponentTypesBuilder interning API.
//
// Spec: definitions.py:1326-1333 (record), :1478-1504 (variant),
//
//	:1413-1428 (option), :1439-1460 (result).
package conformance

import (
	"encoding/binary"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstanceTypeRecordRoundtrip verifies that a record with mixed
// field types round-trips through LowerFlat / LiftFlat.
//
// Spec: definitions.py:1326-1333 record lower/lift iterates fields
// in declaration order, each field lowered/lifted according to its
// own type.
// Canonical test: run_tests.py::test_roundtrips exercises composite
// lift/lower.
func TestInstanceTypeRecordRoundtrip(t *testing.T) {
	b := newBuilder()
	recType := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.U32},
		{Name: "y", Type: types.S16},
		{Name: "z", Type: types.Bool},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	val := types.ValRecord(map[string]types.Val{
		"x": types.ValU32(42),
		"y": types.ValS16(-7),
		"z": types.ValBool(true),
	})

	flat, err := abi.LowerFlat(lowerCtx, recType, val)
	require.NoError(t, err)
	require.Equal(t, 3, len(flat))

	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(liftCtx, recType, iter)
	require.NoError(t, err)
	require.Equal(t, types.ValKindRecord, lifted.Kind())

	xVal, ok := lifted.RecordField("x")
	require.True(t, ok)
	require.Equal(t, uint32(42), xVal.U32())

	yVal, ok := lifted.RecordField("y")
	require.True(t, ok)
	require.Equal(t, int16(-7), yVal.S16())

	zVal, ok := lifted.RecordField("z")
	require.True(t, ok)
	require.True(t, zVal.Bool())
}

// TestInstanceTypeVariantRoundtrip verifies that a variant with
// mixed payload types round-trips through LowerFlat / LiftFlat.
//
// Spec: definitions.py:1478-1504 variant lower/lift uses
// discriminant + joined payload types.
// Canonical test: run_tests.py::test_roundtrips variant cases.
func TestInstanceTypeVariantRoundtrip(t *testing.T) {
	b := newBuilder()
	varType := b.InternVariant([]types.VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some-u32", Payload: types.U32, HasPayload: true},
		{Name: "some-bool", Payload: types.Bool, HasPayload: true},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("unit_case", func(t *testing.T) {
		val := types.ValVariant("none", nil)
		flat, err := abi.LowerFlat(lowerCtx, varType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, varType, iter)
		require.NoError(t, err)
		caseName, payload := lifted.Variant()
		require.Equal(t, "none", caseName)
		require.Nil(t, payload)
	})

	t.Run("u32_case", func(t *testing.T) {
		p := types.ValU32(99)
		val := types.ValVariant("some-u32", &p)
		flat, err := abi.LowerFlat(lowerCtx, varType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, varType, iter)
		require.NoError(t, err)
		caseName, payload := lifted.Variant()
		require.Equal(t, "some-u32", caseName)
		require.NotNil(t, payload)
		require.Equal(t, uint32(99), payload.U32())
	})

	t.Run("bool_case", func(t *testing.T) {
		p := types.ValBool(true)
		val := types.ValVariant("some-bool", &p)
		flat, err := abi.LowerFlat(lowerCtx, varType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, varType, iter)
		require.NoError(t, err)
		caseName, payload := lifted.Variant()
		require.Equal(t, "some-bool", caseName)
		require.NotNil(t, payload)
		require.True(t, payload.Bool())
	})
}

// TestInstanceTypeTupleRoundtrip verifies that a tuple with mixed
// element types round-trips through LowerFlat / LiftFlat.
//
// Spec: definitions.py:126-127 TupleType is positional record;
// lift/lower follows record semantics.
// Canonical test: run_tests.py::test_roundtrips tuple fixtures.
func TestInstanceTypeTupleRoundtrip(t *testing.T) {
	b := newBuilder()
	tupType := b.InternTuple([]types.ValType{types.U32, types.S64, types.Bool})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	val := types.ValTuple([]types.Val{
		types.ValU32(100),
		types.ValS64(-200),
		types.ValBool(false),
	})

	flat, err := abi.LowerFlat(lowerCtx, tupType, val)
	require.NoError(t, err)
	require.Equal(t, 3, len(flat))

	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(liftCtx, tupType, iter)
	require.NoError(t, err)
	require.Equal(t, types.ValKindTuple, lifted.Kind())

	elems := lifted.Tuple()
	require.Equal(t, 3, len(elems))
	require.Equal(t, uint32(100), elems[0].U32())
	require.Equal(t, int64(-200), elems[1].S64())
	require.False(t, elems[2].Bool())
}

// TestInstanceTypeOptionRoundtrip verifies that option<T> round-trips
// through LowerFlat / LiftFlat for both None and Some cases.
//
// Spec: definitions.py:1413-1428 option is variant{none, some(T)}.
// Canonical test: run_tests.py::test_roundtrips option fixtures.
func TestInstanceTypeOptionRoundtrip(t *testing.T) {
	b := newBuilder()
	optType := b.InternOption(types.U32)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("none", func(t *testing.T) {
		val := types.ValOption(nil)
		flat, err := abi.LowerFlat(lowerCtx, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindOption, lifted.Kind())
		require.Nil(t, lifted.Option())
	})

	t.Run("some", func(t *testing.T) {
		p := types.ValU32(42)
		val := types.ValOption(&p)
		flat, err := abi.LowerFlat(lowerCtx, optType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, optType, iter)
		require.NoError(t, err)
		require.Equal(t, types.ValKindOption, lifted.Kind())
		require.NotNil(t, lifted.Option())
		require.Equal(t, uint32(42), lifted.Option().U32())
	})
}

// TestInstanceTypeResultRoundtrip verifies that result<T, E> round-trips
// through LowerFlat / LiftFlat for ok and error cases.
//
// Spec: definitions.py:1439-1460 result is variant{ok(T), error(E)}.
// Canonical test: run_tests.py::test_roundtrips result fixtures.
func TestInstanceTypeResultRoundtrip(t *testing.T) {
	b := newBuilder()
	resType := b.InternResult(types.U32, types.S32, true, true)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("ok", func(t *testing.T) {
		okVal := types.ValU32(77)
		val := types.ValResultOk(&okVal)
		flat, err := abi.LowerFlat(lowerCtx, resType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resType, iter)
		require.NoError(t, err)
		isOk, okP, _ := lifted.Result()
		require.True(t, isOk)
		require.NotNil(t, okP)
		require.Equal(t, uint32(77), okP.U32())
	})

	t.Run("error", func(t *testing.T) {
		errVal := types.ValS32(-1)
		val := types.ValResultError(&errVal)
		flat, err := abi.LowerFlat(lowerCtx, resType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resType, iter)
		require.NoError(t, err)
		isOk, _, errP := lifted.Result()
		require.False(t, isOk)
		require.NotNil(t, errP)
		require.Equal(t, int32(-1), errP.S32())
	})
}

// TestInstanceTypeListRoundtrip verifies that a dynamic list<u32>
// round-trips through LowerFlat (with realloc) / LiftFlat (with
// heap read).
//
// Spec: definitions.py:1075,1133,1714 list memory layout is
// (ptr: i32, len: i32); elements are stored inline in linear memory.
// Canonical test: run_tests.py::test_heap list cases.
func TestInstanceTypeListRoundtrip(t *testing.T) {
	b := newBuilder()
	listU32 := b.InternList(types.U32)
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	alloc := uint32(256)
	lowerCtx := &abi.LowerContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			if align > 1 {
				alloc = (alloc + align - 1) &^ (align - 1)
			}
			result := alloc
			alloc += newSize
			return result, nil
		},
	}
	liftCtx := &abi.LiftContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
	}

	elements := []types.Val{
		types.ValU32(10),
		types.ValU32(20),
		types.ValU32(30),
	}
	val := types.ValList(elements)

	flat, err := abi.LowerFlat(lowerCtx, listU32, val)
	require.NoError(t, err)
	require.Equal(t, 2, len(flat)) // ptr, len

	// Verify memory contents
	ptr := uint32(flat[0])
	length := uint32(flat[1])
	require.Equal(t, uint32(3), length)
	for i := uint32(0); i < length; i++ {
		data, ok := mem.Read(ptr+i*4, 4)
		require.True(t, ok)
		v := binary.LittleEndian.Uint32(data)
		require.Equal(t, uint32((i+1)*10), v)
	}

	// Lift back
	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(liftCtx, listU32, iter)
	require.NoError(t, err)
	require.Equal(t, types.ValKindList, lifted.Kind())
	elems := lifted.List()
	require.Equal(t, 3, len(elems))
	require.Equal(t, uint32(10), elems[0].U32())
	require.Equal(t, uint32(20), elems[1].U32())
	require.Equal(t, uint32(30), elems[2].U32())
}

// TestInstanceTypeResultNoPayloads verifies result<_, _> (no ok,
// no error payload) round-trips correctly.
//
// Spec: definitions.py:1439-1460 result with both payloads absent.
func TestInstanceTypeResultNoPayloads(t *testing.T) {
	b := newBuilder()
	// result with no ok and no error payload
	resType := b.InternResult(types.ValType{}, types.ValType{}, false, false)
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	t.Run("ok_no_payload", func(t *testing.T) {
		val := types.ValResultOk(nil)
		flat, err := abi.LowerFlat(lowerCtx, resType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resType, iter)
		require.NoError(t, err)
		isOk, okP, _ := lifted.Result()
		require.True(t, isOk)
		require.Nil(t, okP)
	})

	t.Run("error_no_payload", func(t *testing.T) {
		val := types.ValResultError(nil)
		flat, err := abi.LowerFlat(lowerCtx, resType, val)
		require.NoError(t, err)

		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, resType, iter)
		require.NoError(t, err)
		isOk, _, errP := lifted.Result()
		require.False(t, isOk)
		require.Nil(t, errP)
	})
}
