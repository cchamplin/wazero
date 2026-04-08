// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// defDecodeContext constructs a fresh scope + builder and invokes
// decodeDefinedType against the given payload. It returns the produced
// ValType plus a reference to the builder's in-progress ComponentTypes
// so callers can inspect interned entries.
func defDecode(t *testing.T, data []byte) (types.ValType, *types.ComponentTypesBuilder) {
	t.Helper()
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	vt, err := decodeDefinedType(r, scope, b)
	require.NoError(t, err)
	return vt, b
}

func defDecodeErr(t *testing.T, data []byte) error {
	t.Helper()
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	_, err := decodeDefinedType(r, scope, b)
	return err
}

// --- record ---

func TestDecodeRecordType(t *testing.T) {
	// Record with 2 fields: (a: s32, b: u64)
	data := []byte{
		0x72,      // record opcode
		0x02,      // 2 fields
		0x01, 'a', // field name "a"
		0x7a,      // s32
		0x01, 'b', // field name "b"
		0x77, // u64
	}

	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindRecord, vt.Kind)
	ct := b.Finish()
	require.Equal(t, 1, len(ct.Records))
	rec := ct.Records[vt.Index]
	require.Equal(t, 2, len(rec.Fields))
	require.Equal(t, "a", rec.Fields[0].Name)
	require.Equal(t, types.S32, rec.Fields[0].Type)
	require.Equal(t, "b", rec.Fields[1].Name)
	require.Equal(t, types.U64, rec.Fields[1].Type)
}

func TestDecodeRecordType_Empty(t *testing.T) {
	// Empty record - spec requires at least 1 field
	data := []byte{
		0x72, // record opcode
		0x00, // 0 fields
	}

	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "record type must have at least 1 field")
}

func TestDecodeRecordType_SingleField(t *testing.T) {
	// Record with 1 field: (value: string)
	data := []byte{
		0x72,                          // record opcode
		0x01,                          // 1 field
		0x05, 'v', 'a', 'l', 'u', 'e', // field name "value"
		0x73, // string
	}

	vt, b := defDecode(t, data)
	ct := b.Finish()
	rec := ct.Records[vt.Index]
	require.Equal(t, 1, len(rec.Fields))
	require.Equal(t, "value", rec.Fields[0].Name)
	require.Equal(t, types.String_, rec.Fields[0].Type)
}

func TestDecodeRecordType_DuplicateFieldNames(t *testing.T) {
	data := []byte{
		0x72,      // record opcode
		0x02,      // 2 fields
		0x01, 'a', // field name "a"
		0x7a,      // s32
		0x01, 'a', // field name "a" (duplicate)
		0x77, // u64
	}

	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate record field name")
}

// --- variant ---

func TestDecodeVariantType(t *testing.T) {
	// Variant with 2 cases: { none, some(s32) }
	data := []byte{
		0x71,                     // variant opcode
		0x02,                     // 2 cases
		0x04, 'n', 'o', 'n', 'e', // case name "none"
		0x00,                     // no type (discriminant only case)
		0x00,                     // no refines
		0x04, 's', 'o', 'm', 'e', // case name "some"
		0x01, // has type
		0x7a, // s32
		0x00, // no refines
	}

	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindVariant, vt.Kind)
	ct := b.Finish()
	v := ct.Variants[vt.Index]
	require.Equal(t, 2, len(v.Cases))
	require.Equal(t, "none", v.Cases[0].Name)
	require.False(t, v.Cases[0].HasPayload)
	require.Equal(t, "some", v.Cases[1].Name)
	require.True(t, v.Cases[1].HasPayload)
	require.Equal(t, types.S32, v.Cases[1].Payload)
}

func TestDecodeVariantType_Empty(t *testing.T) {
	data := []byte{
		0x71, // variant opcode
		0x00, // 0 cases
	}

	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "variant type must have at least 1 case")
}

