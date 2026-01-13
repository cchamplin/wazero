package abi

import (
	"math"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestLowerFlatS32(t *testing.T) {
	val := component.ValS32(-42)
	flat, err := LowerFlat(nil, types.S32{}, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	// Verify round-trip works
	iter := NewFlatIter(flat)
	lifted, err := LiftFlat(nil, types.S32{}, iter)
	require.NoError(t, err)
	require.Equal(t, int32(-42), lifted.S32())
}

func TestLowerFlatU64(t *testing.T) {
	val := component.ValU64(0xDEADBEEF12345678)
	flat, err := LowerFlat(nil, types.U64{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0xDEADBEEF12345678}, flat)
}

func TestLowerFlatBool(t *testing.T) {
	val := component.ValBool(true)
	flat, err := LowerFlat(nil, types.Bool{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, flat)
}

func TestLowerFlatBoolFalse(t *testing.T) {
	val := component.ValBool(false)
	flat, err := LowerFlat(nil, types.Bool{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0}, flat)
}

func TestLowerFlatS8(t *testing.T) {
	val := component.ValS8(-128)
	flat, err := LowerFlat(nil, types.S8{}, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	// Verify round-trip works
	iter := NewFlatIter(flat)
	lifted, err := LiftFlat(nil, types.S8{}, iter)
	require.NoError(t, err)
	require.Equal(t, int8(-128), lifted.S8())
}

func TestLowerFlatU8(t *testing.T) {
	val := component.ValU8(255)
	flat, err := LowerFlat(nil, types.U8{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{255}, flat)
}

func TestLowerFlatS16(t *testing.T) {
	val := component.ValS16(-32768)
	flat, err := LowerFlat(nil, types.S16{}, val)
	require.NoError(t, err)
	require.Equal(t, 1, len(flat))
	// Verify round-trip works
	iter := NewFlatIter(flat)
	lifted, err := LiftFlat(nil, types.S16{}, iter)
	require.NoError(t, err)
	require.Equal(t, int16(-32768), lifted.S16())
}

func TestLowerFlatU16(t *testing.T) {
	val := component.ValU16(65535)
	flat, err := LowerFlat(nil, types.U16{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{65535}, flat)
}

func TestLowerFlatU32(t *testing.T) {
	val := component.ValU32(0xDEADBEEF)
	flat, err := LowerFlat(nil, types.U32{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0xDEADBEEF}, flat)
}

func TestLowerFlatS64(t *testing.T) {
	val := component.ValS64(-1)
	flat, err := LowerFlat(nil, types.S64{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0xFFFFFFFFFFFFFFFF}, flat)
}

func TestLowerFlatF32(t *testing.T) {
	val := component.ValF32(3.14)
	flat, err := LowerFlat(nil, types.F32{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{uint64(math.Float32bits(3.14))}, flat)
}

func TestLowerFlatF64(t *testing.T) {
	val := component.ValF64(3.14159265359)
	flat, err := LowerFlat(nil, types.F64{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{math.Float64bits(3.14159265359)}, flat)
}

func TestLowerFlatChar(t *testing.T) {
	val := component.ValChar(0x1F600) // Unicode smiley face
	flat, err := LowerFlat(nil, types.Char{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0x1F600}, flat)
}

func TestLowerFlatUnsupportedType(t *testing.T) {
	val := component.ValString("test")
	_, err := LowerFlat(nil, types.String{}, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported flat lower")
}

func TestLowerFlatRecord(t *testing.T) {
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}
	val := component.ValRecord(map[string]component.Val{
		"a": component.ValS32(42),
		"b": component.ValU64(100),
	})

	flat, err := LowerFlat(nil, recType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{42, 100}, flat)
}

func TestLowerFlatRecordNested(t *testing.T) {
	innerType := types.Record{
		Fields: []types.Field{
			{Name: "inner", Type: types.S32{}},
		},
	}
	outerType := types.Record{
		Fields: []types.Field{
			{Name: "outer", Type: innerType},
			{Name: "value", Type: types.U64{}},
		},
	}
	val := component.ValRecord(map[string]component.Val{
		"outer": component.ValRecord(map[string]component.Val{
			"inner": component.ValS32(42),
		}),
		"value": component.ValU64(100),
	})

	flat, err := LowerFlat(nil, outerType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{42, 100}, flat)
}

func TestLowerFlatRecordEmpty(t *testing.T) {
	recType := types.Record{
		Fields: []types.Field{},
	}
	val := component.ValRecord(map[string]component.Val{})

	flat, err := LowerFlat(nil, recType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{}, flat)
}

func TestLowerFlatRecordMissingField(t *testing.T) {
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}
	// Missing field "b"
	val := component.ValRecord(map[string]component.Val{
		"a": component.ValS32(42),
	})

	_, err := LowerFlat(nil, recType, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing record field")
}

// TestLowerFlatRoundTrip verifies that LiftFlat(LowerFlat(val)) == val for all primitive types
func TestLowerFlatRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		typ  types.ValType
		val  component.Val
	}{
		{"Bool true", types.Bool{}, component.ValBool(true)},
		{"Bool false", types.Bool{}, component.ValBool(false)},
		{"S8 positive", types.S8{}, component.ValS8(42)},
		{"S8 negative", types.S8{}, component.ValS8(-42)},
		{"U8", types.U8{}, component.ValU8(200)},
		{"S16 positive", types.S16{}, component.ValS16(1000)},
		{"S16 negative", types.S16{}, component.ValS16(-1000)},
		{"U16", types.U16{}, component.ValU16(50000)},
		{"S32 positive", types.S32{}, component.ValS32(100000)},
		{"S32 negative", types.S32{}, component.ValS32(-100000)},
		{"U32", types.U32{}, component.ValU32(3000000000)},
		{"S64 positive", types.S64{}, component.ValS64(9000000000000)},
		{"S64 negative", types.S64{}, component.ValS64(-9000000000000)},
		{"U64", types.U64{}, component.ValU64(0xDEADBEEF12345678)},
		{"F32", types.F32{}, component.ValF32(3.14)},
		{"F64", types.F64{}, component.ValF64(3.14159265359)},
		{"Char ASCII", types.Char{}, component.ValChar('A')},
		{"Char Unicode", types.Char{}, component.ValChar(0x1F600)},
		{"Record simple", types.Record{Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		}}, component.ValRecord(map[string]component.Val{
			"a": component.ValS32(42),
			"b": component.ValU64(100),
		})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Lower to flat
			flat, err := LowerFlat(nil, tt.typ, tt.val)
			require.NoError(t, err)

			// Lift back
			iter := NewFlatIter(flat)
			lifted, err := LiftFlat(nil, tt.typ, iter)
			require.NoError(t, err)

			// Compare based on type
			switch tt.typ.(type) {
			case types.Bool:
				require.Equal(t, tt.val.Bool(), lifted.Bool())
			case types.S8:
				require.Equal(t, tt.val.S8(), lifted.S8())
			case types.U8:
				require.Equal(t, tt.val.U8(), lifted.U8())
			case types.S16:
				require.Equal(t, tt.val.S16(), lifted.S16())
			case types.U16:
				require.Equal(t, tt.val.U16(), lifted.U16())
			case types.S32:
				require.Equal(t, tt.val.S32(), lifted.S32())
			case types.U32:
				require.Equal(t, tt.val.U32(), lifted.U32())
			case types.S64:
				require.Equal(t, tt.val.S64(), lifted.S64())
			case types.U64:
				require.Equal(t, tt.val.U64(), lifted.U64())
			case types.F32:
				require.Equal(t, math.Float32bits(tt.val.F32()), math.Float32bits(lifted.F32()))
			case types.F64:
				require.Equal(t, tt.val.F64(), lifted.F64())
			case types.Char:
				require.Equal(t, tt.val.Char(), lifted.Char())
			case types.Record:
				origRec := tt.val.Record()
				liftedRec := lifted.Record()
				require.Equal(t, len(origRec), len(liftedRec))
				for k, v := range origRec {
					switch v.Kind() {
					case component.ValKindS32:
						require.Equal(t, v.S32(), liftedRec[k].S32())
					case component.ValKindU64:
						require.Equal(t, v.U64(), liftedRec[k].U64())
					}
				}
			}
		})
	}
}
