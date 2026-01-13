package abi

import (
	"encoding/binary"
	"fmt"
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

// --- String Tests ---

func TestLowerFlatString(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	val := component.ValString("hello")
	flat, err := LowerFlat(ctx, types.String{}, val)
	require.NoError(t, err)
	require.Equal(t, 2, len(flat))
	require.Equal(t, uint64(16), flat[0]) // ptr
	require.Equal(t, uint64(5), flat[1])  // len
	require.Equal(t, "hello", string(data[16:21]))
}

func TestLowerFlatStringEmpty(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			t.Fatal("Realloc should not be called for empty string")
			return 0, nil
		},
	}

	val := component.ValString("")
	flat, err := LowerFlat(ctx, types.String{}, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0, 0}, flat)
}

func TestLowerFlatStringUnicode(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(16)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	val := component.ValString("日本語")
	flat, err := LowerFlat(ctx, types.String{}, val)
	require.NoError(t, err)
	require.Equal(t, uint64(16), flat[0]) // ptr
	require.Equal(t, uint64(9), flat[1])  // 9 bytes for 3 UTF-8 chars
	require.Equal(t, "日本語", string(data[16:25]))
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

func TestLowerFlatVariant(t *testing.T) {
	// variant { none, some(s32) }
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}
	payload := component.ValS32(42)
	val := component.ValVariant("some", &payload)

	flat, err := LowerFlat(nil, varType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 42}, flat)
}

func TestLowerFlatVariantNoPayload(t *testing.T) {
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}
	val := component.ValVariant("none", nil)

	flat, err := LowerFlat(nil, varType, val)
	require.NoError(t, err)
	// Discriminant=0, then padding for the s32 payload slot
	require.Equal(t, []uint64{0, 0}, flat)
}

func TestLowerFlatVariantUnknownCase(t *testing.T) {
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}
	val := component.ValVariant("unknown", nil)

	_, err := LowerFlat(nil, varType, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown variant case")
}

func TestLowerFlatVariantMissingPayload(t *testing.T) {
	varType := types.Variant{
		Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		},
	}
	// Case "some" requires a payload but none provided
	val := component.ValVariant("some", nil)

	_, err := LowerFlat(nil, varType, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires a payload")
}

// --- Tuple Tests ---

func TestLowerFlatTuple(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{types.S32{}, types.U64{}},
	}
	val := component.ValTuple([]component.Val{
		component.ValS32(42),
		component.ValU64(100),
	})

	flat, err := LowerFlat(nil, tupleType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{42, 100}, flat)
}

func TestLowerFlatTupleEmpty(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{},
	}
	val := component.ValTuple([]component.Val{})

	flat, err := LowerFlat(nil, tupleType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{}, flat)
}

func TestLowerFlatTupleNested(t *testing.T) {
	innerTuple := types.Tuple{Types: []types.ValType{types.U64{}, types.Bool{}}}
	outerTuple := types.Tuple{Types: []types.ValType{types.S32{}, innerTuple}}

	val := component.ValTuple([]component.Val{
		component.ValS32(42),
		component.ValTuple([]component.Val{
			component.ValU64(100),
			component.ValBool(true),
		}),
	})

	flat, err := LowerFlat(nil, outerTuple, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{42, 100, 1}, flat)
}

func TestLowerFlatTupleBoundsError(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{types.S32{}, types.U64{}},
	}
	// Tuple with only 1 element when 2 expected
	val := component.ValTuple([]component.Val{component.ValS32(42)})
	_, err := LowerFlat(nil, tupleType, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected 2")
}

// --- Option Tests ---

func TestLowerFlatOptionSome(t *testing.T) {
	optType := types.Option{Some: types.S32{}}
	payload := component.ValS32(42)
	val := component.ValOption(&payload)

	flat, err := LowerFlat(nil, optType, val)
	require.NoError(t, err)
	// discriminant=1 (some), payload=42
	require.Equal(t, []uint64{1, 42}, flat)
}

