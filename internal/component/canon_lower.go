// internal/component/canon_lower.go

package component

import (
	"context"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
)

// EnumType represents an enumeration type for lowering.
// It maps enum case names to their discriminant indices.
type EnumType struct {
	Cases []string
}

// FlagsType represents a flags type for lowering.
// Each flag is represented as a bit in a bitvector.
type FlagsType struct {
	Flags []string
}

// VariantType represents a variant type for lowering.
// A variant is a discriminated union with multiple cases.
type VariantType struct {
	Cases []VariantCaseForLower
}

// VariantCaseForLower represents a single case in a variant type for lowering.
// This is separate from the VariantCase in component.go to have a simpler
// type representation for the lowering operation.
type VariantCaseForLower struct {
	Name string
	Type PayloadType // nil for cases with no payload
}

// PayloadType represents type information for a variant case payload.
// It provides the FlattenCount method to determine how many flat values
// the payload produces when lowered.
type PayloadType interface {
	// FlattenCount returns the number of flat core wasm values this type produces.
	FlattenCount() int
}

// PrimitiveType represents a primitive type for variant payload.
type PrimitiveType struct {
	Name string // "s32", "u32", "s64", "u64", "f32", "f64", "bool", "string", etc.
}

// FlattenCount returns the number of flat values for a primitive type.
func (p *PrimitiveType) FlattenCount() int {
	switch p.Name {
	case "string":
		return 2 // ptr, len
	case "s64", "u64", "f64":
		return 1
	default:
		return 1 // s32, u32, s8, u8, s16, u16, f32, bool, char
	}
}

// LoweredFunc wraps a component-level HostFunc as a core wasm function.
// This is produced by canon lower, which takes a component function and creates
// a core wasm function that can be provided as an import to core modules.
type LoweredFunc struct {
	// callback is the component-level function to call
	callback HostFunc

	// funcType describes the component function's parameter and result types
	funcType *FuncType

	// options contains canonical ABI options (string encoding, memory, realloc)
	options *CanonicalOptions

	// memory is the resolved memory for canonical ABI operations (if needed)
	memory api.Memory

	// reallocFunc is the resolved realloc function for memory allocation (if needed)
	reallocFunc api.Function

	// instance is the component instance for resource table access
	instance *Instance
}

// CanonLower creates a core wasm function from a component function.
// This implements the canonical lower operation which:
// 1. Takes core wasm arguments from the stack
// 2. Lifts them to component-level Vals
// 3. Calls the component function
// 4. Lowers the results back to core wasm values
//
// Parameters:
//   - callback: The component function to wrap
//   - funcType: The component function's type signature (optional, enables type-aware conversion)
//   - options: Canonical ABI options (string encoding, etc.)
func CanonLower(callback HostFunc, funcType *FuncType, options *CanonicalOptions) *LoweredFunc {
	if options == nil {
		options = &CanonicalOptions{
			StringEncoding: StringEncodingUTF8,
		}
	}
	return &LoweredFunc{
		callback: callback,
		funcType: funcType,
		options:  options,
	}
}

// SetMemory sets the memory for canonical ABI operations.
// Required for lowering strings, lists, and other heap-allocated types.
func (f *LoweredFunc) SetMemory(memory api.Memory) {
	f.memory = memory
}

// SetRealloc sets the realloc function for memory allocation.
// Required for lowering strings, lists, and other heap-allocated types.
func (f *LoweredFunc) SetRealloc(reallocFunc api.Function) {
	f.reallocFunc = reallocFunc
}

// SetInstance sets the component instance for resource table access.
func (f *LoweredFunc) SetInstance(instance *Instance) {
	f.instance = instance
}

// CoreSignature returns the core wasm parameter and result types for this lowered function.
// This is used to register the function with the runtime's host module builder.
func (f *LoweredFunc) CoreSignature() (paramTypes []api.ValueType, resultTypes []api.ValueType) {
	if f.funcType == nil {
		// Without type information, return empty (will be treated as no params, no results)
		return nil, nil
	}

	// Convert component params to core wasm types
	for _, param := range f.funcType.Params {
		coreTypes := componentTypeToCoreTypes(param.ValType)
		paramTypes = append(paramTypes, coreTypes...)
	}

	// Convert component results to core wasm types
	for _, result := range f.funcType.Results {
		coreTypes := componentTypeToCoreTypes(result.ValType)
		resultTypes = append(resultTypes, coreTypes...)
	}

	return paramTypes, resultTypes
}

