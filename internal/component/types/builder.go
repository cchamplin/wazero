// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
)

// FuncTypeIdx is the index of a TypeFunc in ComponentTypes.Funcs.
type FuncTypeIdx uint32

// ComponentTypesBuilder assembles a *ComponentTypes during decoding.
// After Finish() the builder is consumed; further Intern* calls panic.
// Go equivalent of Rust's "consumed self" idiom. The returned
// *ComponentTypes is safe for concurrent reads.
//
// Spec / wasmtime parity: ComponentTypesBuilder mirrors wasmtime's
// types_builder.rs:38-124. Each Intern method computes a structural
// hash, scans the bucket for an existing entry, and either returns
// the existing index or appends a new entry with precomputed ABI.
type ComponentTypesBuilder struct {
	ct       ComponentTypes
	finished bool

	recordIntern    map[uint64][]uint32
	variantIntern   map[uint64][]uint32
	listIntern      map[uint64][]uint32
	fixedListIntern map[uint64][]uint32
	tupleIntern     map[uint64][]uint32
	flagsIntern     map[uint64][]uint32
	enumIntern      map[uint64][]uint32
	optionIntern    map[uint64][]uint32
	resultIntern    map[uint64][]uint32
	streamIntern    map[uint64][]uint32
	futureIntern    map[uint64][]uint32
	errCtxIntern    map[uint64][]uint32
	funcIntern      map[uint64][]uint32
}

// NewComponentTypesBuilder creates an empty builder.
func NewComponentTypesBuilder() *ComponentTypesBuilder {
	return &ComponentTypesBuilder{
		recordIntern:    map[uint64][]uint32{},
		variantIntern:   map[uint64][]uint32{},
		listIntern:      map[uint64][]uint32{},
		fixedListIntern: map[uint64][]uint32{},
		tupleIntern:     map[uint64][]uint32{},
		flagsIntern:     map[uint64][]uint32{},
		enumIntern:      map[uint64][]uint32{},
		optionIntern:    map[uint64][]uint32{},
		resultIntern:    map[uint64][]uint32{},
		streamIntern:    map[uint64][]uint32{},
		futureIntern:    map[uint64][]uint32{},
		errCtxIntern:    map[uint64][]uint32{},
		funcIntern:      map[uint64][]uint32{},
	}
}

func (b *ComponentTypesBuilder) panicIfFinished() {
	if b.finished {
		panic("ComponentTypesBuilder: Intern* called after Finish")
	}
}

// --- hashing helpers ---

func hashU32(h hash.Hash64, v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	h.Write(buf[:])
}

func hashU8(h hash.Hash64, v uint8) {
	h.Write([]byte{v})
}

func hashString(h hash.Hash64, s string) {
	hashU32(h, uint32(len(s)))
	h.Write([]byte(s))
}

func hashValType(h hash.Hash64, v ValType) {
	hashU8(h, uint8(v.Kind))
	hashU32(h, v.Index)
}

func newHash() hash.Hash64 {
	return fnv.New64a()
}

// --- record ---

func (b *ComponentTypesBuilder) InternRecord(fields []RecordField) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(fields)))
	for _, f := range fields {
		hashString(h, f.Name)
		hashValType(h, f.Type)
	}
	key := h.Sum64()
	for _, idx := range b.recordIntern[key] {
		if recordsEqual(b.ct.Records[idx].Fields, fields) {
			return ValType{Kind: TypeKindRecord, Index: idx}
		}
	}
	abi := computeRecordABI(fields, &b.ct)
	idx := uint32(len(b.ct.Records))
	b.ct.Records = append(b.ct.Records, TypeRecord{Fields: append([]RecordField(nil), fields...), ABI: abi})
	b.recordIntern[key] = append(b.recordIntern[key], idx)
	return ValType{Kind: TypeKindRecord, Index: idx}
}

func recordsEqual(a, b []RecordField) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Type != b[i].Type {
			return false
		}
	}
	return true
}

// --- variant ---

