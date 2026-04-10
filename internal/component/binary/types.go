// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package binary: type-section decoder.
//
// The decoder walks the binary type section one entry at a time and, for
// each value type, calls through to a *types.ComponentTypesBuilder which
// interns the entry and returns a *types.ValType. The decoder does not
// construct any intermediate per-kind *TypeDef structs — the builder is
// the single source of truth for composite-type shape and ABI.
//
// Scope-local index tracking is provided by typeScope: binary scope-local
// index N corresponds to scope.entries[N], tagged as either a value type
// or a resource declaration (or an instance/component type declaration
// once nested type support is complete). Lookup rules are in decodeValType.

package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Type definition opcodes (top-level type-section entries that are not
// ValType opcodes).
const (
	TypeOpFuncSync      byte = 0x40 // Sync function type
	TypeOpFuncAsync     byte = 0x43 // Async function type
	TypeOpComponent     byte = 0x41 // Component type
	TypeOpInstance      byte = 0x42 // Instance type
	TypeOpResourceSync  byte = 0x3f // Resource type (sync destructor)
	TypeOpResourceAsync byte = 0x3e // Resource type (async destructor)
)

// Result encoding for function-type results.
const (
	ResultSingle byte = 0x00 // Single result value
	ResultNamed  byte = 0x01 // Named results (or empty)
)

// ResourceTypeDef carries the raw resource-declaration metadata decoded
// from the binary. It lives on the binary package only: the decoder
// interns an Abstract TypeResourceTable entry on the builder and records
// the resulting ResourceTableIdx in the scope. The destructor /
// callback function indices remain in ResourceTypeDef until runtime
// linking consumes them (via ComponentLinker.bindResourceTypes).
type ResourceTypeDef struct {
	// Destructor is the function index of the destructor, or nil if none.
	Destructor *uint32
	// AsyncDestructor indicates if this resource has an async destructor
	// (decoded from the 0x3e opcode).
	AsyncDestructor bool
	// Callback is the function index of the async callback, or nil if none.
	Callback *uint32
	// ResourceTableIdx is the TypeResourceTable index in *types.ComponentTypes.
	ResourceTableIdx types.ResourceTableIdx
}

// scopeEntryKind discriminates between value-type and resource-declaration
// entries in the scope-local type slice. The binary format uses a single
// flat type-section index space for both kinds; the discriminator catches
// ill-formed inputs at decode time (e.g., own<5> where 5 is a record).
type scopeEntryKind uint8

const (
	scopeEntryValType  scopeEntryKind = iota // value type (record, variant, list, ...)
	scopeEntryResource                       // resource declaration
	// TODO: scopeEntryInstance, scopeEntryComponent for full nested type support
	scopeEntryOther // function / instance / component type — not addressable as a ValType
	scopeEntryAlias // unresolved type alias (export or outer) — may resolve to any kind;
	// own<>/borrow<> are allowed to reference these slots because the actual
	// kind can only be determined after cross-component resolution.
	// An abstract resource placeholder is interned at append time so that
	// own<>/borrow<> handle construction succeeds.
)

// scopeEntry is one entry in a typeScope, tagged by kind.
type scopeEntry struct {
	kind     scopeEntryKind
	valType  types.ValType          // valid iff kind == scopeEntryValType
	resource types.ResourceTableIdx // valid iff kind == scopeEntryResource
	imported bool                   // true when the resource was introduced by an import (not locally defined)
}

// typeScope tracks scope-local type indices during decode. Each scope is
// a flat []scopeEntry — binary scope-local index N corresponds to
// scope.entries[N]. parent chains up the scope hierarchy so `outer`
// aliases resolve across nested scopes.
type typeScope struct {
	entries []scopeEntry
	parent  *typeScope
}

func newTypeScope(parent *typeScope) *typeScope {
	return &typeScope{parent: parent}
}

func (s *typeScope) appendValType(vt types.ValType) {
	s.entries = append(s.entries, scopeEntry{
		kind:    scopeEntryValType,
		valType: vt,
	})
}

func (s *typeScope) appendResource(rtIdx types.ResourceTableIdx) {
	s.entries = append(s.entries, scopeEntry{
		kind:     scopeEntryResource,
		resource: rtIdx,
	})
}

// appendImportedResource records a resource type that was introduced by an
// import (e.g. `(import "x" (type $x (sub resource)))`). Imported resources
// are usable with own<>/borrow<>/resource.drop but NOT with resource.new or
// resource.rep — those operations require locally-defined resources.
// Spec: canonical-abi/definitions.py canon_resource_new / canon_resource_rep.
func (s *typeScope) appendImportedResource(rtIdx types.ResourceTableIdx) {
	s.entries = append(s.entries, scopeEntry{
		kind:     scopeEntryResource,
		resource: rtIdx,
		imported: true,
	})
}

// appendOther records a scope slot that is neither a value type nor a
// resource declaration (function types, instance/component type
// declarations). Referring to such a slot from a value-type position is
// rejected at lookup time.
func (s *typeScope) appendOther() {
	s.entries = append(s.entries, scopeEntry{
		kind: scopeEntryOther,
	})
}

// appendAlias records a scope slot for an unresolved type alias
// (export alias or outer alias). The actual kind of the aliased type
// is not known at decode time and requires cross-component resolution.
// An abstract resource placeholder is provided so that own<>/borrow<>
// handle construction can proceed.
func (s *typeScope) appendAlias(placeholder types.ResourceTableIdx) {
	s.entries = append(s.entries, scopeEntry{
		kind:     scopeEntryAlias,
		resource: placeholder,
	})
}

// decodeValType reads a valtype from the reader.
//
// The grammar has three forms:
//   - a primitive opcode in 0x73..0x7f (plus 0x64 for error-context)
//   - own<> (0x69) / borrow<> (0x68) followed by a LEB128 type index that
//     must point to a resource declaration in scope
//   - a LEB128 type index that must point to a value-type entry in scope
//
// All three forms are resolved against the supplied scope; the result is
// always an already-interned types.ValType.
func decodeValType(
	r *bytes.Reader,
	scope *typeScope,
	b *types.ComponentTypesBuilder,
) (types.ValType, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return types.ValType{}, err
	}

	// Primitive: direct mapping, no builder interaction.
	if IsPrimValType(opcode) {
		return primitiveOpcodeToValType(opcode, b)
	}

	// own<R>
	if opcode == ValTypeOpcodeOwn {
		resIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return types.ValType{}, fmt.Errorf("decode own handle type index: %w", err)
		}
		if int(resIdx) >= len(scope.entries) {
			return types.ValType{}, fmt.Errorf("own<> type index %d out of range", resIdx)
		}
		entry := scope.entries[resIdx]
		if entry.kind != scopeEntryResource && entry.kind != scopeEntryAlias {
			return types.ValType{}, fmt.Errorf("own<> references type index %d which is not a resource declaration", resIdx)
		}
		return b.InternOwnHandle(entry.resource), nil
	}

	// borrow<R>
	if opcode == ValTypeOpcodeBorrow {
		resIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return types.ValType{}, fmt.Errorf("decode borrow handle type index: %w", err)
		}
		if int(resIdx) >= len(scope.entries) {
			return types.ValType{}, fmt.Errorf("borrow<> type index %d out of range", resIdx)
		}
		entry := scope.entries[resIdx]
		if entry.kind != scopeEntryResource && entry.kind != scopeEntryAlias {
			return types.ValType{}, fmt.Errorf("borrow<> references type index %d which is not a resource declaration", resIdx)
		}
		return b.InternBorrowHandle(entry.resource), nil
	}

	// Type index reference: unread the byte so we can decode the full LEB128.
	if err := r.UnreadByte(); err != nil {
		return types.ValType{}, fmt.Errorf("unread byte: %w", err)
	}
	idx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("decode type index: %w", err)
	}
	if int(idx) >= len(scope.entries) {
		return types.ValType{}, fmt.Errorf("type index %d out of range", idx)
	}
	entry := scope.entries[idx]
	switch entry.kind {
	case scopeEntryValType:
		return entry.valType, nil
	case scopeEntryAlias:
		// Alias entries are placeholders that resolve at link time.
		// Return the alias's resource as an own-handle placeholder so
		// that the binary stream stays aligned. Full validation occurs
		// during instantiation when the alias is resolved.
		return b.InternOwnHandle(entry.resource), nil
	default:
		return types.ValType{}, fmt.Errorf("type index %d refers to a non-value-type declaration (kind %d)", idx, entry.kind)
	}
}