func TestDecodeVariantType_WithRefines(t *testing.T) {
	data := []byte{
		0x71,                     // variant opcode
		0x02,                     // 2 cases
		0x04, 'b', 'a', 's', 'e', // case name "base"
		0x01,                                    // has type
		0x7a,                                    // s32
		0x00,                                    // no refines
		0x07, 'd', 'e', 'r', 'i', 'v', 'e', 'd', // case name "derived"
		0x01, // has type
		0x79, // u32
		0x01, // has refines
		0x00, // refines case index 0
	}

	vt, b := defDecode(t, data)
	ct := b.Finish()
	v := ct.Variants[vt.Index]
	require.Equal(t, 2, len(v.Cases))
	require.Equal(t, "base", v.Cases[0].Name)
	require.Equal(t, "derived", v.Cases[1].Name)
	require.True(t, v.Cases[1].HasPayload)
}

func TestDecodeVariantType_SingleCaseNoType(t *testing.T) {
	data := []byte{
		0x71,                     // variant opcode
		0x01,                     // 1 case
		0x04, 'u', 'n', 'i', 't', // case name "unit"
		0x00, // no type
		0x00, // no refines
	}

	vt, b := defDecode(t, data)
	ct := b.Finish()
	v := ct.Variants[vt.Index]
	require.Equal(t, 1, len(v.Cases))
	require.Equal(t, "unit", v.Cases[0].Name)
	require.False(t, v.Cases[0].HasPayload)
}

func TestDecodeVariantType_DuplicateCaseNames(t *testing.T) {
	data := []byte{
		0x71,                     // variant opcode
		0x02,                     // 2 cases
		0x04, 'n', 'o', 'n', 'e', // case name "none"
		0x00,                     // no type
		0x00,                     // no refines
		0x04, 'n', 'o', 'n', 'e', // case name "none" (duplicate)
		0x01, // has type
		0x7a, // s32
		0x00, // no refines
	}

	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate variant case name")
}

// --- list ---

func TestDecodeListType(t *testing.T) {
	// list<s32>
	data := []byte{
		0x70, // list opcode
		0x7a, // s32 element type
	}

	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindList, vt.Kind)
	ct := b.Finish()
	require.Equal(t, types.S32, ct.Lists[vt.Index].Element)
}

// --- tuple ---

func TestDecodeTupleType(t *testing.T) {
	// tuple<s32, u64>
	data := []byte{
		0x6f, // tuple opcode
		0x02, // 2 elements
		0x7a, // s32
		0x77, // u64
	}
	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindTuple, vt.Kind)
	ct := b.Finish()
	tup := ct.Tuples[vt.Index]
	require.Equal(t, 2, len(tup.Types))
	require.Equal(t, types.S32, tup.Types[0])
	require.Equal(t, types.U64, tup.Types[1])
}

func TestDecodeTupleType_Empty(t *testing.T) {
	data := []byte{
		0x6f, // tuple opcode
		0x00, // 0 elements
	}
	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tuple type must have at least 1 element")
}

func TestDecodeTupleType_Single(t *testing.T) {
	data := []byte{
		0x6f, // tuple opcode
		0x01, // 1 element
		0x73, // string
	}
	vt, b := defDecode(t, data)
	ct := b.Finish()
	tup := ct.Tuples[vt.Index]
	require.Equal(t, 1, len(tup.Types))
	require.Equal(t, types.String_, tup.Types[0])
}

// --- flags ---

func TestDecodeFlagsType(t *testing.T) {
	data := []byte{
		0x6e,                     // flags opcode
		0x02,                     // 2 flags
		0x04, 'r', 'e', 'a', 'd', // "read"
		0x05, 'w', 'r', 'i', 't', 'e', // "write"
	}
	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindFlags, vt.Kind)
	ct := b.Finish()
	require.Equal(t, []string{"read", "write"}, ct.Flags[vt.Index].Names)
}

func TestDecodeFlagsType_Empty(t *testing.T) {
	data := []byte{
		0x6e, // flags opcode
		0x00, // 0 flags
	}
	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "flags type must have at least 1 flag")
}

func TestDecodeFlagsType_Single(t *testing.T) {
	data := []byte{
		0x6e,                                    // flags opcode
		0x01,                                    // 1 flag
		0x07, 'e', 'n', 'a', 'b', 'l', 'e', 'd', // "enabled"
	}
	vt, b := defDecode(t, data)
	ct := b.Finish()
	require.Equal(t, []string{"enabled"}, ct.Flags[vt.Index].Names)
}

