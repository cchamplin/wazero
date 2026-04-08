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

// decodeSingleValType is a small test helper that builds a fresh scope
// and builder and decodes one valtype from the supplied bytes. It is
// used by the primitive, own<>/borrow<> and type-index tests where the
// caller doesn't care about the surrounding type-section state.
func decodeSingleValType(t *testing.T, data []byte) types.ValType {
	t.Helper()
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	vt, err := decodeValType(r, scope, b)
	require.NoError(t, err)
	return vt
}

func decodeSingleValTypeErr(t *testing.T, data []byte) error {
	t.Helper()
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	_, err := decodeValType(r, scope, b)
	return err
}

func TestDecodeValType_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected types.ValType
	}{
		{"bool", 0x7f, types.Bool},
		{"s8", 0x7e, types.S8},
		{"u8", 0x7d, types.U8},
		{"s16", 0x7c, types.S16},
		{"u16", 0x7b, types.U16},
		{"s32", 0x7a, types.S32},
		{"u32", 0x79, types.U32},
		{"s64", 0x78, types.S64},
		{"u64", 0x77, types.U64},
		{"f32", 0x76, types.F32},
		{"f64", 0x75, types.F64},
		{"char", 0x74, types.Char},
		{"string", 0x73, types.String_},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			got := decodeSingleValType(t, []byte{tc.input})
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestDecodeValType_TypeIndex(t *testing.T) {
	// Type-index references require a scope slot of matching kind.
	// Prepopulate the scope with a few value-type entries so each
	// index points at a valid target.
	scope := newTypeScope(nil)
	for _, vt := range []types.ValType{
		types.U32, // index 0
		types.S32, // index 1
		types.Bool,
		types.F32,
		types.F64,
	} {
		scope.appendValType(vt)
	}
	// Pad the scope out to index 256 so the multi-byte LEB128 cases
	// have a valid target.
	for i := len(scope.entries); i <= 256; i++ {
		scope.appendValType(types.U32)
	}

	b := types.NewComponentTypesBuilder()

	tests := []struct {
		name     string
		input    []byte
		expected types.ValType
	}{
		{"index 0", []byte{0x00}, types.U32},
		{"index 1", []byte{0x01}, types.S32},
		{"index 128", []byte{0x80, 0x01}, types.U32},
		{"index 256", []byte{0x80, 0x02}, types.U32},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			got, err := decodeValType(r, scope, b)
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestDecodeFuncType(t *testing.T) {
	// Function: (param "a" s32) (param "b" s32) -> s32
	input := []byte{
		0x40,      // sync functype
		0x02,      // 2 params
		0x01, 'a', // param name "a"
		0x7a,      // s32
		0x01, 'b', // param name "b"
		0x7a, // s32
		0x00, // single result
		0x7a, // s32
	}

	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(input)
	ftIdx, err := decodeFuncType(r, scope, b)
	require.NoError(t, err)

	ct := b.Finish()
	require.Equal(t, 1, len(ct.Funcs))
	fn := ct.Funcs[ftIdx]
	require.Equal(t, []string{"a", "b"}, fn.ParamNames)
	require.Equal(t, types.TypeKindTuple, fn.Params.Kind)
	paramTuple := ct.Tuples[fn.Params.Index]
	require.Equal(t, 2, len(paramTuple.Types))
	require.Equal(t, types.S32, paramTuple.Types[0])
	require.Equal(t, types.S32, paramTuple.Types[1])
	require.Equal(t, types.TypeKindTuple, fn.Results.Kind)
	resultTuple := ct.Tuples[fn.Results.Index]
	require.Equal(t, 1, len(resultTuple.Types))
	require.Equal(t, types.S32, resultTuple.Types[0])
}

func TestDecodeFuncType_NoParams(t *testing.T) {
	// () -> string
	input := []byte{
		0x40, // sync functype
		0x00, // 0 params
		0x00, // single result
		0x73, // string
	}

	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(input)
	ftIdx, err := decodeFuncType(r, scope, b)
	require.NoError(t, err)
	ct := b.Finish()
	fn := ct.Funcs[ftIdx]
	require.Equal(t, []string(nil), fn.ParamNames)
	require.Equal(t, types.TypeKindTuple, fn.Params.Kind)
	require.Equal(t, 0, len(ct.Tuples[fn.Params.Index].Types))
	require.Equal(t, types.TypeKindTuple, fn.Results.Kind)
	resultTuple := ct.Tuples[fn.Results.Index]
	require.Equal(t, 1, len(resultTuple.Types))
	require.Equal(t, types.String_, resultTuple.Types[0])
}

func TestDecodeFuncType_NoResults(t *testing.T) {
	// (param "x" u32) -> ()
	input := []byte{
		0x40,      // sync functype
		0x01,      // 1 param
		0x01, 'x', // param name "x"
		0x79, // u32
		0x01, // named results (vec)
		0x00, // 0 results
	}

	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(input)
	ftIdx, err := decodeFuncType(r, scope, b)
	require.NoError(t, err)
	ct := b.Finish()
	fn := ct.Funcs[ftIdx]
	require.Equal(t, []string{"x"}, fn.ParamNames)
	paramTuple := ct.Tuples[fn.Params.Index]
	require.Equal(t, 1, len(paramTuple.Types))
	require.Equal(t, types.U32, paramTuple.Types[0])
	resultTuple := ct.Tuples[fn.Results.Index]
	require.Equal(t, 0, len(resultTuple.Types))
}

func TestDecodeFuncType_NamedResults(t *testing.T) {
	// () -> (result "ok" bool) (result "err" string)
	input := []byte{
		0x40,           // sync functype
		0x00,           // 0 params
		0x01,           // named results (vec)
		0x02,           // 2 results
		0x02, 'o', 'k', // result name "ok"
		0x7f,                // bool
		0x03, 'e', 'r', 'r', // result name "err"
		0x73, // string
	}

	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(input)
	ftIdx, err := decodeFuncType(r, scope, b)
	require.NoError(t, err)
	ct := b.Finish()
	fn := ct.Funcs[ftIdx]
	resultTuple := ct.Tuples[fn.Results.Index]
	require.Equal(t, 2, len(resultTuple.Types))
	require.Equal(t, types.Bool, resultTuple.Types[0])
	require.Equal(t, types.String_, resultTuple.Types[1])
}

func TestDecodeFuncType_AsyncFunc(t *testing.T) {
	// async () -> s32
	input := []byte{
		0x43, // async functype
		0x00, // 0 params
		0x00, // single result
		0x7a, // s32
	}

	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(input)
	ftIdx, err := decodeFuncType(r, scope, b)
	require.NoError(t, err)
	ct := b.Finish()
	fn := ct.Funcs[ftIdx]
	require.True(t, fn.Async)
}

func TestDecodeFuncType_InvalidOpcode(t *testing.T) {
	input := []byte{
		0x41, // component type opcode — not a function type
		0x00,
	}

	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(input)
	_, err := decodeFuncType(r, scope, b)
	require.Error(t, err)
}

func TestDecodeName(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"empty", []byte{0x00}, ""},
		{"single char", []byte{0x01, 'a'}, "a"},
		{"multiple chars", []byte{0x05, 'h', 'e', 'l', 'l', 'o'}, "hello"},
		{"utf8", []byte{0x04, 0xc3, 0xa9, 0x6c, 0x69}, "\xc3\xa9li"},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			result, err := decodeName(r)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

// --- resource declarations ---

func TestDecodeResourceDecl(t *testing.T) {
	// 0x7f = rep type i32, 0x00 = no destructor
	data := []byte{0x7f, 0x00}
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	resourceDef, err := decodeResourceDecl(r, scope, b, false)
	require.NoError(t, err)
	require.Nil(t, resourceDef.Destructor)
	// The builder assigned a ResourceTableIdx to the interned entry.
	ct := b.Finish()
	require.Equal(t, 1, len(ct.ResourceTables))
	require.False(t, ct.ResourceTables[0].Concrete)
	require.Equal(t, types.ResourceTableIdx(0), resourceDef.ResourceTableIdx)
}

func TestDecodeResourceDecl_WithDestructor(t *testing.T) {
	data := []byte{0x7f, 0x01, 0x05}
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	resourceDef, err := decodeResourceDecl(r, scope, b, false)
	require.NoError(t, err)
	require.NotNil(t, resourceDef.Destructor)
	require.Equal(t, uint32(5), *resourceDef.Destructor)
}

func TestDecodeResourceDecl_WithLargeDestructorIndex(t *testing.T) {
	data := []byte{0x7f, 0x01, 0x80, 0x01}
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	resourceDef, err := decodeResourceDecl(r, scope, b, false)
	require.NoError(t, err)
	require.NotNil(t, resourceDef.Destructor)
	require.Equal(t, uint32(128), *resourceDef.Destructor)
}

func TestDecodeResourceDecl_InvalidRepType(t *testing.T) {
	data := []byte{0x7e, 0x00}
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	_, err := decodeResourceDecl(r, scope, b, false)
	require.Error(t, err)
}

func TestDecodeResourceDecl_InvalidDestructorFlag(t *testing.T) {
	data := []byte{0x7f, 0x02}
	scope := newTypeScope(nil)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader(data)
	_, err := decodeResourceDecl(r, scope, b, false)
	require.Error(t, err)
}

// --- own<> / borrow<> handles ---

func TestDecodeOwnType(t *testing.T) {
	// own<R> requires a scope entry of kind scopeEntryResource at the
	// target index. Prepopulate slot 3 with a resource.
	scope := newTypeScope(nil)
	for i := 0; i < 3; i++ {
		scope.appendValType(types.U32)
	}
	scope.appendResource(types.ResourceTableIdx(7))

	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader([]byte{0x69, 0x03})
	vt, err := decodeValType(r, scope, b)
	require.NoError(t, err)
	require.Equal(t, types.TypeKindOwn, vt.Kind)
	require.Equal(t, uint32(7), vt.Index)
}

func TestDecodeOwnType_Error(t *testing.T) {
	// Missing type index after 0x69 opcode.
	err := decodeSingleValTypeErr(t, []byte{0x69})
	require.Error(t, err)
}

func TestDecodeOwnType_NotAResource(t *testing.T) {
	// own<N> where N is a value-type entry must fail.
	scope := newTypeScope(nil)
	scope.appendValType(types.U32)
	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader([]byte{0x69, 0x00})
	_, err := decodeValType(r, scope, b)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a resource declaration")
}

func TestDecodeBorrowType(t *testing.T) {
	scope := newTypeScope(nil)
	for i := 0; i < 2; i++ {
		scope.appendValType(types.U32)
	}
	scope.appendResource(types.ResourceTableIdx(42))

	b := types.NewComponentTypesBuilder()
	r := bytes.NewReader([]byte{0x68, 0x02})
	vt, err := decodeValType(r, scope, b)
	require.NoError(t, err)
	require.Equal(t, types.TypeKindBorrow, vt.Kind)
	require.Equal(t, uint32(42), vt.Index)
}

func TestDecodeBorrowType_Error(t *testing.T) {
	err := decodeSingleValTypeErr(t, []byte{0x68})
	require.Error(t, err)
}
