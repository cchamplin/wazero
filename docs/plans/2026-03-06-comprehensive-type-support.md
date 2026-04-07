# Comprehensive Type Support for ExportedFunc.Call()

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add support for all component model value types in ExportedFunc.Call() parameter lowering and result lifting, with comprehensive integration tests.

**Architecture:** Create a `lowerParam` helper method on ExportedFunc that handles recursive lowering of all value types (mirrors abi.LowerFlat but lives in `component` package to avoid circular imports since `abi` imports `component`). Add missing type cases to `liftResolvedType` for result lifting. Extend the go-repro-plugin test component with echo functions for every type, plus multi-param and mixed-type test cases.

**Tech Stack:** Go, WASM Component Model, wit-bindgen 0.53.1, wasm-tools 1.245.1

**Circular Import Constraint:** `internal/component/abi` imports `internal/component`, so `instance.go` (package `component`) CANNOT import `abi`. All lowering/lifting logic added to `instance.go` must be self-contained. The implementations mirror `abi.LowerFlat`/`abi.LiftFlat` but are not duplicates in a harmful sense - they operate on different input types (`Val.Kind()` dispatch vs `types.ValType` dispatch) and serve different call paths (host→guest exports vs internal canonical ABI).

---

## Task 1: Add `lowerParam` Helper Method

Adds a single recursive method on `ExportedFunc` that can lower any `Val` to flat core wasm values using resolved type information. This replaces the limited inline switch and handles all types.

**Files:**
- Modify: `internal/component/instance.go`

**Step 1: Add the `lowerParam` method after the existing `liftResolvedPrimitiveVal` method (after ~line 1079)**

This method handles all ValKinds with recursive descent for composite types. When type info is available (via resolver), it uses it for proper field ordering, case indices, etc.

