package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// LowerContext provides context for lowering operations.
// For primitive types, the context is not used, but it will be required
// for composite types (strings, lists, records) that need memory allocation.
type LowerContext struct {
	Memory  Memory
	Opts    *Options
	Realloc func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
}

// LowerFlat lowers a component Val to flat core wasm values.
func LowerFlat(ctx *LowerContext, typ types.ValType, val component.Val) ([]uint64, error) {
	switch typ.(type) {
	case types.Bool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case types.S8:
		return []uint64{uint64(uint32(int32(val.S8())))}, nil
	case types.U8:
		return []uint64{uint64(val.U8())}, nil
	case types.S16:
		return []uint64{uint64(uint32(int32(val.S16())))}, nil
	case types.U16:
		return []uint64{uint64(val.U16())}, nil
	case types.S32:
		return []uint64{uint64(uint32(val.S32()))}, nil
	case types.U32:
		return []uint64{uint64(val.U32())}, nil
	case types.S64:
		return []uint64{uint64(val.S64())}, nil
	case types.U64:
		return []uint64{val.U64()}, nil
	case types.F32:
		return []uint64{uint64(math.Float32bits(val.F32()))}, nil
	case types.F64:
		return []uint64{math.Float64bits(val.F64())}, nil
	case types.Char:
		return []uint64{uint64(val.Char())}, nil
	case types.Record:
		t := typ.(types.Record)
		rec := val.Record()
		result := []uint64{}
		for _, f := range t.Fields {
			fieldVal, ok := rec[f.Name]
			if !ok {
				return nil, fmt.Errorf("missing record field: %s", f.Name)
			}
			flat, err := LowerFlat(ctx, f.Type, fieldVal)
			if err != nil {
				return nil, fmt.Errorf("lower record field %s: %w", f.Name, err)
			}
			result = append(result, flat...)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported flat lower for type: %T", typ)
	}
}
