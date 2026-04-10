package abi

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/runtime"
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

// LiftFlat lifts a flat representation to a component Val. Dispatches on
// typ.Kind and reads composite-type content via ctx.Types.<slice>[typ.Index].
func LiftFlat(ctx *LiftContext, typ types.ValType, iter *FlatIter) (types.Val, error) {
	switch typ.Kind {
	case types.TypeKindBool:
		return types.ValBool(iter.NextI32() != 0), nil
	case types.TypeKindS8:
		return types.ValS8(int8(iter.NextI32())), nil
	case types.TypeKindU8:
		return types.ValU8(uint8(iter.NextI32())), nil
	case types.TypeKindS16:
		return types.ValS16(int16(iter.NextI32())), nil
	case types.TypeKindU16:
		return types.ValU16(uint16(iter.NextI32())), nil
	case types.TypeKindS32:
		return types.ValS32(int32(iter.NextI32())), nil
	case types.TypeKindU32:
		return types.ValU32(iter.NextI32()), nil
	case types.TypeKindS64:
		return types.ValS64(int64(iter.NextI64())), nil
	case types.TypeKindU64:
		return types.ValU64(iter.NextI64()), nil
	case types.TypeKindF32:
		return types.ValF32(canonicalizeNaN32(iter.NextF32())), nil
	case types.TypeKindF64:
		return types.ValF64(canonicalizeNaN64(iter.NextF64())), nil
	case types.TypeKindChar:
		c := iter.NextI32()
		if !isValidUnicodeScalar(c) {
			return types.Val{}, fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		return types.ValChar(rune(c)), nil
	case types.TypeKindString:
		ptr := iter.NextI32()
		taggedLen := iter.NextI32()
		s, err := liftStringFromPtrLen(ctx, ptr, taggedLen)
		if err != nil {
			return types.Val{}, err
		}
		return types.ValString(s), nil
	case types.TypeKindRecord:
		rec := &ctx.Types.Records[typ.Index]
		fields := make(map[string]types.Val, len(rec.Fields))
		for _, f := range rec.Fields {
			fv, err := LiftFlat(ctx, f.Type, iter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
			}
			fields[f.Name] = fv
		}
		return types.ValRecord(fields), nil
	case types.TypeKindVariant:
		variant := &ctx.Types.Variants[typ.Index]
		disc := iter.NextI32()
		if int(disc) >= len(variant.Cases) {
			return types.Val{}, fmt.Errorf("invalid variant discriminant: %d", disc)
		}
		c := variant.Cases[disc]

		// Calculate joined flat types for the variant payload.
		// Per Canonical ABI spec lines 2962-2989, variant payloads use joined types.
		flatTypes := flattenVariantPayload(ctx.Types, variant)

		var payload *types.Val
		if c.HasPayload {
			caseFlat := flattenType(ctx.Types, c.Payload)
			coercedValues := make([]uint64, len(caseFlat))

			// Read and coerce each payload value.
			for i := 0; i < len(caseFlat); i++ {
				have := flatTypes[i]
				want := caseFlat[i]
				var rawValue uint64
				if have == api.ValueTypeI64 || have == api.ValueTypeF64 {
					rawValue = iter.NextI64()
				} else {
					rawValue = uint64(iter.NextI32())
				}
				coercedValues[i] = coerceFlatValue(rawValue, have, want)
			}

			// Skip remaining padding in the joined flat layout.
			for i := len(caseFlat); i < len(flatTypes); i++ {
				if flatTypes[i] == api.ValueTypeI64 || flatTypes[i] == api.ValueTypeF64 {
					iter.NextI64()
				} else {
					iter.NextI32()
				}
			}

			coerceIter := NewFlatIter(coercedValues)
			p, err := LiftFlat(ctx, c.Payload, coerceIter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift variant payload: %w", err)
			}
			payload = &p
		} else {
			for i := 0; i < len(flatTypes); i++ {
				if flatTypes[i] == api.ValueTypeI64 || flatTypes[i] == api.ValueTypeF64 {
					iter.NextI64()
				} else {
					iter.NextI32()
				}
			}
		}
		return types.ValVariant(c.Name, payload), nil
	case types.TypeKindTuple:
		tup := &ctx.Types.Tuples[typ.Index]
		out := make([]types.Val, len(tup.Types))
		for i, t := range tup.Types {
			v, err := LiftFlat(ctx, t, iter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift tuple element %d: %w", i, err)
			}
			out[i] = v
		}
		return types.ValTuple(out), nil
	case types.TypeKindList:
		list := &ctx.Types.Lists[typ.Index]
		// Dynamic list: read ptr and length from flat.
		ptr := iter.NextI32()
		length := iter.NextI32()

		if length == 0 {
			return types.ValList([]types.Val{}), nil
		}

		elemABI := list.Element.ABI(ctx.Types)
		if ptr%elemABI.Align32 != 0 {
			return types.Val{}, fmt.Errorf("list element pointer not aligned: ptr=%d, required alignment=%d", ptr, elemABI.Align32)
		}

		if ctx == nil || ctx.Memory == nil {
			return types.Val{}, fmt.Errorf("lift list: memory context required for non-empty list")
		}

		elemSize := elemABI.Size32
		maxOffset := uint64(ptr) + uint64(length)*uint64(elemSize)
		if maxOffset > uint64(ctx.Memory.Size()) {
			return types.Val{}, fmt.Errorf("list data exceeds memory bounds: ptr=%d, len=%d, elemSize=%d", ptr, length, elemSize)
		}

		elems := make([]types.Val, length)
		for i := uint32(0); i < length; i++ {
			elem, err := LiftHeap(ctx, list.Element, ptr+i*elemSize)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift list element %d: %w", i, err)
			}
			elems[i] = elem
		}
		return types.ValList(elems), nil
	case types.TypeKindFixedList:
		fl := &ctx.Types.FixedLists[typ.Index]
		elems := make([]types.Val, fl.Length)
		for i := uint32(0); i < fl.Length; i++ {
			elem, err := LiftFlat(ctx, fl.Element, iter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift fixed list element %d: %w", i, err)
			}
			elems[i] = elem
		}
		return types.ValList(elems), nil
	case types.TypeKindFlags:
		fl := &ctx.Types.Flags[typ.Index]
		flags := make(map[string]bool, len(fl.Names))
		if len(fl.Names) == 0 {
			return types.ValFlags(flags), nil
		}
		numI32s := (len(fl.Names) + 31) / 32
		for i32Idx := 0; i32Idx < numI32s; i32Idx++ {
			bits := iter.NextI32()
			for bit := 0; bit < 32; bit++ {
				flagIdx := i32Idx*32 + bit
				if flagIdx >= len(fl.Names) {
					break
				}
				flags[fl.Names[flagIdx]] = (bits & (1 << bit)) != 0
			}
		}
		return types.ValFlags(flags), nil
	case types.TypeKindEnum:
		en := &ctx.Types.Enums[typ.Index]
		disc := iter.NextI32()
		if int(disc) >= len(en.Names) {
			return types.Val{}, fmt.Errorf("invalid enum discriminant: %d", disc)
		}
		return types.ValEnum(en.Names[disc]), nil
	case types.TypeKindOption:
		opt := &ctx.Types.Options[typ.Index]
		disc := iter.NextI32()
		payloadABI := opt.Element.ABI(ctx.Types)
		payloadFlat := int(payloadABI.FlattenCount)
		if disc == 0 {
			// None - skip payload slots.
			for i := 0; i < payloadFlat; i++ {
				iter.NextI64()
			}
			return types.ValOption(nil), nil
		}
		payload, err := LiftFlat(ctx, opt.Element, iter)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift option payload: %w", err)
		}
		return types.ValOption(&payload), nil
	case types.TypeKindResult:
		res := &ctx.Types.Results[typ.Index]
		disc := iter.NextI32()
		okFlat, errFlat := 0, 0
		if res.HasOK {
			okFlat = int(res.OK.ABI(ctx.Types).FlattenCount)
		}
		if res.HasErr {
			errFlat = int(res.Err.ABI(ctx.Types).FlattenCount)
		}
		maxFlat := okFlat
		if errFlat > maxFlat {
			maxFlat = errFlat
		}

		if disc == 0 {
			if res.HasOK {
				okVal, err := LiftFlat(ctx, res.OK, iter)
				if err != nil {
					return types.Val{}, fmt.Errorf("lift result ok: %w", err)
				}
				for i := okFlat; i < maxFlat; i++ {
					iter.NextI64()
				}
				return types.ValResultOk(&okVal), nil
			}
			for i := 0; i < maxFlat; i++ {
				iter.NextI64()
			}
			return types.ValResultOk(nil), nil
		}
		if res.HasErr {
			errVal, err := LiftFlat(ctx, res.Err, iter)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift result error: %w", err)
			}
			for i := errFlat; i < maxFlat; i++ {
				iter.NextI64()
			}
			return types.ValResultError(&errVal), nil
		}
		for i := 0; i < maxFlat; i++ {
			iter.NextI64()
		}
		return types.ValResultError(nil), nil
	case types.TypeKindOwn:
		return liftOwnHandle(ctx, typ, iter.NextI32())
	case types.TypeKindBorrow:
		return liftBorrowHandle(ctx, typ, iter.NextI32())
	case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
		return types.Val{}, fmt.Errorf(
			"component-model async types not yet supported: kind=%d", typ.Kind)
	default:
		return types.Val{}, fmt.Errorf("LiftFlat: unknown TypeKind %d", typ.Kind)
	}
}

