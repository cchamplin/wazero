// internal/component/canon_lower.go

package component

import (
	"context"
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/api"
)

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
		// String needs memory allocation - not yet supported in untyped path
		return nil, fmt.Errorf("string lowering requires memory context")
	default:
		return lowerValToFlat(val)
	}
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
	default:
		return nil, fmt.Errorf("unsupported Val kind for untyped lowering: %s", val.Kind())
	}
}