func TestLowerFlatOptionNone(t *testing.T) {
	optType := types.Option{Some: types.S32{}}
	val := component.ValOption(nil)

	flat, err := LowerFlat(nil, optType, val)
	require.NoError(t, err)
	// discriminant=0 (none), padding for s32
	require.Equal(t, []uint64{0, 0}, flat)
}

func TestLowerFlatOptionWithRecord(t *testing.T) {
	recType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.S32{}},
			{Name: "b", Type: types.U64{}},
		},
	}
	optType := types.Option{Some: recType}
	payload := component.ValRecord(map[string]component.Val{
		"a": component.ValS32(10),
		"b": component.ValU64(20),
	})
	val := component.ValOption(&payload)

	flat, err := LowerFlat(nil, optType, val)
	require.NoError(t, err)
	// discriminant=1, a=10, b=20
	require.Equal(t, []uint64{1, 10, 20}, flat)
}

// --- Result Tests ---

func TestLowerFlatResultOk(t *testing.T) {
	resType := types.Result{Ok: types.S32{}, Error: types.U32{}}
	payload := component.ValS32(42)
	val := component.ValResultOk(&payload)

	flat, err := LowerFlat(nil, resType, val)
	require.NoError(t, err)
	// discriminant=0 (ok), payload=42
	require.Equal(t, []uint64{0, 42}, flat)
}

func TestLowerFlatResultErr(t *testing.T) {
	resType := types.Result{Ok: types.S32{}, Error: types.U32{}}
	payload := component.ValU32(99)
	val := component.ValResultError(&payload)

	flat, err := LowerFlat(nil, resType, val)
	require.NoError(t, err)
	// discriminant=1 (error), payload=99
	require.Equal(t, []uint64{1, 99}, flat)
}

func TestLowerFlatResultOkNoPayload(t *testing.T) {
	resType := types.Result{Ok: nil, Error: types.U32{}}
	val := component.ValResultOk(nil)

	flat, err := LowerFlat(nil, resType, val)
	require.NoError(t, err)
	// discriminant=0, padding for error u32
	require.Equal(t, []uint64{0, 0}, flat)
}

func TestLowerFlatResultErrNoPayload(t *testing.T) {
	resType := types.Result{Ok: types.S32{}, Error: nil}
	val := component.ValResultError(nil)

	flat, err := LowerFlat(nil, resType, val)
	require.NoError(t, err)
	// discriminant=1, padding for ok s32
	require.Equal(t, []uint64{1, 0}, flat)
}

func TestLowerFlatResultWithDifferentPayloadSizes(t *testing.T) {
	// result<u64, u32> - ok is larger than error
	resType := types.Result{Ok: types.U64{}, Error: types.U32{}}
	payload := component.ValU64(12345678901234)
	val := component.ValResultOk(&payload)

	flat, err := LowerFlat(nil, resType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0, 12345678901234}, flat)
}

// --- Enum Tests ---

func TestLowerFlatEnum(t *testing.T) {
	enumType := types.Enum{Cases: []string{"a", "b", "c"}}
	val := component.ValEnum("b")

	flat, err := LowerFlat(nil, enumType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, flat) // "b" is index 1
}

func TestLowerFlatEnumFirstCase(t *testing.T) {
	enumType := types.Enum{Cases: []string{"first", "second", "third"}}
	val := component.ValEnum("first")

	flat, err := LowerFlat(nil, enumType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0}, flat)
}

func TestLowerFlatEnumLastCase(t *testing.T) {
	enumType := types.Enum{Cases: []string{"a", "b", "c"}}
	val := component.ValEnum("c")

	flat, err := LowerFlat(nil, enumType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{2}, flat)
}

func TestLowerFlatEnumInvalidCase(t *testing.T) {
	enumType := types.Enum{Cases: []string{"a", "b", "c"}}
	val := component.ValEnum("unknown")

	_, err := LowerFlat(nil, enumType, val)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown enum case")
}

// --- Flags Tests ---

func TestLowerFlatFlags(t *testing.T) {
	flagsType := types.Flags{Names: []string{"read", "write", "execute"}}
	val := component.ValFlags(map[string]bool{
		"read":    true,
		"write":   false,
		"execute": true,
	})

	flat, err := LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0b101}, flat) // bits 0 and 2 set
}

