package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// FlatIter iterates over flattened core wasm values.
type FlatIter struct {
	values []uint64
	pos    int
}

// NewFlatIter creates a new flat value iterator.
func NewFlatIter(values []uint64) *FlatIter {
	return &FlatIter{values: values}
}

// NextI32 returns the next value as i32.
func (f *FlatIter) NextI32() uint32 {
	v := f.values[f.pos]
	f.pos++
	return uint32(v)
}

// NextI64 returns the next value as i64.
func (f *FlatIter) NextI64() uint64 {
	v := f.values[f.pos]
	f.pos++
	return v
}

// NextF32 returns the next value as f32.
func (f *FlatIter) NextF32() float32 {
	v := f.values[f.pos]
	f.pos++
	return math.Float32frombits(uint32(v))
}

// NextF64 returns the next value as f64.
func (f *FlatIter) NextF64() float64 {
	v := f.values[f.pos]
	f.pos++
	return math.Float64frombits(v)
}

// LiftFlat lifts a flat representation to a component Val.
func LiftFlat(ctx *LiftContext, typ types.ValType, iter *FlatIter) (component.Val, error) {
	switch typ.(type) {
	case types.Bool:
		return component.ValBool(iter.NextI32() != 0), nil
	case types.S8:
		return component.ValS8(int8(iter.NextI32())), nil
	case types.U8:
		return component.ValU8(uint8(iter.NextI32())), nil
	case types.S16:
		return component.ValS16(int16(iter.NextI32())), nil
	case types.U16:
		return component.ValU16(uint16(iter.NextI32())), nil
	case types.S32:
		return component.ValS32(int32(iter.NextI32())), nil
	case types.U32:
		return component.ValU32(iter.NextI32()), nil
	case types.S64:
		return component.ValS64(int64(iter.NextI64())), nil
	case types.U64:
		return component.ValU64(iter.NextI64()), nil
	case types.F32:
		return component.ValF32(iter.NextF32()), nil
	case types.F64:
		return component.ValF64(iter.NextF64()), nil
	case types.Char:
		return component.ValChar(rune(iter.NextI32())), nil
	case types.Record:
		t := typ.(types.Record)
		fields := make(map[string]component.Val)
		for _, f := range t.Fields {
			fieldVal, err := LiftFlat(ctx, f.Type, iter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
			}
			fields[f.Name] = fieldVal
		}
		return component.ValRecord(fields), nil
	default:
		return component.Val{}, fmt.Errorf("unsupported flat lift for type: %T", typ)
	}
}
