// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: type-system edge-case tests exercise boundary
// conditions in type construction and ABI computation: large field
// counts, discriminant size boundaries, nested composites, and
// extreme element counts.
//
// Spec: definitions.py type construction.
// No counterpart (justified): wazero boundary tests for type system
// correctness under extreme inputs.
package conformance

import (
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestTypeEdgeCaseLargeRecord verifies that a record with many fields
// (more than MaxFlatParams) gets the correct ABI: FlattenCount still
// grows (even though such a type would spill to memory at the function
// boundary).
//
// Spec: definitions.py:1726-1730 flatten_record — sum of field
// FlattenCounts has no upper bound. The MaxFlatParams limit applies
// at flatten_functype (:1673), not at the type level.
// No counterpart (justified): wazero boundary test.
func TestTypeEdgeCaseLargeRecord(t *testing.T) {
	const fieldCount = 32 // more than MaxFlatParams (16)
	b := newBuilder()
	fields := make([]types.RecordField, fieldCount)
	for i := 0; i < fieldCount; i++ {
		fields[i] = types.RecordField{
			Name: fmt.Sprintf("f%d", i),
			Type: types.U32,
		}
	}
	recType := b.InternRecord(fields)
	ct := b.Finish()

	abiInfo := recType.ABI(ct)
	require.Equal(t, int32(fieldCount), abiInfo.FlattenCount)
	// 32 u32 fields * 4 bytes = 128 bytes, align 4
	require.Equal(t, uint32(128), abiInfo.Size32)
	require.Equal(t, uint32(4), abiInfo.Align32)
}

// TestTypeEdgeCaseDiscriminantSizeBoundary256 verifies that a variant
// with exactly 256 cases uses a 1-byte discriminant, while 257 cases
// requires a 2-byte discriminant.
//
// Spec: definitions.py:1096-1103 discriminant_type —
// cases <= 256 -> u8, cases <= 65536 -> u16, else -> u32.
// No counterpart (justified): wazero boundary test.
func TestTypeEdgeCaseDiscriminantSizeBoundary256(t *testing.T) {
	t.Run("256_cases_1byte_disc", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 256)
		for i := range cases {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("c%d", i), HasPayload: false}
		}
		varType := b.InternVariant(cases)
		ct := b.Finish()

		variant := &ct.Variants[varType.Index]
		require.Equal(t, uint8(1), variant.Disc.DiscSize)
	})

	t.Run("257_cases_2byte_disc", func(t *testing.T) {
		b := newBuilder()
		cases := make([]types.VariantCase, 257)
		for i := range cases {
			cases[i] = types.VariantCase{Name: fmt.Sprintf("c%d", i), HasPayload: false}
		}
		varType := b.InternVariant(cases)
		ct := b.Finish()

		variant := &ct.Variants[varType.Index]
		require.Equal(t, uint8(2), variant.Disc.DiscSize)
	})
}

// TestTypeEdgeCaseNestedRecord verifies that a record containing
// another record correctly computes composite ABI (size, alignment,
// FlattenCount).
//
// Spec: definitions.py:1087-1091 alignment_record,
// :1145-1151 elem_size_record — nested fields contribute recursively.
// No counterpart (justified): wazero boundary test.
func TestTypeEdgeCaseNestedRecord(t *testing.T) {
	b := newBuilder()
	inner := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.U32},
		{Name: "b", Type: types.U64},
	})
	outer := b.InternRecord([]types.RecordField{
		{Name: "x", Type: types.U8},
		{Name: "y", Type: inner},
	})
	ct := b.Finish()

	// inner: align=8, size=alignTo(4,8)+8=16, FlattenCount=2
	innerABI := inner.ABI(ct)
	require.Equal(t, uint32(16), innerABI.Size32)
	require.Equal(t, uint32(8), innerABI.Align32)
	require.Equal(t, int32(2), innerABI.FlattenCount)

	// outer: u8 (size=1,align=1) + inner (size=16,align=8)
	// layout: offset 0: u8 (1 byte), pad to align 8 -> offset 8: inner (16 bytes)
	// total: 8+16=24, align=8 -> size=24
	outerABI := outer.ABI(ct)
	require.Equal(t, uint32(24), outerABI.Size32)
	require.Equal(t, uint32(8), outerABI.Align32)
	require.Equal(t, int32(3), outerABI.FlattenCount) // 1 (u8) + 2 (inner)
}

