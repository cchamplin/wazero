package types

import "testing"

func TestTypeKindConstants(t *testing.T) {
	// Confirm the discriminator order matches the design doc.
	// Spec: definitions.py:103-180 (canonical type list).
	cases := []struct {
		k    TypeKind
		want uint8
	}{
		{TypeKindBool, 0},
		{TypeKindS8, 1},
		{TypeKindU8, 2},
		{TypeKindString, 12},
		{TypeKindList, 13},
		{TypeKindFixedList, 14},
		{TypeKindRecord, 15},
		{TypeKindOwn, 22},
		{TypeKindBorrow, 23},
		{TypeKindStream, 24},
		{TypeKindFuture, 25},
		{TypeKindErrorContext, 26},
	}
	for _, c := range cases {
		if uint8(c.k) != c.want {
			t.Errorf("TypeKind(%v) = %d, want %d", c.k, uint8(c.k), c.want)
		}
	}
}

func TestValTypeIsZero(t *testing.T) {
	var z ValType
	if !z.IsZero() {
		t.Errorf("zero ValType.IsZero() = false, want true")
	}
	if Bool.IsZero() {
		t.Errorf("Bool.IsZero() = true, want false")
	}
	if (ValType{Kind: TypeKindRecord, Index: 5}).IsZero() {
		t.Errorf("non-zero ValType.IsZero() = true, want false")
	}
}

func TestNamedScalarConstants(t *testing.T) {
	cases := []struct {
		name string
		v    ValType
		kind TypeKind
	}{
		{"Bool", Bool, TypeKindBool},
		{"S8", S8, TypeKindS8},
		{"U8", U8, TypeKindU8},
		{"S16", S16, TypeKindS16},
		{"U16", U16, TypeKindU16},
		{"S32", S32, TypeKindS32},
		{"U32", U32, TypeKindU32},
		{"S64", S64, TypeKindS64},
		{"U64", U64, TypeKindU64},
		{"F32", F32, TypeKindF32},
		{"F64", F64, TypeKindF64},
		{"Char", Char, TypeKindChar},
		{"String_", String_, TypeKindString},
	}
	for _, c := range cases {
		if c.v.Kind != c.kind {
			t.Errorf("%s.Kind = %v, want %v", c.name, c.v.Kind, c.kind)
		}
		if c.v.Index != 0 {
			t.Errorf("%s.Index = %d, want 0", c.name, c.v.Index)
		}
	}
}

func TestValTypeComparable(t *testing.T) {
	// ValType is a value-type struct and must be usable as a map key.
	m := map[ValType]string{
		Bool:                                "bool",
		U32:                                 "u32",
		{Kind: TypeKindRecord, Index: 5}:    "record5",
		{Kind: TypeKindRecord, Index: 6}:    "record6",
	}
	if m[Bool] != "bool" {
		t.Errorf("map lookup of Bool failed")
	}
	if m[ValType{Kind: TypeKindRecord, Index: 5}] != "record5" {
		t.Errorf("map lookup of record5 failed")
	}
}
