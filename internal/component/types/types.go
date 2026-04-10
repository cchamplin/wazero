// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// TypeKind discriminates the variants of ValType. For scalar kinds the
// Index field of ValType is unused. For composite kinds Index points
// into the corresponding slice on *ComponentTypes.
//
// Spec: debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:103-180
type TypeKind uint8

const (
	TypeKindBool TypeKind = iota
	TypeKindS8
	TypeKindU8
	TypeKindS16
	TypeKindU16
	TypeKindS32
	TypeKindU32
	TypeKindS64
	TypeKindU64
	TypeKindF32
	TypeKindF64
	TypeKindChar
	TypeKindString
	TypeKindList         // Index -> ComponentTypes.Lists (dynamic length)
	TypeKindFixedList    // Index -> ComponentTypes.FixedLists (fixed length, distinct type)
	TypeKindRecord       // Index -> ComponentTypes.Records
	TypeKindTuple        // Index -> ComponentTypes.Tuples
	TypeKindVariant      // Index -> ComponentTypes.Variants
	TypeKindEnum         // Index -> ComponentTypes.Enums
	TypeKindOption       // Index -> ComponentTypes.Options
	TypeKindResult       // Index -> ComponentTypes.Results
	TypeKindFlags        // Index -> ComponentTypes.Flags
	TypeKindOwn          // Index -> ComponentTypes.ResourceTables
	TypeKindBorrow       // Index -> ComponentTypes.ResourceTables
	TypeKindStream       // Index -> ComponentTypes.Streams (lift/lower traps)
	TypeKindFuture       // Index -> ComponentTypes.Futures (lift/lower traps)
	TypeKindErrorContext // Index -> ComponentTypes.ErrorContextTables (lift/lower traps)
)

// ValType identifies a single component-model value type. 8 bytes total.
// Comparable with ==, usable as a map key, copyable by value. Pass by
// value through lift/lower dispatch.
//
// For scalar kinds (TypeKindBool through TypeKindString), Index is zero
// and ignored. For composite kinds Index is the offset into the matching
// ComponentTypes slice.
type ValType struct {
	Kind  TypeKind
	Index uint32
}

// IsZero reports whether v is the zero ValType. Zero is distinguishable
// from a legitimate TypeKindBool value only by context; the builder
// never returns a zero ValType.
func (v ValType) IsZero() bool { return v == ValType{} }

// Named scalar constants. These are the only non-composite ValType
// values that can be constructed without a builder.
var (
	Bool    = ValType{Kind: TypeKindBool}
	S8      = ValType{Kind: TypeKindS8}
	U8      = ValType{Kind: TypeKindU8}
	S16     = ValType{Kind: TypeKindS16}
	U16     = ValType{Kind: TypeKindU16}
	S32     = ValType{Kind: TypeKindS32}
	U32     = ValType{Kind: TypeKindU32}
	S64     = ValType{Kind: TypeKindS64}
	U64     = ValType{Kind: TypeKindU64}
	F32     = ValType{Kind: TypeKindF32}
	F64     = ValType{Kind: TypeKindF64}
	Char    = ValType{Kind: TypeKindChar}
	String_ = ValType{Kind: TypeKindString}
)

// StringEncoding specifies the string encoding for Canonical ABI.
//
// Spec: definitions.py StringEncoding enum (UTF-8, UTF-16, Latin1+UTF-16).
type StringEncoding uint8

const (
	StringEncodingUTF8 StringEncoding = iota
	StringEncodingUTF16
	StringEncodingLatin1UTF16
)

// ComponentTypes is the per-top-level-component immutable type bag.
// Built by ComponentTypesBuilder during binary decode, frozen at Finish,
// and threaded through all subsequent lift/lower / validation / linking.
// One pointer identity per compiled component drives the fast-path
// type-equality short-circuit during cross-component type checking
// (added in Session 2).
type ComponentTypes struct {
	Records            []TypeRecord
	Variants           []TypeVariant
	Lists              []TypeList            // dynamic-length lists only
	FixedLists         []TypeFixedLengthList // fixed-length lists are a distinct type
	Tuples             []TypeTuple
	Flags              []TypeFlags
	Enums              []TypeEnum
	Options            []TypeOption
	Results            []TypeResult
	ResourceTables     []TypeResourceTable
	Streams            []TypeStream
	Futures            []TypeFuture
	ErrorContextTables []TypeErrorContextTable
	Funcs              []TypeFunc
}