// primitiveOpcodeToValType converts a primitive-valtype opcode to the
// matching types.ValType. Error-context is handled via the builder
// because it carries a per-component table identity.
func primitiveOpcodeToValType(opcode byte, b *types.ComponentTypesBuilder) (types.ValType, error) {
	switch opcode {
	case byte(PrimValTypeBool):
		return types.Bool, nil
	case byte(PrimValTypeS8):
		return types.S8, nil
	case byte(PrimValTypeU8):
		return types.U8, nil
	case byte(PrimValTypeS16):
		return types.S16, nil
	case byte(PrimValTypeU16):
		return types.U16, nil
	case byte(PrimValTypeS32):
		return types.S32, nil
	case byte(PrimValTypeU32):
		return types.U32, nil
	case byte(PrimValTypeS64):
		return types.S64, nil
	case byte(PrimValTypeU64):
		return types.U64, nil
	case byte(PrimValTypeF32):
		return types.F32, nil
	case byte(PrimValTypeF64):
		return types.F64, nil
	case byte(PrimValTypeChar):
		return types.Char, nil
	case byte(PrimValTypeString):
		return types.String_, nil
	case byte(PrimValTypeErrorContext):
		return b.InternErrorContextTable(), nil
	default:
		return types.ValType{}, fmt.Errorf("unknown primitive valtype opcode: 0x%02x", opcode)
	}
}

