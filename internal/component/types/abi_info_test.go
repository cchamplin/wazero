// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "testing"

// TestScalarABI verifies the scalar ABI table against canonical-ABI spec
// values. Citations are inline at each entry. Spec authority:
// debug-vendored/component-model/design/mvp/canonical-abi/definitions.py
// at lines 1065-1138 and 1705-1719.
func TestScalarABI(t *testing.T) {
	cases := []struct {
		v        ValType
		size32   uint32
		align32  uint32
		size64   uint32
		align64  uint32
		flatten  int32
		specCite string
	}{
		{Bool, 1, 1, 1, 1, 1, "definitions.py:1065,1123,1705"},
		{S8, 1, 1, 1, 1, 1, "definitions.py:1066,1124,1706"},
		{U8, 1, 1, 1, 1, 1, "definitions.py:1066,1124,1706"},
		{S16, 2, 2, 2, 2, 1, "definitions.py:1067,1125,1706"},
		{U16, 2, 2, 2, 2, 1, "definitions.py:1067,1125,1706"},
		{S32, 4, 4, 4, 4, 1, "definitions.py:1068,1126,1706"},
		{U32, 4, 4, 4, 4, 1, "definitions.py:1068,1126,1706"},
		{S64, 8, 8, 8, 8, 1, "definitions.py:1069,1127,1708"},
		{U64, 8, 8, 8, 8, 1, "definitions.py:1069,1127,1708"},
		{F32, 4, 4, 4, 4, 1, "definitions.py:1070,1128,1709"},
		{F64, 8, 8, 8, 8, 1, "definitions.py:1071,1129,1710"},
		{Char, 4, 4, 4, 4, 1, "definitions.py:1072,1130,1711"},
		// Strings: memory32 = 8/4 (ptr+len i32), memory64 = 16/8.
		// Spec: definitions.py:1073,1131,1712 (memory32 only).
		// Wasmtime: types.rs:678-684 POINTER_PAIR (memory64 doubles).
		// This is divergence (3) from the literal spec — see design doc.
		{String_, 8, 4, 16, 8, 2, "definitions.py:1073,1131,1712"},
	}
	for _, c := range cases {
		ct := &ComponentTypes{} // scalar ABIs do not need a populated ct
		got := c.v.ABI(ct)
		if got.Size32 != c.size32 || got.Align32 != c.align32 ||
			got.Size64 != c.size64 || got.Align64 != c.align64 ||
			got.FlattenCount != c.flatten {
			t.Errorf("%v.ABI = {%d/%d/%d/%d/flat=%d}, want {%d/%d/%d/%d/flat=%d} (%s)",
				c.v.Kind, got.Size32, got.Align32, got.Size64, got.Align64, got.FlattenCount,
				c.size32, c.align32, c.size64, c.align64, c.flatten, c.specCite)
		}
	}
}

func TestScalarABIHandles(t *testing.T) {
	// own/borrow/stream/future/error-context all encode as i32 handles.
	// Spec: definitions.py:1079,1080,1132,1137,1138,1713,1718,1719.
	// All have size 4, align 4, flatten 1.
	ct := &ComponentTypes{}
	for _, k := range []TypeKind{
		TypeKindOwn, TypeKindBorrow,
		TypeKindStream, TypeKindFuture, TypeKindErrorContext,
	} {
		v := ValType{Kind: k}
		got := v.ABI(ct)
		if got.Size32 != 4 || got.Align32 != 4 || got.FlattenCount != 1 {
			t.Errorf("kind %v ABI = {%d/%d/flat=%d}, want {4/4/1}",
				k, got.Size32, got.Align32, got.FlattenCount)
		}
	}
}