// componentTypeToCoreTypes converts a component type to core wasm types.
// This implements the flattening of component types to core wasm types
// according to the canonical ABI.
func componentTypeToCoreTypes(typeRef ValTypeRef) []api.ValueType {
	if !typeRef.IsPrimitive {
		// Non-primitive types are represented as i32 pointer for now
		// Full implementation would handle records, variants, etc.
		return []api.ValueType{api.ValueTypeI32}
	}

	switch typeRef.Primitive {
	case 0x7f: // bool
		return []api.ValueType{api.ValueTypeI32}
	case 0x7e: // s8
		return []api.ValueType{api.ValueTypeI32}
	case 0x7d: // u8
		return []api.ValueType{api.ValueTypeI32}
	case 0x7c: // s16
		return []api.ValueType{api.ValueTypeI32}
	case 0x7b: // u16
		return []api.ValueType{api.ValueTypeI32}
	case 0x7a: // s32
		return []api.ValueType{api.ValueTypeI32}
	case 0x79: // u32
		return []api.ValueType{api.ValueTypeI32}
	case 0x78: // s64
		return []api.ValueType{api.ValueTypeI64}
	case 0x77: // u64
		return []api.ValueType{api.ValueTypeI64}
	case 0x76: // f32
		return []api.ValueType{api.ValueTypeF32}
	case 0x75: // f64
		return []api.ValueType{api.ValueTypeF64}
	case 0x74: // char
		return []api.ValueType{api.ValueTypeI32}
	case 0x73: // string
		// String is represented as (ptr: i32, len: i32)
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	default:
		return []api.ValueType{api.ValueTypeI32}
	}
}

// CallWithStack invokes the lowered function with core wasm stack values.
// This is the main entry point when core wasm code calls the lowered function.
//
// The stack contains core wasm values (i32, i64, f32, f64) which are:
// 1. Lifted to component-level Vals based on the function type
// 2. Passed to the component callback
// 3. Results are lowered back to core wasm values
func (f *LoweredFunc) CallWithStack(ctx context.Context, stack []uint64) ([]uint64, error) {
	// Lift arguments from core wasm to component Vals
	args, err := f.liftArguments(stack)
	if err != nil {
		return nil, fmt.Errorf("lift arguments: %w", err)
	}

	// Call the component function
	results, err := f.callback(ctx, args)
	if err != nil {
		return nil, err
	}

	// Lower results from component Vals to core wasm
	coreResults, err := f.lowerResults(results)
	if err != nil {
		return nil, fmt.Errorf("lower results: %w", err)
	}

	return coreResults, nil
}

// liftArguments lifts core wasm values to component Vals.
func (f *LoweredFunc) liftArguments(stack []uint64) ([]Val, error) {
	// If we have type information, use it for proper lifting
	if f.funcType != nil && len(f.funcType.Params) > 0 {
		return f.liftArgumentsTyped(stack)
	}

	// Without type info, assume simple numeric types (i32)
	// This is a fallback for when type information is not available
	args := make([]Val, len(stack))
	for i, v := range stack {
		args[i] = ValS32(int32(v))
	}
	return args, nil
}

// flatIter is a helper to iterate over flat core wasm values.
type flatIter struct {
	values []uint64
	pos    int
}

func newFlatIter(values []uint64) *flatIter {
	return &flatIter{values: values}
}

func (f *flatIter) nextI32() uint32 {
	if f.pos >= len(f.values) {
		return 0
	}
	v := f.values[f.pos]
	f.pos++
	return uint32(v)
}

func (f *flatIter) nextI64() uint64 {
	if f.pos >= len(f.values) {
		return 0
	}
	v := f.values[f.pos]
	f.pos++
	return v
}

func (f *flatIter) nextF32() float32 {
	return math.Float32frombits(f.nextI32())
}

func (f *flatIter) nextF64() float64 {
	return math.Float64frombits(f.nextI64())
}

// liftArgumentsTyped lifts arguments using type information.
func (f *LoweredFunc) liftArgumentsTyped(stack []uint64) ([]Val, error) {
	iter := newFlatIter(stack)
	args := make([]Val, len(f.funcType.Params))

	for i, param := range f.funcType.Params {
		val, err := f.liftValFromFlat(iter, param.ValType)
		if err != nil {
			return nil, fmt.Errorf("lift param %d: %w", i, err)
		}
		args[i] = val
	}

	return args, nil
}