func TestLowerFlatFlagsAllSet(t *testing.T) {
	flagsType := types.Flags{Names: []string{"a", "b", "c"}}
	val := component.ValFlags(map[string]bool{
		"a": true,
		"b": true,
		"c": true,
	})

	flat, err := LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0b111}, flat)
}

func TestLowerFlatFlagsNoneSet(t *testing.T) {
	flagsType := types.Flags{Names: []string{"a", "b", "c"}}
	val := component.ValFlags(map[string]bool{
		"a": false,
		"b": false,
		"c": false,
	})

	flat, err := LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0}, flat)
}

func TestLowerFlatFlagsEmpty(t *testing.T) {
	flagsType := types.Flags{Names: []string{}}
	val := component.ValFlags(map[string]bool{})

	flat, err := LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{}, flat)
}

func TestLowerFlatFlagsMany(t *testing.T) {
	// flags with 32 flags (uses single i32)
	names := make([]string, 32)
	for i := 0; i < 32; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	flagsType := types.Flags{Names: names}
	flagMap := make(map[string]bool)
	for _, n := range names {
		flagMap[n] = false
	}
	// Set bits 0, 15, 31
	flagMap["flag0"] = true
	flagMap["flag15"] = true
	flagMap["flag31"] = true
	val := component.ValFlags(flagMap)

	flat, err := LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{(1 << 0) | (1 << 15) | (1 << 31)}, flat)
}