// decodeDefinedType reads a defined (composite) type from the reader and
// returns its interned types.ValType. The opcode byte has already been
// consumed by a lookahead in the caller for the top-level dispatch
// variant; for nested use (inside decodeValType / scope definitions) the
// opcode is read inside this function. To keep the contract consistent
// here, callers that have already consumed the opcode must re-dispatch
// via the per-kind helpers directly — see decodeTypeSection.
func decodeDefinedType(
	r *bytes.Reader,
	scope *typeScope,
	b *types.ComponentTypesBuilder,
) (types.ValType, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return types.ValType{}, err
	}
	switch opcode {
	case ValTypeOpcodeRecord:
		return decodeRecord(r, scope, b)
	case ValTypeOpcodeVariant:
		return decodeVariant(r, scope, b)
	case ValTypeOpcodeList:
		return decodeList(r, scope, b)
	case ValTypeOpcodeFixedSizeList:
		return decodeFixedLengthList(r, scope, b)
	case ValTypeOpcodeTuple:
		return decodeTuple(r, scope, b)
	case ValTypeOpcodeFlags:
		return decodeFlags(r, b)
	case ValTypeOpcodeEnum:
		return decodeEnum(r, b)
	case ValTypeOpcodeOption:
		return decodeOption(r, scope, b)
	case ValTypeOpcodeResult:
		return decodeResult(r, scope, b)
	case ValTypeOpcodeStream:
		return decodeStream(r, scope, b)
	case ValTypeOpcodeFuture:
		return decodeFuture(r, scope, b)
	default:
		return types.ValType{}, fmt.Errorf("unknown defined type opcode: 0x%02x", opcode)
	}
}

// decodeRecord reads a record payload (the fields vector) and interns
// the result. The 0x72 opcode has already been consumed.
// Format: <field_count> (<name> <type>)*
func decodeRecord(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	fieldCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read field count: %w", err)
	}
	if fieldCount == 0 {
		return types.ValType{}, fmt.Errorf("record type must have at least 1 field")
	}
	fields := make([]types.RecordField, fieldCount)
	names := make([]string, fieldCount)
	for i := uint32(0); i < fieldCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read field %d name: %w", i, err)
		}
		vt, err := decodeValType(r, scope, b)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read field %d type: %w", i, err)
		}
		fields[i] = types.RecordField{Name: name, Type: vt}
		names[i] = name
	}
	if err := checkUniqueNames(names, "record field"); err != nil {
		return types.ValType{}, err
	}
	return b.InternRecord(fields), nil
}

