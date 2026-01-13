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
	case types.String:
		ptr := iter.NextI32()
		taggedLen := iter.NextI32()
		s, err := liftStringFromPtrLen(ctx, ptr, taggedLen)
		if err != nil {
			return component.Val{}, err
		}
		return component.ValString(s), nil
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
	case types.Variant:
		t := typ.(types.Variant)
		disc := iter.NextI32()
		if int(disc) >= len(t.Cases) {
			return component.Val{}, fmt.Errorf("invalid variant discriminant: %d", disc)
		}
		c := t.Cases[disc]

		// Calculate max payload flatten count for padding
		maxPayloadFlat := 0
		for _, vc := range t.Cases {
			if vc.Type != nil {
				if n := vc.Type.FlattenCount(); n > maxPayloadFlat {
					maxPayloadFlat = n
				}
			}
		}

		var payload *component.Val
		payloadConsumed := 0
		if c.Type != nil {
			p, err := LiftFlat(ctx, c.Type, iter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift variant payload: %w", err)
			}
			payload = &p
			payloadConsumed = c.Type.FlattenCount()
		}

		// Skip remaining padding values
		for i := payloadConsumed; i < maxPayloadFlat; i++ {
			iter.NextI64() // Consume as i64 (largest type)
		}

		return component.ValVariant(c.Name, payload), nil

	case types.Tuple:
		t := typ.(types.Tuple)
		elems := make([]component.Val, len(t.Types))
		for i, elemType := range t.Types {
			elemVal, err := LiftFlat(ctx, elemType, iter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift tuple element %d: %w", i, err)
			}
			elems[i] = elemVal
		}
		return component.ValTuple(elems), nil

	case types.Option:
		t := typ.(types.Option)
		disc := iter.NextI32()
		payloadFlat := 0
		if t.Some != nil {
			payloadFlat = t.Some.FlattenCount()
		}
		if disc == 0 {
			// None - skip payload slots
			for i := 0; i < payloadFlat; i++ {
				iter.NextI64()
			}
			return component.ValOption(nil), nil
		}
		// Some
		if t.Some != nil {
			payload, err := LiftFlat(ctx, t.Some, iter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift option payload: %w", err)
			}
			return component.ValOption(&payload), nil
		}
		// Unit option (Some with no payload type) - return empty Val as marker
		emptyVal := component.Val{}
		return component.ValOption(&emptyVal), nil

	case types.Result:
		t := typ.(types.Result)
		disc := iter.NextI32()
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

		if disc == 0 {
			// Ok
			if t.Ok != nil {
				okVal, err := LiftFlat(ctx, t.Ok, iter)
				if err != nil {
					return component.Val{}, fmt.Errorf("lift result ok: %w", err)
				}
				// Skip remaining padding
				for i := okFlat; i < maxFlat; i++ {
					iter.NextI64()
				}
				return component.ValResultOk(&okVal), nil
			}
			for i := 0; i < maxFlat; i++ {
				iter.NextI64()
			}
			return component.ValResultOk(nil), nil
		}
		// Error
		if t.Error != nil {
			errVal, err := LiftFlat(ctx, t.Error, iter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift result error: %w", err)
			}
			for i := errFlat; i < maxFlat; i++ {
				iter.NextI64()
			}
			return component.ValResultError(&errVal), nil
		}
		for i := 0; i < maxFlat; i++ {
			iter.NextI64()
		}
		return component.ValResultError(nil), nil

	case types.Enum:
		t := typ.(types.Enum)
		disc := iter.NextI32()
		if int(disc) >= len(t.Cases) {
			return component.Val{}, fmt.Errorf("invalid enum discriminant: %d", disc)
		}
		return component.ValEnum(t.Cases[disc]), nil

	case types.Flags:
		t := typ.(types.Flags)
		flags := make(map[string]bool)
		if len(t.Names) == 0 {
			return component.ValFlags(flags), nil
		}
		// Read the required number of i32s
		numI32s := (len(t.Names) + 31) / 32
		for i32Idx := 0; i32Idx < numI32s; i32Idx++ {
			bits := iter.NextI32()
			for bit := 0; bit < 32; bit++ {
				flagIdx := i32Idx*32 + bit
				if flagIdx >= len(t.Names) {
					break
				}
				flags[t.Names[flagIdx]] = (bits & (1 << bit)) != 0
			}
		}
		return component.ValFlags(flags), nil

	case types.List:
		// TODO: List flat representation is [ptr, len]. Actual element lifting
		// requires heap access via LiftContext.Memory. For now, consume the
		// flat values and return an empty list. Full implementation in heap lift.
		_ = iter.NextI32() // ptr (unused until heap lift)
		_ = iter.NextI32() // len (unused until heap lift)
		return component.ValList([]component.Val{}), nil

	default:
		return component.Val{}, fmt.Errorf("unsupported flat lift for type: %T", typ)
	}
}