// LiftHeap lifts a value from heap memory at the given offset. Dispatches
// on typ.Kind like LiftFlat, but reads scalar bytes directly from memory
// instead of consuming the flat iterator.
func LiftHeap(ctx *LiftContext, typ types.ValType, offset uint32) (types.Val, error) {
	switch typ.Kind {
	case types.TypeKindBool:
		v, err := ctx.ReadU8(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift bool: %w", err)
		}
		return types.ValBool(v != 0), nil
	case types.TypeKindU8:
		v, err := ctx.ReadU8(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift u8: %w", err)
		}
		return types.ValU8(v), nil
	case types.TypeKindS8:
		v, err := ctx.ReadU8(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift s8: %w", err)
		}
		return types.ValS8(int8(v)), nil
	case types.TypeKindU16:
		v, err := ctx.ReadU16(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift u16: %w", err)
		}
		return types.ValU16(v), nil
	case types.TypeKindS16:
		v, err := ctx.ReadU16(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift s16: %w", err)
		}
		return types.ValS16(int16(v)), nil
	case types.TypeKindU32:
		v, err := ctx.ReadU32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift u32: %w", err)
		}
		return types.ValU32(v), nil
	case types.TypeKindS32:
		v, err := ctx.ReadU32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift s32: %w", err)
		}
		return types.ValS32(int32(v)), nil
	case types.TypeKindU64:
		v, err := ctx.ReadU64(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift u64: %w", err)
		}
		return types.ValU64(v), nil
	case types.TypeKindS64:
		v, err := ctx.ReadU64(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift s64: %w", err)
		}
		return types.ValS64(int64(v)), nil
	case types.TypeKindF32:
		v, err := ctx.ReadF32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift f32: %w", err)
		}
		return types.ValF32(canonicalizeNaN32(v)), nil
	case types.TypeKindF64:
		v, err := ctx.ReadF64(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift f64: %w", err)
		}
		return types.ValF64(canonicalizeNaN64(v)), nil
	case types.TypeKindChar:
		c, err := ctx.ReadU32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift char: %w", err)
		}
		if !isValidUnicodeScalar(c) {
			return types.Val{}, fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		return types.ValChar(rune(c)), nil
	case types.TypeKindString:
		s, err := LiftString(ctx, offset)
		if err != nil {
			return types.Val{}, err
		}
		return types.ValString(s), nil
	case types.TypeKindRecord:
		rec := &ctx.Types.Records[typ.Index]
		fields := make(map[string]types.Val, len(rec.Fields))
		fieldOffset := uint32(0)
		for _, f := range rec.Fields {
			fa := f.Type.ABI(ctx.Types)
			fieldOffset = types.AlignTo(fieldOffset, fa.Align32)
			fieldVal, err := LiftHeap(ctx, f.Type, offset+fieldOffset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift record field %s: %w", f.Name, err)
			}
			fields[f.Name] = fieldVal
			fieldOffset += fa.Size32
		}
		return types.ValRecord(fields), nil
	case types.TypeKindTuple:
		tup := &ctx.Types.Tuples[typ.Index]
		elems := make([]types.Val, len(tup.Types))
		elemOffset := uint32(0)
		for i, elemType := range tup.Types {
			ea := elemType.ABI(ctx.Types)
			elemOffset = types.AlignTo(elemOffset, ea.Align32)
			elem, err := LiftHeap(ctx, elemType, offset+elemOffset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift tuple element %d: %w", i, err)
			}
			elems[i] = elem
			elemOffset += ea.Size32
		}
		return types.ValTuple(elems), nil
	case types.TypeKindVariant:
		variant := &ctx.Types.Variants[typ.Index]
		discSize := variant.Disc.DiscSize
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
			return types.Val{}, fmt.Errorf("lift variant discriminant: %w", discErr)
		}
		if int(disc) >= len(variant.Cases) {
			return types.Val{}, fmt.Errorf("invalid variant discriminant: %d", disc)
		}
		c := variant.Cases[disc]

		var payload *types.Val
		if c.HasPayload {
			p, err := LiftHeap(ctx, c.Payload, offset+variant.Disc.PayloadOffset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift variant payload: %w", err)
			}
			payload = &p
		}
		return types.ValVariant(c.Name, payload), nil
	case types.TypeKindOption:
		opt := &ctx.Types.Options[typ.Index]
		disc, err := ctx.ReadU8(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift option discriminant: %w", err)
		}
		if disc == 0 {
			return types.ValOption(nil), nil
		}
		p, err := LiftHeap(ctx, opt.Element, offset+opt.Disc.PayloadOffset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift option payload: %w", err)
		}
		return types.ValOption(&p), nil
	case types.TypeKindResult:
		res := &ctx.Types.Results[typ.Index]
		disc, err := ctx.ReadU8(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift result discriminant: %w", err)
		}
		payloadOffset := res.Disc.PayloadOffset
		if disc == 0 {
			if res.HasOK {
				ok, err := LiftHeap(ctx, res.OK, offset+payloadOffset)
				if err != nil {
					return types.Val{}, err
				}
				return types.ValResultOk(&ok), nil
			}
			return types.ValResultOk(nil), nil
		}
		if res.HasErr {
			e, err := LiftHeap(ctx, res.Err, offset+payloadOffset)
			if err != nil {
				return types.Val{}, err
			}
			return types.ValResultError(&e), nil
		}
		return types.ValResultError(nil), nil
	case types.TypeKindEnum:
		en := &ctx.Types.Enums[typ.Index]
		discSize := en.Disc.DiscSize
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
			return types.Val{}, fmt.Errorf("lift enum discriminant: %w", discErr)
		}
		if int(disc) >= len(en.Names) {
			return types.Val{}, fmt.Errorf("invalid enum discriminant: %d", disc)
		}
		return types.ValEnum(en.Names[disc]), nil
	case types.TypeKindFlags:
		fl := &ctx.Types.Flags[typ.Index]
		flags := make(map[string]bool, len(fl.Names))
		n := len(fl.Names)
		if n == 0 {
			return types.ValFlags(flags), nil
		}
		switch {
		case n <= 8:
			bits, err := ctx.ReadU8(offset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift flags: %w", err)
			}
			for i, name := range fl.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		case n <= 16:
			bits, err := ctx.ReadU16(offset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift flags: %w", err)
			}
			for i, name := range fl.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		case n <= 32:
			bits, err := ctx.ReadU32(offset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift flags: %w", err)
			}
			for i, name := range fl.Names {
				flags[name] = (bits & (1 << i)) != 0
			}
		default:
			for i, name := range fl.Names {
				wordIdx := i / 32
				bit := i % 32
				word, err := ctx.ReadU32(offset + uint32(wordIdx*4))
				if err != nil {
					return types.Val{}, fmt.Errorf("lift flags word %d: %w", wordIdx, err)
				}
				flags[name] = (word & (1 << bit)) != 0
			}
		}
		return types.ValFlags(flags), nil
	case types.TypeKindList:
		list := &ctx.Types.Lists[typ.Index]
		ptr, err := ctx.ReadU32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift list ptr: %w", err)
		}
		length, err := ctx.ReadU32(offset + 4)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift list length: %w", err)
		}

		elemABI := list.Element.ABI(ctx.Types)
		elemSize := elemABI.Size32

		if length > 0 {
			if ptr%elemABI.Align32 != 0 {
				return types.Val{}, fmt.Errorf("list element pointer not aligned: ptr=%d, required alignment=%d", ptr, elemABI.Align32)
			}
			maxOffset := uint64(ptr) + uint64(length)*uint64(elemSize)
			if maxOffset > uint64(ctx.Memory.Size()) {
				return types.Val{}, fmt.Errorf("list data exceeds memory bounds: ptr=%d, len=%d, elemSize=%d", ptr, length, elemSize)
			}
		}

		elems := make([]types.Val, length)
		for i := uint32(0); i < length; i++ {
			elem, err := LiftHeap(ctx, list.Element, ptr+i*elemSize)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift list element %d: %w", i, err)
			}
			elems[i] = elem
		}
		return types.ValList(elems), nil
	case types.TypeKindFixedList:
		fl := &ctx.Types.FixedLists[typ.Index]
		elems := make([]types.Val, fl.Length)
		elemABI := fl.Element.ABI(ctx.Types)
		elemSize := elemABI.Size32
		for i := uint32(0); i < fl.Length; i++ {
			elemOffset := offset + i*elemSize
			elem, err := LiftHeap(ctx, fl.Element, elemOffset)
			if err != nil {
				return types.Val{}, fmt.Errorf("lift fixed list element %d: %w", i, err)
			}
			elems[i] = elem
		}
		return types.ValList(elems), nil
	case types.TypeKindOwn:
		h, err := ctx.ReadU32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift own handle: %w", err)
		}
		return liftOwnHandle(ctx, typ, h)
	case types.TypeKindBorrow:
		h, err := ctx.ReadU32(offset)
		if err != nil {
			return types.Val{}, fmt.Errorf("lift borrow handle: %w", err)
		}
		return liftBorrowHandle(ctx, typ, h)
	case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
		return types.Val{}, fmt.Errorf(
			"component-model async types not yet supported: kind=%d", typ.Kind)
	default:
		return types.Val{}, fmt.Errorf("LiftHeap: unknown TypeKind %d", typ.Kind)
	}
}

