package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
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
		return component.ValF32(canonicalizeNaN32(iter.NextF32())), nil
	case types.F64:
		return component.ValF64(canonicalizeNaN64(iter.NextF64())), nil
	case types.Char:
		c := iter.NextI32()
		if !isValidUnicodeScalar(c) {
			return component.Val{}, fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		return component.ValChar(rune(c)), nil
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

		// Calculate the joined flat types for the variant payload
		// Per Canonical ABI spec lines 2962-2989, variant payloads use joined types
		flatTypes := flattenVariantPayload(t)

		var payload *component.Val
		if c.Type != nil {
			// Get the actual case's flat types
			caseFlat := flattenType(c.Type)
			coercedValues := make([]uint64, len(caseFlat))

			// Read and coerce each payload value
			for i := 0; i < len(caseFlat); i++ {
				have := flatTypes[i]
				want := caseFlat[i]
				// Read as the joined type (widest possible type)
				var rawValue uint64
				if have == api.ValueTypeI64 || have == api.ValueTypeF64 {
					rawValue = iter.NextI64()
				} else {
					rawValue = uint64(iter.NextI32())
				}
				coercedValues[i] = coerceFlatValue(rawValue, have, want)
			}

			// Skip remaining padding in the joined flat layout
			for i := len(caseFlat); i < len(flatTypes); i++ {
				if flatTypes[i] == api.ValueTypeI64 || flatTypes[i] == api.ValueTypeF64 {
					iter.NextI64()
				} else {
					iter.NextI32()
				}
			}

			// Lift the payload using coerced values
			coerceIter := NewFlatIter(coercedValues)
			p, err := LiftFlat(ctx, c.Type, coerceIter)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift variant payload: %w", err)
			}
			payload = &p
		} else {
			// No payload - skip all padding
			for i := 0; i < len(flatTypes); i++ {
				if flatTypes[i] == api.ValueTypeI64 || flatTypes[i] == api.ValueTypeF64 {
					iter.NextI64()
				} else {
					iter.NextI32()
				}
			}
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
		t := typ.(types.List)

		if t.Length != nil {
			// Fixed-length list: lift each element from flat values
			length := *t.Length
			elems := make([]component.Val, length)
			for i := uint32(0); i < length; i++ {
				elem, err := LiftFlat(ctx, t.Element, iter)
				if err != nil {
					return component.Val{}, fmt.Errorf("lift fixed list element %d: %w", i, err)
				}
				elems[i] = elem
			}
			return component.ValList(elems), nil
		}

		// Dynamic list: read ptr and length
		ptr := iter.NextI32()
		length := iter.NextI32()

		// Empty list case - no memory access needed
		if length == 0 {
			return component.ValList([]component.Val{}), nil
		}

		// Validate alignment per spec line 2153
		elemAlign := t.Element.Align()
		if ptr%elemAlign != 0 {
			return component.Val{}, fmt.Errorf("list element pointer not aligned: ptr=%d, required alignment=%d", ptr, elemAlign)
		}

		// Need memory context for non-empty lists
		if ctx == nil || ctx.Memory == nil {
			return component.Val{}, fmt.Errorf("lift list: memory context required for non-empty list")
		}

		// Validate bounds
		elemSize := t.Element.Size()
		maxOffset := uint64(ptr) + uint64(length)*uint64(elemSize)
		if maxOffset > uint64(ctx.Memory.Size()) {
			return component.Val{}, fmt.Errorf("list data exceeds memory bounds: ptr=%d, len=%d, elemSize=%d", ptr, length, elemSize)
		}

		// Lift each element from heap
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
		return component.Val{}, fmt.Errorf("unsupported flat lift for type: %T", typ)
	}
}

