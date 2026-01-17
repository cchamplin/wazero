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

// decodeCoreModuleType decodes a core module type.
// Format: vec(moduletypedecl)
// moduletypedecl ::= 0x00 import              (import)
//
//	| 0x01 core:type           (type)
//	| 0x02 alias               (outer alias)
//	| 0x03 export              (export)
func decodeCoreModuleType(r *bytes.Reader) (*component.CoreModuleTypeDef, error) {
	declCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read declaration count: %w", err)
	}

	moduleType := &component.CoreModuleTypeDef{}

	for i := uint32(0); i < declCount; i++ {
		declKind, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read declaration %d kind: %w", i, err)
		}

		switch declKind {
		case 0x00: // import
			moduleName, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read import %d module name: %w", i, err)
			}
			importName, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read import %d name: %w", i, err)
			}
			kind, err := r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("read import %d kind: %w", i, err)
			}
			// Skip type index based on kind
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return nil, fmt.Errorf("read import %d type index: %w", i, err)
			}
			moduleType.Imports = append(moduleType.Imports, component.CoreImportType{
				Module: moduleName,
				Name:   importName,
				Kind:   kind,
			})

		case 0x01: // type
			// Nested core type - read opcode and decode accordingly
			opcode, err := r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("read type declaration %d opcode: %w", i, err)
			}
			switch opcode {
			case 0x60: // func type
				if _, err := decodeCoreFunc(r); err != nil {
					return nil, fmt.Errorf("read type declaration %d func: %w", i, err)
				}
			default:
				return nil, fmt.Errorf("unsupported core type opcode in module type: 0x%02x", opcode)
			}

		case 0x02: // outer alias
			// Skip outer alias
			if _, _, err := leb128.DecodeUint32(r); err != nil { // count
				return nil, fmt.Errorf("read alias %d count: %w", i, err)
			}
			if _, _, err := leb128.DecodeUint32(r); err != nil { // index
				return nil, fmt.Errorf("read alias %d index: %w", i, err)
			}

		case 0x03: // export
			exportName, err := decodeName(r)
			if err != nil {
				return nil, fmt.Errorf("read export %d name: %w", i, err)
			}
			kind, err := r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("read export %d kind: %w", i, err)
			}
			// Skip type index
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return nil, fmt.Errorf("read export %d type index: %w", i, err)
			}
			moduleType.Exports = append(moduleType.Exports, component.CoreExportType{
				Name: exportName,
				Kind: kind,
			})

		default:
			return nil, fmt.Errorf("unknown module type declaration kind: 0x%02x", declKind)
		}
	}

	return moduleType, nil
}
