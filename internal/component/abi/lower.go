package abi

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// LowerFlat lowers a component Val to flat core wasm values. Dispatches
// on typ.Kind and reads composite-type content via ctx.Types.<slice>[typ.Index].
func LowerFlat(ctx *LowerContext, typ types.ValType, val types.Val) ([]uint64, error) {
	switch typ.Kind {
	case types.TypeKindBool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case types.TypeKindS8:
		return []uint64{uint64(uint32(int32(val.S8())))}, nil
	case types.TypeKindU8:
		return []uint64{uint64(val.U8())}, nil
	case types.TypeKindS16:
		return []uint64{uint64(uint32(int32(val.S16())))}, nil
	case types.TypeKindU16:
		return []uint64{uint64(val.U16())}, nil
	case types.TypeKindS32:
		return []uint64{uint64(uint32(val.S32()))}, nil
	case types.TypeKindU32:
		return []uint64{uint64(val.U32())}, nil
	case types.TypeKindS64:
		return []uint64{uint64(val.S64())}, nil
	case types.TypeKindU64:
		return []uint64{val.U64()}, nil
	case types.TypeKindF32:
		return []uint64{uint64(math.Float32bits(val.F32()))}, nil
	case types.TypeKindF64:
		return []uint64{math.Float64bits(val.F64())}, nil
	case types.TypeKindChar:
		c := val.Char()
		if !isValidUnicodeScalarRune(c) {
			return nil, fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		return []uint64{uint64(c)}, nil
	case types.TypeKindString:
		ptr, taggedLen, err := LowerString(ctx, val.StringVal())
		if err != nil {
			return nil, err
		}
		return []uint64{uint64(ptr), uint64(taggedLen)}, nil
	case types.TypeKindRecord:
		rec := &ctx.Types.Records[typ.Index]
		valRec := val.Record()
		var result []uint64
		for _, f := range rec.Fields {
			fieldVal, ok := valRec[f.Name]
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
	case types.TypeKindVariant:
		variant := &ctx.Types.Variants[typ.Index]
		caseName, payload := val.Variant()

		caseIdx := -1
		var caseHasPayload bool
		var caseType types.ValType
		for i, c := range variant.Cases {
			if c.Name == caseName {
				caseIdx = i
				caseHasPayload = c.HasPayload
				caseType = c.Payload
				break
			}
		}
		if caseIdx == -1 {
			return nil, fmt.Errorf("unknown variant case: %s", caseName)
		}
		if caseHasPayload && payload == nil {
			return nil, fmt.Errorf("variant case %q requires a payload", caseName)
		}

		flatTypes := flattenVariantPayload(ctx.Types, variant)
		result := []uint64{uint64(caseIdx)}

		if caseHasPayload && payload != nil {
			payloadFlat, err := LowerFlat(ctx, caseType, *payload)
			if err != nil {
				return nil, fmt.Errorf("lower variant payload: %w", err)
			}

			caseFlat := flattenType(ctx.Types, caseType)
			for i, pv := range payloadFlat {
				have := caseFlat[i]
				want := flatTypes[i]
				result = append(result, coerceFlatValueForLower(pv, have, want))
			}
			for i := len(payloadFlat); i < len(flatTypes); i++ {
				result = append(result, 0)
			}
		} else {
			for i := 0; i < len(flatTypes); i++ {
				result = append(result, 0)
			}
		}
		return result, nil

	case types.TypeKindTuple:
		tup := &ctx.Types.Tuples[typ.Index]
		elems := val.Tuple()
		if len(elems) != len(tup.Types) {
			return nil, fmt.Errorf("tuple has %d elements, expected %d", len(elems), len(tup.Types))
		}
		var result []uint64
		for i, elemType := range tup.Types {
			flat, err := LowerFlat(ctx, elemType, elems[i])
			if err != nil {
				return nil, fmt.Errorf("lower tuple element %d: %w", i, err)
			}
			result = append(result, flat...)
		}
		return result, nil

	case types.TypeKindOption:
		opt := &ctx.Types.Options[typ.Index]
		payload := val.Option()
		payloadABI := opt.Element.ABI(ctx.Types)
		payloadFlat := int(payloadABI.FlattenCount)

		if payload == nil {
			result := []uint64{0}
			for i := 0; i < payloadFlat; i++ {
				result = append(result, 0)
			}
			return result, nil
		}
		result := []uint64{1}
		flat, err := LowerFlat(ctx, opt.Element, *payload)
		if err != nil {
			return nil, fmt.Errorf("lower option payload: %w", err)
		}
		result = append(result, flat...)
		return result, nil

	case types.TypeKindResult:
		res := &ctx.Types.Results[typ.Index]
		isOk, okVal, errVal := val.Result()

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

		if isOk {
			result := []uint64{0}
			payloadCount := 0
			if res.HasOK && okVal != nil {
				flat, err := LowerFlat(ctx, res.OK, *okVal)
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
		result := []uint64{1}
		payloadCount := 0
		if res.HasErr && errVal != nil {
			flat, err := LowerFlat(ctx, res.Err, *errVal)
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

	case types.TypeKindEnum:
		en := &ctx.Types.Enums[typ.Index]
		caseName := val.Enum()
		for i, c := range en.Names {
			if c == caseName {
				return []uint64{uint64(i)}, nil
			}
		}
		return nil, fmt.Errorf("unknown enum case: %s", caseName)

	case types.TypeKindFlags:
		fl := &ctx.Types.Flags[typ.Index]
		flags := val.Flags()
		if len(fl.Names) == 0 {
			return []uint64{}, nil
		}
		numI32s := (len(fl.Names) + 31) / 32
		result := make([]uint64, numI32s)
		for i, name := range fl.Names {
			if flags[name] {
				i32Idx := i / 32
				bit := i % 32
				result[i32Idx] |= 1 << bit
			}
		}
		return result, nil

	case types.TypeKindList:
		list := &ctx.Types.Lists[typ.Index]
		elements := val.List()
		length := uint32(len(elements))

		if ctx == nil || ctx.Realloc == nil {
			return nil, fmt.Errorf("lower list: realloc function required")
		}

		elemABI := list.Element.ABI(ctx.Types)
		elemSize := elemABI.Size32
		elemAlign := elemABI.Align32
		if elemAlign == 0 {
			elemAlign = 1
		}
		totalSize := length * elemSize

		// Per canonical ABI: realloc is always called, even for empty lists.
		// The returned pointer must be aligned and within memory bounds.
		ptr, err := ctx.Realloc(0, 0, elemAlign, totalSize)
		if err != nil {
			return nil, fmt.Errorf("lower list: realloc failed: %w", err)
		}

		for i := uint32(0); i < length; i++ {
			offset := ptr + i*elemSize
			if err := LowerHeap(ctx, list.Element, elements[i], offset); err != nil {
				return nil, fmt.Errorf("lower list element %d: %w", i, err)
			}
		}

		return []uint64{uint64(ptr), uint64(length)}, nil

	case types.TypeKindFixedList:
		fl := &ctx.Types.FixedLists[typ.Index]
		elements := val.List()
		if uint32(len(elements)) != fl.Length {
			return nil, fmt.Errorf("fixed list length mismatch: got %d, expected %d", len(elements), fl.Length)
		}
		var result []uint64
		for i, elem := range elements {
			flat, err := LowerFlat(ctx, fl.Element, elem)
			if err != nil {
				return nil, fmt.Errorf("lower fixed list element %d: %w", i, err)
			}
			result = append(result, flat...)
		}
		return result, nil

	case types.TypeKindOwn:
		return lowerOwnHandleFlat(ctx, typ, val)
	case types.TypeKindBorrow:
		return lowerBorrowHandleFlat(ctx, typ, val)

	case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
		return nil, fmt.Errorf(
			"component-model async types not yet supported: kind=%d", typ.Kind)

	default:
		return nil, fmt.Errorf("LowerFlat: unknown TypeKind %d", typ.Kind)
	}
}

// LowerHeap lowers a component Val to heap memory at the given offset.
// Dispatches on typ.Kind like LowerFlat, but writes scalar bytes directly
// to memory instead of producing a flat slice.
func LowerHeap(ctx *LowerContext, typ types.ValType, val types.Val, offset uint32) error {
	switch typ.Kind {
	case types.TypeKindBool:
		if val.Bool() {
			writeUint8(ctx.Memory, offset, 1)
		} else {
			writeUint8(ctx.Memory, offset, 0)
		}
		return nil
	case types.TypeKindS8:
		writeUint8(ctx.Memory, offset, uint8(val.S8()))
		return nil
	case types.TypeKindU8:
		writeUint8(ctx.Memory, offset, val.U8())
		return nil
	case types.TypeKindS16:
		writeUint16Le(ctx.Memory, offset, uint16(val.S16()))
		return nil
	case types.TypeKindU16:
		writeUint16Le(ctx.Memory, offset, val.U16())
		return nil
	case types.TypeKindS32:
		writeUint32Le(ctx.Memory, offset, uint32(val.S32()))
		return nil
	case types.TypeKindU32:
		writeUint32Le(ctx.Memory, offset, val.U32())
		return nil
	case types.TypeKindS64:
		writeUint64Le(ctx.Memory, offset, uint64(val.S64()))
		return nil
	case types.TypeKindU64:
		writeUint64Le(ctx.Memory, offset, val.U64())
		return nil
	case types.TypeKindF32:
		writeUint32Le(ctx.Memory, offset, math.Float32bits(val.F32()))
		return nil
	case types.TypeKindF64:
		writeUint64Le(ctx.Memory, offset, math.Float64bits(val.F64()))
		return nil
	case types.TypeKindChar:
		c := val.Char()
		if !isValidUnicodeScalarRune(c) {
			return fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		writeUint32Le(ctx.Memory, offset, uint32(c))
		return nil
	case types.TypeKindString:
		ptr, taggedLen, err := LowerString(ctx, val.StringVal())
		if err != nil {
			return err
		}
		writeUint32Le(ctx.Memory, offset, ptr)
		writeUint32Le(ctx.Memory, offset+4, taggedLen)
		return nil

	case types.TypeKindRecord:
		rec := &ctx.Types.Records[typ.Index]
		valRec := val.Record()
		fieldOffset := uint32(0)
		for _, f := range rec.Fields {
			fa := f.Type.ABI(ctx.Types)
			fieldOffset = types.AlignTo(fieldOffset, fa.Align32)
			fieldVal, ok := valRec[f.Name]
			if !ok {
				return fmt.Errorf("missing record field: %s", f.Name)
			}
			if err := LowerHeap(ctx, f.Type, fieldVal, offset+fieldOffset); err != nil {
				return fmt.Errorf("lower record field %s: %w", f.Name, err)
			}
			fieldOffset += fa.Size32
		}
		return nil

	case types.TypeKindTuple:
		tup := &ctx.Types.Tuples[typ.Index]
		elems := val.Tuple()
		if len(elems) != len(tup.Types) {
			return fmt.Errorf("tuple has %d elements, expected %d", len(elems), len(tup.Types))
		}
		elemOffset := uint32(0)
		for i, elemType := range tup.Types {
			ea := elemType.ABI(ctx.Types)
			elemOffset = types.AlignTo(elemOffset, ea.Align32)
			if err := LowerHeap(ctx, elemType, elems[i], offset+elemOffset); err != nil {
				return fmt.Errorf("lower tuple element %d: %w", i, err)
			}
			elemOffset += ea.Size32
		}
		return nil

	case types.TypeKindVariant:
		variant := &ctx.Types.Variants[typ.Index]
		caseName, payload := val.Variant()

		caseIdx := -1
		var caseHasPayload bool
		var caseType types.ValType
		for i, c := range variant.Cases {
			if c.Name == caseName {
				caseIdx = i
				caseHasPayload = c.HasPayload
				caseType = c.Payload
				break
			}
		}
		if caseIdx == -1 {
			return fmt.Errorf("unknown variant case: %s", caseName)
		}

		discSize := variant.Disc.DiscSize
		switch discSize {
		case 1:
			writeUint8(ctx.Memory, offset, uint8(caseIdx))
		case 2:
			writeUint16Le(ctx.Memory, offset, uint16(caseIdx))
		default:
			writeUint32Le(ctx.Memory, offset, uint32(caseIdx))
		}

		if caseHasPayload && payload != nil {
			if err := LowerHeap(ctx, caseType, *payload, offset+variant.Disc.PayloadOffset); err != nil {
				return fmt.Errorf("lower variant payload: %w", err)
			}
		}
		return nil

	case types.TypeKindOption:
		opt := &ctx.Types.Options[typ.Index]
		payload := val.Option()

		if payload == nil {
			writeUint8(ctx.Memory, offset, 0)
			return nil
		}
		writeUint8(ctx.Memory, offset, 1)
		if err := LowerHeap(ctx, opt.Element, *payload, offset+opt.Disc.PayloadOffset); err != nil {
			return fmt.Errorf("lower option payload: %w", err)
		}
		return nil

	case types.TypeKindResult:
		res := &ctx.Types.Results[typ.Index]
		isOk, okVal, errVal := val.Result()
		payloadOffset := res.Disc.PayloadOffset

		if isOk {
			writeUint8(ctx.Memory, offset, 0)
			if res.HasOK && okVal != nil {
				if err := LowerHeap(ctx, res.OK, *okVal, offset+payloadOffset); err != nil {
					return fmt.Errorf("lower result ok: %w", err)
				}
			}
		} else {
			writeUint8(ctx.Memory, offset, 1)
			if res.HasErr && errVal != nil {
				if err := LowerHeap(ctx, res.Err, *errVal, offset+payloadOffset); err != nil {
					return fmt.Errorf("lower result error: %w", err)
				}
			}
		}
		return nil

	case types.TypeKindEnum:
		en := &ctx.Types.Enums[typ.Index]
		caseName := val.Enum()
		caseIdx := -1
		for i, c := range en.Names {
			if c == caseName {
				caseIdx = i
				break
			}
		}
		if caseIdx == -1 {
			return fmt.Errorf("unknown enum case: %s", caseName)
		}

		switch en.Disc.DiscSize {
		case 1:
			writeUint8(ctx.Memory, offset, uint8(caseIdx))
		case 2:
			writeUint16Le(ctx.Memory, offset, uint16(caseIdx))
		default:
			writeUint32Le(ctx.Memory, offset, uint32(caseIdx))
		}
		return nil

	case types.TypeKindFlags:
		fl := &ctx.Types.Flags[typ.Index]
		flags := val.Flags()
		n := len(fl.Names)
		if n == 0 {
			return nil
		}
		switch {
		case n <= 8:
			var bits uint8
			for i, name := range fl.Names {
				if flags[name] {
					bits |= 1 << i
				}
			}
			writeUint8(ctx.Memory, offset, bits)
		case n <= 16:
			var bits uint16
			for i, name := range fl.Names {
				if flags[name] {
					bits |= 1 << i
				}
			}
			writeUint16Le(ctx.Memory, offset, bits)
		case n <= 32:
			var bits uint32
			for i, name := range fl.Names {
				if flags[name] {
					bits |= 1 << i
				}
			}
			writeUint32Le(ctx.Memory, offset, bits)
		default:
			for i, name := range fl.Names {
				if flags[name] {
					wordIdx := i / 32
					bit := i % 32
					wordOffset := offset + uint32(wordIdx*4)
					data, ok := ctx.Memory.Read(wordOffset, 4)
					if !ok {
						return fmt.Errorf("failed to read flags word at offset %d", wordOffset)
					}
					word := binary.LittleEndian.Uint32(data)
					word |= 1 << bit
					writeUint32Le(ctx.Memory, wordOffset, word)
				}
			}
		}
		return nil

	case types.TypeKindList:
		list := &ctx.Types.Lists[typ.Index]
		elements := val.List()
		length := uint32(len(elements))

		if length == 0 {
			writeUint32Le(ctx.Memory, offset, 0)
			writeUint32Le(ctx.Memory, offset+4, 0)
			return nil
		}

		if ctx.Realloc == nil {
			return fmt.Errorf("lower list: realloc function required for non-empty list")
		}

		elemABI := list.Element.ABI(ctx.Types)
		elemSize := elemABI.Size32
		elemAlign := elemABI.Align32
		totalSize := length * elemSize

		ptr, err := ctx.Realloc(0, 0, elemAlign, totalSize)
		if err != nil {
			return fmt.Errorf("lower list: realloc failed: %w", err)
		}

		for i := uint32(0); i < length; i++ {
			elemOffset := ptr + i*elemSize
			if err := LowerHeap(ctx, list.Element, elements[i], elemOffset); err != nil {
				return fmt.Errorf("lower list element %d: %w", i, err)
			}
		}

		writeUint32Le(ctx.Memory, offset, ptr)
		writeUint32Le(ctx.Memory, offset+4, length)
		return nil

	case types.TypeKindFixedList:
		fl := &ctx.Types.FixedLists[typ.Index]
		elements := val.List()
		if uint32(len(elements)) != fl.Length {
			return fmt.Errorf("fixed list length mismatch: got %d, expected %d", len(elements), fl.Length)
		}
		elemABI := fl.Element.ABI(ctx.Types)
		elemSize := elemABI.Size32
		for i, elem := range elements {
			elemOffset := offset + uint32(i)*elemSize
			if err := LowerHeap(ctx, fl.Element, elem, elemOffset); err != nil {
				return fmt.Errorf("lower fixed list element %d: %w", i, err)
			}
		}
		return nil

	case types.TypeKindOwn:
		flat, err := lowerOwnHandleFlat(ctx, typ, val)
		if err != nil {
			return err
		}
		writeUint32Le(ctx.Memory, offset, uint32(flat[0]))
		return nil
	case types.TypeKindBorrow:
		flat, err := lowerBorrowHandleFlat(ctx, typ, val)
		if err != nil {
			return err
		}
		writeUint32Le(ctx.Memory, offset, uint32(flat[0]))
		return nil

	case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
		return fmt.Errorf(
			"component-model async types not yet supported: kind=%d", typ.Kind)

	default:
		return fmt.Errorf("LowerHeap: unknown TypeKind %d", typ.Kind)
	}
}

// lowerOwnHandleFlat is the TypeKindOwn lowering arm. It resolves the
// resource type, allocates a new owned handle in the caller's table, and
// returns the single-i32 flat encoding.
//
// Spec: definitions.py (lower_own).
func lowerOwnHandleFlat(ctx *LowerContext, typ types.ValType, val types.Val) ([]uint64, error) {
	if ctx == nil || ctx.Instance == nil {
		return nil, fmt.Errorf("lower own: no component instance available")
	}
	if ctx.Types == nil {
		return nil, fmt.Errorf("lower own: no component types available")
	}
	if int(typ.Index) >= len(ctx.Types.ResourceTables) {
		return nil, fmt.Errorf("lower own: resource table index %d out of range", typ.Index)
	}
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return nil, fmt.Errorf(
			"cannot lower abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return nil, fmt.Errorf(
			"no resource type for instance %d declaration %d "+
				"(resource concrete promotion not yet wired — session 2)",
			rt.Instance, rt.Resource)
	}
	rep := val.Own()
	h, err := ctx.Instance.Table.NewResourceHandle(uint32(rep), true /* own */, expectedRT)
	if err != nil {
		return nil, err
	}
	return []uint64{uint64(h.Index())}, nil
}

// lowerBorrowHandleFlat is the TypeKindBorrow lowering arm. It resolves
// the resource type and folds in the same-instance optimization from
// CanonicalABI.md:2677-2683: if the calling instance is the one that
// defined the resource, return rep directly without allocating a new
// handle. Otherwise, allocate a borrow handle in the caller's table.
func lowerBorrowHandleFlat(ctx *LowerContext, typ types.ValType, val types.Val) ([]uint64, error) {
	if ctx == nil || ctx.Instance == nil {
		return nil, fmt.Errorf("lower borrow: no component instance available")
	}
	if ctx.Types == nil {
		return nil, fmt.Errorf("lower borrow: no component types available")
	}
	if int(typ.Index) >= len(ctx.Types.ResourceTables) {
		return nil, fmt.Errorf("lower borrow: resource table index %d out of range", typ.Index)
	}
	rt := ctx.Types.ResourceTables[typ.Index]
	if !rt.Concrete {
		return nil, fmt.Errorf(
			"cannot lower abstract resource at runtime (type %d)", typ.Index)
	}
	expectedRT := ctx.Instance.LookupResourceType(rt.Instance, rt.Resource)
	if expectedRT == nil {
		return nil, fmt.Errorf(
			"no resource type for instance %d declaration %d "+
				"(resource concrete promotion not yet wired — session 2)",
			rt.Instance, rt.Resource)
	}
	rep := val.Borrow()
	// Same-instance optimization (CanonicalABI.md:2677-2683): if the
	// calling instance is the one that defined the resource, return
	// rep directly without allocating a new handle. Comparison is
	// pointer identity on *runtime.ComponentInstance.
	if ctx.Instance == expectedRT.Impl {
		return []uint64{uint64(rep)}, nil
	}
	// Spec: definitions.py:1645 — assert(isinstance(cx.borrow_scope, Task))
	// The CallContext (borrow_scope) must be present for cross-instance borrows.
	if ctx.CallContext == nil {
		return nil, fmt.Errorf("lower borrow: CallContext (borrow_scope) is required for cross-instance borrow")
	}
	// Cross-instance: allocate a borrow handle in the caller's table.
	h, err := ctx.Instance.Table.NewResourceHandle(uint32(rep), false /* borrow */, expectedRT)
	if err != nil {
		return nil, err
	}
	// Spec: definitions.py:1649 — h.borrow_scope.num_borrows += 1
	ctx.CallContext.IncrementBorrows()
	// Set the CallContext on the borrow handle entry so that
	// resource.drop can find it to decrement num_borrows.
	// Spec: definitions.py:1648 — ResourceHandle(..., borrow_scope=cx.borrow_scope)
	_, entry, gerr := ctx.Instance.Table.GetByIndex(h.Index())
	if gerr != nil {
		return nil, fmt.Errorf("lower borrow: internal error: handle %d not found after insert: %w", h.Index(), gerr)
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return nil, fmt.Errorf("lower borrow: internal error: handle entry is %T, want *ResourceHandleEntry", entry)
	}
	resEntry.CallContext = ctx.CallContext
	return []uint64{uint64(h.Index())}, nil
}

// paramsTupleLayout computes the (size, align, per-element offsets) of
// the synthetic tuple-of-params formed from the given component param
// types. This is the shared abi-layer helper used by the retptr paths
// of LowerParams / LiftParams / LiftResults; the algorithm mirrors
// computeRecordABI / computeTupleABI at
// internal/component/types/abi_info.go:107-142 (the spec's
// alignment(tuple_type) / elem_size(tuple_type) from
// definitions.py:1087-1091, :1145-1151). It is kept in the abi package
// because callers already own a finished *types.ComponentTypes and do
// not want to intern a fresh tuple type just to query its layout.
//
// Spec: definitions.py:1947-1949, :1966-1967 — lift/lower flat values
// retptr branches compute alignment(tuple_type) and
// elem_size(tuple_type) on the TupleType(ts) synthesized from the
// param/result type list.
func paramsTupleLayout(ct *types.ComponentTypes, elems []types.ValType) (size, align uint32, offsets []uint32) {
	size, align = 0, 1
	offsets = make([]uint32, len(elems))
	for i, e := range elems {
		ea := e.ABI(ct)
		if ea.Align32 > align {
			align = ea.Align32
		}
		size = types.AlignTo(size, ea.Align32)
		offsets[i] = size
		size += ea.Size32
	}
	size = types.AlignTo(size, align)
	return size, align, offsets
}

// LowerParams lowers a slice of component-level argument values into the
// flat core-wasm form expected by the callee. Implements the aggregate
// boundary decision from lower_flat_values at
// debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:1954-1974:
//
//   - Flatten paramTypes via FlattenParams.
//   - If len(flatTypes) <= maxFlat: lower each argument with LowerFlat
//     and concatenate the resulting u64 slices.
//   - If len(flatTypes) >  maxFlat: compute the tuple-of-params layout
//     via paramsTupleLayout (same algorithm as computeRecordABI), realloc
//     a buffer, store each arg via LowerHeap at the aligned offset, and
//     return [ptr] as the single flat u64.
//
// The caller is responsible for toggling ctx.Instance.MayLeave around
// this call per definitions.py:1955 / :1973. Keeping LowerParams pure
// with respect to instance state allows test harnesses to exercise it
// with a detached runtime.
//
// Spec: definitions.py:1954-1974 lower_flat_values, MaxFlatParams=16
// from definitions.py constants.
// Wasmtime parallel: runtime/component/func.rs Func::call_raw
// aggregate lowering (the lower_args path).
func LowerParams(ctx *LowerContext, paramTypes []types.ValType, args []types.Val, maxFlat int) ([]uint64, error) {
	if len(paramTypes) != len(args) {
		return nil, fmt.Errorf(
			"LowerParams: %d args for %d param types",
			len(args), len(paramTypes))
	}
	if ctx == nil || ctx.Types == nil {
		return nil, fmt.Errorf("LowerParams: no component types available")
	}

	flatTypes := FlattenParams(ctx.Types, paramTypes)
	if len(flatTypes) <= maxFlat {
		// Flat path: lower each arg and concatenate.
		var result []uint64
		for i, pt := range paramTypes {
			flat, err := LowerFlat(ctx, pt, args[i])
			if err != nil {
				return nil, fmt.Errorf("LowerParams: param %d: %w", i, err)
			}
			result = append(result, flat...)
		}
		return result, nil
	}

	// Retptr path: compute the tuple-of-params layout via the shared
	// helper, realloc a buffer, store each arg via LowerHeap, return
	// [ptr].
	if ctx.Realloc == nil {
		return nil, fmt.Errorf(
			"LowerParams: realloc required for retptr path (%d flat > maxFlat=%d)",
			len(flatTypes), maxFlat)
	}
	tupleSize, tupleAlign, offsets := paramsTupleLayout(ctx.Types, paramTypes)

	ptr, err := ctx.Realloc(0, 0, tupleAlign, tupleSize)
	if err != nil {
		return nil, fmt.Errorf("LowerParams: realloc failed: %w", err)
	}
	for i, pt := range paramTypes {
		if err := LowerHeap(ctx, pt, args[i], ptr+offsets[i]); err != nil {
			return nil, fmt.Errorf("LowerParams: param %d: %w", i, err)
		}
	}
	return []uint64{uint64(ptr)}, nil
}

// LowerResults lowers component Val results into the flat core-wasm
// form returned by a lowered component function. This is the symmetric
// companion to LowerParams for the canon.lower return path; it is
// called from the Go host side of canon.lower once the callee has
// produced its results.
//
// Two modes, driven by needsRetptr (equivalent to the bool returned by
// FlattenResults / CoreSignature):
//
//   - needsRetptr == false: the flattened result width is <= maxFlat
//     and the core function is expected to return the results directly
//     as its core-wasm return values. LowerResults lowers each result
//     with LowerFlat and writes the concatenated u64 values into
//     stack[0..flatResultWidth], left-aligned. The caller uses this
//     prefix as the core return values.
//
//   - needsRetptr == true: the flattened result width exceeds maxFlat.
//     By the convention emitted by CoreSignature at
//     internal/component/abi/flatten.go:41-51, the retptr is appended
//     as the LAST core-wasm parameter of a lowered function (a
//     `(params..., retptr: i32)` signature). The canon.lower host glue
//     places the core-wasm parameter stack in `stack`; therefore
//     stack[len(stack)-1] holds the caller-provided retptr. LowerResults
//     reads that retptr, computes the tuple-of-results layout via the
//     shared paramsTupleLayout helper, and stores each result via
//     LowerHeap at ptr + offsets[i]. This matches wasmtime's
//     runtime/component/func.rs Func::call_raw result-store path (the
//     store_args mirror for results) and the spec's canon_lower path at
//     definitions.py:2104 which passes `flat_args` as `out_param` so
//     that lower_flat_values :1964 reads the retptr from the next i32
//     of the in-iter (definitively the trailing core-wasm param after
//     flat param consumption).
//
// Spec: definitions.py:1954-1974 lower_flat_values;
// definitions.py:2104 canon_lower calls lower_flat_values with
// flat_args as out_param for the result path.
func LowerResults(ctx *LowerContext, resultTypes []types.ValType, results []types.Val, stack []uint64, needsRetptr bool, maxFlat int) error {
	if ctx == nil || ctx.Types == nil {
		return fmt.Errorf("LowerResults: no component types available")
	}
	if len(results) != len(resultTypes) {
		return fmt.Errorf(
			"LowerResults: %d results for %d result types",
			len(results), len(resultTypes))
	}

	if needsRetptr {
		// Retptr path: stack[len(stack)-1] is the caller-provided retptr
		// by the CoreSignature convention (see flatten.go:41-51 — retptr
		// is appended as the final core-wasm param).
		if len(stack) == 0 {
			return fmt.Errorf("LowerResults: needsRetptr=true but stack is empty")
		}
		if ctx.Memory == nil {
			return fmt.Errorf("LowerResults: retptr path requires memory")
		}
		ptr := uint32(stack[len(stack)-1])
		tupleSize, tupleAlign, offsets := paramsTupleLayout(ctx.Types, resultTypes)
		if ptr != types.AlignTo(ptr, tupleAlign) {
			return fmt.Errorf(
				"LowerResults: retptr %d not aligned to %d", ptr, tupleAlign)
		}
		if uint64(ptr)+uint64(tupleSize) > uint64(ctx.Memory.Size()) {
			return fmt.Errorf(
				"LowerResults: retptr %d + tuple size %d out of memory bounds",
				ptr, tupleSize)
		}
		for i, rt := range resultTypes {
			if err := LowerHeap(ctx, rt, results[i], ptr+offsets[i]); err != nil {
				return fmt.Errorf("LowerResults: result %d: %w", i, err)
			}
		}
		return nil
	}

	// Flat path: lower each result and write into stack in order.
	idx := 0
	for i, rt := range resultTypes {
		lowered, err := LowerFlat(ctx, rt, results[i])
		if err != nil {
			return fmt.Errorf("LowerResults: result %d: %w", i, err)
		}
		for _, v := range lowered {
			if idx >= len(stack) {
				return fmt.Errorf(
					"LowerResults: flat overflow stack (idx=%d, len=%d)",
					idx, len(stack))
			}
			stack[idx] = v
			idx++
		}
	}
	return nil
}

// isValidUnicodeScalarRune checks if a rune is a valid Unicode scalar value.
// Unicode scalar values are any code point except high-surrogate and low-surrogate code points.
// Valid ranges: U+0000 to U+D7FF and U+E000 to U+10FFFF
func isValidUnicodeScalarRune(r rune) bool {
	if r >= 0xD800 && r <= 0xDFFF {
		return false
	}
	if r > 0x10FFFF {
		return false
	}
	if r < 0 {
		return false
	}
	return true
}

// coerceFlatValueForLower coerces a flat value from 'have' type to 'want' type for lowering.
// This implements the coercion rules from Canonical ABI spec lines 3088-3094.
// When lowering variants, payload values must be coerced from the case type to the joined type:
// - f32 to i32: value already contains f32 bits encoded as uint64
// - i32 to i64: value is already zero-extended in uint64
// - f32 to i64: value contains f32 bits, already zero-extended in uint64
// - f64 to i64: value already contains f64 bits encoded as uint64
func coerceFlatValueForLower(value uint64, have, want api.ValueType) uint64 {
	if have == want {
		return value
	}
	switch {
	case have == api.ValueTypeF32 && want == api.ValueTypeI32:
		return value
	case have == api.ValueTypeI32 && want == api.ValueTypeI64:
		return value
	case have == api.ValueTypeF32 && want == api.ValueTypeI64:
		return value
	case have == api.ValueTypeF64 && want == api.ValueTypeI64:
		return value
	default:
		return value
	}
}
