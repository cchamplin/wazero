package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

func decodeValueSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read value count: %w", err)
	}

	c.Values = make([]component.ValueDef, count)
	for i := uint32(0); i < count; i++ {
		valType, err := decodeValType(r)
		if err != nil {
			return fmt.Errorf("read value %d type: %w", i, err)
		}

		// Read value bytes - simplified for primitives
		var data []byte
		if valType.IsPrimitive {
			switch valType.Primitive {
			case 0x7a, 0x79: // s32, u32
				val, _, err := leb128.DecodeUint32(r)
				if err != nil {
					return fmt.Errorf("read value %d: %w", i, err)
				}
				data = make([]byte, 4)
				data[0] = byte(val)
				data[1] = byte(val >> 8)
				data[2] = byte(val >> 16)
				data[3] = byte(val >> 24)
			default:
				b, err := r.ReadByte()
				if err != nil {
					return fmt.Errorf("read value %d: %w", i, err)
				}
				data = []byte{b}
			}
		}

		c.Values[i] = component.ValueDef{
			Type: valType,
			Data: data,
		}
	}

	return nil
}