```go
// lowerParam lowers a single component Val to flat core wasm values.
// When resolvedType is non-nil, it uses type information for proper lowering
// of composite types. When nil, falls back to Val.Kind()-based dispatch for primitives.
func (f *ExportedFunc) lowerParam(ctx context.Context, val Val, resolvedType types.ValType) ([]uint64, error) {
	// If we have resolved type info, use type-driven lowering
	if resolvedType != nil {
		return f.lowerTyped(ctx, val, resolvedType)
	}
	// Fallback: kind-based lowering for primitives only
	return f.lowerByKind(ctx, val)
}

// lowerTyped lowers a Val using resolved type information.
// This mirrors abi.LowerFlat but lives in component package to avoid circular imports.
func (f *ExportedFunc) lowerTyped(ctx context.Context, val Val, typ types.ValType) ([]uint64, error) {
	switch t := typ.(type) {
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
		return f.lowerStringParam(ctx, val.StringVal())
	case types.Record:
		rec := val.Record()
		var result []uint64
		for _, field := range t.Fields {
			fieldVal, ok := rec[field.Name]
			if !ok {
				return nil, fmt.Errorf("missing record field: %s", field.Name)
			}
			flat, err := f.lowerTyped(ctx, fieldVal, field.Type)
			if err != nil {
				return nil, fmt.Errorf("lower record field %s: %w", field.Name, err)
			}
			result = append(result, flat...)
		}
		return result, nil
	case types.Tuple:
		elems := val.Tuple()
		if len(elems) != len(t.Types) {
			return nil, fmt.Errorf("tuple has %d elements, expected %d", len(elems), len(t.Types))
		}
		var result []uint64
		for i, elemType := range t.Types {
			flat, err := f.lowerTyped(ctx, elems[i], elemType)
			if err != nil {
				return nil, fmt.Errorf("lower tuple element %d: %w", i, err)
			}
			result = append(result, flat...)
		}
		return result, nil
	case types.Enum:
		caseName := val.Enum()
		for i, c := range t.Cases {
			if c == caseName {
				return []uint64{uint64(i)}, nil
			}
		}
		return nil, fmt.Errorf("unknown enum case: %s", caseName)
	case types.Flags:
		flags := val.Flags()
		if len(t.Names) == 0 {
			return []uint64{}, nil
		}
		numI32s := (len(t.Names) + 31) / 32
		result := make([]uint64, numI32s)
		for i, name := range t.Names {
			if flags[name] {
				result[i/32] |= 1 << (i % 32)
			}
		}
		return result, nil
	case types.Option:
		payload := val.Option()
		payloadFlat := 0
		if t.Some != nil {
			payloadFlat = t.Some.FlattenCount()
		}
		if payload == nil {
			result := make([]uint64, 1+payloadFlat)
			return result, nil
		}
		result := []uint64{1}
		if t.Some != nil {
			flat, err := f.lowerTyped(ctx, *payload, t.Some)
			if err != nil {
				return nil, fmt.Errorf("lower option payload: %w", err)
			}
			result = append(result, flat...)
		}
		return result, nil
	case types.Result:
		isOk, okVal, errVal := val.Result()
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
			result := []uint64{0}
			payloadCount := 0
			if t.Ok != nil && okVal != nil {
				flat, err := f.lowerTyped(ctx, *okVal, t.Ok)
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
		if t.Error != nil && errVal != nil {
			flat, err := f.lowerTyped(ctx, *errVal, t.Error)
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
	case types.Variant:
		caseName, payload := val.Variant()
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
		// Calculate joined flat types for the variant
		joinedFlat := f.flattenVariantPayload(t)
		result := []uint64{uint64(caseIdx)}
		if caseType != nil && payload != nil {
			payloadFlat, err := f.lowerTyped(ctx, *payload, caseType)
			if err != nil {
				return nil, fmt.Errorf("lower variant payload: %w", err)
			}
			// Append payload values (coercion is identity at uint64 level for lowering)
			result = append(result, payloadFlat...)
			// Pad remaining slots
			for i := len(payloadFlat); i < len(joinedFlat); i++ {
				result = append(result, 0)
			}
		} else {
			for i := 0; i < len(joinedFlat); i++ {
				result = append(result, 0)
			}
		}
		return result, nil
	case types.List:
		list := val.List()
		length := uint32(len(list))
		if length == 0 {
			return []uint64{0, 0}, nil
		}
		if f.memory == nil {
			return nil, fmt.Errorf("list lowering requires memory")
		}
		if f.reallocFunc == nil {
			return nil, fmt.Errorf("list lowering requires realloc function")
		}
		elemSize := t.Element.Size()
		elemAlign := t.Element.Align()
		totalSize := length * elemSize
		allocResults, err := f.reallocFunc.Call(ctx, 0, 0, uint64(elemAlign), uint64(totalSize))
		if err != nil {
			return nil, fmt.Errorf("realloc for list failed: %w", err)
		}
		ptr := uint32(allocResults[0])
		for i := uint32(0); i < length; i++ {
			offset := ptr + i*elemSize
			if err := f.lowerToMemory(ctx, list[i], t.Element, offset); err != nil {
				return nil, fmt.Errorf("lower list element %d: %w", i, err)
			}
		}
		return []uint64{uint64(ptr), uint64(length)}, nil
	case types.Own:
		rep := val.Own()
		if f.instance == nil || f.instance.resourceTable == nil {
			return nil, fmt.Errorf("lower own: no resource table available")
		}
		h := f.instance.resourceTable.New(rep, true)
		return []uint64{uint64(h.Index())}, nil
	case types.Borrow:
		rep := val.Borrow()
		if f.instance == nil || f.instance.resourceTable == nil {
			return nil, fmt.Errorf("lower borrow: no resource table available")
		}
		h := f.instance.resourceTable.New(rep, false)
		return []uint64{uint64(h.Index())}, nil
	default:
		return nil, fmt.Errorf("unsupported type for lowering: %T", typ)
	}
}

// lowerByKind lowers a Val using only its runtime Kind (no type info).
// Handles primitives and string. Composite types require type info.
func (f *ExportedFunc) lowerByKind(ctx context.Context, val Val) ([]uint64, error) {
	switch val.Kind() {
	case ValKindBool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case ValKindS8:
		return []uint64{uint64(uint32(int32(val.S8())))}, nil
	case ValKindU8:
		return []uint64{uint64(val.U8())}, nil
	case ValKindS16:
		return []uint64{uint64(uint32(int32(val.S16())))}, nil
	case ValKindU16:
		return []uint64{uint64(val.U16())}, nil
	case ValKindS32:
		return []uint64{uint64(uint32(val.S32()))}, nil
	case ValKindU32:
		return []uint64{uint64(val.U32())}, nil
	case ValKindS64:
		return []uint64{uint64(val.S64())}, nil
	case ValKindU64:
		return []uint64{val.U64()}, nil
	case ValKindF32:
		return []uint64{uint64(math.Float32bits(val.F32()))}, nil
	case ValKindF64:
		return []uint64{math.Float64bits(val.F64())}, nil
	case ValKindChar:
		return []uint64{uint64(val.Char())}, nil
	case ValKindString:
		return f.lowerStringParam(ctx, val.StringVal())
	case ValKindOwn:
		rep := val.Own()
		if f.instance == nil || f.instance.resourceTable == nil {
			return nil, fmt.Errorf("lower own: no resource table available")
		}
		h := f.instance.resourceTable.New(rep, true)
		return []uint64{uint64(h.Index())}, nil
	case ValKindBorrow:
		rep := val.Borrow()
		if f.instance == nil || f.instance.resourceTable == nil {
			return nil, fmt.Errorf("lower borrow: no resource table available")
		}
		h := f.instance.resourceTable.New(rep, false)
		return []uint64{uint64(h.Index())}, nil
	default:
		return nil, fmt.Errorf("unsupported parameter type: %s (type info required for composite types)", val.Kind())
	}
}

// lowerStringParam lowers a string to (ptr, len) in linear memory.
func (f *ExportedFunc) lowerStringParam(ctx context.Context, s string) ([]uint64, error) {
	data := []byte(s)
	length := uint32(len(data))
	if length == 0 {
		return []uint64{0, 0}, nil
	}
	if f.reallocFunc == nil {
		return nil, fmt.Errorf("string lowering requires realloc function")
	}
	if f.memory == nil {
		return nil, fmt.Errorf("string lowering requires memory")
	}
	results, err := f.reallocFunc.Call(ctx, 0, 0, 1, uint64(length))
	if err != nil {
		return nil, fmt.Errorf("realloc for string failed: %w", err)
	}
	ptr := uint32(results[0])
	if !f.memory.Write(ptr, data) {
		return nil, fmt.Errorf("failed to write string to memory at offset %d", ptr)
	}
	return []uint64{uint64(ptr), uint64(length)}, nil
}

// lowerToMemory writes a Val to linear memory at the given offset.
// Used for list elements and other heap-stored values.
func (f *ExportedFunc) lowerToMemory(ctx context.Context, val Val, typ types.ValType, offset uint32) error {
	switch t := typ.(type) {
	case types.Bool:
		b := byte(0)
		if val.Bool() {
			b = 1
		}
		f.memory.Write(offset, []byte{b})
		return nil
	case types.S8:
		f.memory.Write(offset, []byte{byte(val.S8())})
		return nil
	case types.U8:
		f.memory.Write(offset, []byte{val.U8()})
		return nil
	case types.S16:
		buf := make([]byte, 2)
		buf[0] = byte(val.S16())
		buf[1] = byte(int16(val.S16()) >> 8)
		f.memory.Write(offset, buf)
		return nil
	case types.U16:
		buf := make([]byte, 2)
		buf[0] = byte(val.U16())
		buf[1] = byte(val.U16() >> 8)
		f.memory.Write(offset, buf)
		return nil
	case types.S32:
		f.memory.WriteUint32Le(offset, uint32(val.S32()))
		return nil
	case types.U32:
		f.memory.WriteUint32Le(offset, val.U32())
		return nil
	case types.S64:
		f.memory.WriteUint64Le(offset, uint64(val.S64()))
		return nil
	case types.U64:
		f.memory.WriteUint64Le(offset, val.U64())
		return nil
	case types.F32:
		f.memory.WriteFloat32Le(offset, val.F32())
		return nil
	case types.F64:
		f.memory.WriteFloat64Le(offset, val.F64())
		return nil
	case types.Char:
		f.memory.WriteUint32Le(offset, uint32(val.Char()))
		return nil
	case types.String:
		flat, err := f.lowerStringParam(ctx, val.StringVal())
		if err != nil {
			return err
		}
		f.memory.WriteUint32Le(offset, uint32(flat[0]))
		f.memory.WriteUint32Le(offset+4, uint32(flat[1]))
		return nil
	case types.Record:
		rec := val.Record()
		fieldOffset := uint32(0)
		for _, field := range t.Fields {
			align := field.Type.Align()
			if fieldOffset%align != 0 {
				fieldOffset += align - (fieldOffset % align)
			}
			fieldVal, ok := rec[field.Name]
			if !ok {
				return fmt.Errorf("missing record field: %s", field.Name)
			}
			if err := f.lowerToMemory(ctx, fieldVal, field.Type, offset+fieldOffset); err != nil {
				return fmt.Errorf("lower record field %s: %w", field.Name, err)
			}
			fieldOffset += field.Type.Size()
		}
		return nil
	case types.Enum:
		caseName := val.Enum()
		for i, c := range t.Cases {
			if c == caseName {
				switch t.Size() {
				case 1:
					f.memory.Write(offset, []byte{byte(i)})
				case 2:
					buf := make([]byte, 2)
					buf[0] = byte(i)
					buf[1] = byte(i >> 8)
					f.memory.Write(offset, buf)
				default:
					f.memory.WriteUint32Le(offset, uint32(i))
				}
				return nil
			}
		}
		return fmt.Errorf("unknown enum case: %s", caseName)
	default:
		return fmt.Errorf("unsupported type for memory lowering: %T", typ)
	}
}

// flattenVariantPayload computes the joined flat types for a variant's payload.
// This implements the join algorithm from the Canonical ABI spec.
func (f *ExportedFunc) flattenVariantPayload(v types.Variant) []int {
	// We just need the count of joined flat values, not actual types
	// since at the uint64 level, coercion is identity
	maxLen := 0
	for _, c := range v.Cases {
		if c.Type != nil {
			n := c.Type.FlattenCount()
			if n > maxLen {
				maxLen = n
			}
		}
	}
	result := make([]int, maxLen)
	return result
}
```

