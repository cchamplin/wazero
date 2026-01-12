// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// ValType represents any component model value type.
type ValType interface {
	valType() // marker method

	// Size returns the byte size when stored in linear memory.
	Size() uint32

	// Align returns the alignment requirement in bytes.
	Align() uint32

	// FlattenCount returns the number of core wasm values when flattened.
	// Used to determine if values pass in registers (flat) or memory (heap).
	FlattenCount() int
}

// Bool represents a boolean value.
type Bool struct{}

func (Bool) valType()          {}
func (Bool) Size() uint32      { return 1 }
func (Bool) Align() uint32     { return 1 }
func (Bool) FlattenCount() int { return 1 }

// S8 represents a signed 8-bit integer.
type S8 struct{}

func (S8) valType()          {}
func (S8) Size() uint32      { return 1 }
func (S8) Align() uint32     { return 1 }
func (S8) FlattenCount() int { return 1 }

// U8 represents an unsigned 8-bit integer.
type U8 struct{}

func (U8) valType()          {}
func (U8) Size() uint32      { return 1 }
func (U8) Align() uint32     { return 1 }
func (U8) FlattenCount() int { return 1 }

// S16 represents a signed 16-bit integer.
type S16 struct{}

func (S16) valType()          {}
func (S16) Size() uint32      { return 2 }
func (S16) Align() uint32     { return 2 }
func (S16) FlattenCount() int { return 1 }

// U16 represents an unsigned 16-bit integer.
type U16 struct{}

func (U16) valType()          {}
func (U16) Size() uint32      { return 2 }
func (U16) Align() uint32     { return 2 }
func (U16) FlattenCount() int { return 1 }

// S32 represents a signed 32-bit integer.
type S32 struct{}

func (S32) valType()          {}
func (S32) Size() uint32      { return 4 }
func (S32) Align() uint32     { return 4 }
func (S32) FlattenCount() int { return 1 }

// U32 represents an unsigned 32-bit integer.
type U32 struct{}

func (U32) valType()          {}
func (U32) Size() uint32      { return 4 }
func (U32) Align() uint32     { return 4 }
func (U32) FlattenCount() int { return 1 }

// S64 represents a signed 64-bit integer.
type S64 struct{}

func (S64) valType()          {}
func (S64) Size() uint32      { return 8 }
func (S64) Align() uint32     { return 8 }
func (S64) FlattenCount() int { return 1 }

// U64 represents an unsigned 64-bit integer.
type U64 struct{}

func (U64) valType()          {}
func (U64) Size() uint32      { return 8 }
func (U64) Align() uint32     { return 8 }
func (U64) FlattenCount() int { return 1 }

// F32 represents a 32-bit floating point number.
type F32 struct{}

func (F32) valType()          {}
func (F32) Size() uint32      { return 4 }
func (F32) Align() uint32     { return 4 }
func (F32) FlattenCount() int { return 1 }

// F64 represents a 64-bit floating point number.
type F64 struct{}

func (F64) valType()          {}
func (F64) Size() uint32      { return 8 }
func (F64) Align() uint32     { return 8 }
func (F64) FlattenCount() int { return 1 }

// Char represents a Unicode scalar value (code point).
type Char struct{}

func (Char) valType()          {}
func (Char) Size() uint32      { return 4 }
func (Char) Align() uint32     { return 4 }
func (Char) FlattenCount() int { return 1 }

// String represents a UTF-8 encoded string.
// In memory: (ptr: i32, len: i32)
type String struct{}

func (String) valType()          {}
func (String) Size() uint32      { return 8 } // ptr + len
func (String) Align() uint32     { return 4 } // aligned to i32
func (String) FlattenCount() int { return 2 } // ptr, len