func TestLowerFlatFlagsMoreThan32(t *testing.T) {
	// flags with 64 flags (uses two i32s)
	names := make([]string, 64)
	for i := 0; i < 64; i++ {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	flagsType := types.Flags{Names: names}
	flagMap := make(map[string]bool)
	for _, n := range names {
		flagMap[n] = false
	}
	// Set bits 0, 31 in first i32 and bit 0, 31 in second i32 (flags 32, 63)
	flagMap["flag0"] = true
	flagMap["flag31"] = true
	flagMap["flag32"] = true
	flagMap["flag63"] = true
	val := component.ValFlags(flagMap)

	flat, err := LowerFlat(nil, flagsType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{(1 << 0) | (1 << 31), (1 << 0) | (1 << 31)}, flat)
}

// --- List Tests ---

func TestLowerFlatList(t *testing.T) {
	// For flat lower, list doesn't have actual ptr/len to return
	// since we don't have heap allocation context
	listType := types.List{Element: types.S32{}}
	val := component.ValList([]component.Val{
		component.ValS32(1),
		component.ValS32(2),
		component.ValS32(3),
	})

	flat, err := LowerFlat(nil, listType, val)
	require.NoError(t, err)
	// Returns ptr=0, len=0 since actual heap allocation is deferred
	require.Equal(t, []uint64{0, 0}, flat)
}

func TestLowerFlatListEmpty(t *testing.T) {
	listType := types.List{Element: types.U64{}}
	val := component.ValList([]component.Val{})

	flat, err := LowerFlat(nil, listType, val)
	require.NoError(t, err)
	require.Equal(t, []uint64{0, 0}, flat)
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
		{"Variant some", types.Variant{Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		}}, func() component.Val {
			p := component.ValS32(42)
			return component.ValVariant("some", &p)
		}()},
		{"Variant none", types.Variant{Cases: []types.Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: types.S32{}},
		}}, component.ValVariant("none", nil)},

		// New composite types
		{"Tuple simple", types.Tuple{Types: []types.ValType{
			types.S32{}, types.U64{},
		}}, component.ValTuple([]component.Val{
			component.ValS32(42),
			component.ValU64(100),
		})},
		{"Tuple empty", types.Tuple{Types: []types.ValType{}},
			component.ValTuple([]component.Val{})},
		{"Option some", types.Option{Some: types.S32{}}, func() component.Val {
			p := component.ValS32(42)
			return component.ValOption(&p)
		}()},
		{"Option none", types.Option{Some: types.S32{}}, component.ValOption(nil)},
		{"Result ok", types.Result{Ok: types.S32{}, Error: types.U32{}}, func() component.Val {
			p := component.ValS32(42)
			return component.ValResultOk(&p)
		}()},
		{"Result err", types.Result{Ok: types.S32{}, Error: types.U32{}}, func() component.Val {
			p := component.ValU32(99)
			return component.ValResultError(&p)
		}()},
		{"Enum", types.Enum{Cases: []string{"a", "b", "c"}}, component.ValEnum("b")},
		{"Flags", types.Flags{Names: []string{"read", "write", "execute"}}, component.ValFlags(map[string]bool{
			"read":    true,
			"write":   false,
			"execute": true,
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
			case types.Variant:
				origCase, origPayload := tt.val.Variant()
				liftedCase, liftedPayload := lifted.Variant()
				require.Equal(t, origCase, liftedCase)
				if origPayload == nil {
					require.Nil(t, liftedPayload)
				} else {
					require.NotNil(t, liftedPayload)
					// For s32 payload
					if origPayload.Kind() == component.ValKindS32 {
						require.Equal(t, origPayload.S32(), liftedPayload.S32())
					}
				}
			case types.Tuple:
				origElems := tt.val.Tuple()
				liftedElems := lifted.Tuple()
				require.Equal(t, len(origElems), len(liftedElems))
				for i, origElem := range origElems {
					switch origElem.Kind() {
					case component.ValKindS32:
						require.Equal(t, origElem.S32(), liftedElems[i].S32())
					case component.ValKindU64:
						require.Equal(t, origElem.U64(), liftedElems[i].U64())
					}
				}
			case types.Option:
				origPayload := tt.val.Option()
				liftedPayload := lifted.Option()
				if origPayload == nil {
					require.Nil(t, liftedPayload)
				} else {
					require.NotNil(t, liftedPayload)
					if origPayload.Kind() == component.ValKindS32 {
						require.Equal(t, origPayload.S32(), liftedPayload.S32())
					}
				}
			case types.Result:
				origIsOk, origOk, origErr := tt.val.Result()
				liftedIsOk, liftedOk, liftedErr := lifted.Result()
				require.Equal(t, origIsOk, liftedIsOk)
				if origIsOk {
					if origOk == nil {
						require.Nil(t, liftedOk)
					} else {
						require.NotNil(t, liftedOk)
						if origOk.Kind() == component.ValKindS32 {
							require.Equal(t, origOk.S32(), liftedOk.S32())
						}
					}
				} else {
					if origErr == nil {
						require.Nil(t, liftedErr)
					} else {
						require.NotNil(t, liftedErr)
						if origErr.Kind() == component.ValKindU32 {
							require.Equal(t, origErr.U32(), liftedErr.U32())
						}
					}
				}
			case types.Enum:
				require.Equal(t, tt.val.Enum(), lifted.Enum())
			case types.Flags:
				origFlags := tt.val.Flags()
				liftedFlags := lifted.Flags()
				require.Equal(t, len(origFlags), len(liftedFlags))
				for name, value := range origFlags {
					require.Equal(t, value, liftedFlags[name])
				}
			}
		})
	}
}

// --- LowerHeap Tests ---

func TestLowerHeapString(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(24) // String data will be allocated at 24

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	val := component.ValString("hello")
	err := LowerHeap(ctx, types.String{}, val, 0)
	require.NoError(t, err)

	// Verify ptr and len were written at offset 0
	ptr := binary.LittleEndian.Uint32(data[0:])
	length := binary.LittleEndian.Uint32(data[4:])
	require.Equal(t, uint32(24), ptr)
	require.Equal(t, uint32(5), length)
	require.Equal(t, "hello", string(data[24:29]))
}

func TestLowerHeapStringAtOffset(t *testing.T) {
	data := make([]byte, 64)
	allocPtr := uint32(32)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return allocPtr, nil
		},
	}

	val := component.ValString("test")
	err := LowerHeap(ctx, types.String{}, val, 8) // Write ptr/len at offset 8
	require.NoError(t, err)

	// Verify ptr and len were written at offset 8
	ptr := binary.LittleEndian.Uint32(data[8:])
	length := binary.LittleEndian.Uint32(data[12:])
	require.Equal(t, uint32(32), ptr)
	require.Equal(t, uint32(4), length)
	require.Equal(t, "test", string(data[32:36]))
}