// liftValFromFlat lifts a single value from flat representation using type info.
func (f *LoweredFunc) liftValFromFlat(iter *flatIter, typeRef ValTypeRef) (Val, error) {
	if !typeRef.IsPrimitive {
		// For non-primitive types, we would need the full type definition
		// For now, treat as i32 (this is a limitation)
		return ValS32(int32(iter.nextI32())), nil
	}

	switch typeRef.Primitive {
	case 0x7f: // bool
		return ValBool(iter.nextI32() != 0), nil
	case 0x7e: // s8
		return ValS8(int8(iter.nextI32())), nil
	case 0x7d: // u8
		return ValU8(uint8(iter.nextI32())), nil
	case 0x7c: // s16
		return ValS16(int16(iter.nextI32())), nil
	case 0x7b: // u16
		return ValU16(uint16(iter.nextI32())), nil
	case 0x7a: // s32
		return ValS32(int32(iter.nextI32())), nil
	case 0x79: // u32
		return ValU32(iter.nextI32()), nil
	case 0x78: // s64
		return ValS64(int64(iter.nextI64())), nil
	case 0x77: // u64
		return ValU64(iter.nextI64()), nil
	case 0x76: // f32
		return ValF32(iter.nextF32()), nil
	case 0x75: // f64
		return ValF64(iter.nextF64()), nil
	case 0x74: // char
		return ValChar(rune(iter.nextI32())), nil
	case 0x73: // string
		// String needs memory context - for now return empty
		// Full implementation would read ptr/len and load from memory
		ptr := iter.nextI32()
		length := iter.nextI32()
		if f.memory != nil {
			str, err := f.liftString(ptr, length)
			if err != nil {
				return Val{}, err
			}
			return ValString(str), nil
		}
		return ValString(""), fmt.Errorf("string lifting requires memory (ptr=%d, len=%d)", ptr, length)
	default:
		return ValS32(int32(iter.nextI32())), nil
	}
}

// liftString lifts a string from memory.
func (f *LoweredFunc) liftString(ptr, length uint32) (string, error) {
	if f.memory == nil {
		return "", fmt.Errorf("no memory available for string lifting")
	}
	data, ok := f.memory.Read(ptr, length)
	if !ok {
		return "", fmt.Errorf("memory read out of bounds: ptr=%d, len=%d", ptr, length)
	}
	return string(data), nil
}

// lowerResults lowers component Vals to core wasm values.
func (f *LoweredFunc) lowerResults(results []Val) ([]uint64, error) {
	// If we have type information, use it for proper lowering
	if f.funcType != nil && len(f.funcType.Results) > 0 {
		return f.lowerResultsTyped(results)
	}

	// Without type info, convert based on the Val kind
	var coreResults []uint64
	for _, r := range results {
		flat, err := lowerValToFlat(r)
		if err != nil {
			return nil, err
		}
		coreResults = append(coreResults, flat...)
	}
	return coreResults, nil
}

// lowerResultsTyped lowers results using type information.
func (f *LoweredFunc) lowerResultsTyped(results []Val) ([]uint64, error) {
	if len(results) != len(f.funcType.Results) {
		return nil, fmt.Errorf("expected %d results, got %d", len(f.funcType.Results), len(results))
	}

	var coreResults []uint64
	for i, result := range results {
		flat, err := f.lowerValToFlatTyped(results[i], f.funcType.Results[i].ValType)
		if err != nil {
			return nil, fmt.Errorf("lower result %d: %w", i, err)
		}
		_ = result // use indexed access for consistency
		coreResults = append(coreResults, flat...)
	}

	return coreResults, nil
}

// lowerValToFlatTyped lowers a Val to flat representation using type info.
func (f *LoweredFunc) lowerValToFlatTyped(val Val, typeRef ValTypeRef) ([]uint64, error) {
	if !typeRef.IsPrimitive {
		// For non-primitive types, use untyped lowering as fallback
		return lowerValToFlat(val)
	}

	switch typeRef.Primitive {
	case 0x7f: // bool
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case 0x7e: // s8
		return []uint64{uint64(uint32(int32(val.S8())))}, nil
	case 0x7d: // u8
		return []uint64{uint64(val.U8())}, nil
	case 0x7c: // s16
		return []uint64{uint64(uint32(int32(val.S16())))}, nil
	case 0x7b: // u16
		return []uint64{uint64(val.U16())}, nil
	case 0x7a: // s32
		return []uint64{uint64(uint32(val.S32()))}, nil
	case 0x79: // u32
		return []uint64{uint64(val.U32())}, nil
	case 0x78: // s64
		return []uint64{uint64(val.S64())}, nil
	case 0x77: // u64
		return []uint64{val.U64()}, nil
	case 0x76: // f32
		return []uint64{uint64(math.Float32bits(val.F32()))}, nil
	case 0x75: // f64
		return []uint64{math.Float64bits(val.F64())}, nil
	case 0x74: // char
		return []uint64{uint64(val.Char())}, nil
	case 0x73: // string
		return f.lowerString(val.StringVal())
	default:
		return lowerValToFlat(val)
	}
}