// liftOwnHandle implements the TypeKindOwn lift arm.
//
// Spec: definitions.py:1333-1339 (lift_own).
//
//	def lift_own(cx, i, t):
//	  h = cx.inst.table.remove(i)           # :1334
//	  trap_if(not isinstance(h, ResourceHandle))
//	  trap_if(h.rt is not t.rt)             # :1336
//	  trap_if(h.num_lends != 0)             # :1337
//	  trap_if(not h.own)                    # :1338
//	  return h.rep                          # :1339
func liftOwnHandle(ctx *LiftContext, typ types.ValType, handleIdx uint32) (types.Val, error) {
	if ctx == nil || ctx.Instance == nil {
		return types.Val{}, fmt.Errorf("lift own: no component instance available")
	}
	if ctx.Types == nil {
		return types.Val{}, fmt.Errorf("lift own: no component types available")
	}
	if int(typ.Index) >= len(ctx.Types.ResourceTables) {
		return types.Val{}, fmt.Errorf("lift own: resource table index %d out of range", typ.Index)
	}
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return types.Val{}, fmt.Errorf("cannot lift abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return types.Val{}, fmt.Errorf(
			"lift own: no resource type for instance %d declaration %d "+
				"(cross-instance resolution: session 2 wiring)",
			rt.Instance, rt.Resource)
	}

	// Gap 3: GetByIndex bridges Wasm-side u32 to runtime 64-bit Handle.
	// Spec: definitions.py:1334 h = cx.inst.table.remove(i)
	h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)
	if err != nil {
		return types.Val{}, fmt.Errorf("lift own: %w", err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return types.Val{}, fmt.Errorf("lift own: handle %d is not a resource handle", handleIdx)
	}
	// Spec: definitions.py:1336 — trap_if(h.rt is not t.rt)
	if resEntry.RT != expectedRT {
		return types.Val{}, fmt.Errorf("lift own: resource type mismatch")
	}
	// Gap 2: Spec: definitions.py:1337 — trap_if(h.num_lends != 0)
	if resEntry.NumLends != 0 {
		return types.Val{}, fmt.Errorf("lift own: handle has %d outstanding lends", resEntry.NumLends)
	}
	// Gap 1: Spec: definitions.py:1338 — trap_if(not h.own)
	if !resEntry.Own {
		return types.Val{}, fmt.Errorf("lift own: handle %d is a borrow, not an own", handleIdx)
	}
	// All checks passed — remove and return rep.
	if _, err := ctx.Instance.Table.Remove(h); err != nil {
		return types.Val{}, fmt.Errorf("lift own: %w", err)
	}
	// Gap 4: Spec: definitions.py:1339 — return h.rep (not the handle index)
	return types.ValOwn(resEntry.Rep), nil
}

