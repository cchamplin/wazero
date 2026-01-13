// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestRecordType(t *testing.T) {
	// Record { a: u32, b: u64 }
	// Layout: u32 at offset 0, padding 4, u64 at offset 8
	// Size: 16, Align: 8
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: U64{}},
		},
	}

	require.Equal(t, uint32(16), r.Size())
	require.Equal(t, uint32(8), r.Align())
	require.Equal(t, 2, r.FlattenCount()) // Both fields fit flat
}

func TestRecordOffset(t *testing.T) {
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: U64{}},
		},
	}

	offsets := r.FieldOffsets()
	require.Equal(t, uint32(0), offsets[0])  // a at 0
	require.Equal(t, uint32(8), offsets[1])  // b at 8 (aligned)
}

func TestRecordEmpty(t *testing.T) {
	r := Record{Fields: []Field{}}
	require.Equal(t, uint32(0), r.Size())
	require.Equal(t, uint32(1), r.Align())
	require.Equal(t, 0, r.FlattenCount())
}

func TestRecordComplex(t *testing.T) {
	// Record { a: u8, b: u32, c: u16 }
	// Offset 0: a (1 byte)
	// Offset 1-3: padding
	// Offset 4: b (4 bytes)
	// Offset 8: c (2 bytes)
	// Offset 10-11: padding to align 4
	// Size: 12, Align: 4
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U8{}},
			{Name: "b", Type: U32{}},
			{Name: "c", Type: U16{}},
		},
	}

	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())

	offsets := r.FieldOffsets()
	require.Equal(t, uint32(0), offsets[0])
	require.Equal(t, uint32(4), offsets[1])
	require.Equal(t, uint32(8), offsets[2])
}

func TestVariantType(t *testing.T) {
	// variant { none, some(u32) }
	// Discriminant: 1 byte (2 cases < 256)
	// Payload: max(0, 4) = 4 bytes
	// Layout: disc(1) + padding(3) + payload(4) = 8, Align: 4
	v := Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: U32{}},
		},
	}

	require.Equal(t, uint32(8), v.Size())
	require.Equal(t, uint32(4), v.Align())
	require.Equal(t, 2, v.FlattenCount()) // disc + payload
}

func TestVariantDiscriminantSize(t *testing.T) {
	tests := []struct {
		numCases int
		discSize uint32
	}{
		{2, 1},
		{255, 1},
		{256, 1},
		{257, 2},
		{65536, 2},
		{65537, 4},
	}

	for _, tc := range tests {
		cases := make([]Case, tc.numCases)
		for i := range cases {
			cases[i] = Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		v := Variant{Cases: cases}
		require.Equal(t, tc.discSize, v.DiscriminantSize(),
			"numCases=%d", tc.numCases)
	}
}

func TestVariantWithU64Payload(t *testing.T) {
	// variant { a, b(u64) }
	// Discriminant: 1 byte
	// Padding: 7 bytes (align to 8 for u64)
	// Payload: 8 bytes
	// Size: 16, Align: 8
	v := Variant{
		Cases: []Case{
			{Name: "a", Type: nil},
			{Name: "b", Type: U64{}},
		},
	}

	require.Equal(t, uint32(16), v.Size())
	require.Equal(t, uint32(8), v.Align())
}

func TestVariantFlatten(t *testing.T) {
	// variant { a(u32), b(f32) }
	// Flat: [i32 (disc), i32 (joined payload)]
	// Note: u32 and f32 both flatten to single value, join produces i32
	v := Variant{
		Cases: []Case{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: F32{}},
		},
	}

	require.Equal(t, 2, v.FlattenCount())
}