func (b *ComponentTypesBuilder) InternVariant(cases []VariantCase) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(cases)))
	for _, c := range cases {
		hashString(h, c.Name)
		if c.HasPayload {
			hashU8(h, 1)
			hashValType(h, c.Payload)
		} else {
			hashU8(h, 0)
		}
	}
	key := h.Sum64()
	for _, idx := range b.variantIntern[key] {
		if variantsEqual(b.ct.Variants[idx].Cases, cases) {
			return ValType{Kind: TypeKindVariant, Index: idx}
		}
	}
	abi, disc := computeVariantABI(cases, &b.ct)
	idx := uint32(len(b.ct.Variants))
	b.ct.Variants = append(b.ct.Variants, TypeVariant{
		Cases: append([]VariantCase(nil), cases...),
		ABI:   abi,
		Disc:  disc,
	})
	b.variantIntern[key] = append(b.variantIntern[key], idx)
	return ValType{Kind: TypeKindVariant, Index: idx}
}

func variantsEqual(a, b []VariantCase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].HasPayload != b[i].HasPayload {
			return false
		}
		if a[i].HasPayload && a[i].Payload != b[i].Payload {
			return false
		}
	}
	return true
}

// --- list ---

func (b *ComponentTypesBuilder) InternList(elem ValType) ValType {
	b.panicIfFinished()
	h := newHash()
	hashValType(h, elem)
	key := h.Sum64()
	for _, idx := range b.listIntern[key] {
		if b.ct.Lists[idx].Element == elem {
			return ValType{Kind: TypeKindList, Index: idx}
		}
	}
	abi := computeListABI(elem, &b.ct)
	idx := uint32(len(b.ct.Lists))
	b.ct.Lists = append(b.ct.Lists, TypeList{Element: elem, ABI: abi})
	b.listIntern[key] = append(b.listIntern[key], idx)
	return ValType{Kind: TypeKindList, Index: idx}
}

// --- fixed-length list ---

func (b *ComponentTypesBuilder) InternFixedLengthList(elem ValType, length uint32) ValType {
	b.panicIfFinished()
	h := newHash()
	hashValType(h, elem)
	hashU32(h, length)
	key := h.Sum64()
	for _, idx := range b.fixedListIntern[key] {
		fl := &b.ct.FixedLists[idx]
		if fl.Element == elem && fl.Length == length {
			return ValType{Kind: TypeKindFixedList, Index: idx}
		}
	}
	abi := computeFixedListABI(elem, length, &b.ct)
	idx := uint32(len(b.ct.FixedLists))
	b.ct.FixedLists = append(b.ct.FixedLists, TypeFixedLengthList{
		Element: elem, Length: length, ABI: abi,
	})
	b.fixedListIntern[key] = append(b.fixedListIntern[key], idx)
	return ValType{Kind: TypeKindFixedList, Index: idx}
}

// --- tuple ---

func (b *ComponentTypesBuilder) InternTuple(elems []ValType) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(elems)))
	for _, e := range elems {
		hashValType(h, e)
	}
	key := h.Sum64()
	for _, idx := range b.tupleIntern[key] {
		if valTypesEqual(b.ct.Tuples[idx].Types, elems) {
			return ValType{Kind: TypeKindTuple, Index: idx}
		}
	}
	abi := computeTupleABI(elems, &b.ct)
	idx := uint32(len(b.ct.Tuples))
	b.ct.Tuples = append(b.ct.Tuples, TypeTuple{
		Types: append([]ValType(nil), elems...),
		ABI:   abi,
	})
	b.tupleIntern[key] = append(b.tupleIntern[key], idx)
	return ValType{Kind: TypeKindTuple, Index: idx}
}

func valTypesEqual(a, b []ValType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- flags ---

func (b *ComponentTypesBuilder) InternFlags(names []string) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(names)))
	for _, n := range names {
		hashString(h, n)
	}
	key := h.Sum64()
	for _, idx := range b.flagsIntern[key] {
		if stringsEqual(b.ct.Flags[idx].Names, names) {
			return ValType{Kind: TypeKindFlags, Index: idx}
		}
	}
	abi := computeFlagsABI(names)
	idx := uint32(len(b.ct.Flags))
	b.ct.Flags = append(b.ct.Flags, TypeFlags{
		Names: append([]string(nil), names...),
		ABI:   abi,
	})
	b.flagsIntern[key] = append(b.flagsIntern[key], idx)
	return ValType{Kind: TypeKindFlags, Index: idx}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- enum ---

