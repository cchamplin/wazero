// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestRecordType(t *testing.T) {
	// Record { a: u32, b: u64 }
	// Layout: u32 at offset 0, padding 4, u64 at offset 8
	// Size: 16, Align: 8
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: U64{}},
		},
	}

	require.Equal(t, uint32(16), r.Size())
	require.Equal(t, uint32(8), r.Align())
	require.Equal(t, 2, r.FlattenCount()) // Both fields fit flat
}

func TestRecordOffset(t *testing.T) {
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: U64{}},
		},
	}

	offsets := r.FieldOffsets()
	require.Equal(t, uint32(0), offsets[0])  // a at 0
	require.Equal(t, uint32(8), offsets[1])  // b at 8 (aligned)
}

func TestRecordEmpty(t *testing.T) {
	r := Record{Fields: []Field{}}
	// Per spec lines 1930-1932, empty types are not permitted.
	// For defensive programming, we return minimum size of 1.
	require.Equal(t, uint32(1), r.Size())
	require.Equal(t, uint32(1), r.Align())
	require.Equal(t, 0, r.FlattenCount())
}

func TestRecordComplex(t *testing.T) {
	// Record { a: u8, b: u32, c: u16 }
	// Offset 0: a (1 byte)
	// Offset 1-3: padding
	// Offset 4: b (4 bytes)
	// Offset 8: c (2 bytes)
	// Offset 10-11: padding to align 4
	// Size: 12, Align: 4
	r := Record{
		Fields: []Field{
			{Name: "a", Type: U8{}},
			{Name: "b", Type: U32{}},
			{Name: "c", Type: U16{}},
		},
	}

	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())

	offsets := r.FieldOffsets()
	require.Equal(t, uint32(0), offsets[0])
	require.Equal(t, uint32(4), offsets[1])
	require.Equal(t, uint32(8), offsets[2])
}

func TestVariantType(t *testing.T) {
	// variant { none, some(u32) }
	// Discriminant: 1 byte (2 cases < 256)
	// Payload: max(0, 4) = 4 bytes
	// Layout: disc(1) + padding(3) + payload(4) = 8, Align: 4
	v := Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: U32{}},
		},
	}

	require.Equal(t, uint32(8), v.Size())
	require.Equal(t, uint32(4), v.Align())
	require.Equal(t, 2, v.FlattenCount()) // disc + payload
}

func TestVariantDiscriminantSize(t *testing.T) {
	tests := []struct {
		numCases int
		discSize uint32
	}{
		{2, 1},
		{255, 1},
		{256, 1},
		{257, 2},
		{65536, 2},
		{65537, 4},
	}

	for _, tc := range tests {
		cases := make([]Case, tc.numCases)
		for i := range cases {
			cases[i] = Case{Name: fmt.Sprintf("case%d", i), Type: nil}
		}
		v := Variant{Cases: cases}
		require.Equal(t, tc.discSize, v.DiscriminantSize(),
			"numCases=%d", tc.numCases)
	}
}

func TestVariantWithU64Payload(t *testing.T) {
	// variant { a, b(u64) }
	// Discriminant: 1 byte
	// Padding: 7 bytes (align to 8 for u64)
	// Payload: 8 bytes
	// Size: 16, Align: 8
	v := Variant{
		Cases: []Case{
			{Name: "a", Type: nil},
			{Name: "b", Type: U64{}},
		},
	}

	require.Equal(t, uint32(16), v.Size())
	require.Equal(t, uint32(8), v.Align())
}

func TestVariantFlatten(t *testing.T) {
	// variant { a(u32), b(f32) }
	// Flat: [i32 (disc), i32 (joined payload)]
	// Note: u32 and f32 both flatten to single value, join produces i32
	v := Variant{
		Cases: []Case{
			{Name: "a", Type: U32{}},
			{Name: "b", Type: F32{}},
		},
	}

	require.Equal(t, 2, v.FlattenCount())
}

func TestListType(t *testing.T) {
	// list<u32> (variable length)
	// In memory: (ptr: i32, len: i32)
	// Size: 8, Align: 4, Flatten: 2
	l := List{Element: U32{}}

	require.Equal(t, uint32(8), l.Size())
	require.Equal(t, uint32(4), l.Align())
	require.Equal(t, 2, l.FlattenCount())
}

func TestListTypeWithComplexElement(t *testing.T) {
	// list<u64>
	// Still stored as ptr+len regardless of element type
	l := List{Element: U64{}}

	require.Equal(t, uint32(8), l.Size())
	require.Equal(t, uint32(4), l.Align())
	require.Equal(t, 2, l.FlattenCount())
}

func TestListElementSize(t *testing.T) {
	l := List{Element: U64{}}
	require.Equal(t, uint32(8), l.ElementSize())
	require.Equal(t, uint32(8), l.ElementAlign())
}