// liftBorrowHandle implements the TypeKindBorrow lift arm.
//
// Spec: definitions.py:1341-1347 (lift_borrow).
//
//	def lift_borrow(cx, i, t):
//	  assert(isinstance(cx.borrow_scope, Subtask))  # :1342
//	  h = cx.inst.table.get(i)              # :1343
//	  trap_if(not isinstance(h, ResourceHandle))
//	  trap_if(h.rt is not t.rt)             # :1345
//	  cx.borrow_scope.add_lender(h)         # :1346
//	  return h.rep                          # :1347
func liftBorrowHandle(ctx *LiftContext, typ types.ValType, handleIdx uint32) (types.Val, error) {
	if ctx == nil || ctx.Instance == nil {
		return types.Val{}, fmt.Errorf("lift borrow: no component instance available")
	}
	if ctx.Types == nil {
		return types.Val{}, fmt.Errorf("lift borrow: no component types available")
	}
	// Spec: definitions.py:1342 — assert(isinstance(cx.borrow_scope, Subtask))
	if ctx.CallContext == nil {
		return types.Val{}, fmt.Errorf("lift borrow: no borrow scope active")
	}
	if int(typ.Index) >= len(ctx.Types.ResourceTables) {
		return types.Val{}, fmt.Errorf("lift borrow: resource table index %d out of range", typ.Index)
	}
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return types.Val{}, fmt.Errorf("cannot lift abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return types.Val{}, fmt.Errorf(
			"lift borrow: no resource type for instance %d declaration %d "+
				"(cross-instance resolution: session 2 wiring)",
			rt.Instance, rt.Resource)
	}

	// Gap 3: GetByIndex bridges Wasm-side u32 to runtime 64-bit Handle.
	// Spec: definitions.py:1343 h = cx.inst.table.get(i) — note: GET, not remove
	h, entry, err := ctx.Instance.Table.GetByIndex(handleIdx)
	if err != nil {
		return types.Val{}, fmt.Errorf("lift borrow: %w", err)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return types.Val{}, fmt.Errorf("lift borrow: handle %d is not a resource handle", handleIdx)
	}
	// Spec: definitions.py:1345 — trap_if(h.rt is not t.rt)
	if resEntry.RT != expectedRT {
		return types.Val{}, fmt.Errorf("lift borrow: resource type mismatch")
	}
	// Spec: definitions.py:1346 — cx.borrow_scope.add_lender(h)
	// AddLender internally calls IncrementLends — do NOT call IncrementLends separately.
	if err := ctx.CallContext.AddLender(h); err != nil {
		return types.Val{}, fmt.Errorf("lift borrow: %w", err)
	}
	// Gap 4: Spec: definitions.py:1347 — return h.rep (not the handle index)
	return types.ValBorrow(resEntry.Rep), nil
}

