package abi

import (
	"reflect"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestFlattenParams(t *testing.T) {
	tests := []struct {
		name     string
		params   []types.ValType
		expected []api.ValueType
	}{
		{
			name:     "empty",
			params:   nil,
			expected: nil,
		},
		{
			name:     "single s32",
			params:   []types.ValType{types.S32{}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "s32 and s64",
			params:   []types.ValType{types.S32{}, types.S64{}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name:     "string param",
			params:   []types.ValType{types.String{}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, // ptr, len
		},
		{
			name:     "bool",
			params:   []types.ValType{types.Bool{}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "s8 u8 s16 u16",
			params:   []types.ValType{types.S8{}, types.U8{}, types.S16{}, types.U16{}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
		},
		{
			name:     "u32 u64",
			params:   []types.ValType{types.U32{}, types.U64{}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name:     "f32 f64",
			params:   []types.ValType{types.F32{}, types.F64{}},
			expected: []api.ValueType{api.ValueTypeF32, api.ValueTypeF64},
		},
		{
			name:     "char",
			params:   []types.ValType{types.Char{}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "list",
			params:   []types.ValType{types.List{Element: types.S32{}}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, // ptr, len
		},
		{
			name:     "own handle",
			params:   []types.ValType{types.Own{ResourceIdx: 0}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "borrow handle",
			params:   []types.ValType{types.Borrow{ResourceIdx: 0}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "record with two i32 fields",
			params:   []types.ValType{types.Record{Fields: []types.Field{{Name: "a", Type: types.S32{}}, {Name: "b", Type: types.S32{}}}}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		},
		{
			name:     "tuple of s32 and s64",
			params:   []types.ValType{types.Tuple{Types: []types.ValType{types.S32{}, types.S64{}}}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name:     "option of s32",
			params:   []types.ValType{types.Option{Some: types.S32{}}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, // discriminant, payload
		},
		{
			name:     "result of s32 and s32",
			params:   []types.ValType{types.Result{Ok: types.S32{}, Error: types.S32{}}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, // discriminant, payload (max of ok/err)
		},
		{
			name:     "enum",
			params:   []types.ValType{types.Enum{Cases: []string{"a", "b", "c"}}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "flags with 3 names",
			params:   []types.ValType{types.Flags{Names: []string{"a", "b", "c"}}},
			expected: []api.ValueType{api.ValueTypeI32},
		},
		{
			name:     "variant",
			params:   []types.ValType{types.Variant{Cases: []types.Case{{Name: "a", Type: types.S32{}}, {Name: "b", Type: types.S64{}}}}},
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64}, // discriminant + max payload
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlattenParams(tt.params)
			if len(result) != len(tt.expected) {
				t.Errorf("FlattenParams() = %v, want %v", result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("FlattenParams()[%d] = %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFlattenResults(t *testing.T) {
	tests := []struct {
		name        string
		results     []types.ValType
		expected    []api.ValueType
		needsRetptr bool
	}{
		{
			name:        "empty",
			results:     nil,
			expected:    nil,
			needsRetptr: false,
		},
		{
			name:        "single s32",
			results:     []types.ValType{types.S32{}},
			expected:    []api.ValueType{api.ValueTypeI32},
			needsRetptr: false,
		},
		{
			name:        "string result needs retptr",
			results:     []types.ValType{types.String{}},
			expected:    nil, // results via retptr
			needsRetptr: true,
		},
		{
			name:        "single f32",
			results:     []types.ValType{types.F32{}},
			expected:    []api.ValueType{api.ValueTypeF32},
			needsRetptr: false,
		},
		{
			name:        "single f64",
			results:     []types.ValType{types.F64{}},
			expected:    []api.ValueType{api.ValueTypeF64},
			needsRetptr: false,
		},
		{
			name:        "two s32 results needs retptr",
			results:     []types.ValType{types.S32{}, types.S32{}},
			expected:    nil,
			needsRetptr: true,
		},
		{
			name:        "list result needs retptr",
			results:     []types.ValType{types.List{Element: types.S32{}}},
			expected:    nil,
			needsRetptr: true,
		},
		{
			name:        "record result needs retptr",
			results:     []types.ValType{types.Record{Fields: []types.Field{{Name: "a", Type: types.S32{}}, {Name: "b", Type: types.S32{}}}}},
			expected:    nil,
			needsRetptr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, needsRetptr := FlattenResults(tt.results)
			if needsRetptr != tt.needsRetptr {
				t.Errorf("FlattenResults() needsRetptr = %v, want %v", needsRetptr, tt.needsRetptr)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("FlattenResults() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCoreSignature(t *testing.T) {
	tests := []struct {
		name           string
		params         []types.ValType
		results        []types.ValType
		expectParams   []api.ValueType
		expectResults  []api.ValueType
		expectNeedsPtr bool
	}{
		{
			name:           "simple function",
			params:         []types.ValType{types.S32{}},
			results:        []types.ValType{types.S32{}},
			expectParams:   []api.ValueType{api.ValueTypeI32},
			expectResults:  []api.ValueType{api.ValueTypeI32},
			expectNeedsPtr: false,
		},
		{
			name:           "string return needs retptr prepended",
			params:         []types.ValType{types.S32{}},
			results:        []types.ValType{types.String{}},
			expectParams:   []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, // retptr prepended
			expectResults:  nil,
			expectNeedsPtr: true,
		},
		{
			name:           "no params with string result",
			params:         nil,
			results:        []types.ValType{types.String{}},
			expectParams:   []api.ValueType{api.ValueTypeI32}, // just retptr
			expectResults:  nil,
			expectNeedsPtr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, results, needsRetptr := CoreSignature(tt.params, tt.results)
			if needsRetptr != tt.expectNeedsPtr {
				t.Errorf("CoreSignature() needsRetptr = %v, want %v", needsRetptr, tt.expectNeedsPtr)
			}
			if len(params) != len(tt.expectParams) {
				t.Errorf("CoreSignature() params = %v, want %v", params, tt.expectParams)
			} else {
				for i := range params {
					if params[i] != tt.expectParams[i] {
						t.Errorf("CoreSignature() params[%d] = %v, want %v", i, params[i], tt.expectParams[i])
					}
				}
			}
			if len(results) != len(tt.expectResults) {
				t.Errorf("CoreSignature() results = %v, want %v", results, tt.expectResults)
			}
		})
	}
}

func TestFlattenType(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.ValType
		expected []api.ValueType
	}{
		{"bool", types.Bool{}, []api.ValueType{api.ValueTypeI32}},
		{"s8", types.S8{}, []api.ValueType{api.ValueTypeI32}},
		{"u8", types.U8{}, []api.ValueType{api.ValueTypeI32}},
		{"s16", types.S16{}, []api.ValueType{api.ValueTypeI32}},
		{"u16", types.U16{}, []api.ValueType{api.ValueTypeI32}},
		{"s32", types.S32{}, []api.ValueType{api.ValueTypeI32}},
		{"u32", types.U32{}, []api.ValueType{api.ValueTypeI32}},
		{"s64", types.S64{}, []api.ValueType{api.ValueTypeI64}},
		{"u64", types.U64{}, []api.ValueType{api.ValueTypeI64}},
		{"f32", types.F32{}, []api.ValueType{api.ValueTypeF32}},
		{"f64", types.F64{}, []api.ValueType{api.ValueTypeF64}},
		{"char", types.Char{}, []api.ValueType{api.ValueTypeI32}},
		{"string", types.String{}, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}},
		{"list", types.List{Element: types.U8{}}, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}},
		{"own", types.Own{ResourceIdx: 0}, []api.ValueType{api.ValueTypeI32}},
		{"borrow", types.Borrow{ResourceIdx: 0}, []api.ValueType{api.ValueTypeI32}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenType(tt.typ)
			if len(result) != len(tt.expected) {
				t.Errorf("flattenType(%s) = %v, want %v", tt.name, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("flattenType(%s)[%d] = %v, want %v", tt.name, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFlattenVariantJoinSemantics(t *testing.T) {
	tests := []struct {
		name     string
		variant  types.Variant
		expected []api.ValueType
	}{
		{
			name: "i32_and_f32_join_to_i32",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.S32{}},
				{Name: "b", Type: types.F32{}},
			}},
			// Discriminant (i32) + payload joined to i32 (f32 reinterpreted as i32)
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		},
		{
			name: "i32_and_i64_join_to_i64",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.S32{}},
				{Name: "b", Type: types.S64{}},
			}},
			// Discriminant (i32) + payload joined to i64
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name: "f32_and_f64_join_to_i64",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.F32{}},
				{Name: "b", Type: types.F64{}},
			}},
			// Discriminant (i32) + payload joined to i64 (since f32!=f64, join returns i64)
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
		{
			name: "f64_and_i64_join_to_i64",
			variant: types.Variant{Cases: []types.Case{
				{Name: "a", Type: types.F64{}},
				{Name: "b", Type: types.S64{}},
			}},
			// Same type width, join returns i64
			expected: []api.ValueType{api.ValueTypeI32, api.ValueTypeI64},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenType(tt.variant)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("flattenType() = %v, want %v", result, tt.expected)
			}
		})
	}
}