func (b *ComponentTypesBuilder) InternEnum(names []string) ValType {
	b.panicIfFinished()
	h := newHash()
	hashU32(h, uint32(len(names)))
	for _, n := range names {
		hashString(h, n)
	}
	key := h.Sum64()
	for _, idx := range b.enumIntern[key] {
		if stringsEqual(b.ct.Enums[idx].Names, names) {
			return ValType{Kind: TypeKindEnum, Index: idx}
		}
	}
	abi, disc := computeEnumABI(names)
	idx := uint32(len(b.ct.Enums))
	b.ct.Enums = append(b.ct.Enums, TypeEnum{
		Names: append([]string(nil), names...),
		ABI:   abi,
		Disc:  disc,
	})
	b.enumIntern[key] = append(b.enumIntern[key], idx)
	return ValType{Kind: TypeKindEnum, Index: idx}
}

// --- option ---

func (b *ComponentTypesBuilder) InternOption(elem ValType) ValType {
	b.panicIfFinished()
	h := newHash()
	hashValType(h, elem)
	key := h.Sum64()
	for _, idx := range b.optionIntern[key] {
		if b.ct.Options[idx].Element == elem {
			return ValType{Kind: TypeKindOption, Index: idx}
		}
	}
	abi, disc := computeOptionABI(elem, &b.ct)
	idx := uint32(len(b.ct.Options))
	b.ct.Options = append(b.ct.Options, TypeOption{
		Element: elem, ABI: abi, Disc: disc,
	})
	b.optionIntern[key] = append(b.optionIntern[key], idx)
	return ValType{Kind: TypeKindOption, Index: idx}
}

// --- result ---

func (b *ComponentTypesBuilder) InternResult(okType, errType ValType, hasOk, hasErr bool) ValType {
	b.panicIfFinished()
	h := newHash()
	if hasOk {
		hashU8(h, 1)
		hashValType(h, okType)
	} else {
		hashU8(h, 0)
	}
	if hasErr {
		hashU8(h, 1)
		hashValType(h, errType)
	} else {
		hashU8(h, 0)
	}
	key := h.Sum64()
	for _, idx := range b.resultIntern[key] {
		r := &b.ct.Results[idx]
		if r.HasOK == hasOk && r.HasErr == hasErr {
			if (!hasOk || r.OK == okType) && (!hasErr || r.Err == errType) {
				return ValType{Kind: TypeKindResult, Index: idx}
			}
		}
	}
	abi, disc := computeResultABI(okType, errType, hasOk, hasErr, &b.ct)
	idx := uint32(len(b.ct.Results))
	b.ct.Results = append(b.ct.Results, TypeResult{
		OK: okType, Err: errType, HasOK: hasOk, HasErr: hasErr,
		ABI: abi, Disc: disc,
	})
	b.resultIntern[key] = append(b.resultIntern[key], idx)
	return ValType{Kind: TypeKindResult, Index: idx}
}

// --- stream / future / error-context ---

func (b *ComponentTypesBuilder) InternStream(elem ValType, hasElem bool) ValType {
	b.panicIfFinished()
	h := newHash()
	if hasElem {
		hashU8(h, 1)
		hashValType(h, elem)
	} else {
		hashU8(h, 0)
	}
	key := h.Sum64()
	for _, idx := range b.streamIntern[key] {
		s := &b.ct.Streams[idx]
		if s.HasElement == hasElem && (!hasElem || s.Element == elem) {
			return ValType{Kind: TypeKindStream, Index: idx}
		}
	}
	idx := uint32(len(b.ct.Streams))
	b.ct.Streams = append(b.ct.Streams, TypeStream{Element: elem, HasElement: hasElem})
	b.streamIntern[key] = append(b.streamIntern[key], idx)
	return ValType{Kind: TypeKindStream, Index: idx}
}

