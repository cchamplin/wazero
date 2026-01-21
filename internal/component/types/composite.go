// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "fmt"

// Field represents a named field in a record.
type Field struct {
	Name string
	Type ValType
}

// Record represents a record (struct) type with named fields.
type Record struct {
	Fields []Field
}

func (Record) valType() {}

func (r Record) Size() uint32 {
	if len(r.Fields) == 0 {
		// Per Canonical ABI spec, empty records have size 0.
		return 0
	}
	size := uint32(0)
	maxAlign := uint32(1)
	for _, f := range r.Fields {
		align := f.Type.Align()
		if align > maxAlign {
			maxAlign = align
		}
		// Align current offset
		size = alignTo(size, align)
		size += f.Type.Size()
	}
	// Pad to struct alignment
	return alignTo(size, maxAlign)
}

func (r Record) Align() uint32 {
	maxAlign := uint32(1)
	for _, f := range r.Fields {
		if a := f.Type.Align(); a > maxAlign {
			maxAlign = a
		}
	}
	return maxAlign
}

func (r Record) FlattenCount() int {
	count := 0
	for _, f := range r.Fields {
		count += f.Type.FlattenCount()
	}
	return count
}

// FieldOffsets returns the byte offset of each field in memory.
func (r Record) FieldOffsets() []uint32 {
	offsets := make([]uint32, len(r.Fields))
	offset := uint32(0)
	for i, f := range r.Fields {
		offset = alignTo(offset, f.Type.Align())
		offsets[i] = offset
		offset += f.Type.Size()
	}
	return offsets
}

// alignTo rounds offset up to the given alignment.
func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}

// Case represents a case in a variant type.
type Case struct {
	Name string
	Type ValType // nil for cases with no payload
}

// Variant represents a discriminated union type.
type Variant struct {
	Cases []Case
}

func (Variant) valType() {}

// DiscriminantSize returns the size of the discriminant in bytes.
func (v Variant) DiscriminantSize() uint32 {
	n := len(v.Cases)
	switch {
	case n <= 0x100: // 256
		return 1
	case n <= 0x10000: // 65536
		return 2
	default:
		return 4
	}
}

func (v Variant) Size() uint32 {
	discSize := v.DiscriminantSize()
	payloadSize := uint32(0)
	payloadAlign := uint32(1)
	for _, c := range v.Cases {
		if c.Type != nil {
			if s := c.Type.Size(); s > payloadSize {
				payloadSize = s
			}
			if a := c.Type.Align(); a > payloadAlign {
				payloadAlign = a
			}
		}
	}
	// discriminant + padding + payload, aligned to variant alignment
	offset := alignTo(discSize, payloadAlign)
	return alignTo(offset+payloadSize, v.Align())
}

func (v Variant) Align() uint32 {
	align := v.DiscriminantSize()
	for _, c := range v.Cases {
		if c.Type != nil {
			if a := c.Type.Align(); a > align {
				align = a
			}
		}
	}
	return align
}

func (v Variant) FlattenCount() int {
	// discriminant + max payload flattening
	maxPayload := 0
	for _, c := range v.Cases {
		if c.Type != nil {
			if n := c.Type.FlattenCount(); n > maxPayload {
				maxPayload = n
			}
		}
	}
	return 1 + maxPayload
}

// PayloadOffset returns the byte offset where payload data starts.
func (v Variant) PayloadOffset() uint32 {
	payloadAlign := uint32(1)
	for _, c := range v.Cases {
		if c.Type != nil {
			if a := c.Type.Align(); a > payloadAlign {
				payloadAlign = a
			}
		}
	}
	return alignTo(v.DiscriminantSize(), payloadAlign)
}

// List represents a list type.
// If Length is nil, it's a dynamic list (ptr + len in memory).
// If Length is set, it's a fixed-length list (inline elements).
type List struct {
	Element ValType
	Length  *uint32 // nil for dynamic lists, set for fixed-length
}

func (List) valType() {}

// Size returns the size of the list in memory.
// Dynamic lists: 8 bytes (ptr: i32, len: i32)
// Fixed lists: length * element_size
func (l List) Size() uint32 {
	if l.Length != nil {
		return *l.Length * l.Element.Size()
	}
	return 8 // ptr + len
}

