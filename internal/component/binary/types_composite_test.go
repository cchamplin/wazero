// internal/component/binary/types_composite_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeRecordType(t *testing.T) {
	// Record with 2 fields: (a: s32, b: u64)
	// Format: 0x72 <field_count> (<name> <type>)*
	data := []byte{
		0x72,      // record opcode
		0x02,      // 2 fields
		0x01, 'a', // field name "a"
		0x7a,      // s32
		0x01, 'b', // field name "b"
		0x77, // u64
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindRecord, typeDef.Kind)
	require.NotNil(t, typeDef.Record)
	require.Equal(t, 2, len(typeDef.Record.Fields))
	require.Equal(t, "a", typeDef.Record.Fields[0].Name)
	require.Equal(t, "b", typeDef.Record.Fields[1].Name)
}

func TestDecodeRecordType_Empty(t *testing.T) {
	// Empty record: record {}
	data := []byte{
		0x72, // record opcode
		0x00, // 0 fields
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindRecord, typeDef.Kind)
	require.NotNil(t, typeDef.Record)
	require.Equal(t, 0, len(typeDef.Record.Fields))
}

func TestDecodeRecordType_SingleField(t *testing.T) {
	// Record with 1 field: (value: string)
	data := []byte{
		0x72,                             // record opcode
		0x01,                             // 1 field
		0x05, 'v', 'a', 'l', 'u', 'e',    // field name "value"
		0x73, // string
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindRecord, typeDef.Kind)
	require.NotNil(t, typeDef.Record)
	require.Equal(t, 1, len(typeDef.Record.Fields))
	require.Equal(t, "value", typeDef.Record.Fields[0].Name)
	require.True(t, typeDef.Record.Fields[0].Type.IsPrimitive)
	require.Equal(t, byte(0x73), typeDef.Record.Fields[0].Type.Primitive)
}

func TestDecodeRecordType_WithTypeIndex(t *testing.T) {
	// Record with a field that references another type by index
	// (name: string, inner: $0)
	data := []byte{
		0x72,                             // record opcode
		0x02,                             // 2 fields
		0x04, 'n', 'a', 'm', 'e',         // field name "name"
		0x73,                             // string
		0x05, 'i', 'n', 'n', 'e', 'r',    // field name "inner"
		0x00, // type index 0
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindRecord, typeDef.Kind)
	require.NotNil(t, typeDef.Record)
	require.Equal(t, 2, len(typeDef.Record.Fields))
	require.Equal(t, "name", typeDef.Record.Fields[0].Name)
	require.True(t, typeDef.Record.Fields[0].Type.IsPrimitive)
	require.Equal(t, "inner", typeDef.Record.Fields[1].Name)
	require.False(t, typeDef.Record.Fields[1].Type.IsPrimitive)
	require.Equal(t, uint32(0), typeDef.Record.Fields[1].Type.TypeIdx)
}

func TestDecodeVariantType(t *testing.T) {
	// Variant with 2 cases: { none, some(s32) }
	// Format: 0x71 <case_count> (<name> <refines>? <type>?)*
	data := []byte{
		0x71,                         // variant opcode
		0x02,                         // 2 cases
		0x04, 'n', 'o', 'n', 'e',     // case name "none"
		0x00,                         // no refines
		0x00,                         // no type (discriminant only case)
		0x04, 's', 'o', 'm', 'e',     // case name "some"
		0x00,                         // no refines
		0x01,                         // has type
		0x7a,                         // s32
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindVariant, typeDef.Kind)
	require.NotNil(t, typeDef.Variant)
	require.Equal(t, 2, len(typeDef.Variant.Cases))
	require.Equal(t, "none", typeDef.Variant.Cases[0].Name)
	require.Nil(t, typeDef.Variant.Cases[0].Type)
	require.Equal(t, "some", typeDef.Variant.Cases[1].Name)
	require.NotNil(t, typeDef.Variant.Cases[1].Type)
}

func TestDecodeVariantType_Empty(t *testing.T) {
	// Empty variant (edge case)
	data := []byte{
		0x71, // variant opcode
		0x00, // 0 cases
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindVariant, typeDef.Kind)
	require.NotNil(t, typeDef.Variant)
	require.Equal(t, 0, len(typeDef.Variant.Cases))
}

func TestDecodeVariantType_WithRefines(t *testing.T) {
	// Variant with refines: case that refines another case
	// Format: 0x71 <case_count> (<name> <refines_flag> [<refines_idx>] <type_flag> [<type>])*
	data := []byte{
		0x71,                         // variant opcode
		0x02,                         // 2 cases
		0x04, 'b', 'a', 's', 'e',     // case name "base"
		0x00,                         // no refines
		0x01,                         // has type
		0x7a,                         // s32
		0x07, 'd', 'e', 'r', 'i', 'v', 'e', 'd', // case name "derived"
		0x01,                         // has refines
		0x00,                         // refines case index 0
		0x01,                         // has type
		0x79,                         // u32
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindVariant, typeDef.Kind)
	require.NotNil(t, typeDef.Variant)
	require.Equal(t, 2, len(typeDef.Variant.Cases))
	require.Equal(t, "base", typeDef.Variant.Cases[0].Name)
	require.Nil(t, typeDef.Variant.Cases[0].Refines)
	require.NotNil(t, typeDef.Variant.Cases[0].Type)
	require.Equal(t, "derived", typeDef.Variant.Cases[1].Name)
	require.NotNil(t, typeDef.Variant.Cases[1].Refines)
	require.Equal(t, uint32(0), *typeDef.Variant.Cases[1].Refines)
	require.NotNil(t, typeDef.Variant.Cases[1].Type)
}

func TestDecodeVariantType_SingleCaseNoType(t *testing.T) {
	// Variant with a single case that has no payload type
	data := []byte{
		0x71,                         // variant opcode
		0x01,                         // 1 case
		0x04, 'u', 'n', 'i', 't',     // case name "unit"
		0x00,                         // no refines
		0x00,                         // no type
	}

	r := bytes.NewReader(data)
	typeDef, err := decodeDefinedType(r)
	require.NoError(t, err)
	require.Equal(t, TypeDefKindVariant, typeDef.Kind)
	require.NotNil(t, typeDef.Variant)
	require.Equal(t, 1, len(typeDef.Variant.Cases))
	require.Equal(t, "unit", typeDef.Variant.Cases[0].Name)
	require.Nil(t, typeDef.Variant.Cases[0].Type)
	require.Nil(t, typeDef.Variant.Cases[0].Refines)
}
