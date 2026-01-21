package abi

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLiftContext(t *testing.T) {
	// Create a mock memory with some data
	data := make([]byte, 64)
	// Write u32 at offset 8
	binary.LittleEndian.PutUint32(data[8:], 42)

	ctx := &LiftContext{
		Memory: &mockMemory{data: data},
		Opts: &Options{
			StringEncoding: StringEncodingUTF8,
		},
	}

	// Read u32 from offset 8
	val, err := ctx.ReadU32(8)
	require.NoError(t, err)
	require.Equal(t, uint32(42), val)
}

func TestLiftContextReadU8(t *testing.T) {
	data := make([]byte, 64)
	data[0] = 0x42
	ctx := &LiftContext{Memory: &mockMemory{data: data}, Opts: &Options{}}
	val, err := ctx.ReadU8(0)
	require.NoError(t, err)
	require.Equal(t, uint8(0x42), val)
}

func TestLiftContextReadU16(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint16(data[0:], 0x1234)
	ctx := &LiftContext{Memory: &mockMemory{data: data}, Opts: &Options{}}
	val, err := ctx.ReadU16(0)
	require.NoError(t, err)
	require.Equal(t, uint16(0x1234), val)
}

func TestLiftContextReadU64(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint64(data[0:], 0x123456789ABCDEF0)
	ctx := &LiftContext{Memory: &mockMemory{data: data}, Opts: &Options{}}
	val, err := ctx.ReadU64(0)
	require.NoError(t, err)
	require.Equal(t, uint64(0x123456789ABCDEF0), val)
}

func TestLiftContextReadF32(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint32(data[0:], math.Float32bits(3.14))
	ctx := &LiftContext{Memory: &mockMemory{data: data}, Opts: &Options{}}
	val, err := ctx.ReadF32(0)
	require.NoError(t, err)
	require.Equal(t, float32(3.14), val)
}

func TestLiftContextReadF64(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint64(data[0:], math.Float64bits(3.14159265359))
	ctx := &LiftContext{Memory: &mockMemory{data: data}, Opts: &Options{}}
	val, err := ctx.ReadF64(0)
	require.NoError(t, err)
	require.Equal(t, 3.14159265359, val)
}

// Bounds checking tests

func TestLiftContextReadU8BoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read at offset 0
	_, err := ctx.ReadU8(0)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Valid boundary read (last byte)
	_, err = ctx.ReadU8(15)
	if err != nil {
		t.Errorf("unexpected error for boundary read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadU8(16)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}

	// Way out of bounds
	_, err = ctx.ReadU8(100)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextReadU16BoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read
	_, err := ctx.ReadU16(0)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Boundary read (last 2 bytes)
	_, err = ctx.ReadU16(14)
	if err != nil {
		t.Errorf("unexpected error for boundary read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadU16(15)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}

	// Way out of bounds
	_, err = ctx.ReadU16(100)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextReadU32BoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read
	_, err := ctx.ReadU32(0)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Boundary read (last 4 bytes)
	_, err = ctx.ReadU32(12)
	if err != nil {
		t.Errorf("unexpected error for boundary read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadU32(13)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}

	// Way out of bounds
	_, err = ctx.ReadU32(100)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextReadU64BoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read
	_, err := ctx.ReadU64(0)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Boundary read (last 8 bytes)
	_, err = ctx.ReadU64(8)
	if err != nil {
		t.Errorf("unexpected error for boundary read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadU64(9)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}

	// Way out of bounds
	_, err = ctx.ReadU64(100)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextReadF32BoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read
	_, err := ctx.ReadF32(0)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Boundary read (last 4 bytes)
	_, err = ctx.ReadF32(12)
	if err != nil {
		t.Errorf("unexpected error for boundary read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadF32(13)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextReadF64BoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read
	_, err := ctx.ReadF64(0)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Boundary read (last 8 bytes)
	_, err = ctx.ReadF64(8)
	if err != nil {
		t.Errorf("unexpected error for boundary read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadF64(9)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextReadBytesBoundsCheck(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 16)}
	ctx := &LiftContext{Memory: mem}

	// Valid read - full memory
	_, err := ctx.ReadBytes(0, 16)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Valid read - partial
	_, err = ctx.ReadBytes(8, 8)
	if err != nil {
		t.Errorf("unexpected error for valid read: %v", err)
	}

	// Out of bounds - should error, not panic
	_, err = ctx.ReadBytes(0, 17)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}

	// Out of bounds - offset too high
	_, err = ctx.ReadBytes(15, 2)
	if err == nil {
		t.Error("expected error for out-of-bounds read")
	}
}

func TestLiftContextNilMemory(t *testing.T) {
	ctx := &LiftContext{Memory: nil}

	// All read methods should error gracefully with nil memory
	_, err := ctx.ReadU8(0)
	if err == nil {
		t.Error("expected error for nil memory on ReadU8")
	}

	_, err = ctx.ReadU16(0)
	if err == nil {
		t.Error("expected error for nil memory on ReadU16")
	}

	_, err = ctx.ReadU32(0)
	if err == nil {
		t.Error("expected error for nil memory on ReadU32")
	}

	_, err = ctx.ReadU64(0)
	if err == nil {
		t.Error("expected error for nil memory on ReadU64")
	}

	_, err = ctx.ReadF32(0)
	if err == nil {
		t.Error("expected error for nil memory on ReadF32")
	}

	_, err = ctx.ReadF64(0)
	if err == nil {
		t.Error("expected error for nil memory on ReadF64")
	}

	_, err = ctx.ReadBytes(0, 1)
	if err == nil {
		t.Error("expected error for nil memory on ReadBytes")
	}
}

type mockMemory struct {
	data []byte
}

func (m *mockMemory) Read(offset, size uint32) ([]byte, bool) {
	if int(offset+size) > len(m.data) {
		return nil, false
	}
	return m.data[offset : offset+size], true
}

func (m *mockMemory) Write(offset uint32, data []byte) bool {
	if int(offset)+len(data) > len(m.data) {
		return false
	}
	copy(m.data[offset:], data)
	return true
}

func (m *mockMemory) Size() uint32 {
	return uint32(len(m.data))
}

func TestLowerContext_WithSubtask(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 1024)}
	rt := component.NewResourceTable()
	subtask := component.NewSubtask(rt)

	ctx := &LowerContext{
		Memory:  mem,
		Opts:    &Options{StringEncoding: StringEncodingUTF8},
		Subtask: subtask,
	}

	require.Same(t, subtask, ctx.Subtask)
	require.Same(t, subtask.BorrowScope(), ctx.BorrowScope())
}

func TestLowerContext_BorrowScope_NilSubtask(t *testing.T) {
	ctx := &LowerContext{
		Memory: &mockMemory{data: make([]byte, 64)},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		// Subtask is nil
	}

	// BorrowScope should return nil when Subtask is nil
	require.Nil(t, ctx.BorrowScope())
}