// LiftHeap lifts a value from heap memory at the given offset.
func LiftHeap(ctx *LiftContext, typ types.ValType, offset uint32) (component.Val, error) {
	switch t := typ.(type) {
	// Primitives
	case types.Bool:
		v, err := ctx.ReadU8(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift bool: %w", err)
		}
		return component.ValBool(v != 0), nil
	case types.U8:
		v, err := ctx.ReadU8(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift u8: %w", err)
		}
		return component.ValU8(v), nil
	case types.S8:
		v, err := ctx.ReadU8(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift s8: %w", err)
		}
		return component.ValS8(int8(v)), nil
	case types.U16:
		v, err := ctx.ReadU16(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift u16: %w", err)
		}
		return component.ValU16(v), nil
	case types.S16:
		v, err := ctx.ReadU16(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift s16: %w", err)
		}
		return component.ValS16(int16(v)), nil
	case types.U32:
		v, err := ctx.ReadU32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift u32: %w", err)
		}
		return component.ValU32(v), nil
	case types.S32:
		v, err := ctx.ReadU32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift s32: %w", err)
		}
		return component.ValS32(int32(v)), nil
	case types.U64:
		v, err := ctx.ReadU64(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift u64: %w", err)
		}
		return component.ValU64(v), nil
	case types.S64:
		v, err := ctx.ReadU64(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift s64: %w", err)
		}
		return component.ValS64(int64(v)), nil
	case types.F32:
		v, err := ctx.ReadF32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift f32: %w", err)
		}
		return component.ValF32(canonicalizeNaN32(v)), nil
	case types.F64:
		v, err := ctx.ReadF64(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift f64: %w", err)
		}
		return component.ValF64(canonicalizeNaN64(v)), nil
	case types.Char:
		c, err := ctx.ReadU32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift char: %w", err)
		}
		if !isValidUnicodeScalar(c) {
			return component.Val{}, fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		return component.ValChar(rune(c)), nil
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
		var discErr error
		switch discSize {
		case 1:
			v, err := ctx.ReadU8(offset)
			disc, discErr = uint32(v), err
		case 2:
			v, err := ctx.ReadU16(offset)
			disc, discErr = uint32(v), err
		default:
			disc, discErr = ctx.ReadU32(offset)
		}
		if discErr != nil {
			return component.Val{}, fmt.Errorf("lift variant discriminant: %w", discErr)
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
		disc, err := ctx.ReadU8(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift option discriminant: %w", err)
		}
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
		disc, err := ctx.ReadU8(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift result discriminant: %w", err)
		}
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
		var discErr error
		switch discSize {
		case 1:
			v, err := ctx.ReadU8(offset)
			disc, discErr = uint32(v), err
		case 2:
			v, err := ctx.ReadU16(offset)
			disc, discErr = uint32(v), err
		default:
			disc, discErr = ctx.ReadU32(offset)
		}
		if discErr != nil {
			return component.Val{}, fmt.Errorf("lift enum discriminant: %w", discErr)
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
			bits, err := ctx.ReadU8(offset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift flags: %w", err)
			}
			for i, name := range t.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		} else if n <= 16 {
			bits, err := ctx.ReadU16(offset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift flags: %w", err)
			}
			for i, name := range t.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		} else if n <= 32 {
			bits, err := ctx.ReadU32(offset)
			if err != nil {
				return component.Val{}, fmt.Errorf("lift flags: %w", err)
			}
			for i, name := range t.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		} else {
			// Multiple u32s
			for i, name := range t.Names {
				wordIdx := i / 32
				bit := i % 32
				word, err := ctx.ReadU32(offset + uint32(wordIdx*4))
				if err != nil {
					return component.Val{}, fmt.Errorf("lift flags word %d: %w", wordIdx, err)
				}
				flags[name] = (word & (1 << bit)) != 0
			}
		}
		return component.ValFlags(flags), nil

	// List
	case types.List:
		if t.Length != nil {
			// Fixed-length list: elements are inline at offset
			length := *t.Length
			elems := make([]component.Val, length)
			elemSize := t.Element.Size()
			for i := uint32(0); i < length; i++ {
				elemOffset := offset + i*elemSize
				elem, err := LiftHeap(ctx, t.Element, elemOffset)
				if err != nil {
					return component.Val{}, fmt.Errorf("lift fixed list element %d: %w", i, err)
				}
				elems[i] = elem
			}
			return component.ValList(elems), nil
		}

		// Dynamic list: read ptr and length from memory
		ptr, err := ctx.ReadU32(offset)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift list ptr: %w", err)
		}
		length, err := ctx.ReadU32(offset + 4)
		if err != nil {
			return component.Val{}, fmt.Errorf("lift list length: %w", err)
		}

		// Compute element size once for validation and iteration
		elemSize := t.Element.Size()

		if length > 0 {
			// Validate alignment per spec line 2153
			elemAlign := t.Element.Align()
			if ptr%elemAlign != 0 {
				return component.Val{}, fmt.Errorf("list element pointer not aligned: ptr=%d, required alignment=%d", ptr, elemAlign)
			}

			// Validate bounds to prevent overflow and excessive allocation
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

// LiftOwn transfers ownership of a resource out of the component.
// It removes the handle from the table and returns the representation value.
// Traps if the handle has active borrows (NumLends > 0).
// Traps if the handle is not owned (i.e., it's a borrowed handle).
//
// TODO: Per spec lines 2218-2219, should validate:
//   - trap_if(h.rt is not t.rt) - resource type matches
//
// Currently, resource type tracking is not implemented in ResourceTable.
// Full implementation requires tracking which resource type each handle belongs to,
// which is a larger architectural change.
func LiftOwn(ctx *LiftContext, handleIdx uint32) (any, error) {
	if ctx.ResourceTable == nil {
		return nil, fmt.Errorf("lift_own: no resource table available")
	}

	// We need to look up the full handle including generation.
	// First, get the entry to verify it exists and is owned.
	// The handleIdx is just the index; we need to reconstruct the full handle.
	// For proper generation tracking, we iterate through to find the handle.
	// However, the canonical ABI only passes the index, so we construct
	// a handle with the current generation from the table entry.

	// Get the current generation for this index by constructing a probe handle
	// and using Get to validate. The table's Get validates generation.
	// Since we only have the index, we need to try with generation 0 first,
	// but that won't work for reused slots.

	// The proper approach: the table needs to expose a way to get by index.
	// For now, we iterate through possible generations, but this is inefficient.
	// A better design would be to track the full handle in the component instance.

	// Simplified approach: try to find a valid handle for this index
	// by checking if the index is within bounds and the entry is occupied.
	// The ResourceTable.Get validates both index bounds and generation.

	// Since the component model ABI only passes the index (u32), and the
	// generation is stored in the table, we need to query the table to
	// get the current generation for this slot.

	// For the MVP implementation, we'll construct the handle using the
	// generation from direct table access. This requires that the caller
	// passes the correct index that was obtained from a prior New() call.

	// First, verify the index is valid and get the entry
	h := component.Handle(handleIdx) // Start with generation 0

	// Try to get with just the index - the table will validate
	// Note: This is a limitation - the full handle should include generation
	// For proper implementation, the handle passed to LiftOwn should be the
	// full 64-bit handle, but the canonical ABI passes u32 index only.

	// Get the entry first to check ownership
	entry, err := ctx.ResourceTable.Get(h)
	if err != nil {
		// Try to find the entry by scanning generations
		// This is a workaround for the index-only interface
		for gen := uint32(1); gen < 1000; gen++ {
			h = component.MakeHandle(handleIdx, gen)
			entry, err = ctx.ResourceTable.Get(h)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("lift_own: invalid handle index %d: %w", handleIdx, err)
		}
	}

	// Verify this is an owned handle, not a borrow
	if !entry.Own {
		return nil, fmt.Errorf("lift_own: handle is not owned")
	}

	// Remove from table (this checks NumLends > 0 and returns error if so)
	removed, err := ctx.ResourceTable.Remove(h)
	if err != nil {
		return nil, fmt.Errorf("lift_own: %w", err)
	}

	return removed.Rep, nil
}

// LiftBorrow reads a resource representation for borrowing.
// Unlike LiftOwn, it does NOT remove the handle from the table.
// It tracks the lend in the BorrowScope to prevent ownership transfer while borrowed.
//
// TODO: Per spec lines 2237-2238, should validate:
//   - trap_if(h.rt is not t.rt) - resource type matches
//
// Currently, resource type tracking is not implemented in ResourceTable.
// Full implementation requires tracking which resource type each handle belongs to,
// which is a larger architectural change.
func LiftBorrow(ctx *LiftContext, handleIdx uint32) (any, error) {
	if ctx.ResourceTable == nil {
		return nil, fmt.Errorf("lift_borrow: no resource table available")
	}

	// Construct handle from index - similar approach to LiftOwn
	h := component.Handle(handleIdx)

	// Try to get the entry
	entry, err := ctx.ResourceTable.Get(h)
	if err != nil {
		// Try to find the entry by scanning generations
		for gen := uint32(1); gen < 1000; gen++ {
			h = component.MakeHandle(handleIdx, gen)
			entry, err = ctx.ResourceTable.Get(h)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("lift_borrow: invalid handle index %d: %w", handleIdx, err)
		}
	}

	// Track the lend in the borrow scope
	if ctx.BorrowScope != nil {
		if err := ctx.BorrowScope.AddLender(h); err != nil {
			return nil, fmt.Errorf("lift_borrow: tracking lend: %w", err)
		}
	}

	return entry.Rep, nil
}

// isValidUnicodeScalar checks if a value is a valid Unicode scalar value.
// Unicode scalar values are any code point except high-surrogate and low-surrogate code points.
// Valid ranges: U+0000 to U+D7FF and U+E000 to U+10FFFF
func isValidUnicodeScalar(v uint32) bool {
	// Check for surrogates (U+D800 to U+DFFF)
	if v >= 0xD800 && v <= 0xDFFF {
		return false
	}
	// Check for values above maximum Unicode code point
	if v > 0x10FFFF {
		return false
	}
	return true
}

// flattenVariantPayload returns the joined flat types for a variant's payload.
// This uses the join function to compute the widest compatible type for each position.
// Per Canonical ABI spec lines 2962-2989, variant payloads use joined types.
func flattenVariantPayload(v types.Variant) []api.ValueType {
	var flat []api.ValueType
	for _, c := range v.Cases {
		if c.Type != nil {
			caseFlat := flattenType(c.Type)
			for i, ft := range caseFlat {
				if i < len(flat) {
					flat[i] = join(flat[i], ft)
				} else {
					flat = append(flat, ft)
				}
			}
		}
	}
	return flat
}

// coerceFlatValue coerces a flat value from 'have' type to 'want' type.
// This implements the coercion rules from Canonical ABI spec lines 2971-2976.
// When reading variant payloads, values are read using the joined types and
// must be coerced to the actual case type:
// - i32 as f32: reinterpret the bits (value already contains f32 bits)
// - i64 to i32: truncate (wrap)
// - i64 to f32: truncate to i32, then reinterpret as f32
// - i64 as f64: reinterpret the bits (value already contains f64 bits)
func coerceFlatValue(value uint64, have, want api.ValueType) uint64 {
	if have == want {
		return value
	}
	switch {
	case have == api.ValueTypeI32 && want == api.ValueTypeF32:
		// The value is already the i32 bits representing f32, just return it
		return value
	case have == api.ValueTypeI64 && want == api.ValueTypeI32:
		// Wrap i64 to i32 (truncate to low 32 bits)
		return value & 0xFFFFFFFF
	case have == api.ValueTypeI64 && want == api.ValueTypeF32:
		// Wrap i64 to i32, use as f32 bits
		return value & 0xFFFFFFFF
	case have == api.ValueTypeI64 && want == api.ValueTypeF64:
		// i64 bits as f64 - value is already the bits
		return value
	default:
		// No coercion needed or unknown combination
		return value
	}
}
