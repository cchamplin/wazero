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
// If needsRetptr is true, an i32 param is appended for the return pointer.
func CoreSignature(paramTypes, resultTypes []types.ValType) (params, results []api.ValueType, needsRetptr bool) {
	params = FlattenParams(paramTypes)
	results, needsRetptr = FlattenResults(resultTypes)

	if needsRetptr {
		// Append retptr parameter (per Canonical ABI spec, retptr comes after all other params)
		params = append(params, api.ValueTypeI32)
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
		if v.Length != nil {
			// Fixed-length list: flatten each element inline
			var result []api.ValueType
			elemFlat := flattenType(v.Element)
			for i := uint32(0); i < *v.Length; i++ {
				result = append(result, elemFlat...)
			}
			return result
		}
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
// The flattened form is: discriminant (i32) + joined payload types.
// Per spec, payload types are joined across all cases using the join function.
func flattenVariant(v types.Variant) []api.ValueType {
	// Start with discriminant
	result := []api.ValueType{api.ValueTypeI32}

	// Find max payload length and compute joined types
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

	return append(result, flat...)
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

	// Find max payload between ok and error using join
	var okFlat, errFlat []api.ValueType
	if r.Ok != nil {
		okFlat = flattenType(r.Ok)
	}
	if r.Error != nil {
		errFlat = flattenType(r.Error)
	}

	// Join types at each position (similar to flattenVariant)
	var flat []api.ValueType
	for i, ft := range okFlat {
		if i < len(flat) {
			flat[i] = join(flat[i], ft)
		} else {
			flat = append(flat, ft)
		}
	}
	for i, ft := range errFlat {
		if i < len(flat) {
			flat[i] = join(flat[i], ft)
		} else {
			flat = append(flat, ft)
		}
	}

	return append(result, flat...)
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

// join returns the joined type per Canonical ABI spec.
// The join of two types is the type that can represent both:
// - Same types return that type
// - i32 and f32 return i32 (f32 reinterpreted as i32)
// - Any other combination returns i64
func join(a, b api.ValueType) api.ValueType {
	if a == b {
		return a
	}
	if (a == api.ValueTypeI32 && b == api.ValueTypeF32) ||
		(a == api.ValueTypeF32 && b == api.ValueTypeI32) {
		return api.ValueTypeI32
	}
	return api.ValueTypeI64
}