func TestDecodeFlagsType_DuplicateFlagNames(t *testing.T) {
	data := []byte{
		0x6e,                     // flags opcode
		0x02,                     // 2 flags
		0x04, 'r', 'e', 'a', 'd', // "read"
		0x04, 'r', 'e', 'a', 'd', // "read" (duplicate)
	}
	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate flag name")
}

func TestDecodeFlagsType_TooManyFlags(t *testing.T) {
	data := []byte{0x6e, 33} // flags opcode, 33 flags
	for i := 0; i < 33; i++ {
		name := []byte{'f', byte('0' + i%10)}
		data = append(data, byte(len(name)))
		data = append(data, name...)
	}
	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "flags type must have at most 32 flags")
}

// --- enum ---

func TestDecodeEnumType(t *testing.T) {
	data := []byte{
		0x6d,                // enum opcode
		0x03,                // 3 cases
		0x03, 'r', 'e', 'd', // "red"
		0x05, 'g', 'r', 'e', 'e', 'n', // "green"
		0x04, 'b', 'l', 'u', 'e', // "blue"
	}
	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindEnum, vt.Kind)
	ct := b.Finish()
	require.Equal(t, []string{"red", "green", "blue"}, ct.Enums[vt.Index].Names)
}

func TestDecodeEnumType_Empty(t *testing.T) {
	data := []byte{
		0x6d, // enum opcode
		0x00, // 0 cases
	}
	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "enum type must have at least 1 case")
}

func TestDecodeEnumType_DuplicateCaseNames(t *testing.T) {
	data := []byte{
		0x6d,                // enum opcode
		0x02,                // 2 cases
		0x03, 'r', 'e', 'd', // "red"
		0x03, 'r', 'e', 'd', // "red" (duplicate)
	}
	err := defDecodeErr(t, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate enum case name")
}

// --- option ---

func TestDecodeOptionType(t *testing.T) {
	data := []byte{
		0x6b, // option opcode
		0x7a, // s32
	}
	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindOption, vt.Kind)
	ct := b.Finish()
	require.Equal(t, types.S32, ct.Options[vt.Index].Element)
}

func TestDecodeOptionType_String(t *testing.T) {
	data := []byte{
		0x6b, // option opcode
		0x73, // string
	}
	vt, b := defDecode(t, data)
	ct := b.Finish()
	require.Equal(t, types.String_, ct.Options[vt.Index].Element)
}

// --- result ---

func TestDecodeResultType(t *testing.T) {
	data := []byte{
		0x6a, // result opcode
		0x01, // has ok type
		0x7a, // s32
		0x01, // has error type
		0x73, // string
	}
	vt, b := defDecode(t, data)
	require.Equal(t, types.TypeKindResult, vt.Kind)
	ct := b.Finish()
	r := ct.Results[vt.Index]
	require.True(t, r.HasOK)
	require.Equal(t, types.S32, r.OK)
	require.True(t, r.HasErr)
	require.Equal(t, types.String_, r.Err)
}

func TestDecodeResultTypeOkOnly(t *testing.T) {
	data := []byte{
		0x6a, // result opcode
		0x01, // has ok type
		0x7a, // s32
		0x00, // no error type
	}
	vt, b := defDecode(t, data)
	ct := b.Finish()
	r := ct.Results[vt.Index]
	require.True(t, r.HasOK)
	require.Equal(t, types.S32, r.OK)
	require.False(t, r.HasErr)
}

func TestDecodeResultTypeErrorOnly(t *testing.T) {
	data := []byte{
		0x6a, // result opcode
		0x00, // no ok type
		0x01, // has error type
		0x73, // string
	}
	vt, b := defDecode(t, data)
	ct := b.Finish()
	r := ct.Results[vt.Index]
	require.False(t, r.HasOK)
	require.True(t, r.HasErr)
	require.Equal(t, types.String_, r.Err)
}

func TestDecodeResultTypeUnit(t *testing.T) {
	data := []byte{
		0x6a, // result opcode
		0x00, // no ok type
		0x00, // no error type
	}
	vt, b := defDecode(t, data)
	ct := b.Finish()
	r := ct.Results[vt.Index]
	require.False(t, r.HasOK)
	require.False(t, r.HasErr)
}
