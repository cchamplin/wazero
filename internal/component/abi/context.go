package abi

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
)

// Flat ABI limits as defined by the Component Model specification.
// These determine when values are passed in registers (flat) vs. memory (heap).
const (
	// MaxFlatParams is the maximum number of flattened parameter values
	// that can be passed directly. Beyond this, parameters spill to memory.
	MaxFlatParams = 16

	// MaxFlatResults is the maximum number of flattened result values
	// that can be returned directly (for synchronous calls).
	// Beyond this, results spill to memory via a return pointer.
	MaxFlatResults = 1
)

// Canonical NaN bit patterns as per Canonical ABI spec.
// All NaN values are canonicalized to these patterns when lifting.
const (
	CanonicalFloat32NaN = uint32(0x7fc00000)
	CanonicalFloat64NaN = uint64(0x7ff8000000000000)
)

// canonicalizeNaN32 returns the canonical NaN if f is NaN, otherwise returns f unchanged.
func canonicalizeNaN32(f float32) float32 {
	if math.IsNaN(float64(f)) {
		return math.Float32frombits(CanonicalFloat32NaN)
	}
	return f
}

// canonicalizeNaN64 returns the canonical NaN if f is NaN, otherwise returns f unchanged.
func canonicalizeNaN64(f float64) float64 {
	if math.IsNaN(f) {
		return math.Float64frombits(CanonicalFloat64NaN)
	}
	return f
}

// StringEncoding specifies the string encoding for Canonical ABI.
type StringEncoding uint8

const (
	StringEncodingUTF8 StringEncoding = iota
	StringEncodingUTF16
	StringEncodingLatin1UTF16
)

// Options holds Canonical ABI options from canonical definitions.
type Options struct {
	StringEncoding StringEncoding
	MemoryIdx      uint32
	ReallocIdx     *uint32
	PostReturnIdx  *uint32
}

// Memory interface for reading/writing linear memory.
type Memory interface {
	Read(offset, size uint32) ([]byte, bool)
	Write(offset uint32, data []byte) bool
	Size() uint32
}

// LiftContext provides context for lifting operations.
type LiftContext struct {
	Memory        Memory
	Opts          *Options
	ResourceTable *component.ResourceTable
	BorrowScope   *component.BorrowScope
}

// ReadU8 reads a u8 from memory at the given offset with bounds checking.
func (c *LiftContext) ReadU8(offset uint32) (uint8, error) {
	if c.Memory == nil {
		return 0, fmt.Errorf("no memory available")
	}
	data, ok := c.Memory.Read(offset, 1)
	if !ok {
		return 0, fmt.Errorf("memory read out of bounds: offset=%d", offset)
	}
	return data[0], nil
}

// ReadU16 reads a u16 from memory at the given offset with bounds checking.
func (c *LiftContext) ReadU16(offset uint32) (uint16, error) {
	if c.Memory == nil {
		return 0, fmt.Errorf("no memory available")
	}
	data, ok := c.Memory.Read(offset, 2)
	if !ok {
		return 0, fmt.Errorf("memory read out of bounds: offset=%d", offset)
	}
	return binary.LittleEndian.Uint16(data), nil
}

// ReadU32 reads a u32 from memory at the given offset with bounds checking.
func (c *LiftContext) ReadU32(offset uint32) (uint32, error) {
	if c.Memory == nil {
		return 0, fmt.Errorf("no memory available")
	}
	data, ok := c.Memory.Read(offset, 4)
	if !ok {
		return 0, fmt.Errorf("memory read out of bounds: offset=%d", offset)
	}
	return binary.LittleEndian.Uint32(data), nil
}

// ReadU64 reads a u64 from memory at the given offset with bounds checking.
func (c *LiftContext) ReadU64(offset uint32) (uint64, error) {
	if c.Memory == nil {
		return 0, fmt.Errorf("no memory available")
	}
	data, ok := c.Memory.Read(offset, 8)
	if !ok {
		return 0, fmt.Errorf("memory read out of bounds: offset=%d", offset)
	}
	return binary.LittleEndian.Uint64(data), nil
}

// ReadF32 reads a f32 from memory at the given offset with bounds checking.
func (c *LiftContext) ReadF32(offset uint32) (float32, error) {
	bits, err := c.ReadU32(offset)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(bits), nil
}

// ReadF64 reads a f64 from memory at the given offset with bounds checking.
func (c *LiftContext) ReadF64(offset uint32) (float64, error) {
	bits, err := c.ReadU64(offset)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

// ReadBytes reads a byte slice from memory at the given offset with bounds checking.
func (c *LiftContext) ReadBytes(offset, length uint32) ([]byte, error) {
	if c.Memory == nil {
		return nil, fmt.Errorf("no memory available")
	}
	data, ok := c.Memory.Read(offset, length)
	if !ok {
		return nil, fmt.Errorf("memory read out of bounds: offset=%d, length=%d", offset, length)
	}
	return data, nil
}

// LowerContext provides context for lowering operations.
// For primitive types, the context is not used, but it will be required
// for composite types (strings, lists, records) that need memory allocation.
type LowerContext struct {
	Memory        Memory
	Opts          *Options
	Realloc       func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
	ResourceTable *component.ResourceTable
	CallContext   *component.CallContext
}

// writeUint8 writes a uint8 to memory at the given offset.
func writeUint8(m Memory, offset uint32, val uint8) {
	m.Write(offset, []byte{val})
}

// writeUint16Le writes a uint16 to memory at the given offset in little-endian order.
func writeUint16Le(m Memory, offset uint32, val uint16) {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, val)
	m.Write(offset, buf)
}

// writeUint32Le writes a uint32 to memory at the given offset in little-endian order.
func writeUint32Le(m Memory, offset uint32, val uint32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, val)
	m.Write(offset, buf)
}

// writeUint64Le writes a uint64 to memory at the given offset in little-endian order.
func writeUint64Le(m Memory, offset uint32, val uint64) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, val)
	m.Write(offset, buf)
}