func (b *ComponentTypesBuilder) InternFuture(elem ValType, hasElem bool) ValType {
	b.panicIfFinished()
	h := newHash()
	if hasElem {
		hashU8(h, 1)
		hashValType(h, elem)
	} else {
		hashU8(h, 0)
	}
	key := h.Sum64()
	for _, idx := range b.futureIntern[key] {
		f := &b.ct.Futures[idx]
		if f.HasElement == hasElem && (!hasElem || f.Element == elem) {
			return ValType{Kind: TypeKindFuture, Index: idx}
		}
	}
	idx := uint32(len(b.ct.Futures))
	b.ct.Futures = append(b.ct.Futures, TypeFuture{Element: elem, HasElement: hasElem})
	b.futureIntern[key] = append(b.futureIntern[key], idx)
	return ValType{Kind: TypeKindFuture, Index: idx}
}

func (b *ComponentTypesBuilder) InternErrorContextTable() ValType {
	b.panicIfFinished()
	// Single canonical entry — no key.
	if len(b.ct.ErrorContextTables) == 0 {
		b.ct.ErrorContextTables = append(b.ct.ErrorContextTables, TypeErrorContextTable{})
	}
	return ValType{Kind: TypeKindErrorContext, Index: 0}
}

// --- resource handles ---

// InternAbstractResource creates a new Abstract TypeResourceTable entry
// and returns the index. Each call returns a fresh index — abstract
// resource declarations are distinct by construction. Concrete promotion
// at instantiation time is Session 2 work.
func (b *ComponentTypesBuilder) InternAbstractResource() ResourceTableIdx {
	b.panicIfFinished()
	idx := uint32(len(b.ct.ResourceTables))
	b.ct.ResourceTables = append(b.ct.ResourceTables, TypeResourceTable{
		Concrete:    false,
		AbstractIdx: idx,
	})
	return ResourceTableIdx(idx)
}

func (b *ComponentTypesBuilder) InternOwnHandle(rtIdx ResourceTableIdx) ValType {
	b.panicIfFinished()
	return ValType{Kind: TypeKindOwn, Index: uint32(rtIdx)}
}

func (b *ComponentTypesBuilder) InternBorrowHandle(rtIdx ResourceTableIdx) ValType {
	b.panicIfFinished()
	return ValType{Kind: TypeKindBorrow, Index: uint32(rtIdx)}
}

// --- function types ---

func (b *ComponentTypesBuilder) InternFunc(async bool, paramNames []string, params, results ValType) FuncTypeIdx {
	b.panicIfFinished()
	h := newHash()
	if async {
		hashU8(h, 1)
	} else {
		hashU8(h, 0)
	}
	hashU32(h, uint32(len(paramNames)))
	for _, n := range paramNames {
		hashString(h, n)
	}
	hashValType(h, params)
	hashValType(h, results)
	key := h.Sum64()
	for _, idx := range b.funcIntern[key] {
		f := &b.ct.Funcs[idx]
		if f.Async == async && stringsEqual(f.ParamNames, paramNames) &&
			f.Params == params && f.Results == results {
			return FuncTypeIdx(idx)
		}
	}
	idx := uint32(len(b.ct.Funcs))
	b.ct.Funcs = append(b.ct.Funcs, TypeFunc{
		Async:      async,
		ParamNames: append([]string(nil), paramNames...),
		Params:     params,
		Results:    results,
	})
	b.funcIntern[key] = append(b.funcIntern[key], idx)
	return FuncTypeIdx(idx)
}

// --- Finish ---

// Finish freezes the builder and returns the immutable *ComponentTypes.
// After Finish, further Intern* calls panic. The intern maps are nilled
// out so the returned *ComponentTypes carries only the slices.
func (b *ComponentTypesBuilder) Finish() *ComponentTypes {
	b.panicIfFinished()
	b.finished = true
	out := b.ct
	// Drop intern maps so the returned ComponentTypes is cheap to retain.
	b.recordIntern = nil
	b.variantIntern = nil
	b.listIntern = nil
	b.fixedListIntern = nil
	b.tupleIntern = nil
	b.flagsIntern = nil
	b.enumIntern = nil
	b.optionIntern = nil
	b.resultIntern = nil
	b.streamIntern = nil
	b.futureIntern = nil
	b.errCtxIntern = nil
	b.funcIntern = nil
	return &out
}
