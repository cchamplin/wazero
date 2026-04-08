package abi

import (
	"reflect"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// Session 0 note (Task 15): the pre-existing flatten_test.go constructed
// types via the deleted interface-style literals. Those tests have been
// dropped in favour of a minimal set that exercises the new kind-switch
// dispatch through the ComponentTypesBuilder. Full test migration is
// tracked in Task 19 of the Session 0 plan.

func TestFlattenType_Scalars(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.ValType
		expected []api.ValueType
	}{
		{"bool", types.Bool, []api.ValueType{api.ValueTypeI32}},
		{"s8", types.S8, []api.ValueType{api.ValueTypeI32}},
		{"u8", types.U8, []api.ValueType{api.ValueTypeI32}},
		{"s16", types.S16, []api.ValueType{api.ValueTypeI32}},
		{"u16", types.U16, []api.ValueType{api.ValueTypeI32}},
		{"s32", types.S32, []api.ValueType{api.ValueTypeI32}},
		{"u32", types.U32, []api.ValueType{api.ValueTypeI32}},
		{"s64", types.S64, []api.ValueType{api.ValueTypeI64}},
		{"u64", types.U64, []api.ValueType{api.ValueTypeI64}},
		{"f32", types.F32, []api.ValueType{api.ValueTypeF32}},
		{"f64", types.F64, []api.ValueType{api.ValueTypeF64}},
		{"char", types.Char, []api.ValueType{api.ValueTypeI32}},
		{"string", types.String_, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenType(nil, tt.typ)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("flattenType(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestFlattenType_Record(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.S32},
		{Name: "b", Type: types.U64},
	})
	ct := b.Finish()

	result := flattenType(ct, recT)
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI64}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(record) = %v, want %v", result, expected)
	}
}

func TestFlattenType_Tuple(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	tupT := b.InternTuple([]types.ValType{types.S32, types.S64})
	ct := b.Finish()

	result := flattenType(ct, tupT)
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI64}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(tuple) = %v, want %v", result, expected)
	}
}

func TestFlattenType_List(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	listT := b.InternList(types.U32)
	ct := b.Finish()

	result := flattenType(ct, listT)
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(list) = %v, want %v", result, expected)
	}
}

func TestFlattenType_FixedList(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	flT := b.InternFixedLengthList(types.U32, 3)
	ct := b.Finish()

	result := flattenType(ct, flT)
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(fixed-list) = %v, want %v", result, expected)
	}
}

func TestFlattenType_Option(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	optT := b.InternOption(types.S32)
	ct := b.Finish()

	result := flattenType(ct, optT)
	// Discriminant + payload
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(option) = %v, want %v", result, expected)
	}
}

func TestFlattenType_Result(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	resT := b.InternResult(types.S32, types.S32, true, true)
	ct := b.Finish()

	result := flattenType(ct, resT)
	// Discriminant + joined payload (both s32)
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(result) = %v, want %v", result, expected)
	}
}

func TestFlattenType_Enum(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	enumT := b.InternEnum([]string{"a", "b", "c"})
	ct := b.Finish()

	result := flattenType(ct, enumT)
	expected := []api.ValueType{api.ValueTypeI32}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(enum) = %v, want %v", result, expected)
	}
}

func TestFlattenType_Flags(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	flagsT := b.InternFlags([]string{"a", "b", "c"})
	ct := b.Finish()

	result := flattenType(ct, flagsT)
	expected := []api.ValueType{api.ValueTypeI32}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(flags) = %v, want %v", result, expected)
	}
}

func TestFlattenType_Variant_JoinSemantics(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	// variant { a: s32, b: s64 } — joined payload is i64
	varT := b.InternVariant([]types.VariantCase{
		{Name: "a", Payload: types.S32, HasPayload: true},
		{Name: "b", Payload: types.S64, HasPayload: true},
	})
	ct := b.Finish()

	result := flattenType(ct, varT)
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI64}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("flattenType(variant) = %v, want %v", result, expected)
	}
}

func TestFlattenType_OwnBorrow(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	rtIdx := b.InternAbstractResource()
	ownT := b.InternOwnHandle(rtIdx)
	borrowT := b.InternBorrowHandle(rtIdx)
	ct := b.Finish()

	own := flattenType(ct, ownT)
	expected := []api.ValueType{api.ValueTypeI32}
	if !reflect.DeepEqual(own, expected) {
		t.Errorf("flattenType(own) = %v, want %v", own, expected)
	}
	borrow := flattenType(ct, borrowT)
	if !reflect.DeepEqual(borrow, expected) {
		t.Errorf("flattenType(borrow) = %v, want %v", borrow, expected)
	}
}

func TestFlattenType_AsyncTypesAreI32(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	streamT := b.InternStream(types.U32, true)
	futureT := b.InternFuture(types.U32, true)
	errCtxT := b.InternErrorContextTable()
	ct := b.Finish()

	cases := []struct {
		name string
		typ  types.ValType
	}{
		{"Stream", streamT},
		{"Future", futureT},
		{"ErrorContext", errCtxT},
	}
	expected := []api.ValueType{api.ValueTypeI32}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := flattenType(ct, tc.typ)
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("flattenType(%s) = %v, want %v", tc.name, result, expected)
			}
		})
	}
}

func TestFlattenParamsResults(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	recT := b.InternRecord([]types.RecordField{
		{Name: "a", Type: types.S32},
		{Name: "b", Type: types.S32},
	})
	ct := b.Finish()

	params := FlattenParams(ct, []types.ValType{types.S32, types.U64})
	expected := []api.ValueType{api.ValueTypeI32, api.ValueTypeI64}
	if !reflect.DeepEqual(params, expected) {
		t.Errorf("FlattenParams = %v, want %v", params, expected)
	}

	// Single scalar result: no retptr.
	flat, needsRetptr := FlattenResults(ct, []types.ValType{types.S32})
	if needsRetptr {
		t.Errorf("FlattenResults(single scalar): unexpected retptr")
	}
	if !reflect.DeepEqual(flat, []api.ValueType{api.ValueTypeI32}) {
		t.Errorf("FlattenResults(single scalar) = %v", flat)
	}

	// Record result with 2 i32 fields: needs retptr (exceeds MaxFlatResults).
	_, needsRetptr = FlattenResults(ct, []types.ValType{recT})
	if !needsRetptr {
		t.Errorf("FlattenResults(record with 2 fields): expected retptr")
	}
}

func TestCoreSignature(t *testing.T) {
	b := types.NewComponentTypesBuilder()
	ct := b.Finish()

	// Simple function: (s32) -> s32
	params, results, needsRetptr := CoreSignature(ct, []types.ValType{types.S32}, []types.ValType{types.S32})
	if needsRetptr {
		t.Errorf("unexpected retptr for simple function")
	}
	expectedParams := []api.ValueType{api.ValueTypeI32}
	expectedResults := []api.ValueType{api.ValueTypeI32}
	if !reflect.DeepEqual(params, expectedParams) {
		t.Errorf("params = %v, want %v", params, expectedParams)
	}
	if !reflect.DeepEqual(results, expectedResults) {
		t.Errorf("results = %v, want %v", results, expectedResults)
	}

	// String result needs retptr.
	params, results, needsRetptr = CoreSignature(ct, nil, []types.ValType{types.String_})
	if !needsRetptr {
		t.Errorf("expected retptr for string result")
	}
	if results != nil {
		t.Errorf("expected nil results when retptr needed, got %v", results)
	}
	if !reflect.DeepEqual(params, []api.ValueType{api.ValueTypeI32}) {
		t.Errorf("expected [i32] (retptr only), got %v", params)
	}
}
