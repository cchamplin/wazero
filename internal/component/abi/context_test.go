package abi

import (
	"encoding/binary"
	"testing"

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
	val := ctx.ReadU32(8)
	require.Equal(t, uint32(42), val)
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