// TestTypeEdgeCaseNestedRecordRoundtrip verifies that a nested
// record round-trips through LowerFlat / LiftFlat.
//
// Spec: definitions.py:1326-1333 record lower/lift recurses into
// composite field types.
// No counterpart (justified): wazero boundary test for nested
// composites.
func TestTypeEdgeCaseNestedRecordRoundtrip(t *testing.T) {
	b := newBuilder()
	inner := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.U32},
		{Name: "b", Type: types.Bool},
	})
	outer := b.InternRecord([]types.RecordField{
		{Name: "x", Type: inner},
		{Name: "y", Type: types.S16},
	})
	ct := b.Finish()

	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	val := types.ValRecord(map[string]types.Val{
		"x": types.ValRecord(map[string]types.Val{
			"a": types.ValU32(42),
			"b": types.ValBool(true),
		}),
		"y": types.ValS16(-5),
	})

	flat, err := abi.LowerFlat(lowerCtx, outer, val)
	require.NoError(t, err)
	require.Equal(t, 3, len(flat)) // u32 + bool + s16

	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(liftCtx, outer, iter)
	require.NoError(t, err)

	xVal, ok := lifted.RecordField("x")
	require.True(t, ok)
	aVal, ok := xVal.RecordField("a")
	require.True(t, ok)
	require.Equal(t, uint32(42), aVal.U32())
	bVal, ok := xVal.RecordField("b")
	require.True(t, ok)
	require.True(t, bVal.Bool())
	yVal, ok := lifted.RecordField("y")
	require.True(t, ok)
	require.Equal(t, int16(-5), yVal.S16())
}

// TestTypeEdgeCaseEmptyVariant verifies that a variant with zero
// cases (degenerate) or a single case gets correct ABI.
//
// Spec: definitions.py:1093-1094 alignment_variant,
// :1156-1164 elem_size_variant.
// No counterpart (justified): wazero boundary test.
func TestTypeEdgeCaseEmptyVariant(t *testing.T) {
	t.Run("single_case_no_payload", func(t *testing.T) {
		b := newBuilder()
		varType := b.InternVariant([]types.VariantCase{
			{Name: "only", HasPayload: false},
		})
		ct := b.Finish()

		abiInfo := varType.ABI(ct)
		// disc=1 byte, no payload -> size=1, align=1
		require.Equal(t, uint32(1), abiInfo.Size32)
		require.Equal(t, uint32(1), abiInfo.Align32)
		require.Equal(t, int32(1), abiInfo.FlattenCount) // just the discriminant
	})

	t.Run("single_case_with_payload", func(t *testing.T) {
		b := newBuilder()
		varType := b.InternVariant([]types.VariantCase{
			{Name: "only", Payload: types.U32, HasPayload: true},
		})
		ct := b.Finish()

		abiInfo := varType.ABI(ct)
		// disc=1 byte, payload=u32 (4 bytes, align 4)
		// payloadOffset=alignTo(1,4)=4, size=alignTo(4+4,4)=8
		require.Equal(t, uint32(8), abiInfo.Size32)
		require.Equal(t, uint32(4), abiInfo.Align32)
		require.Equal(t, int32(2), abiInfo.FlattenCount) // disc + u32
	})
}

// TestTypeEdgeCaseLargeFlags verifies that flags with >32 labels
// correctly use multi-i32 encoding.
//
// Spec: definitions.py:1112-1117 alignment_flags,
// :1166-1171 elem_size_flags. Flags with n>32 use ceil(n/32) i32s.
// No counterpart (justified): wazero boundary test.
func TestTypeEdgeCaseLargeFlags(t *testing.T) {
	b := newBuilder()
	names := make([]string, 33)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	flagsType := b.InternFlags(names)
	ct := b.Finish()

	abiInfo := flagsType.ABI(ct)
	// 33 flags -> ceil(33/32)=2 i32s -> size=8, align=4
	require.Equal(t, uint32(8), abiInfo.Size32)
	require.Equal(t, uint32(4), abiInfo.Align32)
	require.Equal(t, int32(2), abiInfo.FlattenCount)

	// Roundtrip: set flags in both words
	lowerCtx := &abi.LowerContext{Types: ct}
	liftCtx := &abi.LiftContext{Types: ct}

	flagVals := make(map[string]bool)
	flagVals["flag0"] = true
	flagVals["flag32"] = true // this is in the second i32
	val := types.ValFlags(flagVals)

	flat, err := abi.LowerFlat(lowerCtx, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, 2, len(flat))

	iter := abi.NewFlatIter(flat)
	lifted, err := abi.LiftFlat(liftCtx, flagsType, iter)
	require.NoError(t, err)
	liftedFlags := lifted.Flags()
	require.True(t, liftedFlags["flag0"])
	require.True(t, liftedFlags["flag32"])
	require.False(t, liftedFlags["flag1"])
}
