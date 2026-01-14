package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

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
	case types.String:
		ptr, taggedLen, err := LowerString(ctx, val.StringVal())
		if err != nil {
			return nil, err
		}
		return []uint64{uint64(ptr), uint64(taggedLen)}, nil
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
	case types.Variant:
		t := typ.(types.Variant)
		caseName, payload := val.Variant()

		// Find case index
		caseIdx := -1
		var caseType types.ValType
		for i, c := range t.Cases {
			if c.Name == caseName {
				caseIdx = i
				caseType = c.Type
				break
			}
		}
		if caseIdx == -1 {
			return nil, fmt.Errorf("unknown variant case: %s", caseName)
		}

		if caseType != nil && payload == nil {
			return nil, fmt.Errorf("variant case %q requires a payload", caseName)
		}

		// Calculate max payload flatten count for padding
		maxPayloadFlat := 0
		for _, vc := range t.Cases {
			if vc.Type != nil {
				if n := vc.Type.FlattenCount(); n > maxPayloadFlat {
					maxPayloadFlat = n
				}
			}
		}

		result := []uint64{uint64(caseIdx)}
		payloadCount := 0
		if caseType != nil && payload != nil {
			flat, err := LowerFlat(ctx, caseType, *payload)
			if err != nil {
				return nil, fmt.Errorf("lower variant payload: %w", err)
			}
			result = append(result, flat...)
			payloadCount = len(flat)
		}

		// Add padding zeros for remaining slots
		for i := payloadCount; i < maxPayloadFlat; i++ {
			result = append(result, 0)
		}

		return result, nil

	case types.Tuple:
		t := typ.(types.Tuple)
		elems := val.Tuple()
		if len(elems) != len(t.Types) {
			return nil, fmt.Errorf("tuple has %d elements, expected %d", len(elems), len(t.Types))
		}
		result := []uint64{}
		for i, elemType := range t.Types {
			flat, err := LowerFlat(ctx, elemType, elems[i])
			if err != nil {
				return nil, fmt.Errorf("lower tuple element %d: %w", i, err)
			}
			result = append(result, flat...)
		}
		return result, nil

	case types.Option:
		t := typ.(types.Option)
		payload := val.Option()

		// Calculate payload flatten count for padding
		payloadFlat := 0
		if t.Some != nil {
			payloadFlat = t.Some.FlattenCount()
		}

		if payload == nil {
			// None: discriminant=0, padding
			result := []uint64{0}
			for i := 0; i < payloadFlat; i++ {
				result = append(result, 0)
			}
			return result, nil
		}
		// Some: discriminant=1, payload
		result := []uint64{1}
		if t.Some != nil {
			flat, err := LowerFlat(ctx, t.Some, *payload)
			if err != nil {
				return nil, fmt.Errorf("lower option payload: %w", err)
			}
			result = append(result, flat...)
		}
		return result, nil

	case types.Result:
		t := typ.(types.Result)
		isOk, okVal, errVal := val.Result()

		// Calculate max payload for padding
		okFlat, errFlat := 0, 0
		if t.Ok != nil {
			okFlat = t.Ok.FlattenCount()
		}
		if t.Error != nil {
			errFlat = t.Error.FlattenCount()
		}
		maxFlat := okFlat
		if errFlat > maxFlat {
			maxFlat = errFlat
		}

		if isOk {
			// Ok: discriminant=0, ok payload, padding
			result := []uint64{0}
			payloadCount := 0
			if t.Ok != nil && okVal != nil {
				flat, err := LowerFlat(ctx, t.Ok, *okVal)
				if err != nil {
					return nil, fmt.Errorf("lower result ok: %w", err)
				}
				result = append(result, flat...)
				payloadCount = len(flat)
			}
			for i := payloadCount; i < maxFlat; i++ {
				result = append(result, 0)
			}
			return result, nil
		}
		// Error: discriminant=1, error payload, padding
		result := []uint64{1}
		payloadCount := 0
		if t.Error != nil && errVal != nil {
			flat, err := LowerFlat(ctx, t.Error, *errVal)
			if err != nil {
				return nil, fmt.Errorf("lower result error: %w", err)
			}
			result = append(result, flat...)
			payloadCount = len(flat)
		}
		for i := payloadCount; i < maxFlat; i++ {
			result = append(result, 0)
		}
		return result, nil

	case types.Enum:
		t := typ.(types.Enum)
		caseName := val.Enum()
		for i, c := range t.Cases {
			if c == caseName {
				return []uint64{uint64(i)}, nil
			}
		}
		return nil, fmt.Errorf("unknown enum case: %s", caseName)

	case types.Flags:
		t := typ.(types.Flags)
		flags := val.Flags()
		if len(t.Names) == 0 {
			return []uint64{}, nil
		}
		// Calculate the number of i32s needed
		numI32s := (len(t.Names) + 31) / 32
		result := make([]uint64, numI32s)
		for i, name := range t.Names {
			if flags[name] {
				i32Idx := i / 32
				bit := i % 32
				result[i32Idx] |= 1 << bit
			}
		}
		return result, nil

	case types.List:
		// TODO: List flat representation is [ptr, len]. Actual element lowering
		// requires heap allocation via LowerContext.Realloc. For now, return
		// placeholder zeros. Full implementation in heap lower.
		return []uint64{0, 0}, nil

	default:
		return nil, fmt.Errorf("unsupported flat lower for type: %T", typ)
	}
}