func TestRecordABIEmpty(t *testing.T) {
	// Empty records have size 0. Divergence (1) from the literal spec
	// (definitions.py:1150 asserts s > 0); wasmtime's record_static at
	// types.rs:705-723 returns CanonicalAbiInfo::ZERO. Both wazero and
	// this design preserve the permissive behavior.
	abi := computeRecordABI(nil, &ComponentTypes{})
	if abi.Size32 != 0 || abi.Align32 != 1 || abi.FlattenCount != 0 {
		t.Errorf("empty record ABI = {%d/%d/flat=%d}, want {0/1/0}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}

func TestRecordABISimple(t *testing.T) {
	// record { a: u32, b: u32 } -> size 8, align 4, flatten 2.
	// Spec: alignment_record at definitions.py:1087-1091, elem_size_record
	// at :1145-1151, flatten_record at :1726-1730.
	abi := computeRecordABI([]RecordField{
		{Name: "a", Type: U32},
		{Name: "b", Type: U32},
	}, &ComponentTypes{})
	if abi.Size32 != 8 || abi.Align32 != 4 || abi.FlattenCount != 2 {
		t.Errorf("record{a:u32,b:u32} ABI = {%d/%d/flat=%d}, want {8/4/2}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}

func TestABIVariantDiscriminantSize(t *testing.T) {
	// Spec: discriminant_type at definitions.py:1096-1103.
	// n <= 256 -> 1 byte, n <= 65536 -> 2 bytes, otherwise 4 bytes.
	cases := []struct {
		n    int
		want uint8
	}{
		{1, 1},
		{2, 1},
		{256, 1},
		{257, 2},
		{65536, 2},
		{65537, 4},
	}
	for _, c := range cases {
		got := discriminantSize(c.n)
		if got != c.want {
			t.Errorf("discriminantSize(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestFlagsABISmall(t *testing.T) {
	// Spec: alignment_flags at definitions.py:1112-1117, elem_size_flags
	// at :1166-1171.
	// n <= 8: size 1, align 1, flatten 1.
	cases := []struct {
		n       int
		size    uint32
		align   uint32
		flatten int32
	}{
		{1, 1, 1, 1},
		{8, 1, 1, 1},
		{9, 2, 2, 1},
		{16, 2, 2, 1},
		{17, 4, 4, 1},
		{32, 4, 4, 1},
		// Divergence (2) from literal spec: flags > 32 are permitted via
		// multi-i32 encoding, matching wasmtime's FlagsSize::Size4Plus(n)
		// at types.rs:756-770.
		{33, 8, 4, 2},
		{64, 8, 4, 2},
		{65, 12, 4, 3},
	}
	for _, c := range cases {
		names := make([]string, c.n)
		abi := computeFlagsABI(names)
		if abi.Size32 != c.size || abi.Align32 != c.align || abi.FlattenCount != c.flatten {
			t.Errorf("flags(n=%d) ABI = {%d/%d/flat=%d}, want {%d/%d/%d}",
				c.n, abi.Size32, abi.Align32, abi.FlattenCount,
				c.size, c.align, c.flatten)
		}
	}
}

func TestListDynamicABI(t *testing.T) {
	// Dynamic list: pointer-pair. memory32=8/4, memory64=16/8, flatten=2.
	// Spec: definitions.py:1075,1133,1714. Wasmtime POINTER_PAIR at
	// types.rs:678-684.
	abi := computeListABI(U32, &ComponentTypes{})
	if abi.Size32 != 8 || abi.Align32 != 4 || abi.Size64 != 16 || abi.Align64 != 8 || abi.FlattenCount != 2 {
		t.Errorf("list<u32> ABI = {%d/%d/%d/%d/flat=%d}, want {8/4/16/8/2}",
			abi.Size32, abi.Align32, abi.Size64, abi.Align64, abi.FlattenCount)
	}
}

func TestFixedListABI(t *testing.T) {
	// Fixed-length list: inline elements. size = length * elem.size,
	// align = elem.align, flatten = length * elem.flatten.
	// Spec: alignment_list at :1082-1085, elem_size_list at :1140-1143,
	// flatten_list at :1721-1723.
	abi := computeFixedListABI(U32, 5, &ComponentTypes{})
	if abi.Size32 != 20 || abi.Align32 != 4 || abi.FlattenCount != 5 {
		t.Errorf("list<u32, 5> ABI = {%d/%d/flat=%d}, want {20/4/5}",
			abi.Size32, abi.Align32, abi.FlattenCount)
	}
}