// LiftParams lifts flat ABI parameter values into component Vals. This
// is the symmetric lift companion to LowerParams. Implements the
// aggregate boundary decision from lift_flat_values at
// debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:1943-1952:
//
//   - Flatten paramTypes via FlattenParams.
//   - If len(flatTypes) <= maxFlat: consume the flat iterator per-param
//     via LiftFlat, collecting the resulting Vals in order.
//   - If len(flatTypes) >  maxFlat: read flat[0] as an i32 retptr,
//     validate alignment + memory bounds against the tuple-of-params
//     layout, then iterate LiftHeap at ptr + offsets[i] for each param.
//
// Spec: definitions.py:1943-1952 lift_flat_values, used on the caller
// side of canon.lower to lift args the callee receives.
// Wasmtime parallel: runtime/component/func.rs Func::call_raw
// lift_results path has the analogous load_results branch.
func LiftParams(ctx *LiftContext, paramTypes []types.ValType, flat []uint64, maxFlat int) ([]types.Val, error) {
	if ctx == nil || ctx.Types == nil {
		return nil, fmt.Errorf("LiftParams: no component types available")
	}
	flatTypes := FlattenParams(ctx.Types, paramTypes)
	if len(flatTypes) > maxFlat {
		if len(flat) == 0 {
			return nil, fmt.Errorf("LiftParams: retptr path but flat is empty")
		}
		if ctx.Memory == nil {
			return nil, fmt.Errorf("LiftParams: retptr path requires memory")
		}
		ptr := uint32(flat[0])
		tupleSize, tupleAlign, offsets := paramsTupleLayout(ctx.Types, paramTypes)
		if ptr != types.AlignTo(ptr, tupleAlign) {
			return nil, fmt.Errorf(
				"LiftParams: retptr %d not aligned to %d", ptr, tupleAlign)
		}
		if uint64(ptr)+uint64(tupleSize) > uint64(ctx.Memory.Size()) {
			return nil, fmt.Errorf(
				"LiftParams: retptr %d + tuple size %d out of memory bounds",
				ptr, tupleSize)
		}
		vals := make([]types.Val, len(paramTypes))
		for i, pt := range paramTypes {
			v, err := LiftHeap(ctx, pt, ptr+offsets[i])
			if err != nil {
				return nil, fmt.Errorf("LiftParams: param %d: %w", i, err)
			}
			vals[i] = v
		}
		return vals, nil
	}
	// Flat path: consume flat iterator per-param.
	iter := NewFlatIter(flat)
	vals := make([]types.Val, len(paramTypes))
	for i, pt := range paramTypes {
		v, err := LiftFlat(ctx, pt, iter)
		if err != nil {
			return nil, fmt.Errorf("LiftParams: param %d: %w", i, err)
		}
		vals[i] = v
	}
	return vals, nil
}

