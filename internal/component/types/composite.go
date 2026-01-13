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
