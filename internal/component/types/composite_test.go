// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
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
