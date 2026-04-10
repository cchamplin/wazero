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
			// Disambiguation prefix: 0x00 0x50 vec(typeidx) comptype
			// This is a disambiguated non-final sub type (the 0x50 non-final
			// sub opcode is prefixed with 0x00 to avoid ambiguity with
			// core:moduletype which also starts with 0x50).
			nextByte, err := r.ReadByte()
			if err != nil {
				return fmt.Errorf("read core type %d next byte after 0x00: %w", i, err)
			}
			if nextByte != 0x50 {
				return fmt.Errorf("invalid core type %d: expected 0x50 after 0x00 prefix, got 0x%02x", i, nextByte)
			}
			// Read super type indices and composite type
			subType := &component.CoreSubType{IsFinal: false}
			superCount, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("read core type %d super type count: %w", i, err)
			}
			subType.SuperTypeIndices = make([]uint32, superCount)
			for j := uint32(0); j < superCount; j++ {
				idx, _, err := leb128.DecodeUint32(r)
				if err != nil {
					return fmt.Errorf("read core type %d super type index %d: %w", i, j, err)
				}
				subType.SuperTypeIndices[j] = idx
			}
			compType, err := decodeCoreCompositeType(r)
			if err != nil {
				return fmt.Errorf("decode core type %d composite type: %w", i, err)
			}
			subType.CompositeType = *compType
			c.CoreTypes[i] = component.CoreTypeDef{
				Kind: component.CoreTypeDefKindRecGroup,
				RecGroup: &component.CoreRecGroupTypeDef{
					Types: []component.CoreSubType{*subType},
				},
			}
		case 0x4e:
			// Rec group: 0x4e vec(subtype)
			recGroup, err := decodeCoreRecGroup(r)
			if err != nil {
				return fmt.Errorf("decode core rec group type %d: %w", i, err)
			}
			c.CoreTypes[i] = component.CoreTypeDef{
				Kind:     component.CoreTypeDefKindRecGroup,
				RecGroup: recGroup,
			}
		case 0x4f:
			// Final sub type: 0x4f vec(typeidx) comptype
			subType := &component.CoreSubType{IsFinal: true}
			superCount, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return fmt.Errorf("read core type %d super type count: %w", i, err)
			}
			subType.SuperTypeIndices = make([]uint32, superCount)
			for j := uint32(0); j < superCount; j++ {
				idx, _, err := leb128.DecodeUint32(r)
				if err != nil {
					return fmt.Errorf("read core type %d super type index %d: %w", i, j, err)
				}
				subType.SuperTypeIndices[j] = idx
			}
			compType, err := decodeCoreCompositeType(r)
			if err != nil {
				return fmt.Errorf("decode core type %d composite type: %w", i, err)
			}
			subType.CompositeType = *compType
			c.CoreTypes[i] = component.CoreTypeDef{
				Kind: component.CoreTypeDefKindRecGroup,
				RecGroup: &component.CoreRecGroupTypeDef{
					Types: []component.CoreSubType{*subType},
				},
			}
		case 0x60, 0x5f, 0x5e: // bare composite types (func, struct, array)
			if err := r.UnreadByte(); err != nil {
				return fmt.Errorf("unread core type %d: %w", i, err)
			}
			compType, err := decodeCoreCompositeType(r)
			if err != nil {
				return fmt.Errorf("decode core type %d: %w", i, err)
			}
			// Wrap as a shorthand final sub type
			if compType.Kind == component.CoreCompositeTypeKindFunc {
				c.CoreTypes[i] = component.CoreTypeDef{
					Kind: component.CoreTypeDefKindFunc,
					Func: compType.Func,
				}
			} else {
				c.CoreTypes[i] = component.CoreTypeDef{
					Kind: component.CoreTypeDefKindRecGroup,
					RecGroup: &component.CoreRecGroupTypeDef{
						Types: []component.CoreSubType{{
							IsFinal:       true,
							CompositeType: *compType,
						}},
					},
				}
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
			// Decode import descriptor based on kind
			switch kind {
			case 0x00: // func: typeidx
				if _, _, err := leb128.DecodeUint32(r); err != nil {
					return nil, fmt.Errorf("read import %d func type index: %w", i, err)
				}
			case 0x01: // table: reftype limits
				if _, err := r.ReadByte(); err != nil { // reftype
					return nil, fmt.Errorf("read import %d table reftype: %w", i, err)
				}
				if err := skipCoreLimits(r); err != nil {
					return nil, fmt.Errorf("read import %d table limits: %w", i, err)
				}
			case 0x02: // memory: limits
				if err := skipCoreLimits(r); err != nil {
					return nil, fmt.Errorf("read import %d memory limits: %w", i, err)
				}
			case 0x03: // global: valtype mut
				if _, err := r.ReadByte(); err != nil { // valtype
					return nil, fmt.Errorf("read import %d global valtype: %w", i, err)
				}
				if _, err := r.ReadByte(); err != nil { // mutability
					return nil, fmt.Errorf("read import %d global mutability: %w", i, err)
				}
			case 0x04: // tag: attribute typeidx
				if _, err := r.ReadByte(); err != nil { // attribute byte (always 0x00)
					return nil, fmt.Errorf("read import %d tag attribute: %w", i, err)
				}
				if _, _, err := leb128.DecodeUint32(r); err != nil {
					return nil, fmt.Errorf("read import %d tag type index: %w", i, err)
				}
			default:
				return nil, fmt.Errorf("unknown import %d kind: 0x%02x", i, kind)
			}
			moduleType.Imports = append(moduleType.Imports, component.CoreImportType{
				Module: moduleName,
				Name:   importName,
				Kind:   kind,
			})

		case 0x01: // type
			// Nested core type within module type.
			// This follows the core:deftype encoding minus module type:
			//   core:rectype (0x4e for rec, or bare subtype starting with
			//     0x4f/0x60/0x5f/0x5e)
			//   0x00 0x50 vec(typeidx) comptype  (disambiguated non-final sub)
			if err := skipCoreDefType(r); err != nil {
				return nil, fmt.Errorf("read type declaration %d: %w", i, err)
			}

		case 0x02: // outer alias
			// Format: core:alias = sort aliastarget
			// core:aliastarget = 0x01 count index
			if _, err := r.ReadByte(); err != nil { // core sort byte
				return nil, fmt.Errorf("read alias %d sort: %w", i, err)
			}
			targetByte, err := r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("read alias %d target byte: %w", i, err)
			}
			if targetByte != 0x01 {
				return nil, fmt.Errorf("invalid alias %d: expected target 0x01 (outer), got 0x%02x", i, targetByte)
			}
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
			// Decode export descriptor based on kind (same as import descriptor)
			switch kind {
			case 0x00: // func: typeidx
				if _, _, err := leb128.DecodeUint32(r); err != nil {
					return nil, fmt.Errorf("read export %d func type index: %w", i, err)
				}
			case 0x01: // table: reftype limits
				if _, err := r.ReadByte(); err != nil { // reftype
					return nil, fmt.Errorf("read export %d table reftype: %w", i, err)
				}
				if err := skipCoreLimits(r); err != nil {
					return nil, fmt.Errorf("read export %d table limits: %w", i, err)
				}
			case 0x02: // memory: limits
				if err := skipCoreLimits(r); err != nil {
					return nil, fmt.Errorf("read export %d memory limits: %w", i, err)
				}
			case 0x03: // global: valtype mut
				if _, err := r.ReadByte(); err != nil { // valtype
					return nil, fmt.Errorf("read export %d global valtype: %w", i, err)
				}
				if _, err := r.ReadByte(); err != nil { // mutability
					return nil, fmt.Errorf("read export %d global mutability: %w", i, err)
				}
			case 0x04: // tag: attribute typeidx
				if _, err := r.ReadByte(); err != nil { // attribute byte
					return nil, fmt.Errorf("read export %d tag attribute: %w", i, err)
				}
				if _, _, err := leb128.DecodeUint32(r); err != nil {
					return nil, fmt.Errorf("read export %d tag type index: %w", i, err)
				}
			default:
				return nil, fmt.Errorf("unknown export %d kind: 0x%02x", i, kind)
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

// skipCoreDefType consumes a core:deftype from the binary stream.
// In module type context, this excludes module type (0x50) since nested
// module types are not allowed.
//
// core:deftype ::= rt:<core:rectype>
//
//	| 0x00 0x50 x*:vec(<typeidx>) ct:<comptype>  (disambiguated non-final sub)
//
// core:rectype ::= 0x4e vec(subtype) | subtype
// subtype ::= 0x50 vec(typeidx) comptype | 0x4f vec(typeidx) comptype | comptype
// comptype ::= 0x60 functype | 0x5f structtype | 0x5e arraytype
func skipCoreDefType(r *bytes.Reader) error {
	opcode, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read deftype opcode: %w", err)
	}

	switch opcode {
	case 0x00:
		// Disambiguation prefix: 0x00 0x50 vec(typeidx) comptype
		nextByte, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read byte after 0x00: %w", err)
		}
		if nextByte != 0x50 {
			return fmt.Errorf("expected 0x50 after 0x00 prefix, got 0x%02x", nextByte)
		}
		// Read super type indices
		if err := skipVecU32(r); err != nil {
			return fmt.Errorf("skip super type indices: %w", err)
		}
		// Read composite type
		if _, err := decodeCoreCompositeType(r); err != nil {
			return fmt.Errorf("skip composite type: %w", err)
		}

	case 0x4e:
		// Rec group: 0x4e vec(subtype)
		count, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("read rec group count: %w", err)
		}
		for i := uint32(0); i < count; i++ {
			if _, err := decodeCoreSubType(r); err != nil {
				return fmt.Errorf("skip rec group sub type %d: %w", i, err)
			}
		}

	case 0x4f:
		// Final sub type: 0x4f vec(typeidx) comptype
		if err := skipVecU32(r); err != nil {
			return fmt.Errorf("skip super type indices: %w", err)
		}
		if _, err := decodeCoreCompositeType(r); err != nil {
			return fmt.Errorf("skip composite type: %w", err)
		}

	case 0x60, 0x5f, 0x5e:
		// Bare composite type (shorthand for final sub with no supers)
		if err := r.UnreadByte(); err != nil {
			return err
		}
		if _, err := decodeCoreCompositeType(r); err != nil {
			return fmt.Errorf("skip composite type: %w", err)
		}

	default:
		return fmt.Errorf("unsupported core deftype opcode: 0x%02x", opcode)
	}
	return nil
}

