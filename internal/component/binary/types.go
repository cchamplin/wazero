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
	TypeOpFuncSync      byte = 0x40 // Sync function type
	TypeOpFuncAsync     byte = 0x43 // Async function type
	TypeOpComponent     byte = 0x41 // Component type
	TypeOpInstance      byte = 0x42 // Instance type
	TypeOpResourceSync  byte = 0x3f // Resource type (sync destructor)
	TypeOpResourceAsync byte = 0x3e // Resource type (async destructor)
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

	// For resource types
	Resource *ResourceTypeDef
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

// ResourceTypeDef represents a decoded resource type definition.
// Resources have an optional destructor function that is called when the resource is dropped.
type ResourceTypeDef struct {
	// Destructor is the index of the destructor function, or nil if no destructor.
	Destructor *uint32
	// AsyncDestructor indicates if this resource has an async destructor (0x3e opcode).
	AsyncDestructor bool
	// Callback is the index of the callback function for async destructors, or nil if not specified.
	Callback *uint32
}

// decodeValType reads a valtype from the reader.
// valtypes are either primitive opcodes (0x73-0x7f), handle types (0x68 for borrow, 0x69 for own),
// or type indices (LEB128).
func decodeValType(r *bytes.Reader) (component.ValTypeRef, error) {
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

	// Check if it's a borrow handle type (0x68)
	if b == ValTypeOpcodeBorrow {
		typeIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return component.ValTypeRef{}, fmt.Errorf("decode borrow handle type index: %w", err)
		}
		return component.ValTypeRef{
			IsPrimitive: false,
			IsBorrow:    true,
			TypeIdx:     typeIdx,
		}, nil
	}

	// Check if it's an own handle type (0x69)
	if b == ValTypeOpcodeOwn {
		typeIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return component.ValTypeRef{}, fmt.Errorf("decode own handle type index: %w", err)
		}
		return component.ValTypeRef{
			IsPrimitive: false,
			IsOwn:       true,
			TypeIdx:     typeIdx,
		}, nil
	}

	// Otherwise, it's a type index encoded as LEB128 unsigned 32-bit integer.
	// We need to unread the byte we just read and decode the full LEB128 value.
	if err := r.UnreadByte(); err != nil {
		return component.ValTypeRef{}, fmt.Errorf("unread byte: %w", err)
	}

	typeIdx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return component.ValTypeRef{}, fmt.Errorf("decode type index: %w", err)
	}

	return component.ValTypeRef{
		IsPrimitive: false,
		TypeIdx:     typeIdx,
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
		variant, err := decodeVariantTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode variant type: %w", err)
		}
		return &TypeDef{
			Kind:    TypeDefKindVariant,
			Variant: variant,
		}, nil

	case ValTypeOpcodeList:
		list, err := decodeListTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode list type: %w", err)
		}
		return &TypeDef{
			Kind: TypeDefKindList,
			List: list,
		}, nil

	case ValTypeOpcodeTuple:
		tuple, err := decodeTupleTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode tuple type: %w", err)
		}
		return &TypeDef{
			Kind:  TypeDefKindTuple,
			Tuple: tuple,
		}, nil

	case ValTypeOpcodeFlags:
		flags, err := decodeFlagsTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode flags type: %w", err)
		}
		return &TypeDef{
			Kind:  TypeDefKindFlags,
			Flags: flags,
		}, nil

	case ValTypeOpcodeEnum:
		enum, err := decodeEnumTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode enum type: %w", err)
		}
		return &TypeDef{
			Kind: TypeDefKindEnum,
			Enum: enum,
		}, nil

	case ValTypeOpcodeOption:
		option, err := decodeOptionTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode option type: %w", err)
		}
		return &TypeDef{
			Kind:   TypeDefKindOption,
			Option: option,
		}, nil

	case ValTypeOpcodeResult:
		result, err := decodeResultTypeDef(r)
		if err != nil {
			return nil, fmt.Errorf("decode result type: %w", err)
		}
		return &TypeDef{
			Kind:   TypeDefKindResult,
			Result: result,
		}, nil

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

	if fieldCount == 0 {
		return nil, fmt.Errorf("record type must have at least 1 field")
	}

	fields := make([]RecordField, fieldCount)
	fieldNames := make([]string, fieldCount)
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
		fieldNames[i] = name
	}

	if err := checkUniqueNames(fieldNames, "record field"); err != nil {
		return nil, err
	}

	return &RecordTypeDef{
		Fields: fields,
	}, nil
}

