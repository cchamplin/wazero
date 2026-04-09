package abi

import (
	"encoding/binary"
	"math"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// byteMemory is a direct []byte-backed api.Memory for bounds-check tests that
// need sub-page memories. Unlike wazerotest.NewMemory, which rounds up to a
// full 64 KiB page, byteMemory keeps exactly the requested number of bytes.
type byteMemory struct {
	internalapi.WazeroOnlyType
	buf []byte
}

// newByteMemory returns a byteMemory of exactly n bytes.
func newByteMemory(n int) *byteMemory {
	return &byteMemory{buf: make([]byte, n)}
}

// byteMemoryDefinition satisfies api.MemoryDefinition for byteMemory.
type byteMemoryDefinition struct {
	internalapi.WazeroOnlyType
}

func (d byteMemoryDefinition) ModuleName() string                          { return "" }
func (d byteMemoryDefinition) Index() uint32                               { return 0 }
func (d byteMemoryDefinition) Import() (string, string, bool)              { return "", "", false }
func (d byteMemoryDefinition) ExportNames() []string                       { return nil }
func (d byteMemoryDefinition) Min() uint32                                 { return 0 }
func (d byteMemoryDefinition) Max() (uint32, bool)                         { return 0, false }

func (m *byteMemory) Definition() api.MemoryDefinition {
	return byteMemoryDefinition{}
}

func (m *byteMemory) Size() uint32 {
	return uint32(len(m.buf))
}

func (m *byteMemory) Grow(deltaPages uint32) (uint32, bool) {
	return 0, false
}

// inRange returns true if [offset, offset+length) is within m.buf.
func (m *byteMemory) inRange(offset, length uint32) bool {
	end := uint64(offset) + uint64(length)
	return end <= uint64(len(m.buf))
}

func (m *byteMemory) ReadByteAt(offset uint32) (byte, bool) {
	if !m.inRange(offset, 1) {
		return 0, false
	}
	return m.buf[offset], true
}

func (m *byteMemory) ReadUint16Le(offset uint32) (uint16, bool) {
	if !m.inRange(offset, 2) {
		return 0, false
	}
	return binary.LittleEndian.Uint16(m.buf[offset:]), true
}

func (m *byteMemory) ReadUint32Le(offset uint32) (uint32, bool) {
	if !m.inRange(offset, 4) {
		return 0, false
	}
	return binary.LittleEndian.Uint32(m.buf[offset:]), true
}

func (m *byteMemory) ReadFloat32Le(offset uint32) (float32, bool) {
	if !m.inRange(offset, 4) {
		return 0, false
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(m.buf[offset:])), true
}

func (m *byteMemory) ReadUint64Le(offset uint32) (uint64, bool) {
	if !m.inRange(offset, 8) {
		return 0, false
	}
	return binary.LittleEndian.Uint64(m.buf[offset:]), true
}

func (m *byteMemory) ReadFloat64Le(offset uint32) (float64, bool) {
	if !m.inRange(offset, 8) {
		return 0, false
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(m.buf[offset:])), true
}

func (m *byteMemory) Read(offset, byteCount uint32) ([]byte, bool) {
	if !m.inRange(offset, byteCount) {
		return nil, false
	}
	return m.buf[offset : offset+byteCount], true
}

func (m *byteMemory) WriteByteAt(offset uint32, v byte) bool {
	if !m.inRange(offset, 1) {
		return false
	}
	m.buf[offset] = v
	return true
}

func (m *byteMemory) WriteUint16Le(offset uint32, v uint16) bool {
	if !m.inRange(offset, 2) {
		return false
	}
	binary.LittleEndian.PutUint16(m.buf[offset:], v)
	return true
}

func (m *byteMemory) WriteUint32Le(offset, v uint32) bool {
	if !m.inRange(offset, 4) {
		return false
	}
	binary.LittleEndian.PutUint32(m.buf[offset:], v)
	return true
}

func (m *byteMemory) WriteFloat32Le(offset uint32, v float32) bool {
	if !m.inRange(offset, 4) {
		return false
	}
	binary.LittleEndian.PutUint32(m.buf[offset:], math.Float32bits(v))
	return true
}

func (m *byteMemory) WriteUint64Le(offset uint32, v uint64) bool {
	if !m.inRange(offset, 8) {
		return false
	}
	binary.LittleEndian.PutUint64(m.buf[offset:], v)
	return true
}

func (m *byteMemory) WriteFloat64Le(offset uint32, v float64) bool {
	if !m.inRange(offset, 8) {
		return false
	}
	binary.LittleEndian.PutUint64(m.buf[offset:], math.Float64bits(v))
	return true
}

func (m *byteMemory) Write(offset uint32, v []byte) bool {
	if !m.inRange(offset, uint32(len(v))) {
		return false
	}
	copy(m.buf[offset:], v)
	return true
}

func (m *byteMemory) WriteString(offset uint32, v string) bool {
	if !m.inRange(offset, uint32(len(v))) {
		return false
	}
	copy(m.buf[offset:], v)
	return true
}