func TestLowerHeapStringEmpty(t *testing.T) {
	data := make([]byte, 64)

	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			t.Fatal("Realloc should not be called for empty string")
			return 0, nil
		},
	}

	val := component.ValString("")
	err := LowerHeap(ctx, types.String{}, val, 0)
	require.NoError(t, err)

	// Empty string: ptr=0, len=0
	ptr := binary.LittleEndian.Uint32(data[0:])
	length := binary.LittleEndian.Uint32(data[4:])
	require.Equal(t, uint32(0), ptr)
	require.Equal(t, uint32(0), length)
}

func TestLowerHeapPrimitives(t *testing.T) {
	data := make([]byte, 64)
	ctx := &LowerContext{
		Memory: &mockMemory{data: data},
		Opts:   &Options{StringEncoding: StringEncodingUTF8},
	}

	// Test Bool
	err := LowerHeap(ctx, types.Bool{}, component.ValBool(true), 0)
	require.NoError(t, err)
	require.Equal(t, uint8(1), data[0])

	err = LowerHeap(ctx, types.Bool{}, component.ValBool(false), 1)
	require.NoError(t, err)
	require.Equal(t, uint8(0), data[1])

	// Test U8/S8
	err = LowerHeap(ctx, types.U8{}, component.ValU8(0xAB), 2)
	require.NoError(t, err)
	require.Equal(t, uint8(0xAB), data[2])

	err = LowerHeap(ctx, types.S8{}, component.ValS8(-1), 3)
	require.NoError(t, err)
	require.Equal(t, uint8(0xFF), data[3])

	// Test U16/S16
	err = LowerHeap(ctx, types.U16{}, component.ValU16(0x1234), 4)
	require.NoError(t, err)
	require.Equal(t, uint16(0x1234), binary.LittleEndian.Uint16(data[4:]))

	err = LowerHeap(ctx, types.S16{}, component.ValS16(-1), 6)
	require.NoError(t, err)
	require.Equal(t, uint16(0xFFFF), binary.LittleEndian.Uint16(data[6:]))

	// Test U32/S32
	err = LowerHeap(ctx, types.U32{}, component.ValU32(0xDEADBEEF), 8)
	require.NoError(t, err)
	require.Equal(t, uint32(0xDEADBEEF), binary.LittleEndian.Uint32(data[8:]))

	err = LowerHeap(ctx, types.S32{}, component.ValS32(-42), 12)
	require.NoError(t, err)
	require.Equal(t, int32(-42), int32(binary.LittleEndian.Uint32(data[12:])))

	// Test U64/S64
	err = LowerHeap(ctx, types.U64{}, component.ValU64(0x123456789ABCDEF0), 16)
	require.NoError(t, err)
	require.Equal(t, uint64(0x123456789ABCDEF0), binary.LittleEndian.Uint64(data[16:]))

	err = LowerHeap(ctx, types.S64{}, component.ValS64(-1), 24)
	require.NoError(t, err)
	require.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), binary.LittleEndian.Uint64(data[24:]))

	// Test F32
	err = LowerHeap(ctx, types.F32{}, component.ValF32(3.14), 32)
	require.NoError(t, err)
	require.Equal(t, math.Float32bits(3.14), binary.LittleEndian.Uint32(data[32:]))

	// Test F64
	err = LowerHeap(ctx, types.F64{}, component.ValF64(3.14159), 40)
	require.NoError(t, err)
	require.Equal(t, math.Float64bits(3.14159), binary.LittleEndian.Uint64(data[40:]))

	// Test Char
	err = LowerHeap(ctx, types.Char{}, component.ValChar(0x1F600), 48)
	require.NoError(t, err)
	require.Equal(t, uint32(0x1F600), binary.LittleEndian.Uint32(data[48:]))
}