func TestOptionType(t *testing.T) {
	// option<u32> is variant { none, some(u32) }
	o := Option{Some: U32{}}

	// Same layout as variant with 2 cases
	// disc(1) + padding(3) + payload(4) = 8, align 4
	require.Equal(t, uint32(8), o.Size())
	require.Equal(t, uint32(4), o.Align())
	require.Equal(t, 2, o.FlattenCount()) // disc + payload
}

func TestOptionNone(t *testing.T) {
	// option<string> where Some is a string (8 bytes, align 4)
	o := Option{Some: String{}}

	// disc(1) + padding(3) + payload(8) = 12, align 4
	require.Equal(t, uint32(12), o.Size())
	require.Equal(t, uint32(4), o.Align())
}

func TestOptionU64(t *testing.T) {
	// option<u64>
	// disc(1) + padding(7) + payload(8) = 16, align 8
	o := Option{Some: U64{}}

	require.Equal(t, uint32(16), o.Size())
	require.Equal(t, uint32(8), o.Align())
}

func TestResultType(t *testing.T) {
	// result<u32, string>
	// variant { ok(u32), error(string) }
	// Discriminant: 1 byte
	// Payload: max(4, 8) = 8 bytes, align max(4, 4) = 4
	// Size: disc(1) + padding(3) + payload(8) = 12, align 4
	r := Result{Ok: U32{}, Error: String{}}

	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())
	require.Equal(t, 3, r.FlattenCount()) // disc + max(1, 2)
}

func TestResultOkOnly(t *testing.T) {
	// result<u64, _> (no error payload)
	r := Result{Ok: U64{}, Error: nil}

	// disc(1) + padding(7) + payload(8) = 16, align 8
	require.Equal(t, uint32(16), r.Size())
	require.Equal(t, uint32(8), r.Align())
}

func TestResultErrorOnly(t *testing.T) {
	// result<_, string> (no ok payload)
	r := Result{Ok: nil, Error: String{}}

	// disc(1) + padding(3) + payload(8) = 12, align 4
	require.Equal(t, uint32(12), r.Size())
	require.Equal(t, uint32(4), r.Align())
}

func TestResultUnit(t *testing.T) {
	// result (no payloads)
	r := Result{Ok: nil, Error: nil}

	// Just discriminant: 1 byte, align 1
	require.Equal(t, uint32(1), r.Size())
	require.Equal(t, uint32(1), r.Align())
}

func TestFlagsType(t *testing.T) {
	// flags { read, write, execute }
	// 3 flags fits in u8
	f := Flags{Names: []string{"read", "write", "execute"}}

	require.Equal(t, uint32(1), f.Size())
	require.Equal(t, uint32(1), f.Align())
	require.Equal(t, 1, f.FlattenCount())
}

func TestFlagsType8(t *testing.T) {
	// 8 flags still fits in u8
	names := make([]string, 8)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(1), f.Size())
	require.Equal(t, uint32(1), f.Align())
}

func TestFlagsType9(t *testing.T) {
	// 9 flags needs u16
	names := make([]string, 9)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(2), f.Size())
	require.Equal(t, uint32(2), f.Align())
}

func TestFlagsType16(t *testing.T) {
	// 16 flags fits in u16
	names := make([]string, 16)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(2), f.Size())
	require.Equal(t, uint32(2), f.Align())
}

func TestFlagsType17(t *testing.T) {
	// 17 flags needs u32
	names := make([]string, 17)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(4), f.Size())
	require.Equal(t, uint32(4), f.Align())
}

func TestFlagsType33(t *testing.T) {
	// 33 flags needs 2 x u32 = 8 bytes
	names := make([]string, 33)
	for i := range names {
		names[i] = fmt.Sprintf("flag%d", i)
	}
	f := Flags{Names: names}

	require.Equal(t, uint32(8), f.Size())
	require.Equal(t, uint32(4), f.Align())
	require.Equal(t, 2, f.FlattenCount()) // 2 i32s
}

func TestFlagsEmpty(t *testing.T) {
	f := Flags{Names: []string{}}
	require.Equal(t, uint32(0), f.Size())
	require.Equal(t, uint32(1), f.Align())
	require.Equal(t, 0, f.FlattenCount())
}

func TestEnumType(t *testing.T) {
	// enum { red, green, blue }
	e := Enum{Cases: []string{"red", "green", "blue"}}

	require.Equal(t, uint32(1), e.Size())  // 3 cases fits in u8
	require.Equal(t, uint32(1), e.Align())
	require.Equal(t, 1, e.FlattenCount())
}