**Step 2: Refactor the Call() method to use lowerParam**

Replace the existing big switch statement (lines ~208-342) in the `Call` method with delegation to `lowerParam`:

```go
	for i, p := range params {
		var resolvedType types.ValType
		if resolver != nil && f.funcType != nil && i < len(f.funcType.Params) {
			if rt, err := resolver.ResolveValType(f.funcType.Params[i].ValType); err == nil {
				resolvedType = rt
			}
		}
		flat, err := f.lowerParam(ctx, p, resolvedType)
		if err != nil {
			return nil, fmt.Errorf("lower param %d: %w", i, err)
		}
		coreParams = append(coreParams, flat...)
	}
```

This replaces the entire switch from `for i, p := range params {` through the closing `}` before `// === END LOWERING PARAMS`. Remove the now-unused `sort` import if no longer needed elsewhere in the file.

**Step 3: Run existing tests to verify no regressions**

Run: `go test ./internal/component/wasip2test/ -v -count=1`
Expected: All existing tests pass (including TestRepro_StringParameterSupport)

**Step 4: Commit**

```
feat(component): add lowerParam helper for comprehensive type lowering

Replaces the limited inline switch in ExportedFunc.Call() with a
recursive lowerParam method that handles all component model value
types. Cannot delegate to abi.LowerFlat due to circular imports
(abi imports component), so implements equivalent logic in the
component package.
```