// skipVecU32 skips a vec of u32 values.
func skipVecU32(r *bytes.Reader) error {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return err
		}
	}
	return nil
}

// skipCoreLimits advances the reader past a core wasm limits value.
// Format: flag:byte min:u32/u64 [max:u32/u64]
// Flags: 0x00 = min only, 0x01 = min+max, 0x03 = min+max+shared,
//
//	0x04 = memory64 min only, 0x05 = memory64 min+max
func skipCoreLimits(r *bytes.Reader) error {
	flag, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read limits flag: %w", err)
	}
	isMemory64 := flag == 0x04 || flag == 0x05
	hasMax := flag == 0x01 || flag == 0x03 || flag == 0x05

	if isMemory64 {
		// memory64: min/max are u64 LEB128
		if _, _, err := leb128.DecodeInt64(r); err != nil {
			return fmt.Errorf("read limits min (u64): %w", err)
		}
		if hasMax {
			if _, _, err := leb128.DecodeInt64(r); err != nil {
				return fmt.Errorf("read limits max (u64): %w", err)
			}
		}
	} else {
		// Standard: min/max are u32 LEB128
		if _, _, err := leb128.DecodeUint32(r); err != nil {
			return fmt.Errorf("read limits min: %w", err)
		}
		if hasMax {
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return fmt.Errorf("read limits max: %w", err)
			}
		}
	}
	return nil
}
