package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/leb128"
)

func decodeCoreTypeSection(c *component.Component, r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("read core type count: %w", err)
	}

	c.CoreTypes = make([]component.CoreTypeDef, count)
	for i := uint32(0); i < count; i++ {
		opcode, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read core type %d opcode: %w", i, err)
		}

		switch opcode {
		case 0x60: // func type
			funcType, err := decodeCoreFunc(r)
			if err != nil {
				return fmt.Errorf("decode core func type %d: %w", i, err)
			}
			c.CoreTypes[i] = component.CoreTypeDef{
				Kind: component.CoreTypeDefKindFunc,
				Func: funcType,
			}
		case 0x50: // module type
			moduleType, err := decodeCoreModuleType(r)
			if err != nil {
				return fmt.Errorf("decode core module type %d: %w", i, err)
			}
			c.CoreTypes[i] = component.CoreTypeDef{
				Kind:   component.CoreTypeDefKindModule,
				Module: moduleType,
			}
		default:
			return fmt.Errorf("unsupported core type opcode 0x%02x", opcode)
		}
	}

	return nil
}

func decodeCoreFunc(r *bytes.Reader) (*component.CoreFuncTypeDef, error) {
	paramCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read param count: %w", err)
	}

	params := make([]byte, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		vt, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read param %d: %w", i, err)
		}
		params[i] = vt
	}

	resultCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read result count: %w", err)
	}

	results := make([]byte, resultCount)
	for i := uint32(0); i < resultCount; i++ {
		vt, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read result %d: %w", i, err)
		}
		results[i] = vt
	}

	return &component.CoreFuncTypeDef{
		Params:  params,
		Results: results,
	}, nil
}

func decodeCoreModuleType(r *bytes.Reader) (*component.CoreModuleTypeDef, error) {
	// Module type is a sequence of declarations - simplified for now
	return &component.CoreModuleTypeDef{}, nil
}
