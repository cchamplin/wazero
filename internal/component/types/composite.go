// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

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

// List represents a variable-length list type.
type List struct {
	Element ValType
}

func (List) valType() {}

// Size returns the size of the list in memory (pointer + length).
func (List) Size() uint32 { return 8 } // ptr: i32, len: i32

// Align returns the alignment of the list (i32 alignment).
func (List) Align() uint32 { return 4 }

// FlattenCount returns 2 (pointer and length).
func (List) FlattenCount() int { return 2 }

// ElementSize returns the size of each element.
func (l List) ElementSize() uint32 { return l.Element.Size() }

// ElementAlign returns the alignment of each element.
func (l List) ElementAlign() uint32 { return l.Element.Align() }

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