---

## Task 2: Add Missing Result Lifting Types

Extends `liftResolvedType` to handle Enum, Flags, Variant, Tuple, and List result types.

**Files:**
- Modify: `internal/component/instance.go`

**Step 1: Add new cases to the `liftResolvedType` switch (before the `default:` case around line 1039)**

Add cases for Enum, Flags, Tuple, Variant, and List. These types may return via flat values (FlattenCount <= 1) or retptr (FlattenCount > 1).

```go
	case types.Enum:
		if len(coreResults) < 1 {
			return nil, fmt.Errorf("not enough core results for enum")
		}
		discriminant := int(coreResults[0])
		if discriminant < 0 || discriminant >= len(t.Cases) {
			return nil, fmt.Errorf("enum discriminant %d out of range (0..%d)", discriminant, len(t.Cases)-1)
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []Val{ValEnum(t.Cases[discriminant])}
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

	case types.Flags:
		numI32s := (len(t.Names) + 31) / 32
		if len(t.Names) == 0 {
			numI32s = 0
		}
		flagMap := make(map[string]bool)
		if numI32s <= 1 && len(coreResults) >= 1 {
			// Flat return (single i32)
			bits := coreResults[0]
			for i, name := range t.Names {
				if bits&(1<<i) != 0 {
					flagMap[name] = true
				}
			}
		} else if len(coreResults) >= 1 && f.memory != nil {
			// Retptr case
			retptr := uint32(coreResults[0])
			for i, name := range t.Names {
				wordIdx := i / 32
				bit := i % 32
				word, ok := f.memory.ReadUint32Le(retptr + uint32(wordIdx*4))
				if ok && word&(1<<bit) != 0 {
					flagMap[name] = true
				}
			}
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []Val{ValFlags(flagMap)}
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

	case types.Tuple:
		flatCount := t.FlattenCount()
		if flatCount <= 1 && len(coreResults) >= flatCount {
			// Flat case: lift elements from core results
			elems := make([]Val, len(t.Types))
			idx := 0
			for i, elemType := range t.Types {
				if idx < len(coreResults) {
					elems[i] = f.liftResolvedPrimitiveVal(coreResults[idx], elemType)
					idx++
				}
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []Val{ValTuple(elems)}
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		// Retptr case
		if len(coreResults) >= 1 && f.memory != nil {
			retptr := uint32(coreResults[0])
			elems := make([]Val, len(t.Types))
			offset := retptr
			for i, elemType := range t.Types {
				align := elemType.Align()
				if offset%align != 0 {
					offset += align - (offset % align)
				}
				val, size := f.liftFieldFromMemory(offset, elemType)
				elems[i] = val
				offset += size
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []Val{ValTuple(elems)}
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		return nil, fmt.Errorf("not enough core results for tuple")

	case types.Variant:
		if len(coreResults) < 1 {
			return nil, fmt.Errorf("not enough core results for variant")
		}
		flatCount := t.FlattenCount()
		if flatCount <= 1 || len(coreResults) >= flatCount {
			// Flat case
			discriminant := int(coreResults[0])
			if discriminant < 0 || discriminant >= len(t.Cases) {
				return nil, fmt.Errorf("variant discriminant %d out of range", discriminant)
			}
			c := t.Cases[discriminant]
			var payload *Val
			if c.Type != nil && len(coreResults) > 1 {
				v := f.liftResolvedPrimitiveVal(coreResults[1], c.Type)
				payload = &v
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []Val{ValVariant(c.Name, payload)}
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		// Retptr case
		if f.memory != nil {
			retptr := uint32(coreResults[0])
			discSize := t.DiscriminantSize()
			var discriminant int
			switch discSize {
			case 1:
				b, ok := f.memory.ReadByteAt(retptr)
				if !ok {
					return nil, fmt.Errorf("failed to read variant discriminant")
				}
				discriminant = int(b)
			case 2:
				v, ok := f.memory.ReadUint16Le(retptr)
				if !ok {
					return nil, fmt.Errorf("failed to read variant discriminant")
				}
				discriminant = int(v)
			default:
				v, ok := f.memory.ReadUint32Le(retptr)
				if !ok {
					return nil, fmt.Errorf("failed to read variant discriminant")
				}
				discriminant = int(v)
			}
			if discriminant < 0 || discriminant >= len(t.Cases) {
				return nil, fmt.Errorf("variant discriminant %d out of range", discriminant)
			}
			c := t.Cases[discriminant]
			var payload *Val
			if c.Type != nil {
				payloadOffset := t.PayloadOffset()
				v, _ := f.liftFieldFromMemory(retptr+payloadOffset, c.Type)
				payload = &v
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []Val{ValVariant(c.Name, payload)}
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		return nil, fmt.Errorf("variant result requires memory for retptr lifting")

	case types.List:
		// Lists always use retptr (FlattenCount=2 > MAX_FLAT_RESULTS=1)
		if len(coreResults) < 1 || f.memory == nil {
			return nil, fmt.Errorf("list result requires memory")
		}
		retptr := uint32(coreResults[0])
		ptr, ok := f.memory.ReadUint32Le(retptr)
		if !ok {
			return nil, fmt.Errorf("failed to read list ptr from memory")
		}
		length, ok := f.memory.ReadUint32Le(retptr + 4)
		if !ok {
			return nil, fmt.Errorf("failed to read list len from memory")
		}
		elems := make([]Val, length)
		elemSize := t.Element.Size()
		for i := uint32(0); i < length; i++ {
			elemOffset := ptr + i*elemSize
			val, _ := f.liftFieldFromMemory(elemOffset, t.Element)
			elems[i] = val
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []Val{ValList(elems)}
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil
```