// lowerString allocates memory and writes a string using the realloc protocol.
// This implements string lowering for the canonical ABI.
//
// For UTF-8 encoding:
// 1. Convert string to UTF-8 bytes
// 2. Allocate memory via realloc(0, 0, 1, len) - alignment 1 for UTF-8
// 3. Write bytes to allocated memory
// 4. Return (ptr, len) tuple
func (f *LoweredFunc) lowerString(s string) ([]uint64, error) {
	if f.memory == nil {
		return nil, fmt.Errorf("string lowering requires memory")
	}

	// Convert string to UTF-8 bytes
	data := []byte(s)
	length := uint32(len(data))

	if length == 0 {
		// Empty string: return (0, 0)
		return []uint64{0, 0}, nil
	}

	if f.reallocFunc == nil {
		return nil, fmt.Errorf("string lowering requires realloc function")
	}

	// Allocate memory: realloc(0, 0, 1, len) for UTF-8 (alignment = 1)
	results, err := f.reallocFunc.Call(context.Background(), 0, 0, 1, uint64(length))
	if err != nil {
		return nil, fmt.Errorf("realloc for string failed: %w", err)
	}
	ptr := uint32(results[0])

	// Write UTF-8 bytes to memory
	if !f.memory.Write(ptr, data) {
		return nil, fmt.Errorf("failed to write string to memory at offset %d", ptr)
	}

	return []uint64{uint64(ptr), uint64(length)}, nil
}

// lowerValToFlat lowers a Val to flat core wasm values without type information.
// This is used as a fallback when type information is not available.
func lowerValToFlat(val Val) ([]uint64, error) {
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
	case ValKindEnum:
		// Enum types need type info to map case name to discriminant
		// For now, return error; full implementation needs type context
		return nil, fmt.Errorf("enum lowering requires type context")
	default:
		return nil, fmt.Errorf("unsupported Val kind for untyped lowering: %s", val.Kind())
	}
}

// lowerEnumToFlat converts an enum to its discriminant value.
func lowerEnumToFlat(val Val, enumType *EnumType) ([]uint64, error) {
	caseName := val.Enum()
	for i, name := range enumType.Cases {
		if name == caseName {
			return []uint64{uint64(i)}, nil
		}
	}
	return nil, fmt.Errorf("unknown enum case: %s", caseName)
}

// lowerFlagsToFlat converts flags to a bitvector.
// Per Canonical ABI: flags with N <= 32 use u32, N <= 64 use u64, else multiple u32s.
func lowerFlagsToFlat(val Val, flagsType *FlagsType) ([]uint64, error) {
	flags := val.Flags()
	n := len(flagsType.Flags)

	if n <= 32 {
		var bits uint32
		for i, name := range flagsType.Flags {
			if flags[name] {
				bits |= 1 << i
			}
		}
		return []uint64{uint64(bits)}, nil
	}

	if n <= 64 {
		var bits uint64
		for i, name := range flagsType.Flags {
			if flags[name] {
				bits |= 1 << i
			}
		}
		return []uint64{bits}, nil
	}

	// For > 64 flags, use multiple u32 values
	numU32s := (n + 31) / 32
	result := make([]uint64, numU32s)
	for i, name := range flagsType.Flags {
		if flags[name] {
			wordIdx := i / 32
			bitIdx := i % 32
			result[wordIdx] |= 1 << bitIdx
		}
	}
	return result, nil
}

// lowerVariantToFlat converts a variant to flat representation.
// Returns [discriminant, payload..., padding to max case size].
//
// Variants in the Component Model are discriminated unions where:
// 1. The discriminant indicates which case is active
// 2. The payload is the value for that case (if any)
// 3. All cases produce the same flat representation size (padded to max case size)
func lowerVariantToFlat(val Val, variantType *VariantType) ([]uint64, error) {
	caseName, payload := val.Variant()

	// Find the case index (discriminant)
	var caseIdx int = -1
	var caseType PayloadType
	for i, c := range variantType.Cases {
		if c.Name == caseName {
			caseIdx = i
			caseType = c.Type
			break
		}
	}
	if caseIdx < 0 {
		return nil, fmt.Errorf("unknown variant case: %s", caseName)
	}

	// Calculate max payload flatten count for padding
	maxPayloadFlat := 0
	for _, vc := range variantType.Cases {
		if vc.Type != nil {
			if n := vc.Type.FlattenCount(); n > maxPayloadFlat {
				maxPayloadFlat = n
			}
		}
	}

	// Start with discriminant
	result := []uint64{uint64(caseIdx)}

	// Lower payload if present
	payloadCount := 0
	if caseType != nil && payload != nil {
		payloadFlat, err := lowerValToFlat(*payload)
		if err != nil {
			return nil, fmt.Errorf("lowering variant payload: %w", err)
		}
		result = append(result, payloadFlat...)
		payloadCount = len(payloadFlat)
	}

	// Pad to max case size
	for i := payloadCount; i < maxPayloadFlat; i++ {
		result = append(result, 0)
	}

	return result, nil
}