// decodeListTypeDef reads a list type definition from the reader.
// Format: 0x70 <element_type>
func decodeListTypeDef(r *bytes.Reader) (*ListTypeDef, error) {
	elemType, err := decodeValType(r)
	if err != nil {
		return nil, fmt.Errorf("read list element type: %w", err)
	}

	return &ListTypeDef{
		Element: elemType,
	}, nil
}

// decodeVariantTypeDef reads a variant type definition from the reader.
// Format: 0x71 <case_count> (<name> <type_flag> [<type>] <refines_flag> [<refines_idx>])*
// Note: type_flag 0x00 = no type (discriminant only), 0x01 = has type
// Note: refines_flag 0x00 = no refines, 0x01 = has refines index
func decodeVariantTypeDef(r *bytes.Reader) (*VariantTypeDef, error) {
	caseCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read case count: %w", err)
	}

	if caseCount == 0 {
		return nil, fmt.Errorf("variant type must have at least 1 case")
	}

	cases := make([]VariantCase, caseCount)
	caseNames := make([]string, caseCount)
	for i := uint32(0); i < caseCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return nil, fmt.Errorf("read case %d name: %w", i, err)
		}

		// Read type flag (comes BEFORE refines per spec)
		typeFlag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read case %d type flag: %w", i, err)
		}

		var valTypeRef *component.ValTypeRef
		if typeFlag == 0x01 {
			vt, err := decodeValType(r)
			if err != nil {
				return nil, fmt.Errorf("read case %d type: %w", i, err)
			}
			valTypeRef = &vt
		} else if typeFlag != 0x00 {
			return nil, fmt.Errorf("invalid type flag for case %d: 0x%02x", i, typeFlag)
		}

		// Read refines flag (comes AFTER type per spec)
		refinesFlag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read case %d refines flag: %w", i, err)
		}

		var refines *uint32
		if refinesFlag == 0x01 {
			refinesIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read case %d refines index: %w", i, err)
			}
			refines = &refinesIdx
		} else if refinesFlag != 0x00 {
			return nil, fmt.Errorf("invalid refines flag for case %d: 0x%02x", i, refinesFlag)
		}

		cases[i] = VariantCase{
			Name:    name,
			Refines: refines,
			Type:    valTypeRef,
		}
		caseNames[i] = name
	}

	if err := checkUniqueNames(caseNames, "variant case"); err != nil {
		return nil, err
	}

	return &VariantTypeDef{
		Cases: cases,
	}, nil
}

// decodeTupleTypeDef reads a tuple type definition from the reader.
// Format: 0x6f <count> <type>*
func decodeTupleTypeDef(r *bytes.Reader) (*TupleTypeDef, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read tuple element count: %w", err)
	}

	if count == 0 {
		return nil, fmt.Errorf("tuple type must have at least 1 element")
	}

	types := make([]component.ValTypeRef, count)
	for i := uint32(0); i < count; i++ {
		valType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read tuple element %d type: %w", i, err)
		}
		types[i] = valType
	}

	return &TupleTypeDef{
		Types: types,
	}, nil
}