// LiftHeap lifts a value from heap memory at the given offset.
// TODO: The LiftContext.Read* methods currently panic on out-of-bounds access.
// Consider adding error-returning variants for robust error handling.
func LiftHeap(ctx *LiftContext, typ types.ValType, offset uint32) (component.Val, error) {
	switch t := typ.(type) {
	// Primitives
	case types.Bool:
		return component.ValBool(ctx.ReadU8(offset) != 0), nil
	case types.U8:
		return component.ValU8(ctx.ReadU8(offset)), nil
	case types.S8:
		return component.ValS8(int8(ctx.ReadU8(offset))), nil
	case types.U16:
		return component.ValU16(ctx.ReadU16(offset)), nil
	case types.S16:
		return component.ValS16(int16(ctx.ReadU16(offset))), nil
	case types.U32:
		return component.ValU32(ctx.ReadU32(offset)), nil
	case types.S32:
		return component.ValS32(int32(ctx.ReadU32(offset))), nil
	case types.U64:
		return component.ValU64(ctx.ReadU64(offset)), nil
	case types.S64:
		return component.ValS64(int64(ctx.ReadU64(offset))), nil
	case types.F32:
		return component.ValF32(ctx.ReadF32(offset)), nil
	case types.F64:
		return component.ValF64(ctx.ReadF64(offset)), nil
	case types.Char:
		return component.ValChar(rune(ctx.ReadU32(offset))), nil
	case types.String:
		s, err := LiftString(ctx, offset)
		if err != nil {
			return component.Val{}, err
		}
		return component.ValString(s), nil

	// Record
	case types.Record:
		fields := make(map[string]component.Val)
		fieldOffset := uint32(0)
		for _, f := range t.Fields {
			// Align field offset
			align := f.Type.Align()
			if fieldOffset%align != 0 {
				fieldOffset += align - (fieldOffset % align)
			}
			fieldVal, err := LiftHeap(ctx, f.Type, offset+fieldOffset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
			}
			fields[f.Name] = fieldVal
			fieldOffset += f.Type.Size()
		}
		return component.ValRecord(fields), nil

	// Tuple
	case types.Tuple:
		elems := make([]component.Val, len(t.Types))
		elemOffset := uint32(0)
		for i, elemType := range t.Types {
			// Align
			align := elemType.Align()
			if elemOffset%align != 0 {
				elemOffset += align - (elemOffset % align)
			}
			elem, err := LiftHeap(ctx, elemType, offset+elemOffset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift tuple element %d: %w", i, err)
			}
			elems[i] = elem
			elemOffset += elemType.Size()
		}
		return component.ValTuple(elems), nil

	// Variant
	case types.Variant:
		// Read discriminant (size depends on number of cases)
		discSize := t.DiscriminantSize()
		var disc uint32
		switch discSize {
		case 1:
			disc = uint32(ctx.ReadU8(offset))
		case 2:
			disc = uint32(ctx.ReadU16(offset))
		default:
			disc = ctx.ReadU32(offset)
		}
		if int(disc) >= len(t.Cases) {
			return component.Val{}, fmt.Errorf("invalid variant discriminant: %d", disc)
		}
		c := t.Cases[disc]

		// Calculate payload offset (aligned to max payload alignment)
		payloadOffset := t.PayloadOffset()

		var payload *component.Val
		if c.Type != nil {
			p, err := LiftHeap(ctx, c.Type, offset+payloadOffset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift variant payload: %w", err)
			}
			payload = &p
		}
		return component.ValVariant(c.Name, payload), nil

	// Option
	case types.Option:
		disc := ctx.ReadU8(offset)
		if disc == 0 {
			return component.ValOption(nil), nil
		}
		// Calculate payload offset
		payloadAlign := uint32(1)
		if t.Some != nil {
			payloadAlign = t.Some.Align()
		}
		payloadOffset := uint32(1) // discriminant is 1 byte
		if payloadOffset%payloadAlign != 0 {
			payloadOffset += payloadAlign - (payloadOffset % payloadAlign)
		}

		if t.Some != nil {
			p, err := LiftHeap(ctx, t.Some, offset+payloadOffset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift option payload: %w", err)
			}
			return component.ValOption(&p), nil
		}
		emptyVal := component.Val{}
		return component.ValOption(&emptyVal), nil

	// Result
	case types.Result:
		disc := ctx.ReadU8(offset)
		// Calculate max alignment for payload
		payloadAlign := uint32(1)
		if t.Ok != nil && t.Ok.Align() > payloadAlign {
			payloadAlign = t.Ok.Align()
		}
		if t.Error != nil && t.Error.Align() > payloadAlign {
			payloadAlign = t.Error.Align()
		}
		payloadOffset := uint32(1)
		if payloadOffset%payloadAlign != 0 {
			payloadOffset += payloadAlign - (payloadOffset % payloadAlign)
		}

		if disc == 0 { // Ok
			if t.Ok != nil {
				ok, err := LiftHeap(ctx, t.Ok, offset+payloadOffset)
				if err != nil {
					return component.Val{}, err
				}
				return component.ValResultOk(&ok), nil
			}
			return component.ValResultOk(nil), nil
		}
		// Error
		if t.Error != nil {
			e, err := LiftHeap(ctx, t.Error, offset+payloadOffset)
			if err != nil {
				return component.Val{}, err
			}
			return component.ValResultError(&e), nil
		}
		return component.ValResultError(nil), nil

	// Enum
	case types.Enum:
		discSize := t.Size() // Enum's Size() returns the discriminant size
		var disc uint32
		switch discSize {
		case 1:
			disc = uint32(ctx.ReadU8(offset))
		case 2:
			disc = uint32(ctx.ReadU16(offset))
		default:
			disc = ctx.ReadU32(offset)
		}
		if int(disc) >= len(t.Cases) {
			return component.Val{}, fmt.Errorf("invalid enum discriminant: %d", disc)
		}
		return component.ValEnum(t.Cases[disc]), nil

	// Flags
	case types.Flags:
		flags := make(map[string]bool)
		if len(t.Names) == 0 {
			return component.ValFlags(flags), nil
		}
		// Determine storage size
		n := len(t.Names)
		if n <= 8 {
			bits := ctx.ReadU8(offset)
			for i, name := range t.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		} else if n <= 16 {
			bits := ctx.ReadU16(offset)
			for i, name := range t.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		} else if n <= 32 {
			bits := ctx.ReadU32(offset)
			for i, name := range t.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		} else {
			// Multiple u32s
			for i, name := range t.Names {
				wordIdx := i / 32
				bit := i % 32
				word := ctx.ReadU32(offset + uint32(wordIdx*4))
				flags[name] = (word & (1 << bit)) != 0
			}
		}
		return component.ValFlags(flags), nil

	// List
	case types.List:
		ptr := ctx.ReadU32(offset)
		length := ctx.ReadU32(offset + 4)

		// Validate bounds to prevent overflow and excessive allocation
		elemSize := t.Element.Size()
		if length > 0 {
			// Check for potential overflow in ptr + length * elemSize
			maxOffset := uint64(ptr) + uint64(length)*uint64(elemSize)
			if maxOffset > uint64(ctx.Memory.Size()) {
				return component.Val{}, fmt.Errorf("list data exceeds memory bounds: ptr=%d, len=%d, elemSize=%d", ptr, length, elemSize)
			}
		}

		elems := make([]component.Val, length)
		for i := uint32(0); i < length; i++ {
			elem, err := LiftHeap(ctx, t.Element, ptr+i*elemSize)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift list element %d: %w", i, err)
			}
			elems[i] = elem
		}
		return component.ValList(elems), nil

	default:
		return component.Val{}, fmt.Errorf("unsupported heap lift for type: %T", typ)
	}
}
