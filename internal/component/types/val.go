// internal/component/types/val.go

package types

import "fmt"

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

// StringVal returns the string value. Panics if Kind() != ValKindString.
func (v Val) StringVal() string { return v.v.(string) }

// ValRecord creates a record value from field name to value map.
func ValRecord(fields map[string]Val) Val {
	return Val{kind: ValKindRecord, v: fields}
}

// Record returns the value as a record (map of field name to value).
// Panics if Kind() != ValKindRecord.
func (v Val) Record() map[string]Val {
	if v.kind != ValKindRecord {
		panic("Val is not a record")
	}
	return v.v.(map[string]Val)
}

// RecordField returns a specific field from a record value.
// Returns the field value and true if found, or zero Val and false if not found.
// Panics if Kind() != ValKindRecord.
func (v Val) RecordField(name string) (Val, bool) {
	r := v.Record()
	val, ok := r[name]
	return val, ok
}

// variantVal holds a variant's case name and optional payload.
type variantVal struct {
	caseName string
	payload  *Val
}

// ValVariant creates a variant value with the given case and optional payload.
func ValVariant(caseName string, payload *Val) Val {
	return Val{kind: ValKindVariant, v: variantVal{caseName: caseName, payload: payload}}
}

// Variant returns the variant's case name and optional payload.
func (v Val) Variant() (string, *Val) {
	if v.kind != ValKindVariant {
		panic("Val is not a variant")
	}
	vv := v.v.(variantVal)
	return vv.caseName, vv.payload
}

// ValOption creates an option value (Some or None).
func ValOption(payload *Val) Val {
	return Val{kind: ValKindOption, v: payload}
}

// Option returns the option's payload (nil for None).
func (v Val) Option() *Val {
	if v.kind != ValKindOption {
		panic("Val is not an option")
	}
	return v.v.(*Val)
}

// ValList creates a list value from a slice of elements.
func ValList(elements []Val) Val {
	return Val{kind: ValKindList, v: elements}
}

// List returns the list elements.
// Panics if Kind() != ValKindList.
func (v Val) List() []Val {
	if v.kind != ValKindList {
		panic("Val is not a list")
	}
	return v.v.([]Val)
}

// ValTuple creates a tuple value from a slice of elements.
func ValTuple(elements []Val) Val {
	return Val{kind: ValKindTuple, v: elements}
}

// Tuple returns the tuple elements.
// Panics if Kind() != ValKindTuple.
func (v Val) Tuple() []Val {
	if v.kind != ValKindTuple {
		panic("Val is not a tuple")
	}
	return v.v.([]Val)
}

// resultVal holds a result's ok/error state and value.
type resultVal struct {
	isOk bool
	ok   *Val
	err  *Val
}

// ValResultOk creates a result value representing success with an optional value.
func ValResultOk(ok *Val) Val {
	return Val{kind: ValKindResult, v: resultVal{isOk: true, ok: ok, err: nil}}
}

// ValResultError creates a result value representing failure with an optional error value.
func ValResultError(err *Val) Val {
	return Val{kind: ValKindResult, v: resultVal{isOk: false, ok: nil, err: err}}
}

// Result returns whether the result is ok, and the ok/error values.
// Panics if Kind() != ValKindResult.
func (v Val) Result() (isOk bool, ok *Val, err *Val) {
	if v.kind != ValKindResult {
		panic("Val is not a result")
	}
	rv := v.v.(resultVal)
	return rv.isOk, rv.ok, rv.err
}

// ValFlags creates a flags value from a map of flag names to boolean values.
func ValFlags(flags map[string]bool) Val {
	return Val{kind: ValKindFlags, v: flags}
}

// Flags returns the flags as a map of flag names to boolean values.
// Panics if Kind() != ValKindFlags.
func (v Val) Flags() map[string]bool {
	if v.kind != ValKindFlags {
		panic("Val is not a flags")
	}
	return v.v.(map[string]bool)
}

// ValEnum creates an enum value with the given case name.
func ValEnum(caseName string) Val {
	return Val{kind: ValKindEnum, v: caseName}
}

// Enum returns the enum case name.
// Panics if Kind() != ValKindEnum.
func (v Val) Enum() string {
	if v.kind != ValKindEnum {
		panic("Val is not an enum")
	}
	return v.v.(string)
}

// ValOwn creates a Val containing an owning handle.
func ValOwn(handle uint32) Val {
	return Val{kind: ValKindOwn, v: handle}
}

// ValBorrow creates a Val containing a borrowed handle.
func ValBorrow(handle uint32) Val {
	return Val{kind: ValKindBorrow, v: handle}
}

// Own returns the handle index for an own handle.
// Panics if Kind() != ValKindOwn.
func (v Val) Own() uint32 {
	if v.kind != ValKindOwn {
		panic("Val is not an own handle")
	}
	return v.v.(uint32)
}

// Borrow returns the handle index for a borrowed handle.
// Panics if Kind() != ValKindBorrow.
func (v Val) Borrow() uint32 {
	if v.kind != ValKindBorrow {
		panic("Val is not a borrowed handle")
	}
	return v.v.(uint32)
}

// String returns a string representation of the ValKind for debugging.
func (k ValKind) String() string {
	switch k {
	case ValKindBool:
		return "bool"
	case ValKindS8:
		return "s8"
	case ValKindU8:
		return "u8"
	case ValKindS16:
		return "s16"
	case ValKindU16:
		return "u16"
	case ValKindS32:
		return "s32"
	case ValKindU32:
		return "u32"
	case ValKindS64:
		return "s64"
	case ValKindU64:
		return "u64"
	case ValKindF32:
		return "f32"
	case ValKindF64:
		return "f64"
	case ValKindChar:
		return "char"
	case ValKindString:
		return "string"
	case ValKindList:
		return "list"
	case ValKindRecord:
		return "record"
	case ValKindTuple:
		return "tuple"
	case ValKindVariant:
		return "variant"
	case ValKindEnum:
		return "enum"
	case ValKindOption:
		return "option"
	case ValKindResult:
		return "result"
	case ValKindFlags:
		return "flags"
	case ValKindOwn:
		return "own"
	case ValKindBorrow:
		return "borrow"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}
