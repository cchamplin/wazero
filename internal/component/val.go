// internal/component/val.go

package component

// Val represents a dynamically-typed component model value.
// Used when function signatures aren't known at compile time.
type Val struct {
	kind ValKind
	v    any
}

// ValKind identifies the type of a Val.
type ValKind uint8

const (
	ValKindBool ValKind = iota
	ValKindS8
	ValKindU8
	ValKindS16
	ValKindU16
	ValKindS32
	ValKindU32
	ValKindS64
	ValKindU64
	ValKindF32
	ValKindF64
	ValKindChar
	ValKindString
	ValKindList
	ValKindRecord
	ValKindTuple
	ValKindVariant
	ValKindEnum
	ValKindOption
	ValKindResult
	ValKindFlags
	ValKindOwn
	ValKindBorrow
)

// Kind returns the type of this value.
func (v Val) Kind() ValKind { return v.kind }

// Constructors

// ValBool creates a boolean Val.
func ValBool(b bool) Val { return Val{ValKindBool, b} }

// ValS8 creates a signed 8-bit integer Val.
func ValS8(n int8) Val { return Val{ValKindS8, n} }

// ValU8 creates an unsigned 8-bit integer Val.
func ValU8(n uint8) Val { return Val{ValKindU8, n} }

// ValS16 creates a signed 16-bit integer Val.
func ValS16(n int16) Val { return Val{ValKindS16, n} }

// ValU16 creates an unsigned 16-bit integer Val.
func ValU16(n uint16) Val { return Val{ValKindU16, n} }

// ValS32 creates a signed 32-bit integer Val.
func ValS32(n int32) Val { return Val{ValKindS32, n} }

// ValU32 creates an unsigned 32-bit integer Val.
func ValU32(n uint32) Val { return Val{ValKindU32, n} }

// ValS64 creates a signed 64-bit integer Val.
func ValS64(n int64) Val { return Val{ValKindS64, n} }

// ValU64 creates an unsigned 64-bit integer Val.
func ValU64(n uint64) Val { return Val{ValKindU64, n} }

// ValF32 creates a 32-bit floating point Val.
func ValF32(f float32) Val { return Val{ValKindF32, f} }

// ValF64 creates a 64-bit floating point Val.
func ValF64(f float64) Val { return Val{ValKindF64, f} }

// ValChar creates a Unicode character Val.
func ValChar(c rune) Val { return Val{ValKindChar, c} }

// ValString creates a string Val.
func ValString(s string) Val { return Val{ValKindString, s} }

// Accessors

// Bool returns the boolean value. Panics if Kind() != ValKindBool.
func (v Val) Bool() bool { return v.v.(bool) }

// S8 returns the int8 value. Panics if Kind() != ValKindS8.
func (v Val) S8() int8 { return v.v.(int8) }

// U8 returns the uint8 value. Panics if Kind() != ValKindU8.
func (v Val) U8() uint8 { return v.v.(uint8) }

// S16 returns the int16 value. Panics if Kind() != ValKindS16.
func (v Val) S16() int16 { return v.v.(int16) }

// U16 returns the uint16 value. Panics if Kind() != ValKindU16.
func (v Val) U16() uint16 { return v.v.(uint16) }

// S32 returns the int32 value. Panics if Kind() != ValKindS32.
func (v Val) S32() int32 { return v.v.(int32) }

// U32 returns the uint32 value. Panics if Kind() != ValKindU32.
func (v Val) U32() uint32 { return v.v.(uint32) }

// S64 returns the int64 value. Panics if Kind() != ValKindS64.
func (v Val) S64() int64 { return v.v.(int64) }

// U64 returns the uint64 value. Panics if Kind() != ValKindU64.
func (v Val) U64() uint64 { return v.v.(uint64) }

// F32 returns the float32 value. Panics if Kind() != ValKindF32.
func (v Val) F32() float32 { return v.v.(float32) }

// F64 returns the float64 value. Panics if Kind() != ValKindF64.
func (v Val) F64() float64 { return v.v.(float64) }

// Char returns the rune value. Panics if Kind() != ValKindChar.
func (v Val) Char() rune { return v.v.(rune) }

// String returns the string value. Panics if Kind() != ValKindString.
func (v Val) String() string { return v.v.(string) }