// decodeVariant reads a variant payload and interns the result. The 0x71
// opcode has already been consumed.
// Format: <case_count> (<name> <type_flag> [<type>] <refines_flag> [<refines_idx>])*
func decodeVariant(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	caseCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read case count: %w", err)
	}
	if caseCount == 0 {
		return types.ValType{}, fmt.Errorf("variant type must have at least 1 case")
	}
	cases := make([]types.VariantCase, caseCount)
	names := make([]string, caseCount)
	for i := uint32(0); i < caseCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read case %d name: %w", i, err)
		}
		typeFlag, err := r.ReadByte()
		if err != nil {
			return types.ValType{}, fmt.Errorf("read case %d type flag: %w", i, err)
		}
		var payload types.ValType
		hasPayload := false
		switch typeFlag {
		case 0x00:
			// no payload
		case 0x01:
			payload, err = decodeValType(r, scope, b)
			if err != nil {
				return types.ValType{}, fmt.Errorf("read case %d type: %w", i, err)
			}
			hasPayload = true
		default:
			return types.ValType{}, fmt.Errorf("invalid type flag for case %d: 0x%02x", i, typeFlag)
		}
		// Read the refines flag and consume its index if present; we
		// ignore the refines relation at decode time since it does not
		// affect the structural type.
		refinesFlag, err := r.ReadByte()
		if err != nil {
			return types.ValType{}, fmt.Errorf("read case %d refines flag: %w", i, err)
		}
		switch refinesFlag {
		case 0x00:
			// no refines
		case 0x01:
			if _, _, err := leb128.DecodeUint32(r); err != nil {
				return types.ValType{}, fmt.Errorf("read case %d refines index: %w", i, err)
			}
		default:
			return types.ValType{}, fmt.Errorf("invalid refines flag for case %d: 0x%02x", i, refinesFlag)
		}
		cases[i] = types.VariantCase{
			Name:       name,
			Payload:    payload,
			HasPayload: hasPayload,
		}
		names[i] = name
	}
	if err := checkUniqueNames(names, "variant case"); err != nil {
		return types.ValType{}, err
	}
	return b.InternVariant(cases), nil
}

// decodeList reads a list payload and interns the result.
// Format: <element_type>
func decodeList(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	elem, err := decodeValType(r, scope, b)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read list element type: %w", err)
	}
	return b.InternList(elem), nil
}

// decodeFixedLengthList reads a fixed-size list payload and interns the
// result.
// Format: <element_type> <size>
func decodeFixedLengthList(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	elem, err := decodeValType(r, scope, b)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read fixed-size list element type: %w", err)
	}
	size, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read fixed-size list length: %w", err)
	}
	if size == 0 {
		return types.ValType{}, fmt.Errorf("fixed-size list must have length > 0")
	}
	return b.InternFixedLengthList(elem, size), nil
}

// decodeTuple reads a tuple payload and interns the result.
// Format: <count> <type>*
func decodeTuple(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read tuple element count: %w", err)
	}
	if count == 0 {
		return types.ValType{}, fmt.Errorf("tuple type must have at least 1 element")
	}
	elems := make([]types.ValType, count)
	for i := uint32(0); i < count; i++ {
		vt, err := decodeValType(r, scope, b)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read tuple element %d type: %w", i, err)
		}
		elems[i] = vt
	}
	return b.InternTuple(elems), nil
}

// decodeFlags reads a flags payload and interns the result.
// Format: <count> <name>*
func decodeFlags(r *bytes.Reader, b *types.ComponentTypesBuilder) (types.ValType, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read flags count: %w", err)
	}
	if count == 0 {
		return types.ValType{}, fmt.Errorf("flags type must have at least 1 flag")
	}
	if count > 32 {
		return types.ValType{}, fmt.Errorf("flags type must have at most 32 flags, got %d", count)
	}
	names := make([]string, count)
	for i := uint32(0); i < count; i++ {
		name, err := decodeName(r)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read flag %d name: %w", i, err)
		}
		names[i] = name
	}
	if err := checkUniqueNames(names, "flag"); err != nil {
		return types.ValType{}, err
	}
	return b.InternFlags(names), nil
}

// decodeEnum reads an enum payload and interns the result.
// Format: <count> <name>*
func decodeEnum(r *bytes.Reader, b *types.ComponentTypesBuilder) (types.ValType, error) {
	count, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read enum case count: %w", err)
	}
	if count == 0 {
		return types.ValType{}, fmt.Errorf("enum type must have at least 1 case")
	}
	names := make([]string, count)
	for i := uint32(0); i < count; i++ {
		name, err := decodeName(r)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read enum case %d name: %w", i, err)
		}
		names[i] = name
	}
	if err := checkUniqueNames(names, "enum case"); err != nil {
		return types.ValType{}, err
	}
	return b.InternEnum(names), nil
}

// decodeOption reads an option payload and interns the result.
// Format: <element_type>
func decodeOption(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	elem, err := decodeValType(r, scope, b)
	if err != nil {
		return types.ValType{}, fmt.Errorf("read option element type: %w", err)
	}
	return b.InternOption(elem), nil
}