**Step 2: Run existing tests**

Run: `go test ./internal/component/wasip2test/ -v -count=1`
Expected: All existing tests still pass

**Step 3: Commit**

```
feat(component): add result lifting for enum, flags, variant, tuple, list

Extends liftResolvedType to handle all remaining composite types.
Supports both flat returns and retptr-based returns.
```

---

## Task 3: Extend Test Component WIT Interface

Adds new types and echo functions to the go-repro-plugin WIT interface.

**Files:**
- Modify: `internal/component/wasip2test/go-repro-plugin/wit/repro.wit`

**Step 1: Update the WIT file**

Add new types to the `types` interface and new functions to the `handler` interface:

```wit
package test:repro;

/// Shared types used across interfaces.
interface types {
  /// A result record returned from handler exports.
  record process-result {
    value: u32,
    ok: bool,
  }

  /// Color enum for echo-enum test.
  enum color {
    red,
    green,
    blue,
  }

  /// Permissions flags for echo-flags test.
  flags permissions {
    read,
    write,
    execute,
  }

  /// Shape variant for echo-variant test.
  variant shape {
    circle(f64),
    square(f64),
    none,
  }
}

/// Host-provided function that the guest can call.
interface host-ops {
  use types.{process-result};
  get-value: func() -> u32;
  /// Takes a u64 param -- tests that canon lower produces i64 (not i32) in core ABI.
  get-random-len: func(len: u64) -> u64;
}

/// Separate RNG interface mirroring director's host-rng.
interface host-rng {
  get-random-bytes: func(count: u32) -> list<u8>;
}

/// Guest-exported handler interface.
interface handler {
  use types.{process-result, color, permissions, shape};
  process: func() -> process-result;
  process-random: func(len: u64) -> u64;
  process-random-bytes: func(count: u32) -> list<u8>;
  echo-string: func(input: string) -> string;

  /// Primitive echo functions
  echo-bool: func(input: bool) -> bool;
  echo-s8: func(input: s8) -> s8;
  echo-u8: func(input: u8) -> u8;
  echo-s16: func(input: s16) -> s16;
  echo-u16: func(input: u16) -> u16;
  echo-f32: func(input: f32) -> f32;
  echo-f64: func(input: f64) -> f64;
  echo-char: func(input: char) -> char;

  /// Composite type echo functions
  echo-enum: func(input: color) -> color;
  echo-flags: func(input: permissions) -> permissions;
  echo-variant: func(input: shape) -> shape;

  /// Result type functions
  make-ok: func(value: u32) -> result<u32, string>;
  make-err: func(message: string) -> result<u32, string>;

  /// Multi-parameter functions
  add-three: func(a: u32, b: u32, c: u32) -> u32;
  concat-strings: func(a: string, b: string) -> string;
  mixed-params: func(name: string, count: u32, flag: bool) -> string;
}

world test-plugin {
  import host-ops;
  import host-rng;
  export handler;
}
```

