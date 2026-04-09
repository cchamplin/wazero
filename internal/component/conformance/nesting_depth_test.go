// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: nesting-depth tests verify that deeply nested
// structures are handled without stack overflow.
// Restored from Session 0 compile-fix stub. Adapted to the current
// ComponentTypesBuilder API (composite types are interned, not struct
// literals).
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// Spec: definitions.py:103-180 (canonical ABI type algebra).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (LiftContext/LowerContext handle arbitrarily nested
//	composite types via recursive dispatch).
func TestNesting_DeeplyNestedTuples(t *testing.T) {
	const depth = 50

	// Build deeply nested tuple: tuple<tuple<tuple<...<u32>...>>>
	b := types.NewComponentTypesBuilder()
	innerType := types.U32
	for i := 0; i < depth; i++ {
		innerType = b.InternTuple([]types.ValType{innerType})
	}
	ct := b.Finish()

	// Build corresponding value
	innerVal := types.ValU32(12345)
	for i := 0; i < depth; i++ {
		innerVal = types.ValTuple([]types.Val{innerVal})
	}

	t.Run("lower_flat", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, innerType, innerVal)
		require.NoError(t, err)
		require.Equal(t, 1, len(flat))
		require.Equal(t, uint64(12345), flat[0])
	})

	t.Run("lift_flat", func(t *testing.T) {
		ctx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter([]uint64{12345})
		lifted, err := abi.LiftFlat(ctx, innerType, iter)
		require.NoError(t, err)

		// Navigate to innermost value
		current := lifted
		for i := 0; i < depth; i++ {
			tuple := current.Tuple()
			require.Equal(t, 1, len(tuple))
			current = tuple[0]
		}
		require.Equal(t, uint32(12345), current.U32())
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (nested option lowering/lifting).
func TestNesting_DeeplyNestedOptions(t *testing.T) {
	const depth = 30

	b := types.NewComponentTypesBuilder()
	innerType := types.U32
	for i := 0; i < depth; i++ {
		innerType = b.InternOption(innerType)
	}
	ct := b.Finish()

	t.Run("lower_lift_roundtrip", func(t *testing.T) {
		var buildNestedOption func(depth int) types.Val
		buildNestedOption = func(d int) types.Val {
			if d == 0 {
				return types.ValU32(42)
			}
			inner := buildNestedOption(d - 1)
			return types.ValOption(&inner)
		}
		innerVal := buildNestedOption(depth)

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, innerType, innerVal)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, innerType, iter)
		require.NoError(t, err)

		current := lifted
		for i := 0; i < depth; i++ {
			opt := current.Option()
			require.NotNil(t, opt)
			current = *opt
		}
		require.Equal(t, uint32(42), current.U32())
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (nested result lowering/lifting).
func TestNesting_DeeplyNestedResults(t *testing.T) {
	const depth = 20

	b := types.NewComponentTypesBuilder()
	innerType := types.U32
	for i := 0; i < depth; i++ {
		innerType = b.InternResult(innerType, types.String_, true, true)
	}
	ct := b.Finish()

	t.Run("ok_path_roundtrip", func(t *testing.T) {
		var buildNestedResult func(depth int) types.Val
		buildNestedResult = func(d int) types.Val {
			if d == 0 {
				return types.ValU32(42)
			}
			inner := buildNestedResult(d - 1)
			return types.ValResultOk(&inner)
		}
		innerVal := buildNestedResult(depth)

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, innerType, innerVal)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, innerType, iter)
		require.NoError(t, err)

		current := lifted
		for i := 0; i < depth; i++ {
			isOk, okVal, _ := current.Result()
			require.True(t, isOk)
			require.NotNil(t, okVal)
			current = *okVal
		}
		require.Equal(t, uint32(42), current.U32())
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (nested variant lowering/lifting).
func TestNesting_DeeplyNestedVariants(t *testing.T) {
	const depth = 20

	b := types.NewComponentTypesBuilder()
	innerType := types.U32
	for i := 0; i < depth; i++ {
		innerType = b.InternVariant([]types.VariantCase{
			{Name: "inner", Payload: innerType, HasPayload: true},
			{Name: "other", Payload: types.Bool, HasPayload: true},
		})
	}
	ct := b.Finish()

	t.Run("roundtrip", func(t *testing.T) {
		var buildNestedVariant func(depth int) types.Val
		buildNestedVariant = func(d int) types.Val {
			if d == 0 {
				return types.ValU32(999)
			}
			inner := buildNestedVariant(d - 1)
			return types.ValVariant("inner", &inner)
		}
		innerVal := buildNestedVariant(depth)

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, innerType, innerVal)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, innerType, iter)
		require.NoError(t, err)

		current := lifted
		for i := 0; i < depth; i++ {
			caseName, payload := current.Variant()
			require.Equal(t, "inner", caseName)
			require.NotNil(t, payload)
			current = *payload
		}
		require.Equal(t, uint32(999), current.U32())
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra — wide record).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (records with many fields).
func TestNesting_WideRecord(t *testing.T) {
	const numFields = 100

	b := types.NewComponentTypesBuilder()
	fields := make([]types.RecordField, numFields)
	fieldVals := make(map[string]types.Val)
	for i := 0; i < numFields; i++ {
		name := string(rune('a'+i%26)) + string(rune('0'+i/26))
		fields[i] = types.RecordField{Name: name, Type: types.U8}
		fieldVals[name] = types.ValU8(uint8(i))
	}

	recordType := b.InternRecord(fields)
	ct := b.Finish()
	val := types.ValRecord(fieldVals)

	t.Run("roundtrip", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, recordType, val)
		require.NoError(t, err)
		require.Equal(t, numFields, len(flat))

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)

		for i := 0; i < numFields; i++ {
			name := string(rune('a'+i%26)) + string(rune('0'+i/26))
			fieldVal, ok := lifted.RecordField(name)
			require.True(t, ok)
			require.Equal(t, uint8(i), fieldVal.U8())
		}
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra — wide tuple).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (tuples with many elements).
func TestNesting_WideTuple(t *testing.T) {
	const numElements = 100

	b := types.NewComponentTypesBuilder()
	elementTypes := make([]types.ValType, numElements)
	elementVals := make([]types.Val, numElements)
	for i := 0; i < numElements; i++ {
		elementTypes[i] = types.U8
		elementVals[i] = types.ValU8(uint8(i))
	}

	tupleType := b.InternTuple(elementTypes)
	ct := b.Finish()
	val := types.ValTuple(elementVals)

	t.Run("roundtrip", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, tupleType, val)
		require.NoError(t, err)
		require.Equal(t, numElements, len(flat))

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, tupleType, iter)
		require.NoError(t, err)

		elems := lifted.Tuple()
		require.Equal(t, numElements, len(elems))
		for i, elem := range elems {
			require.Equal(t, uint8(i), elem.U8())
		}
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra — large flags).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (flags with many flag names).
func TestNesting_LargeFlags(t *testing.T) {
	const numFlags = 128

	b := types.NewComponentTypesBuilder()
	names := make([]string, numFlags)
	for i := 0; i < numFlags; i++ {
		names[i] = string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('0'+i/10%10))
	}

	flagsType := b.InternFlags(names)
	ct := b.Finish()

	t.Run("all_flags_set", func(t *testing.T) {
		flags := make(map[string]bool)
		for _, name := range names {
			flags[name] = true
		}
		val := types.ValFlags(flags)

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, flagsType, val)
		require.NoError(t, err)
		require.Equal(t, 4, len(flat))

		require.Equal(t, uint64(0xFFFFFFFF), flat[0])
		require.Equal(t, uint64(0xFFFFFFFF), flat[1])
		require.Equal(t, uint64(0xFFFFFFFF), flat[2])
		require.Equal(t, uint64(0xFFFFFFFF), flat[3])
	})

	t.Run("alternating_flags", func(t *testing.T) {
		flags := make(map[string]bool)
		for i, name := range names {
			flags[name] = (i % 2) == 0
		}
		val := types.ValFlags(flags)

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, flagsType, val)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, flagsType, iter)
		require.NoError(t, err)

		liftedFlags := lifted.Flags()
		for i, name := range names {
			expected := (i % 2) == 0
			require.Equal(t, expected, liftedFlags[name], "flag %s at index %d", name, i)
		}
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra — large enum).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (enums with many cases).
func TestNesting_LargeEnum(t *testing.T) {
	const numCases = 500

	b := types.NewComponentTypesBuilder()
	cases := make([]string, numCases)
	for i := 0; i < numCases; i++ {
		cases[i] = string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('0'+i/10%10))
	}

	enumType := b.InternEnum(cases)
	ct := b.Finish()

	t.Run("first_case", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		val := types.ValEnum(cases[0])
		flat, err := abi.LowerFlat(ctx, enumType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])
	})

	t.Run("last_case", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		val := types.ValEnum(cases[numCases-1])
		flat, err := abi.LowerFlat(ctx, enumType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(numCases-1), flat[0])
	})

	t.Run("roundtrip_mid_case", func(t *testing.T) {
		midIdx := numCases / 2
		val := types.ValEnum(cases[midIdx])

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, enumType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(midIdx), flat[0])

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, enumType, iter)
		require.NoError(t, err)
		require.Equal(t, cases[midIdx], lifted.Enum())
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra — large variant).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (variants with many cases).
func TestNesting_LargeVariant(t *testing.T) {
	const numCases = 300

	b := types.NewComponentTypesBuilder()
	cases := make([]types.VariantCase, numCases)
	caseNames := make([]string, numCases)
	for i := 0; i < numCases; i++ {
		name := string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('0'+i/10%10))
		caseNames[i] = name
		if i%2 == 0 {
			cases[i] = types.VariantCase{Name: name, Payload: types.U8, HasPayload: true}
		} else {
			cases[i] = types.VariantCase{Name: name, Payload: types.U32, HasPayload: true}
		}
	}

	variantType := b.InternVariant(cases)
	ct := b.Finish()

	t.Run("first_case", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		payload := types.ValU8(42)
		val := types.ValVariant(caseNames[0], &payload)

		flat, err := abi.LowerFlat(ctx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(0), flat[0])
	})

	t.Run("last_case", func(t *testing.T) {
		ctx := &abi.LowerContext{Types: ct}
		lastIdx := numCases - 1
		var payload types.Val
		if lastIdx%2 == 0 {
			payload = types.ValU8(42)
		} else {
			payload = types.ValU32(12345)
		}
		val := types.ValVariant(caseNames[lastIdx], &payload)

		flat, err := abi.LowerFlat(ctx, variantType, val)
		require.NoError(t, err)
		require.Equal(t, uint64(lastIdx), flat[0])
	})
}

