// internal/component/binary/types.go

package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Type definition opcodes
const (
	TypeOpFuncSync     byte = 0x40 // Sync function type
	TypeOpFuncAsync    byte = 0x43 // Async function type
	TypeOpComponent    byte = 0x41 // Component type
	TypeOpInstance     byte = 0x42 // Instance type
	TypeOpResourceSync byte = 0x3f // Resource type (sync destructor)
)

// Result encoding
const (
	ResultSingle byte = 0x00 // Single result value
	ResultNamed  byte = 0x01 // Named results (or empty)
)

// decodeValType reads a valtype from the reader.
// valtypes are either primitive opcodes (0x73-0x7f) or type indices (LEB128).
func decodeValType(r io.ByteReader) (component.ValTypeRef, error) {
	b, err := r.ReadByte()
	if err != nil {
		return component.ValTypeRef{}, err
	}

	// Check if it's a primitive type (negative SLEB128 range)
	if IsPrimValType(b) {
		return component.ValTypeRef{
			IsPrimitive: true,
			Primitive:   b,
		}, nil
	}

	// Otherwise, it's a type index (need to unread and decode as LEB128)
	// For now, assume single-byte indices (< 128)
	return component.ValTypeRef{
		IsPrimitive: false,
		TypeIdx:     uint32(b),
	}, nil
}

// decodeFuncType reads a component function type.
// Format: 0x40 paramlist resultlist (sync) or 0x43 paramlist resultlist (async)
func decodeFuncType(r *bytes.Reader) (*component.FuncType, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	if opcode != TypeOpFuncSync && opcode != TypeOpFuncAsync {
		return nil, fmt.Errorf("expected functype opcode 0x40 or 0x43, got 0x%02x", opcode)
	}

	ft := &component.FuncType{}

	// Parse params: vec(labelvaltype)
	paramCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read param count: %w", err)
	}

	ft.Params = make([]component.NamedValType, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return nil, fmt.Errorf("read param %d name: %w", i, err)
		}

		valType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read param %d type: %w", i, err)
		}

		ft.Params[i] = component.NamedValType{
			Name:    name,
			ValType: valType,
		}
	}

	// Parse results
	resultTag, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read result tag: %w", err)
	}

	switch resultTag {
	case ResultSingle:
		// Single unnamed result
		valType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read single result type: %w", err)
		}
		ft.Results = []component.NamedValType{{ValType: valType}}

	case ResultNamed:
		// Named results (vec) - count of 0 means no results
		resultCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read result count: %w", err)
		}

		ft.Results = make([]component.NamedValType, resultCount)
		for i := uint32(0); i < resultCount; i++ {
			name, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read result %d name: %w", i, err)
			}

			valType, err := decodeValType(r)
			if err != nil {
				return nil, fmt.Errorf("read result %d type: %w", i, err)
			}

			ft.Results[i] = component.NamedValType{
				Name:    name,
				ValType: valType,
			}
		}

	default:
		return nil, fmt.Errorf("invalid result tag: 0x%02x", resultTag)
	}

	return ft, nil
}

// decodeName reads a length-prefixed UTF-8 name.
func decodeName(r *bytes.Reader) (string, error) {
	length, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}