**Step 2: Commit the WIT changes (no build yet)**

```
feat(component): extend test-plugin WIT with comprehensive type exports
```

---

## Task 4: Implement Go Guest Functions

Adds the Go implementations for all new echo functions.

**Files:**
- Modify: `internal/component/wasip2test/go-repro-plugin/export_test_repro_handler/plugin.go`

**Step 1: Add implementations**

```go
package export_test_repro_handler

import (
	"strconv"
	"wit_component/test_repro_host_ops"
	"wit_component/test_repro_host_rng"
	"wit_component/test_repro_types"
)

// ... existing functions unchanged ...

// EchoBool returns the input bool as-is.
func EchoBool(input bool) bool { return input }

// EchoS8 returns the input s8 as-is.
func EchoS8(input int8) int8 { return input }

// EchoU8 returns the input u8 as-is.
func EchoU8(input uint8) uint8 { return input }

// EchoS16 returns the input s16 as-is.
func EchoS16(input int16) int16 { return input }

// EchoU16 returns the input u16 as-is.
func EchoU16(input uint16) uint16 { return input }

// EchoF32 returns the input f32 as-is.
func EchoF32(input float32) float32 { return input }

// EchoF64 returns the input f64 as-is.
func EchoF64(input float64) float64 { return input }

// EchoChar returns the input char as-is.
func EchoChar(input rune) rune { return input }

// EchoEnum returns the input color enum as-is.
func EchoEnum(input test_repro_types.Color) test_repro_types.Color { return input }

// EchoFlags returns the input permissions flags as-is.
func EchoFlags(input test_repro_types.Permissions) test_repro_types.Permissions { return input }

// EchoVariant returns the input shape variant as-is.
func EchoVariant(input test_repro_types.Shape) test_repro_types.Shape { return input }

// MakeOk returns a successful result with the given value.
func MakeOk(value uint32) wit_types.Result[uint32, string] {
	return wit_types.Ok[uint32, string](value)
}

// MakeErr returns an error result with the given message.
func MakeErr(message string) wit_types.Result[uint32, string] {
	return wit_types.Err[uint32, string](message)
}

// AddThree returns the sum of three u32 values.
func AddThree(a, b, c uint32) uint32 { return a + b + c }

// ConcatStrings concatenates two strings.
func ConcatStrings(a, b string) string { return a + b }

// MixedParams demonstrates mixed parameter types.
func MixedParams(name string, count uint32, flag bool) string {
	if flag {
		return name + ":" + strconv.FormatUint(uint64(count), 10)
	}
	return name
}
```

