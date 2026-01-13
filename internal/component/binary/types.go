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

// TypeDefKind identifies the kind of type definition decoded from binary format.
type TypeDefKind uint8

const (
	TypeDefKindFunc TypeDefKind = iota
	TypeDefKindComponent
	TypeDefKindInstance
	TypeDefKindResource
	TypeDefKindRecord
	TypeDefKindVariant
	TypeDefKindList
	TypeDefKindTuple
	TypeDefKindFlags
	TypeDefKindEnum
	TypeDefKindOption
	TypeDefKindResult
)

// TypeDef represents a decoded type definition from binary format.
// This is a discriminated union of different type kinds.
type TypeDef struct {
	Kind TypeDefKind

	// For FuncType
	Func *component.FuncType

	// For composite types
	Record  *RecordTypeDef
	Variant *VariantTypeDef
	List    *ListTypeDef
	Tuple   *TupleTypeDef
	Flags   *FlagsTypeDef
	Enum    *EnumTypeDef
	Option  *OptionTypeDef
	Result  *ResultTypeDef
}

// RecordTypeDef represents a decoded record type definition.
type RecordTypeDef struct {
	Fields []RecordField
}

// RecordField represents a field in a record type.
type RecordField struct {
	Name string
	Type component.ValTypeRef
}

// VariantTypeDef represents a decoded variant type definition.
type VariantTypeDef struct {
	Cases []VariantCase
}

// VariantCase represents a case in a variant type.
type VariantCase struct {
	Name    string
	Type    *component.ValTypeRef // nil for cases with no payload
	Refines *uint32               // optional index of refined case
}

// ListTypeDef represents a decoded list type definition.
type ListTypeDef struct {
	Element component.ValTypeRef
}

// TupleTypeDef represents a decoded tuple type definition.
type TupleTypeDef struct {
	Types []component.ValTypeRef
}

// FlagsTypeDef represents a decoded flags type definition.
type FlagsTypeDef struct {
	Names []string
}

// EnumTypeDef represents a decoded enum type definition.
type EnumTypeDef struct {
	Cases []string
}

// OptionTypeDef represents a decoded option type definition.
type OptionTypeDef struct {
	Some component.ValTypeRef
}

// ResultTypeDef represents a decoded result type definition.
type ResultTypeDef struct {
	Ok    *component.ValTypeRef // nil for result<_, E>
	Error *component.ValTypeRef // nil for result<T, _>
}

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

// decodeDefinedType reads a defined type from the reader.
// Defined types include composite types (record, variant, list, etc.)
// Format: <opcode> <type-specific-data>
func decodeDefinedType(r *bytes.Reader) (*TypeDef, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch opcode {
	case ValTypeOpcodeRecord:
		record, err := decodeRecordTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode record type: %w", err)
		}
		return &TypeDef{
			Kind:   TypeDefKindRecord,
			Record: record,
		}, nil

	case ValTypeOpcodeVariant:
		return nil, fmt.Errorf("variant type decoding not yet implemented")

	case ValTypeOpcodeList:
		return nil, fmt.Errorf("list type decoding not yet implemented")

	case ValTypeOpcodeTuple:
		return nil, fmt.Errorf("tuple type decoding not yet implemented")

	case ValTypeOpcodeFlags:
		return nil, fmt.Errorf("flags type decoding not yet implemented")

	case ValTypeOpcodeEnum:
		return nil, fmt.Errorf("enum type decoding not yet implemented")

	case ValTypeOpcodeOption:
		return nil, fmt.Errorf("option type decoding not yet implemented")

	case ValTypeOpcodeResult:
		return nil, fmt.Errorf("result type decoding not yet implemented")

	default:
		return nil, fmt.Errorf("unknown defined type opcode: 0x%02x", opcode)
	}
}

// decodeRecordTypeDef reads a record type definition from the reader.
// Format: 0x72 <field_count> (<name> <type>)*
func decodeRecordTypeDef(r *bytes.Reader) (*RecordTypeDef, error) {
	fieldCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read field count: %w", err)
	}

	fields := make([]RecordField, fieldCount)
	for i := uint32(0); i < fieldCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return nil, fmt.Errorf("read field %d name: %w", i, err)
		}

		valType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read field %d type: %w", i, err)
		}

		fields[i] = RecordField{
			Name: name,
			Type: valType,
		}
	}

	return &RecordTypeDef{
		Fields: fields,
	}, nil
}
