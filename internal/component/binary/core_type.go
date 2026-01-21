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
		case 0x00:
			// Disambiguation prefix for non-final sub type (GC proposal rec group).
			// When 0x00 is followed by 0x50, it indicates a rec group (not module type).
			// This is required by the component model spec to disambiguate 0x50 (module type)
			// from 0x50 (non-final sub type in GC proposal).
			nextByte, err := r.ReadByte()
			if err != nil {
				return fmt.Errorf("read core type %d next byte after 0x00: %w", i, err)
			}
			if nextByte != 0x50 {
				return fmt.Errorf("invalid core type %d: expected 0x50 after 0x00 prefix, got 0x%02x", i, nextByte)
			}
			// Parse as rec group (GC proposal type)
			recGroup, err := decodeCoreRecGroup(r)
			if err != nil {
				return fmt.Errorf("decode core rec group type %d: %w", i, err)
			}
			c.CoreTypes[i] = component.CoreTypeDef{
				Kind:     component.CoreTypeDefKindRecGroup,
				RecGroup: recGroup,
			}
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

// decodeCoreRecGroup decodes a GC proposal rec group type.
// This is called after the 0x00 0x50 prefix has been consumed.
// Format: vec(subtype)
// subtype ::= 0x50 vec(typeidx) comptype  (non-final subtype)
//
//	| 0x4f vec(typeidx) comptype  (final subtype)
//	| comptype                     (shorthand for final with no supertypes)
//
// comptype ::= 0x60 functype | 0x5f structtype | 0x5e arraytype
func decodeCoreRecGroup(r *bytes.Reader) (*component.CoreRecGroupTypeDef, error) {
	typeCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read rec group type count: %w", err)
	}

	recGroup := &component.CoreRecGroupTypeDef{
		Types: make([]component.CoreSubType, typeCount),
	}

	for i := uint32(0); i < typeCount; i++ {
		subType, err := decodeCoreSubType(r)
		if err != nil {
			return nil, fmt.Errorf("decode sub type %d: %w", i, err)
		}
		recGroup.Types[i] = *subType
	}

	return recGroup, nil
}

// decodeCoreSubType decodes a GC proposal sub type.
func decodeCoreSubType(r *bytes.Reader) (*component.CoreSubType, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read sub type opcode: %w", err)
	}

	subType := &component.CoreSubType{}

	switch opcode {
	case 0x50: // Non-final subtype
		subType.IsFinal = false
		// Read super type indices
		superCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read super type count: %w", err)
		}
		subType.SuperTypeIndices = make([]uint32, superCount)
		for i := uint32(0); i < superCount; i++ {
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read super type index %d: %w", i, err)
			}
			subType.SuperTypeIndices[i] = idx
		}
		// Read composite type
		compType, err := decodeCoreCompositeType(r)
		if err != nil {
			return nil, fmt.Errorf("decode composite type: %w", err)
		}
		subType.CompositeType = *compType

	case 0x4f: // Final subtype
		subType.IsFinal = true
		// Read super type indices
		superCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read super type count: %w", err)
		}
		subType.SuperTypeIndices = make([]uint32, superCount)
		for i := uint32(0); i < superCount; i++ {
			idx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read super type index %d: %w", i, err)
			}
			subType.SuperTypeIndices[i] = idx
		}
		// Read composite type
		compType, err := decodeCoreCompositeType(r)
		if err != nil {
			return nil, fmt.Errorf("decode composite type: %w", err)
		}
		subType.CompositeType = *compType

	case 0x60, 0x5f, 0x5e: // Shorthand: final subtype with no supertypes
		// Unread the opcode so composite type can read it
		if err := r.UnreadByte(); err != nil {
			return nil, fmt.Errorf("unread byte: %w", err)
		}
		subType.IsFinal = true
		subType.SuperTypeIndices = nil
		compType, err := decodeCoreCompositeType(r)
		if err != nil {
			return nil, fmt.Errorf("decode composite type: %w", err)
		}
		subType.CompositeType = *compType

	default:
		return nil, fmt.Errorf("invalid sub type opcode: 0x%02x", opcode)
	}

	return subType, nil
}

// decodeCoreCompositeType decodes a GC proposal composite type.
func decodeCoreCompositeType(r *bytes.Reader) (*component.CoreCompositeType, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read composite type opcode: %w", err)
	}

	compType := &component.CoreCompositeType{}

	switch opcode {
	case 0x60: // func type
		compType.Kind = component.CoreCompositeTypeKindFunc
		funcType, err := decodeCoreFunc(r)
		if err != nil {
			return nil, fmt.Errorf("decode func type: %w", err)
		}
		compType.Func = funcType

	case 0x5f: // struct type
		compType.Kind = component.CoreCompositeTypeKindStruct
		// Skip struct fields for now (full GC support not required)
		fieldCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read struct field count: %w", err)
		}
		for i := uint32(0); i < fieldCount; i++ {
			// Each field: storagetype mutability
			// storagetype is a valtype (can be packed: i8=0x78, i16=0x77)
			if _, err := r.ReadByte(); err != nil {
				return nil, fmt.Errorf("read struct field %d storage type: %w", i, err)
			}
			if _, err := r.ReadByte(); err != nil {
				return nil, fmt.Errorf("read struct field %d mutability: %w", i, err)
			}
		}

	case 0x5e: // array type
		compType.Kind = component.CoreCompositeTypeKindArray
		// Skip array element type for now (full GC support not required)
		// arraytype: storagetype mutability
		if _, err := r.ReadByte(); err != nil {
			return nil, fmt.Errorf("read array storage type: %w", err)
		}
		if _, err := r.ReadByte(); err != nil {
			return nil, fmt.Errorf("read array mutability: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid composite type opcode: 0x%02x", opcode)
	}

	return compType, nil
}
