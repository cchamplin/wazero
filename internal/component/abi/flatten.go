package abi

import (
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// FlattenParams converts component parameter types to core wasm types.
// This is used to determine the core function signature for lowered component functions.
func FlattenParams(params []types.ValType) []api.ValueType {
	var result []api.ValueType
	for _, p := range params {
		result = append(result, flattenType(p)...)
	}
	return result
}

// FlattenResults converts component result types to core wasm types.
// Returns the flattened types and whether a retptr is needed.
// If needsRetptr is true, the caller must pass a pointer as the first param
// and results will be written there instead of returned directly.
//
// Per the Canonical ABI spec, if the total number of flattened results exceeds
// MaxFlatResults (1 for synchronous calls), results are returned via memory
// using a return pointer parameter.
func FlattenResults(results []types.ValType) ([]api.ValueType, bool) {
	var flat []api.ValueType
	for _, r := range results {
		flat = append(flat, flattenType(r)...)
	}

	if len(flat) > MaxFlatResults {
		return nil, true // Use retptr
	}
	return flat, false
}

// CoreSignature returns the complete core function signature for a lowered function.
// It computes the flattened parameter and result types according to the Canonical ABI.
// If needsRetptr is true, an i32 param is prepended for the return pointer.
func CoreSignature(paramTypes, resultTypes []types.ValType) (params, results []api.ValueType, needsRetptr bool) {
	params = FlattenParams(paramTypes)
	results, needsRetptr = FlattenResults(resultTypes)

	if needsRetptr {
		// Prepend retptr parameter
		params = append([]api.ValueType{api.ValueTypeI32}, params...)
	}

	return params, results, needsRetptr
}

// flattenType converts a single component type to core wasm types.
// This implements the flattening rules from the Canonical ABI specification.
func flattenType(t types.ValType) []api.ValueType {
	switch v := t.(type) {
	case types.Bool:
		return []api.ValueType{api.ValueTypeI32}
	case types.S8, types.U8, types.S16, types.U16, types.S32, types.U32:
		return []api.ValueType{api.ValueTypeI32}
	case types.S64, types.U64:
		return []api.ValueType{api.ValueTypeI64}
	case types.F32:
		return []api.ValueType{api.ValueTypeF32}
	case types.F64:
		return []api.ValueType{api.ValueTypeF64}
	case types.Char:
		return []api.ValueType{api.ValueTypeI32} // Unicode scalar value
	case types.String:
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32} // ptr, len
	case types.List:
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32} // ptr, len
	case types.Own, types.Borrow:
		return []api.ValueType{api.ValueTypeI32} // Handle index
	case types.Record:
		return flattenRecord(v)
	case types.Tuple:
		return flattenTuple(v)
	case types.Variant:
		return flattenVariant(v)
	case types.Option:
		return flattenOption(v)
	case types.Result:
		return flattenResult(v)
	case types.Enum:
		return []api.ValueType{api.ValueTypeI32} // Discriminant
	case types.Flags:
		return flattenFlags(v)
	default:
		// For unknown types, assume they fit in i32 as a fallback
		return []api.ValueType{api.ValueTypeI32}
	}
}

// flattenRecord flattens a record type by flattening each field sequentially.
func flattenRecord(r types.Record) []api.ValueType {
	var result []api.ValueType
	for _, f := range r.Fields {
		result = append(result, flattenType(f.Type)...)
	}
	return result
}

// flattenTuple flattens a tuple type by flattening each element sequentially.
func flattenTuple(t types.Tuple) []api.ValueType {
	var result []api.ValueType
	for _, elemType := range t.Types {
		result = append(result, flattenType(elemType)...)
	}
	return result
}

// flattenVariant flattens a variant type.
// The flattened form is: discriminant (i32) + max payload flattening.
// The payload uses the largest flattening among all cases.
func flattenVariant(v types.Variant) []api.ValueType {
	// Start with discriminant
	result := []api.ValueType{api.ValueTypeI32}

	// Find max payload flattening
	var maxPayload []api.ValueType
	for _, c := range v.Cases {
		if c.Type != nil {
			payload := flattenType(c.Type)
			if len(payload) > len(maxPayload) {
				maxPayload = payload
			}
		}
	}

	// Append flattened payload slots
	// For variants, we need to use the join of all payload types
	// For simplicity, we use the widest type (i64) for each slot
	for i := 0; i < len(maxPayload); i++ {
		// Find the widest type at this position across all cases
		widestType := api.ValueTypeI32
		for _, c := range v.Cases {
			if c.Type != nil {
				caseFlat := flattenType(c.Type)
				if i < len(caseFlat) {
					if isWiderType(caseFlat[i], widestType) {
						widestType = caseFlat[i]
					}
				}
			}
		}
		result = append(result, widestType)
	}

	return result
}

// flattenOption flattens an option type.
// Option is sugar for variant { none, some(T) }.
func flattenOption(o types.Option) []api.ValueType {
	// Discriminant (none=0, some=1)
	result := []api.ValueType{api.ValueTypeI32}

	// Payload for some case
	if o.Some != nil {
		result = append(result, flattenType(o.Some)...)
	}

	return result
}

// flattenResult flattens a result type.
// Result is sugar for variant { ok(T), error(E) }.
func flattenResult(r types.Result) []api.ValueType {
	// Discriminant (ok=0, error=1)
	result := []api.ValueType{api.ValueTypeI32}

	// Find max payload between ok and error
	var okFlat, errFlat []api.ValueType
	if r.Ok != nil {
		okFlat = flattenType(r.Ok)
	}
	if r.Error != nil {
		errFlat = flattenType(r.Error)
	}

	// Take the max payload count
	maxLen := len(okFlat)
	if len(errFlat) > maxLen {
		maxLen = len(errFlat)
	}

	// Join the types at each position
	for i := 0; i < maxLen; i++ {
		var okType, errType api.ValueType
		if i < len(okFlat) {
			okType = okFlat[i]
		}
		if i < len(errFlat) {
			errType = errFlat[i]
		}
		// Use the wider type
		if isWiderType(errType, okType) {
			result = append(result, errType)
		} else {
			result = append(result, okType)
		}
	}

	return result
}

// flattenFlags flattens a flags type.
// Flags are represented as one or more i32 values depending on the number of flags.
func flattenFlags(f types.Flags) []api.ValueType {
	n := len(f.Names)
	if n == 0 {
		return nil
	}
	// Number of i32s needed: ceil(n / 32)
	numI32s := (n + 31) / 32
	result := make([]api.ValueType, numI32s)
	for i := range result {
		result[i] = api.ValueTypeI32
	}
	return result
}

// isWiderType returns true if a is a wider type than b.
// Width order: i32 < f32 < i64 < f64
func isWiderType(a, b api.ValueType) bool {
	return typeWidth(a) > typeWidth(b)
}

// typeWidth returns the width ordering of a type.
func typeWidth(t api.ValueType) int {
	switch t {
	case api.ValueTypeI32:
		return 1
	case api.ValueTypeF32:
		return 2
	case api.ValueTypeI64:
		return 3
	case api.ValueTypeF64:
		return 4
	default:
		return 0
	}
}