// decodeFlagsTypeDef reads a flags type definition from the reader.
// Format: 0x6e <count> <name>*
// Spec requires: 0 < count <= 32
func decodeFlagsTypeDef(r *bytes.Reader) (*FlagsTypeDef, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read flags count: %w", err)
	}

	if count == 0 {
		return nil, fmt.Errorf("flags type must have at least 1 flag")
	}

	if count > 32 {
		return nil, fmt.Errorf("flags type must have at most 32 flags, got %d", count)
	}

	names := make([]string, count)
	for i := uint32(0); i < count; i++ {
		name, err := decodeName(r)
		if err != nil {
			return nil, fmt.Errorf("read flag %d name: %w", i, err)
		}
		names[i] = name
	}

	if err := checkUniqueNames(names, "flag"); err != nil {
		return nil, err
	}

	return &FlagsTypeDef{
		Names: names,
	}, nil
}

// decodeEnumTypeDef reads an enum type definition from the reader.
// Format: 0x6d <count> <name>*
func decodeEnumTypeDef(r *bytes.Reader) (*EnumTypeDef, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read enum case count: %w", err)
	}

	if count == 0 {
		return nil, fmt.Errorf("enum type must have at least 1 case")
	}

	cases := make([]string, count)
	for i := uint32(0); i < count; i++ {
		name, err := decodeName(r)
		if err != nil {
			return nil, fmt.Errorf("read enum case %d name: %w", i, err)
		}
		cases[i] = name
	}

	if err := checkUniqueNames(cases, "enum case"); err != nil {
		return nil, err
	}

	return &EnumTypeDef{
		Cases: cases,
	}, nil
}

// decodeOptionTypeDef reads an option type definition from the reader.
// Format: 0x6b <type>
func decodeOptionTypeDef(r *bytes.Reader) (*OptionTypeDef, error) {
	someType, err := decodeValType(r)
	if err != nil {
		return nil, fmt.Errorf("read option some type: %w", err)
	}

	return &OptionTypeDef{Some: someType}, nil
}

// decodeResultTypeDef reads a result type definition from the reader.
// Format: 0x6a <ok_flag> [<ok_type>] <error_flag> [<error_type>]
// ok_flag: 0x00 = no ok type, 0x01 = has ok type
// error_flag: 0x00 = no error type, 0x01 = has error type
func decodeResultTypeDef(r *bytes.Reader) (*ResultTypeDef, error) {
	result := &ResultTypeDef{}

	// Read ok type flag
	okFlag, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read result ok flag: %w", err)
	}

	if okFlag == 0x01 {
		okType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read result ok type: %w", err)
		}
		result.Ok = &okType
	} else if okFlag != 0x00 {
		return nil, fmt.Errorf("invalid result ok flag: 0x%02x", okFlag)
	}

	// Read error type flag
	errorFlag, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read result error flag: %w", err)
	}

	if errorFlag == 0x01 {
		errorType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read result error type: %w", err)
		}
		result.Error = &errorType
	} else if errorFlag != 0x00 {
		return nil, fmt.Errorf("invalid result error flag: 0x%02x", errorFlag)
	}

	return result, nil
}

// decodeStreamTypeDef decodes a stream type definition.
// Format: 0x66 <has_element> [element_type] <has_end> [end_type]
func decodeStreamTypeDef(r *bytes.Reader) (*component.StreamTypeDef, error) {
	stream := &component.StreamTypeDef{}

	hasElement, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read has element: %w", err)
	}
	if hasElement == 0x01 {
		elemType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read element type: %w", err)
		}
		stream.ElementType = &elemType
	} else if hasElement != 0x00 {
		return nil, fmt.Errorf("invalid has element flag: 0x%02x", hasElement)
	}

	hasEnd, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read has end: %w", err)
	}
	if hasEnd == 0x01 {
		endType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read end type: %w", err)
		}
		stream.EndType = &endType
	} else if hasEnd != 0x00 {
		return nil, fmt.Errorf("invalid has end flag: 0x%02x", hasEnd)
	}

	return stream, nil
}

