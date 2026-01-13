package abi

import (
	"encoding/binary"
	"math"
)

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
	Memory Memory
	Opts   *Options
}

// ReadU8 reads a u8 from memory at the given offset.
func (c *LiftContext) ReadU8(offset uint32) uint8 {
	data, _ := c.Memory.Read(offset, 1)
	return data[0]
}

// ReadU16 reads a u16 from memory at the given offset.
func (c *LiftContext) ReadU16(offset uint32) uint16 {
	data, _ := c.Memory.Read(offset, 2)
	return binary.LittleEndian.Uint16(data)
}

// ReadU32 reads a u32 from memory at the given offset.
func (c *LiftContext) ReadU32(offset uint32) uint32 {
	data, _ := c.Memory.Read(offset, 4)
	return binary.LittleEndian.Uint32(data)
}

// ReadU64 reads a u64 from memory at the given offset.
func (c *LiftContext) ReadU64(offset uint32) uint64 {
	data, _ := c.Memory.Read(offset, 8)
	return binary.LittleEndian.Uint64(data)
}

// ReadF32 reads a f32 from memory at the given offset.
func (c *LiftContext) ReadF32(offset uint32) float32 {
	bits := c.ReadU32(offset)
	return math.Float32frombits(bits)
}

// ReadF64 reads a f64 from memory at the given offset.
func (c *LiftContext) ReadF64(offset uint32) float64 {
	bits := c.ReadU64(offset)
	return math.Float64frombits(bits)
}
