// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// RecordField is one field of a record type. Order is significant
// (spec-defined); names are unique within the record.
type RecordField struct {
	Name string
	Type ValType
}

// TypeRecord is a record (struct) type with named, ordered fields.
// Spec: definitions.py:118-121 (RecordType).
type TypeRecord struct {
	Fields []RecordField
	ABI    CanonicalABIInfo
}

// VariantCase is one case of a variant type. HasPayload distinguishes
// the unit case from the payload case (Payload is zero-valued iff
// HasPayload is false).
type VariantCase struct {
	Name       string
	Payload    ValType
	HasPayload bool
}

// TypeVariant is a discriminated-union variant type.
// Spec: definitions.py:128-132 (VariantType).
type TypeVariant struct {
	Cases []VariantCase
	ABI   CanonicalABIInfo
	Disc  DiscriminantInfo
}

// TypeList is a dynamic-length list. Memory layout is (ptr: i32, len: i32).
// Fixed-length lists are a distinct type — see TypeFixedLengthList.
// Spec: definitions.py:122-125 (ListType with l == None).
type TypeList struct {
	Element ValType
	ABI     CanonicalABIInfo
}

// TypeFixedLengthList is a list with a compile-time-known length. Memory
// layout is `length` elements stored inline, not via ptr+len indirection.
// Distinct from TypeList because spec and wasmtime treat them as distinct
// types: a function expecting `list<u32>` cannot accept a `list<u32, 5>`
// and vice versa.
//
// Spec: definitions.py:122-125 — `ListType(t, l)` with `l != None`.
// Wasmtime: environ/src/component/types.rs uses TypeListIndex (lists)
// and TypeFixedLengthListIndex (fixed-length lists) as distinct keys.
type TypeFixedLengthList struct {
	Element ValType
	Length  uint32 // > 0 per spec
	ABI     CanonicalABIInfo
}

// TypeTuple is a positional record (anonymous struct).
// Spec: definitions.py:126-127 (TupleType).
type TypeTuple struct {
	Types []ValType
	ABI   CanonicalABIInfo
}

// TypeFlags is a set of named boolean flags packed into i32 words.
// Spec: definitions.py:166-168 (FlagsType).
type TypeFlags struct {
	Names []string
	ABI   CanonicalABIInfo
}

// TypeEnum is a discriminant-only variant (no payloads).
// Spec: definitions.py:163-165 (EnumType).
type TypeEnum struct {
	Names []string
	ABI   CanonicalABIInfo
	Disc  DiscriminantInfo
}

// TypeOption is syntactic sugar for variant{none, some(T)}.
// Spec: definitions.py:160-162 (OptionType).
type TypeOption struct {
	Element ValType
	ABI     CanonicalABIInfo
	Disc    DiscriminantInfo
}

// TypeResult is syntactic sugar for variant{ok(T), error(E)}.
// Spec: definitions.py:155-159 (ResultType).
type TypeResult struct {
	OK     ValType
	Err    ValType
	HasOK  bool
	HasErr bool
	ABI    CanonicalABIInfo
	Disc   DiscriminantInfo
}

// TypeStream is an async stream-of-element type. Lift/lower traps
// in Session 0; per-instance table identity is added when async lands.
type TypeStream struct {
	Element    ValType
	HasElement bool
}

// TypeFuture is an async future-of-element type. Lift/lower traps
// in Session 0; per-instance table identity is added when async lands.
type TypeFuture struct {
	Element    ValType
	HasElement bool
}

// TypeErrorContextTable is intentionally empty for Session 0. The
// per-instance table identity layering analogous to TypeResourceTable
// is added when async lift/lower lands.
type TypeErrorContextTable struct{}

// TypeFunc represents a component function type. Matches the wasmtime
// shape (environ/src/component/types.rs:557-566): params and results
// are each a Tuple (a ValType of TypeKindTuple). One mechanism for
// "ordered list of types," reused.
type TypeFunc struct {
	Async      bool
	ParamNames []string
	Params     ValType // TypeKindTuple
	Results    ValType // TypeKindTuple
}
