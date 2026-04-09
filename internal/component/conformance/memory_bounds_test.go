// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: memory-bounds edge-case tests verify that
// lift operations correctly trap when reading beyond memory bounds.
// Uses wazerotest.NewMemory (page-aligned) for boundary testing.
//
// Spec: definitions.py:1947-1948 trap_if(ptr + elem_size > len(memory)).
package conformance

import (
	"encoding/binary"
	"testing"

	"github.com/tetratelabs/wazero/experimental/wazerotest"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestMemoryBoundsShortReadU32 verifies that LiftHeap for a u32 at
// the very end of memory fails when there are not enough bytes.
//
// Spec: definitions.py:1947-1948
// trap_if(ptr + elem_size > len(memory)).
func TestMemoryBoundsShortReadU32(t *testing.T) {
	// One page = 65536 bytes. Try to read a u32 starting at offset
	// 65534, which needs 4 bytes but only 2 remain.
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	liftCtx := &abi.LiftContext{Memory: mem}

	_, err := abi.LiftHeap(liftCtx, types.U32, uint32(mem.Size()-2))
	require.Error(t, err)
}

// TestMemoryBoundsExactFitU32 verifies that LiftHeap for a u32 at
// the exact boundary succeeds when there are exactly enough bytes.
//
// Spec: definitions.py:1947-1948 — ptr + elem_size == len(memory)
// is NOT a trap (the check is strict >).
func TestMemoryBoundsExactFitU32(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	offset := uint32(mem.Size() - 4) // exactly 4 bytes left
	// Write a known value at the boundary.
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, 0xDEADBEEF)
	ok := mem.Write(offset, buf)
	require.True(t, ok)

	liftCtx := &abi.LiftContext{Memory: mem}
	val, err := abi.LiftHeap(liftCtx, types.U32, offset)
	require.NoError(t, err)
	require.Equal(t, uint32(0xDEADBEEF), val.U32())
}

// TestMemoryBoundsShortReadU64 verifies that LiftHeap for a u64 at
// a boundary where fewer than 8 bytes remain produces an error.
//
// Spec: definitions.py:1947-1948
// trap_if(ptr + elem_size > len(memory)).
func TestMemoryBoundsShortReadU64(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	liftCtx := &abi.LiftContext{Memory: mem}

	// 4 bytes remain — not enough for u64
	_, err := abi.LiftHeap(liftCtx, types.U64, uint32(mem.Size()-4))
	require.Error(t, err)
}

// TestMemoryBoundsZeroLengthListAtBoundary verifies that lifting an
// empty list at the memory boundary does not trap, since no bytes
// need to be read.
//
// Spec: definitions.py:1947-1948 — zero-length list requires no
// memory access, so ptr == len(memory) is valid.
func TestMemoryBoundsZeroLengthListAtBoundary(t *testing.T) {
	b := newBuilder()
	listU32 := b.InternList(types.U32)
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	liftCtx := &abi.LiftContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
	}

	// ptr at the very end of memory, length=0
	iter := abi.NewFlatIter([]uint64{uint64(mem.Size()), 0})
	val, err := abi.LiftFlat(liftCtx, listU32, iter)
	require.NoError(t, err)
	require.Equal(t, types.ValKindList, val.Kind())
	require.Equal(t, 0, len(val.List()))
}

// TestMemoryBoundsListExceedsMemory verifies that lifting a list
// whose elements would extend past the end of memory traps.
//
// Spec: definitions.py:1947-1948
// trap_if(ptr + elem_size * length > len(memory)).
func TestMemoryBoundsListExceedsMemory(t *testing.T) {
	b := newBuilder()
	listU32 := b.InternList(types.U32)
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	liftCtx := &abi.LiftContext{
		Types:  ct,
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
	}

	// ptr near end, length would overflow past memory
	ptr := uint32(mem.Size() - 8) // 8 bytes remain
	length := uint32(3)           // 3 * 4 = 12 bytes needed > 8 available
	iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
	_, err := abi.LiftFlat(liftCtx, listU32, iter)
	require.Error(t, err)
}

// TestMemoryBoundsStringShortRead verifies that lifting a string
// whose declared byte length exceeds available memory traps.
//
// Spec: definitions.py:1252-1278 load_string reads ptr+byteLen
// from memory; the underlying memory read must succeed.
func TestMemoryBoundsStringShortRead(t *testing.T) {
	mem := wazerotest.NewMemory(wazerotest.PageSize)
	liftCtx := &abi.LiftContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
	}

	// String at offset near end of memory with length extending past end
	ptr := uint32(mem.Size() - 4)
	length := uint32(100) // claims 100 bytes but only 4 remain
	iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
	_, err := abi.LiftFlat(liftCtx, types.String_, iter)
	require.Error(t, err)
}

// TestMemoryBoundsHeapRecordShortRead verifies that LiftHeap for a
// record at a boundary where the last field overflows traps.
//
// Spec: definitions.py:1947-1948 — each field read must be in bounds.
func TestMemoryBoundsHeapRecordShortRead(t *testing.T) {
	b := newBuilder()
	recType := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.U32},
		{Name: "b", Type: types.U32},
	})
	ct := b.Finish()

	mem := wazerotest.NewMemory(wazerotest.PageSize)
	liftCtx := &abi.LiftContext{
		Types:  ct,
		Memory: mem,
	}

	// Record needs 8 bytes (2 * u32). Place at offset where only 4 bytes remain.
	offset := uint32(mem.Size() - 4)
	_, err := abi.LiftHeap(liftCtx, recType, offset)
	require.Error(t, err)
}