// LiftResults lifts flat ABI result values into component Vals. Mirrors
// LiftParams with the MAX_FLAT_RESULTS threshold (single-result cap
// for synchronous calls). Called from canon.lift at the return-path
// boundary (definitions.py:1997) where the spec function iterates the
// core-wasm return values back into component Vals.
//
// Spec: definitions.py:1943-1952 lift_flat_values applied to
// canon_lift return-path at definitions.py:1997.
func LiftResults(ctx *LiftContext, resultTypes []types.ValType, flat []uint64, maxFlat int) ([]types.Val, error) {
	if ctx == nil || ctx.Types == nil {
		return nil, fmt.Errorf("LiftResults: no component types available")
	}
	// Compute the flattened width directly (FlattenResults collapses to
	// nil+needsRetptr=true at the MaxFlatResults threshold, but we
	// respect the caller-supplied maxFlat parameter here).
	width := 0
	for _, rt := range resultTypes {
		width += len(flattenType(ctx.Types, rt))
	}
	if width > maxFlat {
		if len(flat) == 0 {
			return nil, fmt.Errorf("LiftResults: retptr path but flat is empty")
		}
		if ctx.Memory == nil {
			return nil, fmt.Errorf("LiftResults: retptr path requires memory")
		}
		ptr := uint32(flat[0])
		tupleSize, tupleAlign, offsets := paramsTupleLayout(ctx.Types, resultTypes)
		if ptr != types.AlignTo(ptr, tupleAlign) {
			return nil, fmt.Errorf(
				"LiftResults: retptr %d not aligned to %d", ptr, tupleAlign)
		}
		if uint64(ptr)+uint64(tupleSize) > uint64(ctx.Memory.Size()) {
			return nil, fmt.Errorf(
				"LiftResults: retptr %d + tuple size %d out of memory bounds",
				ptr, tupleSize)
		}
		vals := make([]types.Val, len(resultTypes))
		for i, rt := range resultTypes {
			v, err := LiftHeap(ctx, rt, ptr+offsets[i])
			if err != nil {
				return nil, fmt.Errorf("LiftResults: result %d: %w", i, err)
			}
			vals[i] = v
		}
		return vals, nil
	}
	// Flat path: consume flat iterator per-result.
	iter := NewFlatIter(flat)
	vals := make([]types.Val, len(resultTypes))
	for i, rt := range resultTypes {
		v, err := LiftFlat(ctx, rt, iter)
		if err != nil {
			return nil, fmt.Errorf("LiftResults: result %d: %w", i, err)
		}
		vals[i] = v
	}
	return vals, nil
}


// isValidUnicodeScalar checks if a value is a valid Unicode scalar value.
// Unicode scalar values are any code point except high-surrogate and low-surrogate code points.
// Valid ranges: U+0000 to U+D7FF and U+E000 to U+10FFFF
func isValidUnicodeScalar(v uint32) bool {
	if v >= 0xD800 && v <= 0xDFFF {
		return false
	}
	if v > 0x10FFFF {
		return false
	}
	return true
}

// flattenVariantPayload returns the joined flat types for a variant's payload.
// Per Canonical ABI spec lines 2962-2989, variant payloads use joined types.
func flattenVariantPayload(ct *types.ComponentTypes, v *types.TypeVariant) []api.ValueType {
	var flat []api.ValueType
	for _, c := range v.Cases {
		if c.HasPayload {
			caseFlat := flattenType(ct, c.Payload)
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
		return value
	case have == api.ValueTypeI64 && want == api.ValueTypeI32:
		return value & 0xFFFFFFFF
	case have == api.ValueTypeI64 && want == api.ValueTypeF32:
		return value & 0xFFFFFFFF
	case have == api.ValueTypeI64 && want == api.ValueTypeF64:
		return value
	default:
		return value
	}
}