// decodeResult reads a result payload and interns the result.
// Format: <ok_flag> [<ok_type>] <err_flag> [<err_type>]
func decodeResult(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	okFlag, err := r.ReadByte()
	if err != nil {
		return types.ValType{}, fmt.Errorf("read result ok flag: %w", err)
	}
	var okType types.ValType
	hasOK := false
	switch okFlag {
	case 0x00:
		// none
	case 0x01:
		okType, err = decodeValType(r, scope, b)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read result ok type: %w", err)
		}
		hasOK = true
	default:
		return types.ValType{}, fmt.Errorf("invalid result ok flag: 0x%02x", okFlag)
	}

	errFlag, err := r.ReadByte()
	if err != nil {
		return types.ValType{}, fmt.Errorf("read result err flag: %w", err)
	}
	var errType types.ValType
	hasErr := false
	switch errFlag {
	case 0x00:
		// none
	case 0x01:
		errType, err = decodeValType(r, scope, b)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read result err type: %w", err)
		}
		hasErr = true
	default:
		return types.ValType{}, fmt.Errorf("invalid result err flag: 0x%02x", errFlag)
	}

	return b.InternResult(okType, errType, hasOK, hasErr), nil
}

// decodeStream reads a stream payload and interns the result.
// Format: <has_element> [<element_type>]
//
// Note: Wasmtime's component-model-binary format historically encoded
// stream as <has_element> [element_type] <has_end> [end_type]; the
// current spec (definitions.py as of 2025) drops the end type. Wazero
// tracks the current spec.
func decodeStream(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	hasElem, err := r.ReadByte()
	if err != nil {
		return types.ValType{}, fmt.Errorf("read stream has-element flag: %w", err)
	}
	var elem types.ValType
	has := false
	switch hasElem {
	case 0x00:
		// none
	case 0x01:
		elem, err = decodeValType(r, scope, b)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read stream element type: %w", err)
		}
		has = true
	default:
		return types.ValType{}, fmt.Errorf("invalid stream has-element flag: 0x%02x", hasElem)
	}
	return b.InternStream(elem, has), nil
}

// decodeFuture reads a future payload and interns the result.
// Format: <has_payload> [<payload_type>]
func decodeFuture(r *bytes.Reader, scope *typeScope, b *types.ComponentTypesBuilder) (types.ValType, error) {
	hasPayload, err := r.ReadByte()
	if err != nil {
		return types.ValType{}, fmt.Errorf("read future has-payload flag: %w", err)
	}
	var elem types.ValType
	has := false
	switch hasPayload {
	case 0x00:
		// none
	case 0x01:
		elem, err = decodeValType(r, scope, b)
		if err != nil {
			return types.ValType{}, fmt.Errorf("read future payload type: %w", err)
		}
		has = true
	default:
		return types.ValType{}, fmt.Errorf("invalid future has-payload flag: 0x%02x", hasPayload)
	}
	return b.InternFuture(elem, has), nil
}