func TestEnumType256(t *testing.T) {
	// 256 cases still fits in u8
	cases := make([]string, 256)
	for i := range cases {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	e := Enum{Cases: cases}

	require.Equal(t, uint32(1), e.Size())
	require.Equal(t, uint32(1), e.Align())
}

func TestEnumType257(t *testing.T) {
	// 257 cases needs u16
	cases := make([]string, 257)
	for i := range cases {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	e := Enum{Cases: cases}

	require.Equal(t, uint32(2), e.Size())
	require.Equal(t, uint32(2), e.Align())
}

func TestEnumType65537(t *testing.T) {
	// 65537 cases needs u32
	cases := make([]string, 65537)
	for i := range cases {
		cases[i] = fmt.Sprintf("case%d", i)
	}
	e := Enum{Cases: cases}

	require.Equal(t, uint32(4), e.Size())
	require.Equal(t, uint32(4), e.Align())
}

func TestTupleType(t *testing.T) {
	// tuple<u32, u64>
	// Same layout as record with positional fields
	tup := Tuple{Types: []ValType{U32{}, U64{}}}

	require.Equal(t, uint32(16), tup.Size())
	require.Equal(t, uint32(8), tup.Align())
	require.Equal(t, 2, tup.FlattenCount())
}

func TestTupleEmpty(t *testing.T) {
	tup := Tuple{Types: []ValType{}}
	// Per spec lines 1930-1932, empty types are not permitted.
	// For defensive programming, we return minimum size of 1.
	require.Equal(t, uint32(1), tup.Size())
	require.Equal(t, uint32(1), tup.Align())
	require.Equal(t, 0, tup.FlattenCount())
}

func TestTupleSingle(t *testing.T) {
	tup := Tuple{Types: []ValType{U32{}}}
	require.Equal(t, uint32(4), tup.Size())
	require.Equal(t, uint32(4), tup.Align())
	require.Equal(t, 1, tup.FlattenCount())
}

func TestTupleComplex(t *testing.T) {
	// tuple<u8, u32, u16>
	// Same as record { 0: u8, 1: u32, 2: u16 }
	tup := Tuple{Types: []ValType{U8{}, U32{}, U16{}}}

	require.Equal(t, uint32(12), tup.Size())
	require.Equal(t, uint32(4), tup.Align())
}

func TestTupleOffsets(t *testing.T) {
	tup := Tuple{Types: []ValType{U8{}, U32{}, U16{}}}
	offsets := tup.ElementOffsets()

	require.Equal(t, uint32(0), offsets[0])  // u8 at 0
	require.Equal(t, uint32(4), offsets[1])  // u32 at 4 (aligned)
	require.Equal(t, uint32(8), offsets[2])  // u16 at 8
}

func TestFixedLengthListAlignment(t *testing.T) {
	// Fixed-length list alignment = element alignment
	length := uint32(3)
	fixedList := List{Element: U32{}, Length: &length}
	if got := fixedList.Align(); got != 4 {
		t.Errorf("fixed list Align() = %d, want 4 (element alignment)", got)
	}

	// Dynamic list alignment = 4 (pointer alignment)
	dynamicList := List{Element: U8{}}
	if got := dynamicList.Align(); got != 4 {
		t.Errorf("dynamic list Align() = %d, want 4 (pointer alignment)", got)
	}
}

func TestFixedLengthListSize(t *testing.T) {
	// Fixed-length list size = length * element_size
	length := uint32(3)
	fixedList := List{Element: U32{}, Length: &length}
	if got := fixedList.Size(); got != 12 { // 3 * 4
		t.Errorf("fixed list Size() = %d, want 12", got)
	}

	// Dynamic list size = 8 (ptr + len)
	dynamicList := List{Element: U32{}}
	if got := dynamicList.Size(); got != 8 {
		t.Errorf("dynamic list Size() = %d, want 8", got)
	}
}

func TestFixedLengthListFlattenCount(t *testing.T) {
	// Fixed-length list flattens to length * element_flatten_count
	length := uint32(3)
	fixedList := List{Element: U32{}, Length: &length}
	if got := fixedList.FlattenCount(); got != 3 { // 3 * 1
		t.Errorf("fixed list FlattenCount() = %d, want 3", got)
	}

	// Dynamic list flattens to 2 (ptr, len)
	dynamicList := List{Element: U32{}}
	if got := dynamicList.FlattenCount(); got != 2 {
		t.Errorf("dynamic list FlattenCount() = %d, want 2", got)
	}
}

func TestFixedLengthListIsFixedLength(t *testing.T) {
	length := uint32(5)
	fixedList := List{Element: U32{}, Length: &length}
	if !fixedList.IsFixedLength() {
		t.Error("fixed list should return true for IsFixedLength()")
	}

	dynamicList := List{Element: U32{}}
	if dynamicList.IsFixedLength() {
		t.Error("dynamic list should return false for IsFixedLength()")
	}
}

func TestEmptyRecordSize(t *testing.T) {
	// Per spec line 1963, empty records should have size > 0
	// The spec says "Empty types, such as records with no fields, are not permitted"
	// However, for defensive programming we return minimum size of 1

	emptyRecord := Record{Fields: []Field{}}
	size := emptyRecord.Size()
	t.Logf("Empty record size = %d", size)

	// The spec says size must be > 0, assert 1963
	if size == 0 {
		t.Error("Empty record should have non-zero size")
	}
}

func TestEmptyTupleSize(t *testing.T) {
	emptyTuple := Tuple{Types: []ValType{}}
	size := emptyTuple.Size()
	t.Logf("Empty tuple size = %d", size)

	if size == 0 {
		t.Error("Empty tuple should have non-zero size")
	}
}