// LowerHeap lowers a component Val to heap memory at the given offset.
func LowerHeap(ctx *LowerContext, typ types.ValType, val component.Val, offset uint32) error {
	switch typ.(type) {
	case types.Bool:
		if val.Bool() {
			writeUint8(ctx.Memory, offset, 1)
		} else {
			writeUint8(ctx.Memory, offset, 0)
		}
		return nil
	case types.S8:
		writeUint8(ctx.Memory, offset, uint8(val.S8()))
		return nil
	case types.U8:
		writeUint8(ctx.Memory, offset, val.U8())
		return nil
	case types.S16:
		writeUint16Le(ctx.Memory, offset, uint16(val.S16()))
		return nil
	case types.U16:
		writeUint16Le(ctx.Memory, offset, val.U16())
		return nil
	case types.S32:
		writeUint32Le(ctx.Memory, offset, uint32(val.S32()))
		return nil
	case types.U32:
		writeUint32Le(ctx.Memory, offset, val.U32())
		return nil
	case types.S64:
		writeUint64Le(ctx.Memory, offset, uint64(val.S64()))
		return nil
	case types.U64:
		writeUint64Le(ctx.Memory, offset, val.U64())
		return nil
	case types.F32:
		writeUint32Le(ctx.Memory, offset, math.Float32bits(val.F32()))
		return nil
	case types.F64:
		writeUint64Le(ctx.Memory, offset, math.Float64bits(val.F64()))
		return nil
	case types.Char:
		writeUint32Le(ctx.Memory, offset, uint32(val.Char()))
		return nil
	case types.String:
		ptr, taggedLen, err := LowerString(ctx, val.StringVal())
		if err != nil {
			return err
		}
		writeUint32Le(ctx.Memory, offset, ptr)
		writeUint32Le(ctx.Memory, offset+4, taggedLen)
		return nil
	default:
		return fmt.Errorf("unsupported heap lower for type: %T", typ)
	}
}

// LowerOwn receives ownership of a resource into the component.
// Creates a new owned handle in the table and returns its index.
// This is the opposite of LiftOwn - it takes a representation value
// and creates a new handle in the component's resource table.
func LowerOwn(ctx *LowerContext, rep any) (uint32, error) {
	if ctx.ResourceTable == nil {
		return 0, fmt.Errorf("lower_own: no resource table available")
	}

	h := ctx.ResourceTable.New(rep, true)
	return h.Index(), nil
}

// LowerBorrow receives a borrowed resource into the component.
// Creates a borrowed handle in the table and tracks it in CallContext.
// This implements canon_lower for borrow<T> types.
func LowerBorrow(ctx *LowerContext, rep any) (uint32, error) {
	if ctx.ResourceTable == nil {
		return 0, fmt.Errorf("lower_borrow: no resource table available")
	}

	h := ctx.ResourceTable.New(rep, false) // own=false for borrowed

	// Track borrow in call context for return validation
	if ctx.CallContext != nil {
		ctx.CallContext.IncrementBorrows()
	}

	return h.Index(), nil
}
