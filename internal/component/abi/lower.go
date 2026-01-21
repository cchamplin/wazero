package abi

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
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
		c := val.Char()
		if !isValidUnicodeScalarRune(c) {
			return nil, fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		return []uint64{uint64(c)}, nil
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

		// Calculate joined flat types per Canonical ABI spec lines 3077-3098
		flatTypes := flattenVariantPayload(t)

		result := []uint64{uint64(caseIdx)}

		if caseType != nil && payload != nil {
			// Lower the payload
			payloadFlat, err := LowerFlat(ctx, caseType, *payload)
			if err != nil {
				return nil, fmt.Errorf("lower variant payload: %w", err)
			}

			// Coerce each payload value from case type to joined type
			caseFlat := flattenType(caseType)
			for i, pv := range payloadFlat {
				have := caseFlat[i]
				want := flatTypes[i]
				result = append(result, coerceFlatValueForLower(pv, have, want))
			}

			// Pad remaining slots with zeros
			for i := len(payloadFlat); i < len(flatTypes); i++ {
				result = append(result, 0)
			}
		} else {
			// No payload - pad all slots with zeros
			for i := 0; i < len(flatTypes); i++ {
				result = append(result, 0)
			}
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
		t := typ.(types.List)
		elements := val.List()

		if t.Length != nil {
			// Fixed-length list: validate length and lower each element
			if uint32(len(elements)) != *t.Length {
				return nil, fmt.Errorf("fixed list length mismatch: got %d, expected %d", len(elements), *t.Length)
			}

			var result []uint64
			for i, elem := range elements {
				flat, err := LowerFlat(ctx, t.Element, elem)
				if err != nil {
					return nil, fmt.Errorf("lower fixed list element %d: %w", i, err)
				}
				result = append(result, flat...)
			}
			return result, nil
		}

		// Dynamic list: existing code
		length := uint32(len(elements))

		// Empty list case - no allocation needed
		if length == 0 {
			return []uint64{0, 0}, nil
		}

		// Need realloc for non-empty lists
		if ctx == nil || ctx.Realloc == nil {
			return nil, fmt.Errorf("lower list: realloc function required for non-empty list")
		}

		// Calculate total size needed
		elemSize := t.Element.Size()
		elemAlign := t.Element.Align()
		totalSize := length * elemSize

		// Allocate memory for the list
		ptr, err := ctx.Realloc(0, 0, elemAlign, totalSize)
		if err != nil {
			return nil, fmt.Errorf("lower list: realloc failed: %w", err)
		}

		// Lower each element to heap
		for i := uint32(0); i < length; i++ {
			offset := ptr + i*elemSize
			if err := LowerHeap(ctx, t.Element, elements[i], offset); err != nil {
				return nil, fmt.Errorf("lower list element %d: %w", i, err)
			}
		}

		return []uint64{uint64(ptr), uint64(length)}, nil

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
		c := val.Char()
		if !isValidUnicodeScalarRune(c) {
			return fmt.Errorf("invalid char value: U+%04X is not a valid Unicode scalar value", c)
		}
		writeUint32Le(ctx.Memory, offset, uint32(c))
		return nil
	case types.String:
		ptr, taggedLen, err := LowerString(ctx, val.StringVal())
		if err != nil {
			return err
		}
		writeUint32Le(ctx.Memory, offset, ptr)
		writeUint32Le(ctx.Memory, offset+4, taggedLen)
		return nil

	case types.Record:
		t := typ.(types.Record)
		rec := val.Record()
		fieldOffset := uint32(0)
		for _, f := range t.Fields {
			// Align field offset
			align := f.Type.Align()
			if fieldOffset%align != 0 {
				fieldOffset += align - (fieldOffset % align)
			}
			fieldVal, ok := rec[f.Name]
			if !ok {
				return fmt.Errorf("missing record field: %s", f.Name)
			}
			if err := LowerHeap(ctx, f.Type, fieldVal, offset+fieldOffset); err != nil {
				return fmt.Errorf("lower record field %s: %w", f.Name, err)
			}
			fieldOffset += f.Type.Size()
		}
		return nil

	case types.Tuple:
		t := typ.(types.Tuple)
		elems := val.Tuple()
		if len(elems) != len(t.Types) {
			return fmt.Errorf("tuple has %d elements, expected %d", len(elems), len(t.Types))
		}
		elemOffset := uint32(0)
		for i, elemType := range t.Types {
			// Align
			align := elemType.Align()
			if elemOffset%align != 0 {
				elemOffset += align - (elemOffset % align)
			}
			if err := LowerHeap(ctx, elemType, elems[i], offset+elemOffset); err != nil {
				return fmt.Errorf("lower tuple element %d: %w", i, err)
			}
			elemOffset += elemType.Size()
		}
		return nil

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
			return fmt.Errorf("unknown variant case: %s", caseName)
		}

		// Write discriminant
		discSize := t.DiscriminantSize()
		switch discSize {
		case 1:
			writeUint8(ctx.Memory, offset, uint8(caseIdx))
		case 2:
			writeUint16Le(ctx.Memory, offset, uint16(caseIdx))
		default:
			writeUint32Le(ctx.Memory, offset, uint32(caseIdx))
		}

		// Write payload if present
		if caseType != nil && payload != nil {
			payloadOffset := t.PayloadOffset()
			if err := LowerHeap(ctx, caseType, *payload, offset+payloadOffset); err != nil {
				return fmt.Errorf("lower variant payload: %w", err)
			}
		}
		return nil

	case types.Option:
		t := typ.(types.Option)
		payload := val.Option()

		if payload == nil {
			// None: discriminant = 0
			writeUint8(ctx.Memory, offset, 0)
			return nil
		}

		// Some: discriminant = 1
		writeUint8(ctx.Memory, offset, 1)

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
			if err := LowerHeap(ctx, t.Some, *payload, offset+payloadOffset); err != nil {
				return fmt.Errorf("lower option payload: %w", err)
			}
		}
		return nil

	case types.Result:
		t := typ.(types.Result)
		isOk, okVal, errVal := val.Result()

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

		if isOk {
			// Ok: discriminant = 0
			writeUint8(ctx.Memory, offset, 0)
			if t.Ok != nil && okVal != nil {
				if err := LowerHeap(ctx, t.Ok, *okVal, offset+payloadOffset); err != nil {
					return fmt.Errorf("lower result ok: %w", err)
				}
			}
		} else {
			// Error: discriminant = 1
			writeUint8(ctx.Memory, offset, 1)
			if t.Error != nil && errVal != nil {
				if err := LowerHeap(ctx, t.Error, *errVal, offset+payloadOffset); err != nil {
					return fmt.Errorf("lower result error: %w", err)
				}
			}
		}
		return nil

	case types.Enum:
		t := typ.(types.Enum)
		caseName := val.Enum()
		caseIdx := -1
		for i, c := range t.Cases {
			if c == caseName {
				caseIdx = i
				break
			}
		}
		if caseIdx == -1 {
			return fmt.Errorf("unknown enum case: %s", caseName)
		}

		// Write discriminant based on enum size
		discSize := t.Size()
		switch discSize {
		case 1:
			writeUint8(ctx.Memory, offset, uint8(caseIdx))
		case 2:
			writeUint16Le(ctx.Memory, offset, uint16(caseIdx))
		default:
			writeUint32Le(ctx.Memory, offset, uint32(caseIdx))
		}
		return nil

	case types.Flags:
		t := typ.(types.Flags)
		flags := val.Flags()
		if len(t.Names) == 0 {
			return nil
		}

		n := len(t.Names)
		if n <= 8 {
			var bits uint8
			for i, name := range t.Names {
				if flags[name] {
					bits |= 1 << i
				}
			}
			writeUint8(ctx.Memory, offset, bits)
		} else if n <= 16 {
			var bits uint16
			for i, name := range t.Names {
				if flags[name] {
					bits |= 1 << i
				}
			}
			writeUint16Le(ctx.Memory, offset, bits)
		} else if n <= 32 {
			var bits uint32
			for i, name := range t.Names {
				if flags[name] {
					bits |= 1 << i
				}
			}
			writeUint32Le(ctx.Memory, offset, bits)
		} else {
			// Multiple u32s
			for i, name := range t.Names {
				if flags[name] {
					wordIdx := i / 32
					bit := i % 32
					wordOffset := offset + uint32(wordIdx*4)
					// Read, modify, write
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

	case types.List:
		t := typ.(types.List)
		elements := val.List()

		if t.Length != nil {
			// Fixed-length list: validate length and write elements inline
			if uint32(len(elements)) != *t.Length {
				return fmt.Errorf("fixed list length mismatch: got %d, expected %d", len(elements), *t.Length)
			}

			elemSize := t.Element.Size()
			for i, elem := range elements {
				elemOffset := offset + uint32(i)*elemSize
				if err := LowerHeap(ctx, t.Element, elem, elemOffset); err != nil {
					return fmt.Errorf("lower fixed list element %d: %w", i, err)
				}
			}
			return nil
		}

		// Dynamic list: existing code
		length := uint32(len(elements))

		// Empty list case
		if length == 0 {
			writeUint32Le(ctx.Memory, offset, 0)   // ptr
			writeUint32Le(ctx.Memory, offset+4, 0) // len
			return nil
		}

		// Need realloc for non-empty lists
		if ctx.Realloc == nil {
			return fmt.Errorf("lower list: realloc function required for non-empty list")
		}

		// Calculate total size needed
		elemSize := t.Element.Size()
		elemAlign := t.Element.Align()
		totalSize := length * elemSize

		// Allocate memory for the list elements
		ptr, err := ctx.Realloc(0, 0, elemAlign, totalSize)
		if err != nil {
			return fmt.Errorf("lower list: realloc failed: %w", err)
		}

		// Lower each element to heap
		for i := uint32(0); i < length; i++ {
			elemOffset := ptr + i*elemSize
			if err := LowerHeap(ctx, t.Element, elements[i], elemOffset); err != nil {
				return fmt.Errorf("lower list element %d: %w", i, err)
			}
		}

		// Write ptr and length at the given offset
		writeUint32Le(ctx.Memory, offset, ptr)
		writeUint32Le(ctx.Memory, offset+4, length)
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

// isValidUnicodeScalarRune checks if a rune is a valid Unicode scalar value.
// Unicode scalar values are any code point except high-surrogate and low-surrogate code points.
// Valid ranges: U+0000 to U+D7FF and U+E000 to U+10FFFF
func isValidUnicodeScalarRune(r rune) bool {
	// Check for surrogates (U+D800 to U+DFFF)
	if r >= 0xD800 && r <= 0xDFFF {
		return false
	}
	// Check for values above maximum Unicode code point
	if r > 0x10FFFF {
		return false
	}
	// Check for negative values (rune is int32)
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
		// f32 bits encoded as i32 - value already has the bits
		return value
	case have == api.ValueTypeI32 && want == api.ValueTypeI64:
		// i32 zero-extended to i64 - value is already in uint64
		return value
	case have == api.ValueTypeF32 && want == api.ValueTypeI64:
		// f32 bits encoded as i32, then zero-extended to i64
		return value
	case have == api.ValueTypeF64 && want == api.ValueTypeI64:
		// f64 bits encoded as i64 - value already has the bits
		return value
	default:
		return value
	}
}