NOTE: The exact types for Color, Permissions, Shape, and Result will depend on what wit-bindgen generates. The generated type names and import paths must be adjusted after running wit-bindgen. Check:
- `test_repro_types/wit_bindings.go` for type definitions
- The wit_types import path for Result/Option generics
- Generated enum constants (e.g., `ColorRed`, `ColorGreen`, `ColorBlue`)
- Generated flags type (likely a struct or bitfield)
- Generated variant type (likely a struct with tag + value)

**Step 2: Regenerate bindings and build**

```bash
cd internal/component/wasip2test/go-repro-plugin
bash build.sh
```

If the build fails, adjust the Go source to match the generated type names and signatures. The `wit_exports.go` file will be regenerated by wit-bindgen and must not be manually edited.

**Step 3: Verify the component builds**

Expected: `component.wasm` is produced without errors.

**Step 4: Commit**

```
feat(component): implement Go guest functions for comprehensive type tests
```

---

## Task 5: Add Integration Tests

Adds test cases for every new echo function, plus multi-param and mixed-type tests.

**Files:**
- Modify: `internal/component/wasip2test/repro_test.go`

**Step 1: Add test helper for component setup** (reuse existing `newReproInstance`)

**Step 2: Add individual test functions**

Each test follows the same pattern: get handler instance, get function, call with typed Val, verify result.