// decodeFutureTypeDef decodes a future type definition.
// Format: 0x65 <has_payload> [payload_type]
func decodeFutureTypeDef(r *bytes.Reader) (*component.FutureTypeDef, error) {
	future := &component.FutureTypeDef{}

	hasPayload, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read has payload: %w", err)
	}
	if hasPayload == 0x01 {
		payloadType, err := decodeValType(r)
		if err != nil {
			return nil, fmt.Errorf("read payload type: %w", err)
		}
		future.PayloadType = &payloadType
	} else if hasPayload != 0x00 {
		return nil, fmt.Errorf("invalid has payload flag: 0x%02x", hasPayload)
	}

	return future, nil
}

// decodeFixedSizeListTypeDef decodes a fixed-size list type definition.
// Format: 0x67 <element_type> <size>
// Spec requires: size > 0
func decodeFixedSizeListTypeDef(r *bytes.Reader) (*component.FixedSizeListTypeDef, error) {
	elemType, err := decodeValType(r)
	if err != nil {
		return nil, fmt.Errorf("read element type: %w", err)
	}

	size, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read size: %w", err)
	}

	if size == 0 {
		return nil, fmt.Errorf("fixed-size list must have length > 0")
	}

	return &component.FixedSizeListTypeDef{
		ElementType: elemType,
		Size:        size,
	}, nil
}

// decodeResourceTypeDef reads a resource type definition from the reader.
// For sync resources (0x3f): Format is 0x7f dtor_flag [dtor_idx]
// For async resources (0x3e): Format is 0x7f f:<funcidx> cb?:<funcidx>?
// The 0x7f byte indicates i32 representation (always required).
// For sync: dtor_flag 0x00 = no destructor, 0x01 = has destructor followed by funcidx.
// For async: destructor funcidx is required, followed by optional callback flag and index.
func decodeResourceTypeDef(r *bytes.Reader) (*ResourceTypeDef, error) {
	return decodeResourceTypeDefWithAsync(r, false)
}

// decodeResourceTypeDefWithAsync reads a resource type definition from the reader.
// The isAsync parameter indicates whether this is an async resource type (0x3e opcode).
func decodeResourceTypeDefWithAsync(r *bytes.Reader, isAsync bool) (*ResourceTypeDef, error) {
	// Read rep type (must be 0x7f for i32)
	repType, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read resource rep type: %w", err)
	}
	if repType != 0x7f {
		return nil, fmt.Errorf("unsupported resource rep type: 0x%02x (expected 0x7f for i32)", repType)
	}

	result := &ResourceTypeDef{
		AsyncDestructor: isAsync,
	}

	if isAsync {
		// Async resource: destructor funcidx is required
		dtorIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read async resource destructor index: %w", err)
		}
		result.Destructor = &dtorIdx

		// Optional callback flag
		callbackFlag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read async resource callback flag: %w", err)
		}

		switch callbackFlag {
		case 0x00:
			// No callback
			result.Callback = nil
		case 0x01:
			// Has callback, read function index
			callbackIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read async resource callback index: %w", err)
			}
			result.Callback = &callbackIdx
		default:
			return nil, fmt.Errorf("invalid async resource callback flag: 0x%02x (expected 0x00 or 0x01)", callbackFlag)
		}
	} else {
		// Sync resource: destructor is optional via flag
		dtorFlag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read resource destructor flag: %w", err)
		}

		switch dtorFlag {
		case 0x00:
			// No destructor
			result.Destructor = nil
		case 0x01:
			// Has destructor, read function index
			dtorIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read resource destructor index: %w", err)
			}
			result.Destructor = &dtorIdx
		default:
			return nil, fmt.Errorf("invalid resource destructor flag: 0x%02x (expected 0x00 or 0x01)", dtorFlag)
		}
	}

	return result, nil
}

// checkUniqueNames validates that all names in the slice are unique.
// It returns an error if a duplicate is found, including the context (e.g., "record field").
func checkUniqueNames(names []string, context string) error {
	seen := make(map[string]bool)
	for i, name := range names {
		if seen[name] {
			return fmt.Errorf("duplicate %s name at index %d: %q", context, i, name)
		}
		seen[name] = true
	}
	return nil
}