// Align returns the alignment of the list.
// Dynamic lists: 4 (pointer alignment)
// Fixed lists: element alignment
func (l List) Align() uint32 {
	if l.Length != nil {
		return l.Element.Align()
	}
	return 4
}

// FlattenCount returns the number of core wasm values when flattened.
// Dynamic lists: 2 (pointer and length)
// Fixed lists: length * element_flatten_count
func (l List) FlattenCount() int {
	if l.Length != nil {
		return int(*l.Length) * l.Element.FlattenCount()
	}
	return 2
}

// ElementSize returns the size of each element.
func (l List) ElementSize() uint32 { return l.Element.Size() }

// ElementAlign returns the alignment of each element.
func (l List) ElementAlign() uint32 { return l.Element.Align() }

// IsFixedLength returns true if this is a fixed-length list.
func (l List) IsFixedLength() bool { return l.Length != nil }

// Option represents an optional value type (sugar for variant { none, some(T) }).
type Option struct {
	Some ValType
}

func (Option) valType() {}

func (o Option) Size() uint32 {
	return o.asVariant().Size()
}

func (o Option) Align() uint32 {
	return o.asVariant().Align()
}

func (o Option) FlattenCount() int {
	return o.asVariant().FlattenCount()
}

// asVariant returns the equivalent Variant representation.
func (o Option) asVariant() Variant {
	return Variant{
		Cases: []Case{
			{Name: "none", Type: nil},
			{Name: "some", Type: o.Some},
		},
	}
}

// Result represents a result type (sugar for variant { ok(T), error(E) }).
type Result struct {
	Ok    ValType // nil for result<_, E>
	Error ValType // nil for result<T, _>
}

func (Result) valType() {}

func (r Result) Size() uint32 {
	return r.asVariant().Size()
}

func (r Result) Align() uint32 {
	return r.asVariant().Align()
}

func (r Result) FlattenCount() int {
	return r.asVariant().FlattenCount()
}

// asVariant returns the equivalent Variant representation.
func (r Result) asVariant() Variant {
	return Variant{
		Cases: []Case{
			{Name: "ok", Type: r.Ok},
			{Name: "error", Type: r.Error},
		},
	}
}

// Enum represents an enumeration type (discriminant-only variant).
type Enum struct {
	Cases []string
}

func (Enum) valType() {}

func (e Enum) Size() uint32 {
	n := len(e.Cases)
	switch {
	case n <= 0x100: // 256
		return 1
	case n <= 0x10000: // 65536
		return 2
	default:
		return 4
	}
}

func (e Enum) Align() uint32 {
	return e.Size()
}

func (Enum) FlattenCount() int {
	return 1
}

// Flags represents a flags (bitfield) type.
type Flags struct {
	Names []string
}

func (Flags) valType() {}

func (f Flags) Size() uint32 {
	n := len(f.Names)
	switch {
	case n == 0:
		return 0
	case n <= 8:
		return 1
	case n <= 16:
		return 2
	default:
		// Round up to multiple of 32 bits
		return 4 * uint32((n+31)/32)
	}
}

func (f Flags) Align() uint32 {
	n := len(f.Names)
	switch {
	case n == 0:
		return 1
	case n <= 8:
		return 1
	case n <= 16:
		return 2
	default:
		return 4
	}
}

func (f Flags) FlattenCount() int {
	n := len(f.Names)
	if n == 0 {
		return 0
	}
	// Number of i32s needed
	return (n + 31) / 32
}

// Tuple represents a tuple type (positional record).
type Tuple struct {
	Types []ValType
}

func (Tuple) valType() {}

func (t Tuple) Size() uint32 {
	return t.asRecord().Size()
}

func (t Tuple) Align() uint32 {
	return t.asRecord().Align()
}

func (t Tuple) FlattenCount() int {
	return t.asRecord().FlattenCount()
}

// ElementOffsets returns the byte offset of each element in memory.
func (t Tuple) ElementOffsets() []uint32 {
	return t.asRecord().FieldOffsets()
}

// asRecord returns the equivalent Record representation.
func (t Tuple) asRecord() Record {
	fields := make([]Field, len(t.Types))
	for i, typ := range t.Types {
		fields[i] = Field{Name: fmt.Sprintf("%d", i), Type: typ}
	}
	return Record{Fields: fields}
}
