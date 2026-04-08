package abi

import (
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// FlattenParams converts component parameter types to core wasm types.
// This is used to determine the core function signature for lowered component functions.
func FlattenParams(ct *types.ComponentTypes, params []types.ValType) []api.ValueType {
	var result []api.ValueType
	for _, p := range params {
		result = append(result, flattenType(ct, p)...)
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
func FlattenResults(ct *types.ComponentTypes, results []types.ValType) ([]api.ValueType, bool) {
	var flat []api.ValueType
	for _, r := range results {
		flat = append(flat, flattenType(ct, r)...)
	}

	if len(flat) > MaxFlatResults {
		return nil, true // Use retptr
	}
	return flat, false
}

// CoreSignature returns the complete core function signature for a lowered function.
// It computes the flattened parameter and result types according to the Canonical ABI.
// If needsRetptr is true, an i32 param is appended for the return pointer.
func CoreSignature(ct *types.ComponentTypes, paramTypes, resultTypes []types.ValType) (params, results []api.ValueType, needsRetptr bool) {
	params = FlattenParams(ct, paramTypes)
	results, needsRetptr = FlattenResults(ct, resultTypes)

	if needsRetptr {
		// Append retptr parameter (per Canonical ABI spec, retptr comes after all other params)
		params = append(params, api.ValueTypeI32)
	}

	return params, results, needsRetptr
}

// flattenType converts a single component type to core wasm types by
// dispatching on typ.Kind. Composite types are looked up via ct.
// This implements the flattening rules from the Canonical ABI specification.
func flattenType(ct *types.ComponentTypes, t types.ValType) []api.ValueType {
	switch t.Kind {
	case types.TypeKindBool,
		types.TypeKindS8, types.TypeKindU8,
		types.TypeKindS16, types.TypeKindU16,
		types.TypeKindS32, types.TypeKindU32:
		return []api.ValueType{api.ValueTypeI32}
	case types.TypeKindS64, types.TypeKindU64:
		return []api.ValueType{api.ValueTypeI64}
	case types.TypeKindF32:
		return []api.ValueType{api.ValueTypeF32}
	case types.TypeKindF64:
		return []api.ValueType{api.ValueTypeF64}
	case types.TypeKindChar:
		return []api.ValueType{api.ValueTypeI32}
	case types.TypeKindString:
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	case types.TypeKindList:
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	case types.TypeKindFixedList:
		fl := &ct.FixedLists[t.Index]
		var result []api.ValueType
		elemFlat := flattenType(ct, fl.Element)
		for i := uint32(0); i < fl.Length; i++ {
			result = append(result, elemFlat...)
		}
		return result
	case types.TypeKindRecord:
		rec := &ct.Records[t.Index]
		var result []api.ValueType
		for _, f := range rec.Fields {
			result = append(result, flattenType(ct, f.Type)...)
		}
		return result
	case types.TypeKindTuple:
		tup := &ct.Tuples[t.Index]
		var result []api.ValueType
		for _, elemType := range tup.Types {
			result = append(result, flattenType(ct, elemType)...)
		}
		return result
	case types.TypeKindVariant:
		variant := &ct.Variants[t.Index]
		result := []api.ValueType{api.ValueTypeI32}
		var flat []api.ValueType
		for _, c := range variant.Cases {
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
		return append(result, flat...)
	case types.TypeKindOption:
		opt := &ct.Options[t.Index]
		result := []api.ValueType{api.ValueTypeI32}
		result = append(result, flattenType(ct, opt.Element)...)
		return result
	case types.TypeKindResult:
		res := &ct.Results[t.Index]
		result := []api.ValueType{api.ValueTypeI32}
		var okFlat, errFlat []api.ValueType
		if res.HasOK {
			okFlat = flattenType(ct, res.OK)
		}
		if res.HasErr {
			errFlat = flattenType(ct, res.Err)
		}
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
	case types.TypeKindEnum:
		return []api.ValueType{api.ValueTypeI32}
	case types.TypeKindFlags:
		fl := &ct.Flags[t.Index]
		n := len(fl.Names)
		if n == 0 {
			return nil
		}
		numI32s := (n + 31) / 32
		result := make([]api.ValueType, numI32s)
		for i := range result {
			result[i] = api.ValueTypeI32
		}
		return result
	case types.TypeKindOwn, types.TypeKindBorrow:
		return []api.ValueType{api.ValueTypeI32}
	case types.TypeKindStream, types.TypeKindFuture, types.TypeKindErrorContext:
		return []api.ValueType{api.ValueTypeI32}
	default:
		// Fallback: assume i32 for any unrecognized kind.
		return []api.ValueType{api.ValueTypeI32}
	}
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