```go
func TestEchoBool(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst, "handler instance not found")

	f := handlerInst.ExportedFunction("echo-bool")
	require.NotNil(t, f, "echo-bool not found")

	for _, tc := range []bool{true, false} {
		results, err := f.Call(testCtx, component.ValBool(tc))
		require.NoError(t, err, "echo-bool(%v)", tc)
		require.Len(t, results, 1)
		assert.Equal(t, tc, results[0].Bool(), "echo-bool(%v)", tc)
	}
}

func TestEchoS8(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("echo-s8")
	require.NotNil(t, f)

	for _, tc := range []int8{0, 1, -1, 127, -128} {
		results, err := f.Call(testCtx, component.ValS8(tc))
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, tc, results[0].S8())
	}
}

// Similar pattern for echo-u8, echo-s16, echo-u16

func TestEchoF32(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("echo-f32")
	require.NotNil(t, f)

	for _, tc := range []float32{0, 1.5, -1.5, 3.14} {
		results, err := f.Call(testCtx, component.ValF32(tc))
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, tc, results[0].F32())
	}
}

// Similar for echo-f64

func TestEchoChar(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("echo-char")
	require.NotNil(t, f)

	for _, tc := range []rune{'A', 'z', 0, 0x1F600} { // ASCII, zero, emoji
		results, err := f.Call(testCtx, component.ValChar(tc))
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, tc, results[0].Char())
	}
}

func TestEchoEnum(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("echo-enum")
	require.NotNil(t, f)

	for _, tc := range []string{"red", "green", "blue"} {
		results, err := f.Call(testCtx, component.ValEnum(tc))
		require.NoError(t, err, "echo-enum(%s)", tc)
		require.Len(t, results, 1)
		assert.Equal(t, tc, results[0].Enum(), "echo-enum(%s)", tc)
	}
}

func TestEchoFlags(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("echo-flags")
	require.NotNil(t, f)

	testCases := []map[string]bool{
		{"read": true, "write": false, "execute": false},
		{"read": true, "write": true, "execute": true},
		{"read": false, "write": false, "execute": false},
	}
	for _, tc := range testCases {
		results, err := f.Call(testCtx, component.ValFlags(tc))
		require.NoError(t, err)
		require.Len(t, results, 1)
		got := results[0].Flags()
		for k, v := range tc {
			if v {
				assert.True(t, got[k], "flag %s should be set", k)
			}
		}
	}
}

func TestEchoVariant(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("echo-variant")
	require.NotNil(t, f)

	// Test circle case
	radius := component.ValF64(3.14)
	results, err := f.Call(testCtx, component.ValVariant("circle", &radius))
	require.NoError(t, err)
	require.Len(t, results, 1)
	name, payload := results[0].Variant()
	assert.Equal(t, "circle", name)
	require.NotNil(t, payload)
	assert.Equal(t, 3.14, payload.F64())

	// Test none case (no payload)
	results, err = f.Call(testCtx, component.ValVariant("none", nil))
	require.NoError(t, err)
	require.Len(t, results, 1)
	name, payload = results[0].Variant()
	assert.Equal(t, "none", name)
	assert.Nil(t, payload)
}

func TestMakeOk(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("make-ok")
	require.NotNil(t, f)

	results, err := f.Call(testCtx, component.ValU32(42))
	require.NoError(t, err)
	require.Len(t, results, 1)
	isOk, okVal, errVal := results[0].Result()
	assert.True(t, isOk)
	require.NotNil(t, okVal)
	assert.Equal(t, uint32(42), okVal.U32())
	assert.Nil(t, errVal)
}

func TestMakeErr(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("make-err")
	require.NotNil(t, f)

	results, err := f.Call(testCtx, component.ValString("something went wrong"))
	require.NoError(t, err)
	require.Len(t, results, 1)
	isOk, okVal, errVal := results[0].Result()
	assert.False(t, isOk)
	assert.Nil(t, okVal)
	require.NotNil(t, errVal)
	assert.Equal(t, "something went wrong", errVal.StringVal())
}

func TestAddThree(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("add-three")
	require.NotNil(t, f)

	results, err := f.Call(testCtx,
		component.ValU32(10),
		component.ValU32(20),
		component.ValU32(30),
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, uint32(60), results[0].U32())
}

func TestConcatStrings(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("concat-strings")
	require.NotNil(t, f)

	results, err := f.Call(testCtx,
		component.ValString("hello, "),
		component.ValString("world!"),
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "hello, world!", results[0].StringVal())
}

func TestMixedParams(t *testing.T) {
	instance, testCtx, cleanup := newReproInstance(t)
	defer cleanup()
	handlerInst := instance.GetExportedInstance("test:repro/handler")
	require.NotNil(t, handlerInst)

	f := handlerInst.ExportedFunction("mixed-params")
	require.NotNil(t, f)

	// flag=true: returns "alice:42"
	results, err := f.Call(testCtx,
		component.ValString("alice"),
		component.ValU32(42),
		component.ValBool(true),
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "alice:42", results[0].StringVal())

	// flag=false: returns "bob"
	results, err = f.Call(testCtx,
		component.ValString("bob"),
		component.ValU32(99),
		component.ValBool(false),
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "bob", results[0].StringVal())
}
```

**Step 3: Run all tests**

Run: `go test ./internal/component/wasip2test/ -v -count=1`
Expected: ALL tests pass

**Step 4: Commit**

```
test(component): add comprehensive type round-trip integration tests

Tests all primitive types (bool, s8, u8, s16, u16, f32, f64, char),
composite types (enum, flags, variant), result types (ok/err), and
multi-parameter functions (3x u32, 2x string, mixed string+u32+bool).
```

---

## Task 6: Verify and Clean Up

**Step 1: Run full test suite**

Run: `go test ./internal/component/... -count=1`
Expected: ALL tests in the component packages pass

**Step 2: Run the specific repro tests verbosely**

Run: `go test ./internal/component/wasip2test/ -v -run 'TestEcho|TestMake|TestAdd|TestConcat|TestMixed' -count=1`
Expected: All new tests pass with correct values logged

**Step 3: Check for any build warnings or vet issues**

Run: `go vet ./internal/component/...`
Expected: No issues

**Step 4: Final commit if any cleanup needed**

---

## Notes

- The `lowerToMemory` helper in Task 1 doesn't cover all composite types (variant, option, result, tuple, flags, list). If list elements are composite types, this will need extension. For the test cases in this plan, list elements are primitives (u8), so this is sufficient.
- The test for flags may need adjustment depending on how wit-bindgen represents the Permissions type and how the flags values round-trip through the canonical ABI.
- The variant test with f64 payload exercises retptr lifting since variant with f64 has FlattenCount > 1.
- `strconv` must be available in the WASM build environment (it is, as part of Go stdlib).