// Spec: definitions.py:103-180 (canonical ABI type algebra — mixed deep nesting).
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/func.rs (combination of deeply nested different types).
func TestNesting_MixedDeep(t *testing.T) {
	// record { opt: option<tuple<variant { a: u32, b: result<u8, bool> }>> }
	b := types.NewComponentTypesBuilder()
	resultType := b.InternResult(types.U8, types.Bool, true, true)
	variantType := b.InternVariant([]types.VariantCase{
		{Name: "a", Payload: types.U32, HasPayload: true},
		{Name: "b", Payload: resultType, HasPayload: true},
	})
	tupleType := b.InternTuple([]types.ValType{variantType})
	optionType := b.InternOption(tupleType)
	recordType := b.InternRecord([]types.RecordField{
		{Name: "opt", Type: optionType},
	})
	ct := b.Finish()

	t.Run("roundtrip_a_case", func(t *testing.T) {
		payload := types.ValU32(12345)
		variantVal := types.ValVariant("a", &payload)
		tupleVal := types.ValTuple([]types.Val{variantVal})
		optionVal := types.ValOption(&tupleVal)
		recordVal := types.ValRecord(map[string]types.Val{
			"opt": optionVal,
		})

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, recordType, recordVal)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)

		optField, _ := lifted.RecordField("opt")
		optPayload := optField.Option()
		require.NotNil(t, optPayload)
		tupleElems := optPayload.Tuple()
		require.Equal(t, 1, len(tupleElems))
		caseName, casePayload := tupleElems[0].Variant()
		require.Equal(t, "a", caseName)
		require.NotNil(t, casePayload)
		require.Equal(t, uint32(12345), casePayload.U32())
	})

	t.Run("roundtrip_b_case_ok", func(t *testing.T) {
		okVal := types.ValU8(42)
		resultVal := types.ValResultOk(&okVal)
		variantVal := types.ValVariant("b", &resultVal)
		tupleVal := types.ValTuple([]types.Val{variantVal})
		optionVal := types.ValOption(&tupleVal)
		recordVal := types.ValRecord(map[string]types.Val{
			"opt": optionVal,
		})

		ctx := &abi.LowerContext{Types: ct}
		flat, err := abi.LowerFlat(ctx, recordType, recordVal)
		require.NoError(t, err)

		liftCtx := &abi.LiftContext{Types: ct}
		iter := abi.NewFlatIter(flat)
		lifted, err := abi.LiftFlat(liftCtx, recordType, iter)
		require.NoError(t, err)

		optField, _ := lifted.RecordField("opt")
		optPayload := optField.Option()
		tupleElems := optPayload.Tuple()
		caseName, casePayload := tupleElems[0].Variant()
		require.Equal(t, "b", caseName)
		isOk, okPayload, _ := casePayload.Result()
		require.True(t, isOk)
		require.NotNil(t, okPayload)
		require.Equal(t, uint8(42), okPayload.U8())
	})
}