// decodeFuncType reads a component function type, including the leading
// opcode (0x40 sync or 0x43 async). It returns the builder FuncTypeIdx
// for the interned type plus the decoded parameter names.
//
// Format: 0x40 paramlist resultlist  (sync)
//
//	| 0x43 paramlist resultlist  (async)
//
// Both params and results are encoded as vectors of (name, valtype)
// pairs, except results may be a single unnamed valtype via tag 0x00.
func decodeFuncType(
	r *bytes.Reader,
	scope *typeScope,
	b *types.ComponentTypesBuilder,
) (types.FuncTypeIdx, error) {
	opcode, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if opcode != TypeOpFuncSync && opcode != TypeOpFuncAsync {
		return 0, fmt.Errorf("expected functype opcode 0x40 or 0x43, got 0x%02x", opcode)
	}
	async := opcode == TypeOpFuncAsync

	// Parse params: vec(labelvaltype).
	paramCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return 0, fmt.Errorf("read param count: %w", err)
	}
	paramNames := make([]string, paramCount)
	paramTypes := make([]types.ValType, paramCount)
	for i := uint32(0); i < paramCount; i++ {
		name, err := decodeName(r)
		if err != nil {
			return 0, fmt.Errorf("read param %d name: %w", i, err)
		}
		vt, err := decodeValType(r, scope, b)
		if err != nil {
			return 0, fmt.Errorf("read param %d type: %w", i, err)
		}
		paramNames[i] = name
		paramTypes[i] = vt
	}

	// Parse results.
	resultTag, err := r.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("read result tag: %w", err)
	}
	var resultTypes []types.ValType
	switch resultTag {
	case ResultSingle:
		vt, err := decodeValType(r, scope, b)
		if err != nil {
			return 0, fmt.Errorf("read single result type: %w", err)
		}
		resultTypes = []types.ValType{vt}
	case ResultNamed:
		resultCount, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return 0, fmt.Errorf("read result count: %w", err)
		}
		resultTypes = make([]types.ValType, resultCount)
		for i := uint32(0); i < resultCount; i++ {
			// Per the component-model binary spec the vector elements
			// are (name, valtype); the name is discarded for the
			// interned FuncType because wasmtime stores result names
			// positionally via the params-like signature. Wazero keeps
			// the result as an anonymous tuple to match.
			if _, err := decodeName(r); err != nil {
				return 0, fmt.Errorf("read result %d name: %w", i, err)
			}
			vt, err := decodeValType(r, scope, b)
			if err != nil {
				return 0, fmt.Errorf("read result %d type: %w", i, err)
			}
			resultTypes[i] = vt
		}
	default:
		return 0, fmt.Errorf("invalid result tag: 0x%02x", resultTag)
	}

	// Spec validation: function results cannot contain a borrow type.
	// This applies to direct borrow<> values and to borrow<> nested
	// within lists, options, records, tuples, variants, results, etc.
	for i, rt := range resultTypes {
		if b.ContainsBorrow(rt) {
			return 0, fmt.Errorf("function result %d cannot contain a `borrow` type", i)
		}
	}

	// The builder represents both params and results as tuple ValTypes.
	// A zero-length tuple is a valid "unit" signature on both sides; the
	// builder handles the empty-slice case directly.
	paramTuple := b.InternTuple(paramTypes)
	resultTuple := b.InternTuple(resultTypes)
	return b.InternFunc(async, paramNames, paramTuple, resultTuple), nil
}

// decodeResourceDecl reads a resource-declaration payload from the
// reader, interns an Abstract TypeResourceTable entry via the builder,
// and returns the resulting ResourceTypeDef carrying the decoded
// destructor / callback metadata plus the builder-assigned resource
// table index. The caller is responsible for appending the
// ResourceTableIdx to the current scope.
//
// For sync resources (0x3f): Format is 0x7f dtor_flag [dtor_idx]
// For async resources (0x3e): Format is 0x7f f:<funcidx> cb?:<funcidx>?
func decodeResourceDecl(
	r *bytes.Reader,
	_ *typeScope,
	b *types.ComponentTypesBuilder,
	isAsync bool,
) (*ResourceTypeDef, error) {
	repType, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read resource rep type: %w", err)
	}
	if repType != 0x7f {
		return nil, fmt.Errorf("unsupported resource rep type: 0x%02x (expected 0x7f for i32)", repType)
	}

	result := &ResourceTypeDef{AsyncDestructor: isAsync}

	if isAsync {
		dtorIdx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read async resource destructor index: %w", err)
		}
		result.Destructor = &dtorIdx
		callbackFlag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read async resource callback flag: %w", err)
		}
		switch callbackFlag {
		case 0x00:
			result.Callback = nil
		case 0x01:
			callbackIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read async resource callback index: %w", err)
			}
			result.Callback = &callbackIdx
		default:
			return nil, fmt.Errorf("invalid async resource callback flag: 0x%02x (expected 0x00 or 0x01)", callbackFlag)
		}
	} else {
		dtorFlag, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read resource destructor flag: %w", err)
		}
		switch dtorFlag {
		case 0x00:
			result.Destructor = nil
		case 0x01:
			dtorIdx, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read resource destructor index: %w", err)
			}
			result.Destructor = &dtorIdx
		default:
			return nil, fmt.Errorf("invalid resource destructor flag: 0x%02x (expected 0x00 or 0x01)", dtorFlag)
		}
	}

	result.ResourceTableIdx = b.InternAbstractResource()
	return result, nil
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

// checkUniqueNames validates that all names in the slice are unique. It
// returns an error identifying the first duplicate, including the
// supplied context (e.g., "record field").
func checkUniqueNames(names []string, context string) error {
	seen := make(map[string]bool, len(names))
	for i, name := range names {
		if seen[name] {
			return fmt.Errorf("duplicate %s name at index %d: %q", context, i, name)
		}
		seen[name] = true
	}
	return nil
}
