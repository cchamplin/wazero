// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "fmt"

// CanonicalABIInfo carries precomputed size / alignment / flatten data
// for a type in both 32-bit and 64-bit memory modes. Mirrors wasmtime's
// CanonicalAbiInfo at debug-vendored/wasmtime/crates/environ/src/component/types.rs:608+.
//
// Computed once during interning and stored on the composite struct.
// Lift/lower never recomputes.
type CanonicalABIInfo struct {
	Size32, Align32 uint32
	Size64, Align64 uint32
	FlattenCount    int32 // -1 if the type is not representable in flat form
}

// DiscriminantInfo carries derived sizing and offsets for variant-shaped
// types (Variant, Enum, Option, Result). Computed during interning.
type DiscriminantInfo struct {
	DiscSize uint8 // 1, 2, or 4 bytes

	// PayloadOffset is the byte offset of the payload in the memory32
	// discriminated layout. For memory64 the equivalent offset is
	// derivable as AlignTo(DiscSize, variantABI.Align64) — it is not
	// stored separately because the variant's overall Align64 is the
	// max payload align64 (always >= DiscSize for the interesting
	// cases) and AlignTo gives the same result.
	PayloadOffset uint32
}

// scalarABI is a package-level constant table for types whose ABI is
// not dependent on their content. Indexed by TypeKind. Spec citations
// in the test for each entry.
var scalarABI = [...]CanonicalABIInfo{
	TypeKindBool:         {Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1},
	TypeKindS8:           {Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1},
	TypeKindU8:           {Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1},
	TypeKindS16:          {Size32: 2, Align32: 2, Size64: 2, Align64: 2, FlattenCount: 1},
	TypeKindU16:          {Size32: 2, Align32: 2, Size64: 2, Align64: 2, FlattenCount: 1},
	TypeKindS32:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindU32:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindS64:          {Size32: 8, Align32: 8, Size64: 8, Align64: 8, FlattenCount: 1},
	TypeKindU64:          {Size32: 8, Align32: 8, Size64: 8, Align64: 8, FlattenCount: 1},
	TypeKindF32:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindF64:          {Size32: 8, Align32: 8, Size64: 8, Align64: 8, FlattenCount: 1},
	TypeKindChar:         {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindString:       {Size32: 8, Align32: 4, Size64: 16, Align64: 8, FlattenCount: 2}, // ptr+len; memory64 doubles
	TypeKindOwn:          {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindBorrow:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindStream:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindFuture:       {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
	TypeKindErrorContext: {Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1},
}

// ABI returns the canonical-ABI size/align/flatten info for a given
// ValType. Scalar kinds read from a package-level constant table;
// composite kinds dereference into *ComponentTypes.
func (v ValType) ABI(ct *ComponentTypes) CanonicalABIInfo {
	if v.Kind <= TypeKindString {
		return scalarABI[v.Kind]
	}
	switch v.Kind {
	case TypeKindRecord:
		return ct.Records[v.Index].ABI
	case TypeKindVariant:
		return ct.Variants[v.Index].ABI
	case TypeKindList:
		return ct.Lists[v.Index].ABI
	case TypeKindFixedList:
		return ct.FixedLists[v.Index].ABI
	case TypeKindTuple:
		return ct.Tuples[v.Index].ABI
	case TypeKindFlags:
		return ct.Flags[v.Index].ABI
	case TypeKindEnum:
		return ct.Enums[v.Index].ABI
	case TypeKindOption:
		return ct.Options[v.Index].ABI
	case TypeKindResult:
		return ct.Results[v.Index].ABI
	case TypeKindOwn, TypeKindBorrow, TypeKindStream, TypeKindFuture, TypeKindErrorContext:
		return scalarABI[v.Kind]
	}
	panic(fmt.Sprintf("ABI: unknown TypeKind %d", v.Kind))
}

// AlignTo rounds offset up to the given alignment. align must be a
// power of two.
func AlignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}

// computeRecordABI computes the canonical ABI info for a record with
// the given (already-interned) field types.
//
// Spec: alignment_record at definitions.py:1087-1091, elem_size_record
// at :1145-1151, flatten_record at :1726-1730. Wasmtime: record_static
// at types.rs:705-723.
//
// Empty records yield size 0, align 1 — divergence (1) from the literal
// spec which asserts s > 0 at definitions.py:1150. Both wasmtime and
// this design permit empty records.
func computeRecordABI(fields []RecordField, ct *ComponentTypes) CanonicalABIInfo {
	if len(fields) == 0 {
		return CanonicalABIInfo{Size32: 0, Align32: 1, Size64: 0, Align64: 1, FlattenCount: 0}
	}
	var size32, align32 uint32 = 0, 1
	var size64, align64 uint32 = 0, 1
	var flatten int32
	for _, f := range fields {
		fa := f.Type.ABI(ct)
		if fa.Align32 > align32 {
			align32 = fa.Align32
		}
		if fa.Align64 > align64 {
			align64 = fa.Align64
		}
		size32 = AlignTo(size32, fa.Align32) + fa.Size32
		size64 = AlignTo(size64, fa.Align64) + fa.Size64
		flatten += fa.FlattenCount
	}
	size32 = AlignTo(size32, align32)
	size64 = AlignTo(size64, align64)
	return CanonicalABIInfo{
		Size32: size32, Align32: align32,
		Size64: size64, Align64: align64,
		FlattenCount: flatten,
	}
}

// computeTupleABI is record ABI for positional types.
func computeTupleABI(elems []ValType, ct *ComponentTypes) CanonicalABIInfo {
	fs := make([]RecordField, len(elems))
	for i, e := range elems {
		fs[i] = RecordField{Type: e}
	}
	return computeRecordABI(fs, ct)
}

// discriminantSize returns the byte size of the discriminant for a
// variant with n cases. Spec: discriminant_type at definitions.py:1096-1103.
func discriminantSize(n int) uint8 {
	switch {
	case n <= 256:
		return 1
	case n <= 65536:
		return 2
	default:
		return 4
	}
}

// computeVariantABI computes ABI for a variant with the given
// (already-interned) cases.
//
// Spec: alignment_variant at definitions.py:1093-1094, elem_size_variant
// at :1156-1164, flatten_variant at :1732-1741, max_case_alignment at
// :1105-1110, join at :1743-1746.
func computeVariantABI(cases []VariantCase, ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
	disc := discriminantSize(len(cases))
	discA := uint32(disc)

	var maxCaseAlign32, maxCaseSize32 uint32 = 1, 0
	var maxCaseAlign64, maxCaseSize64 uint32 = 1, 0
	var maxCaseFlatten int32
	for _, c := range cases {
		if !c.HasPayload {
			continue
		}
		pa := c.Payload.ABI(ct)
		if pa.Align32 > maxCaseAlign32 {
			maxCaseAlign32 = pa.Align32
		}
		if pa.Align64 > maxCaseAlign64 {
			maxCaseAlign64 = pa.Align64
		}
		if pa.Size32 > maxCaseSize32 {
			maxCaseSize32 = pa.Size32
		}
		if pa.Size64 > maxCaseSize64 {
			maxCaseSize64 = pa.Size64
		}
		if pa.FlattenCount > maxCaseFlatten {
			maxCaseFlatten = pa.FlattenCount
		}
	}

	align32 := discA
	if maxCaseAlign32 > align32 {
		align32 = maxCaseAlign32
	}
	align64 := discA
	if maxCaseAlign64 > align64 {
		align64 = maxCaseAlign64
	}

	payloadOffset32 := AlignTo(discA, maxCaseAlign32)
	size32 := AlignTo(payloadOffset32+maxCaseSize32, align32)
	payloadOffset64 := AlignTo(discA, maxCaseAlign64)
	size64 := AlignTo(payloadOffset64+maxCaseSize64, align64)

	abi := CanonicalABIInfo{
		Size32: size32, Align32: align32,
		Size64: size64, Align64: align64,
		FlattenCount: 1 + maxCaseFlatten,
	}
	return abi, DiscriminantInfo{
		DiscSize:      disc,
		PayloadOffset: payloadOffset32,
	}
}

// computeListABI is the dynamic-list pointer-pair ABI.
// Spec: definitions.py:1075,1133,1714.
func computeListABI(_ ValType, _ *ComponentTypes) CanonicalABIInfo {
	return CanonicalABIInfo{
		Size32: 8, Align32: 4,
		Size64: 16, Align64: 8,
		FlattenCount: 2,
	}
}

// computeFixedListABI is the inline fixed-length-list ABI.
// Spec: alignment_list at :1082-1085, elem_size_list at :1140-1143,
// flatten_list at :1721-1723.
func computeFixedListABI(elem ValType, length uint32, ct *ComponentTypes) CanonicalABIInfo {
	ea := elem.ABI(ct)
	return CanonicalABIInfo{
		Size32: ea.Size32 * length, Align32: ea.Align32,
		Size64: ea.Size64 * length, Align64: ea.Align64,
		FlattenCount: ea.FlattenCount * int32(length),
	}
}

// computeFlagsABI is the flags-with-N-labels ABI.
// Spec: alignment_flags at :1112-1117, elem_size_flags at :1166-1171.
// Divergence (2): n > 32 is permitted via multi-i32 encoding, matching
// wasmtime's FlagsSize::Size4Plus(n).
func computeFlagsABI(names []string) CanonicalABIInfo {
	n := len(names)
	switch {
	case n <= 0:
		return CanonicalABIInfo{Size32: 0, Align32: 1, Size64: 0, Align64: 1, FlattenCount: 0}
	case n <= 8:
		return CanonicalABIInfo{Size32: 1, Align32: 1, Size64: 1, Align64: 1, FlattenCount: 1}
	case n <= 16:
		return CanonicalABIInfo{Size32: 2, Align32: 2, Size64: 2, Align64: 2, FlattenCount: 1}
	case n <= 32:
		return CanonicalABIInfo{Size32: 4, Align32: 4, Size64: 4, Align64: 4, FlattenCount: 1}
	default:
		words := uint32((n + 31) / 32)
		return CanonicalABIInfo{
			Size32: 4 * words, Align32: 4,
			Size64: 4 * words, Align64: 4,
			FlattenCount: int32(words),
		}
	}
}

// computeEnumABI is enum (discriminant only, no payloads).
func computeEnumABI(names []string) (CanonicalABIInfo, DiscriminantInfo) {
	disc := discriminantSize(len(names))
	d := uint32(disc)
	return CanonicalABIInfo{
			Size32: d, Align32: d,
			Size64: d, Align64: d,
			FlattenCount: 1,
		}, DiscriminantInfo{
			DiscSize:      disc,
			PayloadOffset: d,
		}
}

// computeOptionABI is sugar for variant{none, some(T)}.
func computeOptionABI(elem ValType, ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
	return computeVariantABI([]VariantCase{
		{Name: "none", HasPayload: false},
		{Name: "some", Payload: elem, HasPayload: true},
	}, ct)
}

// computeResultABI is sugar for variant{ok(OK), err(Err)} with the
// payloads conditionally present.
func computeResultABI(okT, errT ValType, hasOK, hasErr bool, ct *ComponentTypes) (CanonicalABIInfo, DiscriminantInfo) {
	cases := []VariantCase{
		{Name: "ok", Payload: okT, HasPayload: hasOK},
		{Name: "err", Payload: errT, HasPayload: hasErr},
	}
	return computeVariantABI(cases, ct)
}
