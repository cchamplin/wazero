// internal/component/instance.go

package component

import (
	"context"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// callerInstanceKey is the context key for the caller instance.
type callerInstanceKey struct{}

// GetCallerInstance retrieves the caller instance from context.
// Returns nil if called from host (no caller in context).
func GetCallerInstance(ctx context.Context) *Instance {
	if caller, ok := ctx.Value(callerInstanceKey{}).(*Instance); ok {
		return caller
	}
	return nil
}

// WithCallerInstance returns a context with the caller instance set.
// Used when a component calls another component.
func WithCallerInstance(ctx context.Context, caller *Instance) context.Context {
	return context.WithValue(ctx, callerInstanceKey{}, caller)
}

// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc

	// componentFuncs maps component function indices to their implementations.
	// These come from:
	// - Aliased imports (component-level WASI functions)
	// - Canon lift operations (lifted core functions)
	componentFuncs map[uint32]ComponentFunc

	// Resource management fields
	resourceTable *runtime.ResourceTable // Table for tracking resource handles
	destructors   map[uint32]func(any)   // Destructor functions by resource type index
	callContext   *runtime.CallContext   // Current call context for borrow tracking

	// mayLeaveDisabled tracks whether the component cannot call out.
	// Set to true during lowering and post-return to prevent reentrance.
	// Per Canonical ABI spec, may_leave defaults to true (so this defaults to false).
	mayLeaveDisabled bool

	// activeCallDepth tracks the number of active calls into this instance.
	// Used by call_might_be_recursive to detect reentrance.
	activeCallDepth int32

	// Value index space for start function support
	values         []types.Val
	valuesConsumed []bool

	// Nested component support
	parent   *Instance
	children []*Instance

	// Index spaces for nested component support
	instanceSpace  []*Instance
	typeSpace      []*TypeDef
	componentSpace []*Component

	// Exported instances for API access
	exportedInstances map[string]*Instance
}

// ComponentFunc represents a callable component-level function.
type ComponentFunc struct {
	// Type is the component function type (params and results).
	Type *FuncType

	// Implementation is the actual callable.
	// For imports: the host-provided Definition
	// For canon lift: the lifted core function
	Impl func(ctx context.Context, args []types.Val) ([]types.Val, error)
}

// GetComponentFunc looks up a component function by its index.
// Returns the ComponentFunc and true if found, or an empty struct and false if not.
func (i *Instance) GetComponentFunc(funcIdx uint32) (ComponentFunc, bool) {
	if i.componentFuncs == nil {
		return ComponentFunc{}, false
	}
	f, ok := i.componentFuncs[funcIdx]
	return f, ok
}

// Component returns the component this instance was created from.
func (i *Instance) Component() *Component {
	return i.component
}

// ExportedFunction returns the exported function with the given name,
// or nil if not found.
func (i *Instance) ExportedFunction(name string) *ExportedFunc {
	if i.exports == nil {
		return nil
	}
	return i.exports[name]
}

// ExportedFunc represents an exported component function.
type ExportedFunc struct {
	name           string
	funcType       *FuncType
	coreFunc       api.Function
	canonical      *CanonicalDef
	component      *Component   // reference to parent component for type lookups
	instance       *Instance    // reference to parent instance for memory access
	memory         api.Memory   // resolved memory for canonical ABI operations
	reallocFunc    api.Function // resolved realloc function for memory allocation
	postReturnFunc api.Function // optional post-return function for cleanup after call
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
// Supports primitive types, records (flattened to their fields), and resource handles.
// For resource handles (own/borrow), it uses the Canonical ABI lift/lower operations.
func (f *ExportedFunc) Call(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	// Set up call context and subtask for resource tracking
	callCtx := runtime.NewCallContext()
	var subtask *runtime.Subtask
	var borrowScope *runtime.BorrowScope

	// Initialize resource table if needed
	if f.instance != nil {
		if f.instance.resourceTable == nil {
			f.instance.resourceTable = runtime.NewResourceTable()
		}
		// Create subtask which owns the borrow scope
		subtask = runtime.NewSubtask(f.instance.resourceTable)
		borrowScope = subtask.BorrowScope()

		// Defer subtask cleanup for error paths
		// This ensures the borrow scope is released even if we return early
		defer func() {
			if subtask != nil && subtask.State() == runtime.SubtaskStatePending {
				// If we're returning early (error), resolve with nil result
				subtask.DeliverResolve(nil)
				subtask.StartFinish()
				subtask.Finish() // Ignore errors in defer cleanup
			}
		}()

		// Set the call context for this invocation
		prevCallCtx := f.instance.callContext
		f.instance.callContext = callCtx
		defer func() {
			f.instance.callContext = prevCallCtx
		}()

		// === REENTRANCE CHECK ===
		// Get caller from context if available (nil for host calls)
		caller := GetCallerInstance(ctx)
		if err := f.instance.ValidateNotRecursive(caller); err != nil {
			return nil, err
		}

		// Track this call
		f.instance.EnterCall()
		defer f.instance.ExitCall()
	}

	// Create TypeResolver for dynamic type resolution.
	// Use the instance-aware resolver when available so type aliases
	// (resolved during buildTypeSpace) can be found via the instance's typeSpace.
	var resolver *TypeResolver
	if f.component != nil {
		if f.instance != nil {
			resolver = NewTypeResolverWithInstance(f.component, f.instance)
		} else {
			resolver = NewTypeResolver(f.component)
		}
	}

	// Convert component Vals to core wasm values
	// Records are flattened into their constituent fields
	var coreParams []uint64

	// Resolve all param types upfront (needed for MAX_FLAT_PARAMS check and lowering)
	resolvedTypes := make([]types.ValType, len(params))
	for i := range params {
		if resolver != nil && f.funcType != nil && i < len(f.funcType.Params) {
			if rt, err := resolver.ResolveValType(f.funcType.Params[i].ValType); err == nil {
				resolvedTypes[i] = rt
			}
		}
	}

	// === BEGIN LOWERING PARAMS - may_leave = false ===
	// Per CanonicalABI.md lines 3133, 3151: may_leave must be false during lower_flat_values
	if f.instance != nil {
		f.instance.SetMayLeave(false)
	}
	// Use a flag to track if we've restored mayLeave, so we can do it in defer for errors
	// and explicitly after the loop for the success path
	loweringComplete := false
	defer func() {
		if !loweringComplete && f.instance != nil {
			f.instance.SetMayLeave(true)
		}
	}()

	// Per Canonical ABI: when the total flat count of all params exceeds MAX_FLAT_PARAMS (16),
	// all params are stored contiguously in linear memory and a single i32 pointer is passed.
	const maxFlatParams = 16
	totalFlatCount := 0
	allTypesResolved := true
	for i, rt := range resolvedTypes {
		if rt != nil {
			totalFlatCount += rt.FlattenCount()
		} else {
			allTypesResolved = false
			// Estimate flat count from Val kind for unresolved types
			totalFlatCount += estimateFlatCount(params[i])
		}
	}

	if totalFlatCount > maxFlatParams && allTypesResolved && f.reallocFunc != nil && f.memory != nil {
		// Store all params to memory
		// First compute total size and alignment
		var totalSize uint32
		var maxAlign uint32 = 1
		paramSizes := make([]uint32, len(params))
		paramAligns := make([]uint32, len(params))
		for i, rt := range resolvedTypes {
			paramSizes[i] = rt.Size()
			paramAligns[i] = rt.Align()
			if paramAligns[i] > maxAlign {
				maxAlign = paramAligns[i]
			}
			totalSize = alignTo(totalSize, paramAligns[i])
			totalSize += paramSizes[i]
		}

		// Allocate memory
		results, err := f.reallocFunc.Call(ctx, 0, 0, uint64(maxAlign), uint64(totalSize))
		if err != nil {
			return nil, fmt.Errorf("realloc for flat params failed: %w", err)
		}
		basePtr := uint32(results[0])

		// Write each param to memory at proper offset
		offset := uint32(0)
		for i, p := range params {
			offset = alignTo(offset, paramAligns[i])
			if err := f.lowerToMemory(ctx, p, resolvedTypes[i], basePtr+offset); err != nil {
				return nil, fmt.Errorf("lower param %d to memory: %w", i, err)
			}
			offset += paramSizes[i]
		}

		coreParams = []uint64{uint64(basePtr)}
	} else {
		// Normal flat lowering
		for i, p := range params {
			flat, err := f.lowerParam(ctx, p, resolvedTypes[i], callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower param %d: %w", i, err)
			}
			coreParams = append(coreParams, flat...)
		}
	}

	// === END LOWERING PARAMS - may_leave = true ===
	if f.instance != nil {
		f.instance.SetMayLeave(true)
	}
	loweringComplete = true

	// === RETPTR ALLOCATION ===
	// Per Canonical ABI: when a function's result type has FlattenCount > MAX_FLAT_RESULTS (1),
	// the caller allocates space via realloc and passes a retptr as the last core param.
	// The callee writes the result into memory at that pointer, and returns void.
	// After the call, we synthesize coreResults = [retptr] so the lifting code can read from memory.
	//
	// However, some toolchains (e.g., Go/TinyGo) produce core functions that instead return the
	// retptr as an i32 result (without accepting it as a param). We detect this by comparing the
	// core function's expected param count: if it equals len(coreParams)+1, we pass a retptr;
	// otherwise we assume the core function returns the retptr as its result.
	var retptrVal uint64
	usedRetptr := false
	if f.funcType != nil && len(f.funcType.Results) == 1 && f.reallocFunc != nil && f.memory != nil {
		resultTypeRef := f.funcType.Results[0].ValType

		if !resultTypeRef.IsPrimitive && resolver != nil {
			resolvedType, resolveErr := resolver.ResolveValType(resultTypeRef)
			if resolveErr == nil {
				flatCount := resolvedType.FlattenCount()
				if flatCount > 1 {
					// Check if the core function expects a retptr param
					// by comparing expected params vs what we've lowered so far.
					// Standard ABI: retptr is the last param, core function returns void.
					// Some toolchains (Go): core function returns retptr as i32, no extra param.
					expectedParams := len(f.coreFunc.Definition().ParamTypes())
					needsRetptrParam := expectedParams > len(coreParams)

					if needsRetptrParam {
						retptrSize := resolvedType.Size()
						retptrAlign := resolvedType.Align()
						retptrResults, retptrErr := f.reallocFunc.Call(ctx, 0, 0, uint64(retptrAlign), uint64(retptrSize))
						if retptrErr != nil {
							return nil, fmt.Errorf("realloc for retptr failed: %w", retptrErr)
						}
						retptrVal = retptrResults[0]
						coreParams = append(coreParams, retptrVal)
						usedRetptr = true
					}
				}
			}
		}
	}

	// Call the core function
	coreResults, err := f.coreFunc.Call(ctx, coreParams...)
	if err != nil {
		return nil, err
	}

	// If retptr was used (standard ABI), the core function returns void.
	// Synthesize coreResults = [retptr] so the lifting code reads from memory.
	if usedRetptr && len(coreResults) == 0 {
		coreResults = []uint64{retptrVal}
	}

	// Call the post-return function if specified.
	// Per Canonical ABI spec, the post-return function is called after the main
	// function returns but before control returns to the caller.
	// IMPORTANT: may_leave must be false during post-return to prevent callbacks.
	// Per CanonicalABI.md lines 3287-3289
	if f.postReturnFunc != nil {
		if f.instance != nil {
			f.instance.SetMayLeave(false)
		}
		_, postReturnErr := f.postReturnFunc.Call(ctx, coreResults...)
		if f.instance != nil {
			f.instance.SetMayLeave(true)
		}
		if postReturnErr != nil {
			return nil, fmt.Errorf("post-return function failed: %w", postReturnErr)
		}
	}

	// Convert core results back to component Vals using TypeResolver
	// Check if the result type is a record, option, or handle by examining the function type
	if f.funcType != nil && len(f.funcType.Results) == 1 {
		resultTypeRef := f.funcType.Results[0].ValType
		// Check for own<T> result
		if resultTypeRef.IsOwn && len(coreResults) == 1 {
			// own<T> result: Extract rep and transfer ownership out of component
			handleIdx := uint32(coreResults[0])
			rep, err := f.liftOwn(handleIdx, borrowScope)
			if err != nil {
				return nil, fmt.Errorf("lift own result: %w", err)
			}
			// Validate no outstanding borrows
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			// Return the representation as the handle value
			// The rep is any, so we extract uint32 if it's a handle index
			if handleVal, ok := rep.(uint32); ok {
				result := []types.Val{types.ValOwn(handleVal)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			}
			// If rep is not uint32, return it wrapped in ValOwn with index 0
			// This is a simplification; proper implementation would track the actual handle
			result := []types.Val{types.ValOwn(0)}
			// Complete subtask before return
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}

		// Check for borrow<T> result (rare, but possible)
		if resultTypeRef.IsBorrow && len(coreResults) == 1 {
			// borrow<T> result: Read the rep without removing from table
			handleIdx := uint32(coreResults[0])
			rep, err := f.liftBorrow(handleIdx, borrowScope)
			if err != nil {
				return nil, fmt.Errorf("lift borrow result: %w", err)
			}
			// Validate no outstanding borrows
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			if handleVal, ok := rep.(uint32); ok {
				result := []types.Val{types.ValBorrow(handleVal)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			}
			result := []types.Val{types.ValBorrow(0)}
			// Complete subtask before return
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}

		// Use TypeResolver for defined types when available
		if resolver != nil && !resultTypeRef.IsPrimitive {
			resolvedType, resolveErr := resolver.ResolveValType(resultTypeRef)
			if resolveErr == nil {
				result, liftErr := f.liftResolvedType(resolvedType, coreResults, subtask, callCtx)
				if liftErr == nil {
					return result, nil
				}
				// If lifting fails, fall through to legacy handling
			}
		}

		// Legacy handling for when TypeResolver is not available
		if !resultTypeRef.IsPrimitive && f.component != nil {
			// Result is a defined type - look up the actual type definition.
			// Use TypeIdxToStoredIdx to map from type index space to Types array.
			legacyStoredIdx := resultTypeRef.TypeIdx
			if mapped, ok := f.component.TypeIdxToStoredIdx[resultTypeRef.TypeIdx]; ok {
				legacyStoredIdx = mapped
			}
			var typeDef *TypeDef
			if legacyStoredIdx < uint32(len(f.component.Types)) {
				typeDef = &f.component.Types[legacyStoredIdx]
			}
			if typeDef == nil {
				// Type not found in Types array - skip legacy handling
			} else if typeDef.Option != nil && len(coreResults) == 2 {
				// Option type: first result is discriminant, second is payload
				discriminant := coreResults[0]
				if discriminant == 0 {
					// None
					if err := callCtx.ValidateReturn(); err != nil {
						return nil, err
					}
					result := []types.Val{types.ValOption(nil)}
					// Complete subtask before return
					if subtask != nil {
						subtask.DeliverResolve(result)
						subtask.StartFinish()
						if err := subtask.Finish(); err != nil {
							return nil, fmt.Errorf("subtask finish: %w", err)
						}
					}
					return result, nil
				}
				// Some: Use TypeResolver to determine payload type if available
				payload := f.liftPrimitiveVal(coreResults[1], typeDef.Option.InnerType)
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				result := []types.Val{types.ValOption(&payload)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			} else if typeDef.Record != nil {
				// Record type: Use TypeResolver to get field names and types dynamically
				rec := f.liftRecord(typeDef.Record, coreResults)
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				result := []types.Val{types.ValRecord(rec)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			} else if typeDef.Result != nil && len(coreResults) == 2 {
				// Result type: first result is discriminant, second is payload
				// discriminant 0 = Ok, 1 = Error
				discriminant := coreResults[0]
				if discriminant == 0 {
					// Ok: return success result with value
					// Use TypeResolver to determine payload type if available
					if typeDef.Result.OkType != nil {
						payload := f.liftPrimitiveVal(coreResults[1], *typeDef.Result.OkType)
						if err := callCtx.ValidateReturn(); err != nil {
							return nil, err
						}
						result := []types.Val{types.ValResultOk(&payload)}
						// Complete subtask before return
						if subtask != nil {
							subtask.DeliverResolve(result)
							subtask.StartFinish()
							if err := subtask.Finish(); err != nil {
								return nil, fmt.Errorf("subtask finish: %w", err)
							}
						}
						return result, nil
					}
					// No ok type, return unit result
					if err := callCtx.ValidateReturn(); err != nil {
						return nil, err
					}
					result := []types.Val{types.ValResultOk(nil)}
					// Complete subtask before return
					if subtask != nil {
						subtask.DeliverResolve(result)
						subtask.StartFinish()
						if err := subtask.Finish(); err != nil {
							return nil, fmt.Errorf("subtask finish: %w", err)
						}
					}
					return result, nil
				}
				// Error: return error result with error value
				// Use TypeResolver to determine error type if available
				if typeDef.Result.ErrType != nil {
					errVal := f.liftPrimitiveVal(coreResults[1], *typeDef.Result.ErrType)
					if err := callCtx.ValidateReturn(); err != nil {
						return nil, err
					}
					result := []types.Val{types.ValResultError(&errVal)}
					// Complete subtask before return
					if subtask != nil {
						subtask.DeliverResolve(result)
						subtask.StartFinish()
						if err := subtask.Finish(); err != nil {
							return nil, fmt.Errorf("subtask finish: %w", err)
						}
					}
					return result, nil
				}
				// No error type, return unit error
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				result := []types.Val{types.ValResultError(nil)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			}
		}

		// Handle primitive result types using TypeResolver
		if resultTypeRef.IsPrimitive {
			// Special handling for string return type (primitive 0x73)
			// In Canonical ABI, strings are returned via retptr since FlattenCount=2 > MAX_FLAT_RESULTS=1
			if resultTypeRef.Primitive == 0x73 {
				if len(coreResults) != 1 {
					return nil, fmt.Errorf("string result expected 1 core result (retptr), got %d", len(coreResults))
				}
				if f.memory == nil {
					return nil, fmt.Errorf("string result requires memory but none available")
				}
				retptr := uint32(coreResults[0])
				str, err := f.liftStringFromRetptr(retptr)
				if err != nil {
					return nil, fmt.Errorf("lift string result: %w", err)
				}
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				result := []types.Val{types.ValString(str)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			}
			if len(coreResults) == 1 {
				val := f.liftPrimitiveVal(coreResults[0], resultTypeRef)
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				result := []types.Val{val}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			}
		}
	}

	// Validate that all borrowed handles have been dropped
	if err := callCtx.ValidateReturn(); err != nil {
		return nil, err
	}

	// Handle multiple results or fallback: use TypeResolver when available
	if f.funcType != nil && len(f.funcType.Results) == len(coreResults) {
		results := make([]types.Val, len(coreResults))
		for i, r := range coreResults {
			results[i] = f.liftPrimitiveVal(r, f.funcType.Results[i].ValType)
		}
		// Complete subtask before return
		if subtask != nil {
			subtask.DeliverResolve(results)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return results, nil
	}

	// Final fallback: treat results as s32 (for backwards compatibility)
	results := make([]types.Val, len(coreResults))
	for i, r := range coreResults {
		results[i] = types.ValS32(int32(r))
	}

	// Complete subtask before return
	if subtask != nil {
		subtask.DeliverResolve(results)
		subtask.StartFinish()
		if err := subtask.Finish(); err != nil {
			return nil, fmt.Errorf("subtask finish: %w", err)
		}
	}

	return results, nil
}

// liftPrimitiveVal converts a core wasm value to a component Val based on the ValTypeRef.
// Uses TypeResolver logic to determine the correct primitive type.
func (f *ExportedFunc) liftPrimitiveVal(coreVal uint64, typeRef ValTypeRef) types.Val {
	if typeRef.IsPrimitive {
		switch typeRef.Primitive {
		case 0x7f: // bool
			if coreVal != 0 {
				return types.ValBool(true)
			}
			return types.ValBool(false)
		case 0x7e: // s8
			return types.ValS8(int8(coreVal))
		case 0x7d: // u8
			return types.ValU8(uint8(coreVal))
		case 0x7c: // s16
			return types.ValS16(int16(coreVal))
		case 0x7b: // u16
			return types.ValU16(uint16(coreVal))
		case 0x7a: // s32
			return types.ValS32(int32(coreVal))
		case 0x79: // u32
			return types.ValU32(uint32(coreVal))
		case 0x78: // s64
			return types.ValS64(int64(coreVal))
		case 0x77: // u64
			return types.ValU64(coreVal)
		case 0x76: // f32
			return types.ValF32(math.Float32frombits(uint32(coreVal)))
		case 0x75: // f64
			return types.ValF64(math.Float64frombits(coreVal))
		case 0x74: // char
			return types.ValChar(rune(coreVal))
		}
	}
	// Default fallback to s32
	return types.ValS32(int32(coreVal))
}

// liftStringFromRetptr reads a string from a return pointer.
// In Canonical ABI, when a function returns a string (which has FlattenCount=2),
// the core function returns a pointer to a {ptr: i32, len: i32} struct in memory.
// This method reads that struct and then reads the actual string bytes from memory.
func (f *ExportedFunc) liftStringFromRetptr(retptr uint32) (string, error) {
	if f.memory == nil {
		return "", fmt.Errorf("no memory available for string lifting")
	}

	// Read the (ptr, len) struct from memory at retptr
	// The struct is {ptr: i32, len: i32} = 8 bytes, aligned to 4 bytes
	ptr, ok := f.memory.ReadUint32Le(retptr)
	if !ok {
		return "", fmt.Errorf("failed to read string ptr from memory at offset %d", retptr)
	}
	length, ok := f.memory.ReadUint32Le(retptr + 4)
	if !ok {
		return "", fmt.Errorf("failed to read string len from memory at offset %d", retptr+4)
	}

	// Empty string case
	if length == 0 {
		return "", nil
	}

	// Read the actual string bytes from memory at ptr
	data, ok := f.memory.Read(ptr, length)
	if !ok {
		return "", fmt.Errorf("failed to read string data from memory at ptr=%d len=%d", ptr, length)
	}

	// The canonical lift for UTF-8 strings validates UTF-8
	// Per Canonical ABI spec, lift_flat for strings must validate UTF-8
	// TODO: Support other string encodings based on canonical options
	if !utf8.Valid(data) {
		return "", fmt.Errorf("invalid UTF-8 in string at offset %d", ptr)
	}
	return string(data), nil
}

// liftRecord reconstructs a record Val from flattened core values.
// Uses field names from the record type definition instead of hardcoded names.
func (f *ExportedFunc) liftRecord(recordDef *RecordTypeDef, coreResults []uint64) map[string]types.Val {
	rec := make(map[string]types.Val)

	// Get sorted field names (component model spec requires alphabetical order)
	fieldNames := make([]string, len(recordDef.Fields))
	for i, field := range recordDef.Fields {
		fieldNames[i] = field.Name
	}
	sort.Strings(fieldNames)

	// Build a map from field name to field definition for easy lookup
	fieldMap := make(map[string]*RecordField)
	for i := range recordDef.Fields {
		fieldMap[recordDef.Fields[i].Name] = &recordDef.Fields[i]
	}

	// Lift each field value using its type
	coreIdx := 0
	for _, name := range fieldNames {
		if coreIdx >= len(coreResults) {
			break
		}
		field := fieldMap[name]
		if field != nil {
			rec[name] = f.liftPrimitiveVal(coreResults[coreIdx], field.ValType)
		} else {
			// Fallback for missing field definition
			rec[name] = types.ValS32(int32(coreResults[coreIdx]))
		}
		coreIdx++
	}

	return rec
}

// liftResolvedType lifts core values to a component Val using a resolved type.
// This is the type-resolver-driven path for lifting complex types.
func (f *ExportedFunc) liftResolvedType(resolvedType types.ValType, coreResults []uint64, subtask *runtime.Subtask, callCtx *runtime.CallContext) ([]types.Val, error) {
	switch t := resolvedType.(type) {
	case types.Record:
		rec := make(map[string]types.Val)
		if len(coreResults) < len(t.Fields) && len(coreResults) == 1 && f.memory != nil {
			// Retptr case: the record's flat count exceeds MAX_FLAT_RESULTS=1,
			// so the core function returns a pointer to the record in linear memory.
			retptr := uint32(coreResults[0])
			fieldOffsets := t.FieldOffsets()
			for i, field := range t.Fields {
				val, _ := f.liftFieldFromMemory(retptr+fieldOffsets[i], field.Type)
				rec[field.Name] = val
			}
		} else if len(coreResults) >= len(t.Fields) {
			// Flat case: record fields are returned as individual core results.
			for i, field := range t.Fields {
				rec[field.Name] = f.liftResolvedPrimitiveVal(coreResults[i], field.Type)
			}
		} else {
			return nil, fmt.Errorf("not enough core results for record: have %d, need %d", len(coreResults), len(t.Fields))
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValRecord(rec)}
		// Complete subtask before return
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

	case types.Option:
		if len(coreResults) < 2 {
			return nil, fmt.Errorf("not enough core results for option: have %d, need 2", len(coreResults))
		}
		discriminant := coreResults[0]
		if discriminant == 0 {
			// None
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []types.Val{types.ValOption(nil)}
			// Complete subtask before return
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		// Some
		payload := f.liftResolvedPrimitiveVal(coreResults[1], t.Some)
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValOption(&payload)}
		// Complete subtask before return
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

	case types.Result:
		// Result type: discriminant (i32) + payload
		// When FlattenCount > MAX_FLAT_RESULTS (1), the result is stored in linear memory
		// at a retptr. When flattened, discriminant is coreResults[0] and payload follows.
		if len(coreResults) == 1 && t.FlattenCount() > 1 && f.memory != nil {
			// Retptr case: read from memory
			retptr := uint32(coreResults[0])
			return f.liftResultFromMemory(t, retptr, subtask, callCtx)
		}
		if len(coreResults) < 2 {
			return nil, fmt.Errorf("not enough core results for result: have %d, need 2", len(coreResults))
		}
		discriminant := coreResults[0]
		if discriminant == 0 {
			// Ok
			if t.Ok != nil {
				payload := f.liftResolvedPrimitiveVal(coreResults[1], t.Ok)
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				result := []types.Val{types.ValResultOk(&payload)}
				// Complete subtask before return
				if subtask != nil {
					subtask.DeliverResolve(result)
					subtask.StartFinish()
					if err := subtask.Finish(); err != nil {
						return nil, fmt.Errorf("subtask finish: %w", err)
					}
				}
				return result, nil
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []types.Val{types.ValResultOk(nil)}
			// Complete subtask before return
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		// Error
		if t.Error != nil {
			errVal := f.liftResolvedPrimitiveVal(coreResults[1], t.Error)
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []types.Val{types.ValResultError(&errVal)}
			// Complete subtask before return
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValResultError(nil)}
		// Complete subtask before return
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

	case types.String:
		// String results are returned via retptr since FlattenCount=2 > MAX_FLAT_RESULTS=1
		// The core result is the retptr, and we read (ptr, len) from memory at that location
		if len(coreResults) < 1 {
			return nil, fmt.Errorf("not enough core results for string: have %d, need 1", len(coreResults))
		}
		if f.memory == nil {
			return nil, fmt.Errorf("string result requires memory but none available")
		}
		retptr := uint32(coreResults[0])
		str, err := f.liftStringFromRetptr(retptr)
		if err != nil {
			return nil, fmt.Errorf("lift string result: %w", err)
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValString(str)}
		// Complete subtask before return
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

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
		result := []types.Val{types.ValEnum(t.Cases[discriminant])}
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
		result := []types.Val{types.ValFlags(flagMap)}
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
			elems := make([]types.Val, len(t.Types))
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
			result := []types.Val{types.ValTuple(elems)}
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
			elems := make([]types.Val, len(t.Types))
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
			result := []types.Val{types.ValTuple(elems)}
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
			var payload *types.Val
			if c.Type != nil && len(coreResults) > 1 {
				v := f.liftResolvedPrimitiveVal(coreResults[1], c.Type)
				payload = &v
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []types.Val{types.ValVariant(c.Name, payload)}
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
			var payload *types.Val
			if c.Type != nil {
				payloadOffset := t.PayloadOffset()
				v, _ := f.liftFieldFromMemory(retptr+payloadOffset, c.Type)
				payload = &v
			}
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []types.Val{types.ValVariant(c.Name, payload)}
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
		elems := make([]types.Val, length)
		elemSize := t.Element.Size()
		for i := uint32(0); i < length; i++ {
			elemOffset := ptr + i*elemSize
			val, _ := f.liftFieldFromMemory(elemOffset, t.Element)
			elems[i] = val
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValList(elems)}
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil

	default:
		// For other types, fall back to legacy handling
		return nil, fmt.Errorf("unsupported resolved type for lifting: %T", resolvedType)
	}
}

// liftResolvedPrimitiveVal converts a core value to a Val using a resolved types.ValType.
func (f *ExportedFunc) liftResolvedPrimitiveVal(coreVal uint64, valType types.ValType) types.Val {
	switch valType.(type) {
	case types.Bool:
		if coreVal != 0 {
			return types.ValBool(true)
		}
		return types.ValBool(false)
	case types.S8:
		return types.ValS8(int8(coreVal))
	case types.U8:
		return types.ValU8(uint8(coreVal))
	case types.S16:
		return types.ValS16(int16(coreVal))
	case types.U16:
		return types.ValU16(uint16(coreVal))
	case types.S32:
		return types.ValS32(int32(coreVal))
	case types.U32:
		return types.ValU32(uint32(coreVal))
	case types.S64:
		return types.ValS64(int64(coreVal))
	case types.U64:
		return types.ValU64(coreVal)
	case types.F32:
		return types.ValF32(math.Float32frombits(uint32(coreVal)))
	case types.F64:
		return types.ValF64(math.Float64frombits(coreVal))
	case types.Char:
		return types.ValChar(rune(coreVal))
	default:
		// Default fallback to s32
		return types.ValS32(int32(coreVal))
	}
}

// liftFieldFromMemory reads a typed value from linear memory at the given offset.
// Returns the lifted Val and the number of bytes consumed (for advancing the offset).
func (f *ExportedFunc) liftFieldFromMemory(offset uint32, valType types.ValType) (types.Val, uint32) {
	switch t := valType.(type) {
	case types.Bool:
		_ = t
		b, ok := f.memory.ReadByteAt(offset)
		if !ok {
			return types.ValBool(false), 1
		}
		return types.ValBool(b != 0), 1
	case types.S8:
		b, ok := f.memory.ReadByteAt(offset)
		if !ok {
			return types.ValS8(0), 1
		}
		return types.ValS8(int8(b)), 1
	case types.U8:
		b, ok := f.memory.ReadByteAt(offset)
		if !ok {
			return types.ValU8(0), 1
		}
		return types.ValU8(b), 1
	case types.S16:
		v, ok := f.memory.ReadUint16Le(offset)
		if !ok {
			return types.ValS16(0), 2
		}
		return types.ValS16(int16(v)), 2
	case types.U16:
		v, ok := f.memory.ReadUint16Le(offset)
		if !ok {
			return types.ValU16(0), 2
		}
		return types.ValU16(v), 2
	case types.S32:
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValS32(0), 4
		}
		return types.ValS32(int32(v)), 4
	case types.U32:
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValU32(0), 4
		}
		return types.ValU32(v), 4
	case types.S64:
		v, ok := f.memory.ReadUint64Le(offset)
		if !ok {
			return types.ValS64(0), 8
		}
		return types.ValS64(int64(v)), 8
	case types.U64:
		v, ok := f.memory.ReadUint64Le(offset)
		if !ok {
			return types.ValU64(0), 8
		}
		return types.ValU64(v), 8
	case types.F32:
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValF32(0), 4
		}
		return types.ValF32(math.Float32frombits(v)), 4
	case types.F64:
		v, ok := f.memory.ReadUint64Le(offset)
		if !ok {
			return types.ValF64(0), 8
		}
		return types.ValF64(math.Float64frombits(v)), 8
	case types.Char:
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValChar(0), 4
		}
		return types.ValChar(rune(v)), 4
	case types.String:
		// String in memory: (ptr: i32, len: i32) = 8 bytes
		ptr, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValString(""), 8
		}
		length, ok := f.memory.ReadUint32Le(offset + 4)
		if !ok {
			return types.ValString(""), 8
		}
		if length == 0 {
			return types.ValString(""), 8
		}
		data, ok := f.memory.Read(ptr, length)
		if !ok {
			return types.ValString(""), 8
		}
		return types.ValString(string(data)), 8
	case types.Enum:
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValEnum(""), 4
		}
		idx := int(v)
		if idx >= 0 && idx < len(t.Cases) {
			return types.ValEnum(t.Cases[idx]), 4
		}
		return types.ValEnum(""), 4
	case types.Flags:
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValFlags(nil), 4
		}
		flagMap := make(map[string]bool)
		for i, name := range t.Names {
			flagMap[name] = (v & (1 << uint(i))) != 0
		}
		return types.ValFlags(flagMap), 4
	case types.Option:
		disc, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValOption(nil), 4
		}
		innerSize := t.Some.Size()
		if disc == 0 {
			return types.ValOption(nil), 4 + innerSize
		}
		val, _ := f.liftFieldFromMemory(offset+4, t.Some)
		return types.ValOption(&val), 4 + innerSize
	case types.List:
		ptr, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValList(nil), 8
		}
		length, ok := f.memory.ReadUint32Le(offset + 4)
		if !ok {
			return types.ValList(nil), 8
		}
		if length == 0 {
			return types.ValList(nil), 8
		}
		elemSize := t.Element.Size()
		vals := make([]types.Val, length)
		for i := uint32(0); i < length; i++ {
			val, _ := f.liftFieldFromMemory(ptr+i*elemSize, t.Element)
			vals[i] = val
		}
		return types.ValList(vals), 8
	case types.Record:
		rec := make(map[string]types.Val)
		totalSize := uint32(0)
		for _, field := range t.Fields {
			val, size := f.liftFieldFromMemory(offset+totalSize, field.Type)
			rec[field.Name] = val
			totalSize += size
		}
		return types.ValRecord(rec), totalSize
	default:
		// Fallback: read as u32
		v, ok := f.memory.ReadUint32Le(offset)
		if !ok {
			return types.ValU32(0), 4
		}
		return types.ValU32(v), 4
	}
}

// liftResultFromMemory reads a result type from linear memory at the given retptr.
// The memory layout is: discriminant (i32) at offset 0, payload at aligned offset.
func (f *ExportedFunc) liftResultFromMemory(t types.Result, retptr uint32, subtask *runtime.Subtask, callCtx *runtime.CallContext) ([]types.Val, error) {
	if f.memory == nil {
		return nil, fmt.Errorf("result lifting from memory requires memory")
	}

	// Read discriminant (always a u32/i32 at offset 0)
	discriminant, ok := f.memory.ReadUint32Le(retptr)
	if !ok {
		return nil, fmt.Errorf("failed to read result discriminant from memory at offset %d", retptr)
	}

	// Calculate payload offset: aligned to max case alignment
	// Per Canonical ABI: payload starts after discriminant, aligned to max payload alignment
	maxAlign := uint32(1)
	if t.Ok != nil {
		if a := t.Ok.Align(); a > maxAlign {
			maxAlign = a
		}
	}
	if t.Error != nil {
		if a := t.Error.Align(); a > maxAlign {
			maxAlign = a
		}
	}
	payloadOffset := alignTo(retptr+4, maxAlign)

	if discriminant == 0 {
		// Ok case
		if t.Ok != nil {
			payload, _ := f.liftFieldFromMemory(payloadOffset, t.Ok)
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			result := []types.Val{types.ValResultOk(&payload)}
			if subtask != nil {
				subtask.DeliverResolve(result)
				subtask.StartFinish()
				if err := subtask.Finish(); err != nil {
					return nil, fmt.Errorf("subtask finish: %w", err)
				}
			}
			return result, nil
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValResultOk(nil)}
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil
	}

	// Error case
	if t.Error != nil {
		errPayload, _ := f.liftFieldFromMemory(payloadOffset, t.Error)
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []types.Val{types.ValResultError(&errPayload)}
		if subtask != nil {
			subtask.DeliverResolve(result)
			subtask.StartFinish()
			if err := subtask.Finish(); err != nil {
				return nil, fmt.Errorf("subtask finish: %w", err)
			}
		}
		return result, nil
	}
	if err := callCtx.ValidateReturn(); err != nil {
		return nil, err
	}
	result := []types.Val{types.ValResultError(nil)}
	if subtask != nil {
		subtask.DeliverResolve(result)
		subtask.StartFinish()
		if err := subtask.Finish(); err != nil {
			return nil, fmt.Errorf("subtask finish: %w", err)
		}
	}
	return result, nil
}

// alignTo rounds up offset to the next multiple of align.
func alignTo(offset, align uint32) uint32 {
	return (offset + align - 1) &^ (align - 1)
}

// estimateFlatCount estimates the flat parameter count for a Val without type info.
func estimateFlatCount(val types.Val) int {
	switch val.Kind() {
	case types.ValKindString:
		return 2 // ptr + len
	case types.ValKindList:
		return 2 // ptr + len
	case types.ValKindRecord:
		count := 0
		for _, v := range val.Record() {
			count += estimateFlatCount(v)
		}
		return count
	default:
		return 1
	}
}

// lowerParam dispatches parameter lowering to lowerTyped (when type info is available)
// or lowerByKind (kind-based fallback for primitives).
func (f *ExportedFunc) lowerParam(ctx context.Context, val types.Val, resolvedType types.ValType, callCtx *runtime.CallContext) ([]uint64, error) {
	if resolvedType != nil && (typeMatchesKind(resolvedType, val.Kind()) || typeCanCoerce(resolvedType, val)) {
		return f.lowerTyped(ctx, val, resolvedType, callCtx)
	}
	return f.lowerByKind(ctx, val, callCtx)
}

// typeCanCoerce returns true if lowerTyped can coerce the Val to the expected type
// even when the kinds don't match directly.
func typeCanCoerce(typ types.ValType, val types.Val) bool {
	switch typ.(type) {
	case types.Enum:
		// String can be used as enum case name
		return val.Kind() == types.ValKindString
	case types.Option:
		// Any value can be coerced to option (nil→None, other→Some)
		return true
	case types.Variant:
		// Record with "case" field can be coerced to variant
		return val.Kind() == types.ValKindRecord
	default:
		return false
	}
}

// typeMatchesKind checks if a resolved type is compatible with a Val's runtime kind.
// When they don't match (e.g., funcType says u64 but caller passes a list),
// we fall back to kind-based lowering to avoid panics.
func typeMatchesKind(typ types.ValType, kind types.ValKind) bool {
	switch typ.(type) {
	case types.Bool:
		return kind == types.ValKindBool
	case types.S8:
		return kind == types.ValKindS8
	case types.U8:
		return kind == types.ValKindU8
	case types.S16:
		return kind == types.ValKindS16
	case types.U16:
		return kind == types.ValKindU16
	case types.S32:
		return kind == types.ValKindS32
	case types.U32:
		return kind == types.ValKindU32
	case types.S64:
		return kind == types.ValKindS64
	case types.U64:
		return kind == types.ValKindU64
	case types.F32:
		return kind == types.ValKindF32
	case types.F64:
		return kind == types.ValKindF64
	case types.Char:
		return kind == types.ValKindChar
	case types.String:
		return kind == types.ValKindString
	case types.Record:
		return kind == types.ValKindRecord
	case types.List:
		return kind == types.ValKindList
	case types.Tuple:
		return kind == types.ValKindTuple
	case types.Variant:
		return kind == types.ValKindVariant
	case types.Enum:
		return kind == types.ValKindEnum
	case types.Option:
		return kind == types.ValKindOption
	case types.Result:
		return kind == types.ValKindResult
	case types.Flags:
		return kind == types.ValKindFlags
	case types.Own:
		return kind == types.ValKindOwn
	case types.Borrow:
		return kind == types.ValKindBorrow
	default:
		return false
	}
}

// lowerTyped performs type-driven lowering for all component model value types.
// It uses the resolved type information to correctly flatten composite types.
func (f *ExportedFunc) lowerTyped(ctx context.Context, val types.Val, typ types.ValType, callCtx *runtime.CallContext) ([]uint64, error) {
	switch t := typ.(type) {
	case types.Bool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case types.S8:
		return []uint64{uint64(uint32(uint8(val.S8())))}, nil
	case types.U8:
		return []uint64{uint64(val.U8())}, nil
	case types.S16:
		return []uint64{uint64(uint32(uint16(val.S16())))}, nil
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
		var flat []uint64
		for _, field := range t.Fields {
			fieldVal, ok := rec[field.Name]
			if !ok {
				return nil, fmt.Errorf("missing record field %q", field.Name)
			}
			fieldFlat, err := f.lowerTyped(ctx, fieldVal, field.Type, callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower record field %q: %w", field.Name, err)
			}
			flat = append(flat, fieldFlat...)
		}
		return flat, nil
	case types.Tuple:
		elems := val.Tuple()
		var flat []uint64
		for i, elemType := range t.Types {
			if i >= len(elems) {
				return nil, fmt.Errorf("tuple element %d missing", i)
			}
			elemFlat, err := f.lowerTyped(ctx, elems[i], elemType, callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower tuple element %d: %w", i, err)
			}
			flat = append(flat, elemFlat...)
		}
		return flat, nil
	case types.Enum:
		// Coerce: if caller passed a string (e.g. from map[string]any), use it as enum case name
		var caseName string
		if val.Kind() == types.ValKindString {
			caseName = val.StringVal()
		} else {
			caseName = val.Enum()
		}
		for i, c := range t.Cases {
			if c == caseName {
				return []uint64{uint64(i)}, nil
			}
		}
		return nil, fmt.Errorf("unknown enum case %q", caseName)
	case types.Flags:
		flags := val.Flags()
		numWords := (len(t.Names) + 31) / 32
		if numWords == 0 {
			return nil, nil
		}
		words := make([]uint64, numWords)
		for i, name := range t.Names {
			if flags[name] {
				words[i/32] |= 1 << (uint(i) % 32)
			}
		}
		return words, nil
	case types.Option:
		// Coerce: if the value is not an option kind (e.g. zero Val from nil conversion),
		// treat it as None when v is nil, or wrap as Some otherwise
		if val.Kind() != types.ValKindOption {
			if val.IsZero() {
				// Zero Val (nil) → Option None
				payloadCount := t.Some.FlattenCount()
				flat := make([]uint64, 1+payloadCount)
				return flat, nil
			}
			// Non-nil, non-option Val → wrap as Some
			payloadFlat, err := f.lowerTyped(ctx, val, t.Some, callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower option payload: %w", err)
			}
			flat := make([]uint64, 0, 1+len(payloadFlat))
			flat = append(flat, 1)
			flat = append(flat, payloadFlat...)
			return flat, nil
		}
		opt := val.Option()
		if opt == nil {
			// None: discriminant 0 + zero-filled payload slots
			payloadCount := t.Some.FlattenCount()
			flat := make([]uint64, 1+payloadCount)
			return flat, nil
		}
		// Some: discriminant 1 + payload
		payloadFlat, err := f.lowerTyped(ctx, *opt, t.Some, callCtx)
		if err != nil {
			return nil, fmt.Errorf("lower option payload: %w", err)
		}
		flat := make([]uint64, 0, 1+len(payloadFlat))
		flat = append(flat, 1)
		flat = append(flat, payloadFlat...)
		return flat, nil
	case types.Result:
		isOk, okVal, errVal := val.Result()
		if isOk {
			// ok: discriminant 0
			flat := []uint64{0}
			if t.Ok != nil && okVal != nil {
				payloadFlat, err := f.lowerTyped(ctx, *okVal, t.Ok, callCtx)
				if err != nil {
					return nil, fmt.Errorf("lower result ok: %w", err)
				}
				flat = append(flat, payloadFlat...)
			}
			// Pad to max payload count
			maxPayload := f.resultVariantPayloadCount(t)
			for len(flat) < 1+maxPayload {
				flat = append(flat, 0)
			}
			return flat, nil
		}
		// error: discriminant 1
		flat := []uint64{1}
		if t.Error != nil && errVal != nil {
			payloadFlat, err := f.lowerTyped(ctx, *errVal, t.Error, callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower result error: %w", err)
			}
			flat = append(flat, payloadFlat...)
		}
		maxPayload := f.resultVariantPayloadCount(t)
		for len(flat) < 1+maxPayload {
			flat = append(flat, 0)
		}
		return flat, nil
	case types.Variant:
		// Coerce: if caller passed a record with "case" (and optional "payload") fields,
		// treat it as a variant
		var caseName string
		var payload *types.Val
		if val.Kind() == types.ValKindRecord {
			rec := val.Record()
			if caseVal, ok := rec["case"]; ok && caseVal.Kind() == types.ValKindString {
				caseName = caseVal.StringVal()
				if payloadVal, ok := rec["payload"]; ok {
					payload = &payloadVal
				}
			} else {
				return nil, fmt.Errorf("record used as variant must have a string \"case\" field")
			}
		} else {
			caseName, payload = val.Variant()
		}
		caseIdx := -1
		var caseType types.ValType
		for i, c := range t.Cases {
			if c.Name == caseName {
				caseIdx = i
				caseType = c.Type
				break
			}
		}
		if caseIdx < 0 {
			return nil, fmt.Errorf("unknown variant case %q", caseName)
		}
		flat := []uint64{uint64(caseIdx)}
		if caseType != nil && payload != nil {
			payloadFlat, err := f.lowerTyped(ctx, *payload, caseType, callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower variant case %q: %w", caseName, err)
			}
			flat = append(flat, payloadFlat...)
		}
		maxPayload := f.flattenVariantPayloadCount(t)
		for len(flat) < 1+maxPayload {
			flat = append(flat, 0)
		}
		return flat, nil
	case types.List:
		list := val.List()
		elemSize := t.Element.Size()
		elemAlign := t.Element.Align()
		listLen := uint32(len(list))
		listSize := listLen * elemSize

		if listSize == 0 {
			return []uint64{0, 0}, nil
		}

		if f.reallocFunc == nil {
			return nil, fmt.Errorf("list lowering requires realloc function")
		}
		if f.memory == nil {
			return nil, fmt.Errorf("list lowering requires memory")
		}

		results, err := f.reallocFunc.Call(ctx, 0, 0, uint64(elemAlign), uint64(listSize))
		if err != nil {
			return nil, fmt.Errorf("realloc for list failed: %w", err)
		}
		ptr := uint32(results[0])

		for j, elem := range list {
			offset := ptr + uint32(j)*elemSize
			if err := f.lowerToMemory(ctx, elem, t.Element, offset); err != nil {
				return nil, fmt.Errorf("lower list element %d: %w", j, err)
			}
		}
		return []uint64{uint64(ptr), uint64(listLen)}, nil
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
		if callCtx != nil {
			callCtx.IncrementBorrows()
		}
		return []uint64{uint64(h.Index())}, nil
	default:
		// Fallback to kind-based lowering
		return f.lowerByKind(ctx, val, callCtx)
	}
}

// lowerByKind performs kind-based fallback lowering for primitives, strings,
// and resource handles when no resolved type information is available.
func (f *ExportedFunc) lowerByKind(ctx context.Context, val types.Val, callCtx *runtime.CallContext) ([]uint64, error) {
	switch val.Kind() {
	case types.ValKindBool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case types.ValKindS8:
		return []uint64{uint64(uint32(uint8(val.S8())))}, nil
	case types.ValKindU8:
		return []uint64{uint64(val.U8())}, nil
	case types.ValKindS16:
		return []uint64{uint64(uint32(uint16(val.S16())))}, nil
	case types.ValKindU16:
		return []uint64{uint64(val.U16())}, nil
	case types.ValKindS32:
		return []uint64{uint64(uint32(val.S32()))}, nil
	case types.ValKindU32:
		return []uint64{uint64(val.U32())}, nil
	case types.ValKindS64:
		return []uint64{uint64(val.S64())}, nil
	case types.ValKindU64:
		return []uint64{val.U64()}, nil
	case types.ValKindF32:
		return []uint64{uint64(math.Float32bits(val.F32()))}, nil
	case types.ValKindF64:
		return []uint64{math.Float64bits(val.F64())}, nil
	case types.ValKindChar:
		return []uint64{uint64(val.Char())}, nil
	case types.ValKindString:
		return f.lowerStringParam(ctx, val.StringVal())
	case types.ValKindOwn:
		rep := val.Own()
		if f.instance == nil || f.instance.resourceTable == nil {
			return nil, fmt.Errorf("lower own: no resource table available")
		}
		h := f.instance.resourceTable.New(rep, true)
		return []uint64{uint64(h.Index())}, nil
	case types.ValKindBorrow:
		rep := val.Borrow()
		if f.instance == nil || f.instance.resourceTable == nil {
			return nil, fmt.Errorf("lower borrow: no resource table available")
		}
		h := f.instance.resourceTable.New(rep, false)
		if callCtx != nil {
			callCtx.IncrementBorrows()
		}
		return []uint64{uint64(h.Index())}, nil
	case types.ValKindList:
		// List lowering without type info: infer element size from first element's kind
		list := val.List()
		mem := f.memory
		if mem == nil && f.instance != nil && len(f.instance.coreInstances) > 0 {
			mem = f.instance.coreInstances[0].Memory()
		}
		if mem == nil {
			return nil, fmt.Errorf("list lowering requires memory")
		}
		var elemSize uint32 = 4
		var alignment uint32 = 4
		if len(list) > 0 {
			elemSize = elementSizeForKind(list[0].Kind())
			alignment = alignmentForKind(list[0].Kind())
		}
		listLen := uint32(len(list))
		listSize := listLen * elemSize
		var ptr uint32
		if listSize > 0 {
			if f.reallocFunc == nil {
				return nil, fmt.Errorf("list lowering requires realloc function")
			}
			results, err := f.reallocFunc.Call(ctx, 0, 0, uint64(alignment), uint64(listSize))
			if err != nil {
				return nil, fmt.Errorf("realloc for list failed: %w", err)
			}
			ptr = uint32(results[0])
		}
		for j, elem := range list {
			offset := ptr + uint32(j)*elemSize
			if err := writeListElement(mem, offset, elem); err != nil {
				return nil, fmt.Errorf("failed to write list element %d: %w", j, err)
			}
		}
		return []uint64{uint64(ptr), uint64(listLen)}, nil
	case types.ValKindRecord:
		// Record lowering without type info: flatten fields in alphabetical order
		rec := val.Record()
		fieldNames := make([]string, 0, len(rec))
		for name := range rec {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)
		var result []uint64
		for _, name := range fieldNames {
			field := rec[name]
			flat, err := f.lowerByKind(ctx, field, callCtx)
			if err != nil {
				return nil, fmt.Errorf("lower record field %s: %w", name, err)
			}
			result = append(result, flat...)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported parameter type: %s", val.Kind())
	}
}

// lowerStringParam allocates memory, writes UTF-8 bytes, and returns (ptr, len).
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

// allocate allocates memory using the realloc function.
func (f *ExportedFunc) allocate(ctx context.Context, size, align uint32) (uint32, error) {
	if f.reallocFunc == nil {
		return 0, fmt.Errorf("memory allocation requires realloc function")
	}
	results, err := f.reallocFunc.Call(ctx, 0, 0, uint64(align), uint64(size))
	if err != nil {
		return 0, fmt.Errorf("realloc failed: %w", err)
	}
	return uint32(results[0]), nil
}

// alignTo32 rounds up offset to the given alignment.
func alignTo32(offset, align uint32) uint32 {
	if align == 0 {
		return offset
	}
	return (offset + align - 1) &^ (align - 1)
}

// lowerToMemory writes a val to linear memory at the given offset using the type information.
// Used for list elements and other cases where values must be stored in memory.
func (f *ExportedFunc) lowerToMemory(ctx context.Context, val types.Val, typ types.ValType, offset uint32) error {
	if f.memory == nil {
		return fmt.Errorf("lowerToMemory: no memory available")
	}
	switch t := typ.(type) {
	case types.Bool:
		var b byte
		if val.Bool() {
			b = 1
		}
		if !f.memory.WriteByteAt(offset, b) {
			return fmt.Errorf("failed to write bool at offset %d", offset)
		}
	case types.S8:
		if !f.memory.WriteByteAt(offset, byte(val.S8())) {
			return fmt.Errorf("failed to write s8 at offset %d", offset)
		}
	case types.U8:
		if !f.memory.WriteByteAt(offset, val.U8()) {
			return fmt.Errorf("failed to write u8 at offset %d", offset)
		}
	case types.S16:
		if !f.memory.WriteUint16Le(offset, uint16(val.S16())) {
			return fmt.Errorf("failed to write s16 at offset %d", offset)
		}
	case types.U16:
		if !f.memory.WriteUint16Le(offset, val.U16()) {
			return fmt.Errorf("failed to write u16 at offset %d", offset)
		}
	case types.S32:
		if !f.memory.WriteUint32Le(offset, uint32(val.S32())) {
			return fmt.Errorf("failed to write s32 at offset %d", offset)
		}
	case types.U32:
		if !f.memory.WriteUint32Le(offset, val.U32()) {
			return fmt.Errorf("failed to write u32 at offset %d", offset)
		}
	case types.S64:
		if !f.memory.WriteUint64Le(offset, uint64(val.S64())) {
			return fmt.Errorf("failed to write s64 at offset %d", offset)
		}
	case types.U64:
		if !f.memory.WriteUint64Le(offset, val.U64()) {
			return fmt.Errorf("failed to write u64 at offset %d", offset)
		}
	case types.F32:
		if !f.memory.WriteFloat32Le(offset, val.F32()) {
			return fmt.Errorf("failed to write f32 at offset %d", offset)
		}
	case types.F64:
		if !f.memory.WriteFloat64Le(offset, val.F64()) {
			return fmt.Errorf("failed to write f64 at offset %d", offset)
		}
	case types.Char:
		if !f.memory.WriteUint32Le(offset, uint32(val.Char())) {
			return fmt.Errorf("failed to write char at offset %d", offset)
		}
	case types.String:
		flat, err := f.lowerStringParam(ctx, val.StringVal())
		if err != nil {
			return fmt.Errorf("lower string to memory: %w", err)
		}
		if !f.memory.WriteUint32Le(offset, uint32(flat[0])) {
			return fmt.Errorf("failed to write string ptr at offset %d", offset)
		}
		if !f.memory.WriteUint32Le(offset+4, uint32(flat[1])) {
			return fmt.Errorf("failed to write string len at offset %d", offset+4)
		}
	case types.Record:
		rec := val.Record()
		fieldOffsets := t.FieldOffsets()
		for i, field := range t.Fields {
			fieldVal, ok := rec[field.Name]
			if !ok {
				return fmt.Errorf("missing record field %q", field.Name)
			}
			if err := f.lowerToMemory(ctx, fieldVal, field.Type, offset+fieldOffsets[i]); err != nil {
				return fmt.Errorf("lower record field %q to memory: %w", field.Name, err)
			}
		}
	case types.Enum:
		var caseName string
		if val.Kind() == types.ValKindString {
			caseName = val.StringVal()
		} else {
			caseName = val.Enum()
		}
		for i, c := range t.Cases {
			if c == caseName {
				discSize := t.Size()
				switch discSize {
				case 1:
					if !f.memory.WriteByteAt(offset, byte(i)) {
						return fmt.Errorf("failed to write enum at offset %d", offset)
					}
				case 2:
					if !f.memory.WriteUint16Le(offset, uint16(i)) {
						return fmt.Errorf("failed to write enum at offset %d", offset)
					}
				default:
					if !f.memory.WriteUint32Le(offset, uint32(i)) {
						return fmt.Errorf("failed to write enum at offset %d", offset)
					}
				}
				return nil
			}
		}
		return fmt.Errorf("unknown enum case %q", caseName)
	case types.Option:
		// Coerce: handle non-option Val kinds
		if val.Kind() != types.ValKindOption {
			if val.IsZero() {
				// Zero Val (nil) → Option None
				if !f.memory.WriteByteAt(offset, 0) {
					return fmt.Errorf("failed to write option discriminant at offset %d", offset)
				}
				return nil
			}
			// Non-nil, non-option Val → wrap as Some
			if !f.memory.WriteByteAt(offset, 1) {
				return fmt.Errorf("failed to write option discriminant at offset %d", offset)
			}
			if t.Some != nil {
				payloadAlign := t.Some.Align()
				payloadOffset := offset + alignTo32(1, payloadAlign)
				if err := f.lowerToMemory(ctx, val, t.Some, payloadOffset); err != nil {
					return fmt.Errorf("lower option payload to memory: %w", err)
				}
			}
			return nil
		}
		opt := val.Option()
		if opt == nil {
			// None: write discriminant 0
			if !f.memory.WriteByteAt(offset, 0) {
				return fmt.Errorf("failed to write option discriminant at offset %d", offset)
			}
		} else {
			// Some: write discriminant 1 + payload
			if !f.memory.WriteByteAt(offset, 1) {
				return fmt.Errorf("failed to write option discriminant at offset %d", offset)
			}
			if t.Some != nil {
				// Compute payload offset: discriminant (1 byte) aligned to payload alignment
				payloadAlign := t.Some.Align()
				payloadOffset := offset + alignTo32(1, payloadAlign)
				if err := f.lowerToMemory(ctx, *opt, t.Some, payloadOffset); err != nil {
					return fmt.Errorf("lower option payload to memory: %w", err)
				}
			}
		}
	case types.Variant:
		// Coerce: record with "case"/"payload" → variant
		var caseName string
		var payload *types.Val
		if val.Kind() == types.ValKindRecord {
			rec := val.Record()
			if caseVal, ok := rec["case"]; ok && caseVal.Kind() == types.ValKindString {
				caseName = caseVal.StringVal()
				if payloadVal, ok := rec["payload"]; ok {
					payload = &payloadVal
				}
			} else {
				return fmt.Errorf("record used as variant must have a string \"case\" field")
			}
		} else {
			caseName, payload = val.Variant()
		}
		caseIdx := -1
		var caseType types.ValType
		for i, c := range t.Cases {
			if c.Name == caseName {
				caseIdx = i
				caseType = c.Type
				break
			}
		}
		if caseIdx < 0 {
			return fmt.Errorf("unknown variant case %q", caseName)
		}
		// Write discriminant
		discSize := t.DiscriminantSize()
		switch discSize {
		case 1:
			if !f.memory.WriteByteAt(offset, byte(caseIdx)) {
				return fmt.Errorf("failed to write variant discriminant at offset %d", offset)
			}
		case 2:
			if !f.memory.WriteUint16Le(offset, uint16(caseIdx)) {
				return fmt.Errorf("failed to write variant discriminant at offset %d", offset)
			}
		default:
			if !f.memory.WriteUint32Le(offset, uint32(caseIdx)) {
				return fmt.Errorf("failed to write variant discriminant at offset %d", offset)
			}
		}
		// Write payload if present
		if caseType != nil && payload != nil {
			payloadAlign := caseType.Align()
			payloadOffset := offset + alignTo(discSize, payloadAlign)
			if err := f.lowerToMemory(ctx, *payload, caseType, payloadOffset); err != nil {
				return fmt.Errorf("lower variant case %q to memory: %w", caseName, err)
			}
		}
	case types.Result:
		isOk, okVal, errVal := val.Result()
		// Write discriminant: 0=ok, 1=error
		if isOk {
			if !f.memory.WriteUint32Le(offset, 0) {
				return fmt.Errorf("failed to write result discriminant at offset %d", offset)
			}
			if t.Ok != nil && okVal != nil {
				payloadAlign := t.Ok.Align()
				payloadOffset := offset + alignTo(4, payloadAlign)
				if err := f.lowerToMemory(ctx, *okVal, t.Ok, payloadOffset); err != nil {
					return fmt.Errorf("lower result ok to memory: %w", err)
				}
			}
		} else {
			if !f.memory.WriteUint32Le(offset, 1) {
				return fmt.Errorf("failed to write result discriminant at offset %d", offset)
			}
			if t.Error != nil && errVal != nil {
				payloadAlign := t.Error.Align()
				payloadOffset := offset + alignTo(4, payloadAlign)
				if err := f.lowerToMemory(ctx, *errVal, t.Error, payloadOffset); err != nil {
					return fmt.Errorf("lower result error to memory: %w", err)
				}
			}
		}
	case types.List:
		list := val.List()
		if len(list) == 0 {
			// Empty list: write ptr=0, len=0
			if !f.memory.WriteUint32Le(offset, 0) {
				return fmt.Errorf("failed to write list ptr at offset %d", offset)
			}
			if !f.memory.WriteUint32Le(offset+4, 0) {
				return fmt.Errorf("failed to write list len at offset %d", offset)
			}
		} else {
			elemSize := t.ElementSize()
			totalSize := uint32(len(list)) * elemSize
			// Allocate memory for list elements
			listPtr, err := f.allocate(ctx, totalSize, t.ElementAlign())
			if err != nil {
				return fmt.Errorf("allocate list memory: %w", err)
			}
			// Write each element
			for i, elem := range list {
				elemOffset := listPtr + uint32(i)*elemSize
				if err := f.lowerToMemory(ctx, elem, t.Element, elemOffset); err != nil {
					return fmt.Errorf("lower list element %d to memory: %w", i, err)
				}
			}
			// Write ptr and len
			if !f.memory.WriteUint32Le(offset, listPtr) {
				return fmt.Errorf("failed to write list ptr at offset %d", offset)
			}
			if !f.memory.WriteUint32Le(offset+4, uint32(len(list))) {
				return fmt.Errorf("failed to write list len at offset %d", offset)
			}
		}
	default:
		// Fallback: use writeListElement for kind-based writing
		return writeListElement(f.memory, offset, val)
	}
	return nil
}

// flattenVariantPayloadCount returns the maximum flat count across all variant cases' payloads.
func (f *ExportedFunc) flattenVariantPayloadCount(v types.Variant) int {
	max := 0
	for _, c := range v.Cases {
		if c.Type != nil {
			if n := c.Type.FlattenCount(); n > max {
				max = n
			}
		}
	}
	return max
}

// resultVariantPayloadCount returns the max flat count for a Result's ok/error payloads.
func (f *ExportedFunc) resultVariantPayloadCount(r types.Result) int {
	max := 0
	if r.Ok != nil {
		if n := r.Ok.FlattenCount(); n > max {
			max = n
		}
	}
	if r.Error != nil {
		if n := r.Error.FlattenCount(); n > max {
			max = n
		}
	}
	return max
}

// liftOwn transfers ownership of a resource out of the component.
// It removes the handle from the table and returns the representation value.
// Traps if the handle has active borrows (NumLends > 0).
// Traps if the handle is not owned (i.e., it's a borrowed handle).
func (f *ExportedFunc) liftOwn(handleIdx uint32, borrowScope *runtime.BorrowScope) (any, error) {
	if f.instance == nil || f.instance.resourceTable == nil {
		return nil, fmt.Errorf("lift_own: no resource table available")
	}

	// Construct handle from index - try to find the valid generation
	h := runtime.Handle(handleIdx)

	// Get the entry first to check ownership
	entry, err := f.instance.resourceTable.Get(h)
	if err != nil {
		// Try to find the entry by scanning generations
		for gen := uint32(1); gen < 1000; gen++ {
			h = runtime.MakeHandle(handleIdx, gen)
			entry, err = f.instance.resourceTable.Get(h)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("lift_own: invalid handle index %d: %w", handleIdx, err)
		}
	}

	// Verify this is an owned handle, not a borrow
	if !entry.Own {
		return nil, fmt.Errorf("lift_own: handle is not owned")
	}

	// Remove from table (this checks NumLends > 0 and returns error if so)
	removed, err := f.instance.resourceTable.Remove(h)
	if err != nil {
		return nil, fmt.Errorf("lift_own: %w", err)
	}

	return removed.Rep, nil
}

// liftBorrow reads a resource representation for borrowing.
// Unlike liftOwn, it does NOT remove the handle from the table.
// It tracks the lend in the runtime.BorrowScope to prevent ownership transfer while borrowed.
func (f *ExportedFunc) liftBorrow(handleIdx uint32, borrowScope *runtime.BorrowScope) (any, error) {
	if f.instance == nil || f.instance.resourceTable == nil {
		return nil, fmt.Errorf("lift_borrow: no resource table available")
	}

	// Construct handle from index
	h := runtime.Handle(handleIdx)

	// Try to get the entry
	entry, err := f.instance.resourceTable.Get(h)
	if err != nil {
		// Try to find the entry by scanning generations
		for gen := uint32(1); gen < 1000; gen++ {
			h = runtime.MakeHandle(handleIdx, gen)
			entry, err = f.instance.resourceTable.Get(h)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("lift_borrow: invalid handle index %d: %w", handleIdx, err)
		}
	}

	// Track the lend in the borrow scope
	if borrowScope != nil {
		if err := borrowScope.AddLender(h); err != nil {
			return nil, fmt.Errorf("lift_borrow: tracking lend: %w", err)
		}
	}

	return entry.Rep, nil
}

// ResourceNew implements canon resource.new.
// Creates an owned handle from a representation value.
func (i *Instance) ResourceNew(rep any) (uint32, error) {
	if i.resourceTable == nil {
		i.resourceTable = runtime.NewResourceTable()
	}
	h := i.resourceTable.New(rep, true)
	return uint32(h), nil
}

// ResourceRep implements canon resource.rep.
// Extracts the representation from a handle without removing it.
func (i *Instance) ResourceRep(handleIdx uint32) (any, error) {
	if i.resourceTable == nil {
		return nil, fmt.Errorf("resource.rep: %w", runtime.ErrInvalidHandle)
	}
	h := runtime.Handle(handleIdx)
	entry, err := i.resourceTable.Get(h)
	if err != nil {
		return nil, fmt.Errorf("resource.rep: %w", err)
	}
	return entry.Rep, nil
}

// ResourceDrop implements canon resource.drop.
// Removes the handle and invokes destructor if owned.
// For borrowed handles, decrements the call context borrow count instead.
func (i *Instance) ResourceDrop(handleIdx uint32, resourceTypeIdx uint32) error {
	if i.resourceTable == nil {
		return fmt.Errorf("resource.drop: %w", runtime.ErrInvalidHandle)
	}
	h := runtime.Handle(handleIdx)
	entry, err := i.resourceTable.Remove(h)
	if err != nil {
		return fmt.Errorf("resource.drop: %w", err)
	}

	if entry.Own {
		// Call destructor for owned handles
		if i.destructors != nil {
			if dtor, ok := i.destructors[resourceTypeIdx]; ok {
				dtor(entry.Rep)
			}
		}
	} else {
		// Decrement borrow count for borrowed handles
		if i.callContext != nil {
			i.callContext.DecrementBorrows()
		}
	}

	return nil
}

// SetDestructor registers a destructor function for a resource type.
// The destructor is called when an owned handle of this type is dropped.
func (i *Instance) SetDestructor(resourceTypeIdx uint32, dtor func(any)) {
	if i.destructors == nil {
		i.destructors = make(map[uint32]func(any))
	}
	i.destructors[resourceTypeIdx] = dtor
}

// SetCallContext sets the current call context for borrow tracking.
func (i *Instance) SetCallContext(ctx *runtime.CallContext) {
	i.callContext = ctx
}

// CallContext returns the current call context.
func (i *Instance) CallContext() *runtime.CallContext {
	return i.callContext
}

// MayLeave returns whether this instance is allowed to call out.
// Returns true by default, false during lowering and post-return.
func (i *Instance) MayLeave() bool {
	return !i.mayLeaveDisabled
}

// SetMayLeave sets whether this instance is allowed to call out.
// Called with false at the start of lowering/post-return, true at the end.
func (i *Instance) SetMayLeave(allowed bool) {
	i.mayLeaveDisabled = !allowed
}

// ActiveCallDepth returns the number of active calls into this instance.
func (i *Instance) ActiveCallDepth() int {
	if i == nil {
		return 0
	}
	return int(i.activeCallDepth)
}

// EnterCall increments the active call depth.
// Called at the start of canon_lift.
func (i *Instance) EnterCall() {
	if i != nil {
		i.activeCallDepth++
	}
}

// ExitCall decrements the active call depth.
// Called at the end of canon_lift (including post-return).
func (i *Instance) ExitCall() {
	if i != nil && i.activeCallDepth > 0 {
		i.activeCallDepth--
	}
}

// CallMightBeRecursive checks if a call from caller into this instance might
// cause recursive reentrance. Returns true if:
// 1. caller is the same instance as this instance (self-call)
// 2. There's already an active call in this instance
//
// Per CanonicalABI.md, canon_lift must trap if this returns true.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
	if i == nil || caller == nil {
		// Nil callee or nil caller (host) - no reentrance concern
		return false
	}

	// Check if this is a self-call with an active call already in progress
	if caller == i && i.activeCallDepth > 0 {
		return true
	}

	return false
}

// ValidateMayLeave checks if this instance is allowed to make outgoing calls.
// Returns an error if may_leave is false (during lowering or post-return).
// This implements the trap_if(not inst.may_leave) check from canon_lower.
// Per CanonicalABI.md line 3454
func (i *Instance) ValidateMayLeave() error {
	if i == nil {
		return nil // No instance means no restriction
	}
	if !i.MayLeave() {
		return fmt.Errorf("trap: cannot call out of component while lowering values")
	}
	return nil
}

// ValidateNotRecursive checks if a call from caller would be recursive reentrance.
// Returns an error that should cause a trap if reentrance is detected.
// This implements the trap_if(call_might_be_recursive(caller, inst)) check.
func (i *Instance) ValidateNotRecursive(caller *Instance) error {
	if i.CallMightBeRecursive(caller) {
		return fmt.Errorf("trap: recursive call into same component instance")
	}
	return nil
}

// AddValue adds a value to the instance's value index space.
// Returns the index of the added value.
func (i *Instance) AddValue(v types.Val) uint32 {
	if i.values == nil {
		i.values = make([]types.Val, 0)
		i.valuesConsumed = make([]bool, 0)
	}
	idx := uint32(len(i.values))
	i.values = append(i.values, v)
	i.valuesConsumed = append(i.valuesConsumed, false)
	return idx
}

// GetValue retrieves a value from the value index space.
func (i *Instance) GetValue(idx uint32) (types.Val, error) {
	if idx >= uint32(len(i.values)) {
		return types.Val{}, fmt.Errorf("value index %d out of range (have %d)", idx, len(i.values))
	}
	return i.values[idx], nil
}

// ConsumeValue retrieves and marks a value as consumed.
// Returns error if value already consumed or index out of range.
func (i *Instance) ConsumeValue(idx uint32) (types.Val, error) {
	if idx >= uint32(len(i.values)) {
		return types.Val{}, fmt.Errorf("value index %d out of range (have %d)", idx, len(i.values))
	}
	if i.valuesConsumed[idx] {
		return types.Val{}, fmt.Errorf("value %d already consumed", idx)
	}
	i.valuesConsumed[idx] = true
	return i.values[idx], nil
}

// IsValueConsumed returns whether a value has been consumed.
func (i *Instance) IsValueConsumed(idx uint32) bool {
	if idx >= uint32(len(i.valuesConsumed)) {
		return false
	}
	return i.valuesConsumed[idx]
}

// Parent returns this instance's parent, or nil if top-level.
func (i *Instance) Parent() *Instance {
	return i.parent
}

// Children returns this instance's child instances.
func (i *Instance) Children() []*Instance {
	return i.children
}

// AddChild adds a child instance and sets its parent.
func (i *Instance) AddChild(child *Instance) {
	if i.children == nil {
		i.children = make([]*Instance, 0)
	}
	i.children = append(i.children, child)
	child.parent = i
}

// GetAncestor returns the ancestor at the given depth.
// Depth 0 returns self, depth 1 returns parent, etc.
func (i *Instance) GetAncestor(depth uint32) *Instance {
	current := i
	for d := uint32(0); d < depth && current != nil; d++ {
		current = current.parent
	}
	return current
}

// AddInstanceToSpace adds an instance to the instance index space.
func (i *Instance) AddInstanceToSpace(inst *Instance) uint32 {
	idx := uint32(len(i.instanceSpace))
	i.instanceSpace = append(i.instanceSpace, inst)
	return idx
}

// GetInstanceFromSpace retrieves an instance from the instance index space.
func (i *Instance) GetInstanceFromSpace(idx uint32) *Instance {
	if idx >= uint32(len(i.instanceSpace)) {
		return nil
	}
	return i.instanceSpace[idx]
}

// AddTypeToSpace adds a type definition to the type index space.
func (i *Instance) AddTypeToSpace(t *TypeDef) uint32 {
	idx := uint32(len(i.typeSpace))
	i.typeSpace = append(i.typeSpace, t)
	return idx
}

// GetTypeFromSpace retrieves a type from the type index space.
func (i *Instance) GetTypeFromSpace(idx uint32) *TypeDef {
	if idx >= uint32(len(i.typeSpace)) {
		return nil
	}
	return i.typeSpace[idx]
}

// AddComponentToSpace adds a component to the component index space.
func (i *Instance) AddComponentToSpace(c *Component) uint32 {
	idx := uint32(len(i.componentSpace))
	i.componentSpace = append(i.componentSpace, c)
	return idx
}

// GetComponentFromSpace retrieves a component from the component index space.
func (i *Instance) GetComponentFromSpace(idx uint32) *Component {
	if idx >= uint32(len(i.componentSpace)) {
		return nil
	}
	return i.componentSpace[idx]
}

// AddExportedInstance adds an instance to the exported instances map.
func (i *Instance) AddExportedInstance(name string, inst *Instance) {
	if i.exportedInstances == nil {
		i.exportedInstances = make(map[string]*Instance)
	}
	i.exportedInstances[name] = inst
}

// GetExportedInstance retrieves an exported instance by name.
func (i *Instance) GetExportedInstance(name string) *Instance {
	if i.exportedInstances == nil {
		return nil
	}
	return i.exportedInstances[name]
}

// liftEnum converts a discriminant to an enum Val.
func liftEnum(discriminant uint64, enumType *EnumType) (types.Val, error) {
	idx := int(discriminant)
	if idx < 0 || idx >= len(enumType.Cases) {
		return types.Val{}, fmt.Errorf("invalid enum discriminant %d for type with %d cases",
			discriminant, len(enumType.Cases))
	}
	return types.ValEnum(enumType.Cases[idx]), nil
}

// liftFlags converts a bitvector to a flags Val.
func liftFlags(bitvector uint64, flagsType *FlagsType) (types.Val, error) {
	flags := make(map[string]bool)
	for i, name := range flagsType.Flags {
		if bitvector&(1<<i) != 0 {
			flags[name] = true
		}
	}
	return types.ValFlags(flags), nil
}

// liftVariant converts flat representation to a variant Val.
// The flat representation consists of a discriminant followed by the payload values.
// This is the inverse of lowerVariantToFlat in canon_lower.go.
func liftVariant(flat []uint64, variantType *VariantType) (types.Val, error) {
	if len(flat) < 1 {
		return types.Val{}, fmt.Errorf("variant requires at least discriminant")
	}

	disc := int(flat[0])
	if disc < 0 || disc >= len(variantType.Cases) {
		return types.Val{}, fmt.Errorf("invalid variant discriminant %d for type with %d cases",
			disc, len(variantType.Cases))
	}

	variantCase := variantType.Cases[disc]

	// If the case has no payload type, return variant with nil payload
	if variantCase.Type == nil {
		return types.ValVariant(variantCase.Name, nil), nil
	}

	// Case has a payload - we need to lift it from the remaining flat values
	if len(flat) < 2 {
		return types.Val{}, fmt.Errorf("variant case %q requires payload but none provided", variantCase.Name)
	}

	// Lift the payload based on its type
	payload, err := liftVariantPayload(flat[1], variantCase.Type)
	if err != nil {
		return types.Val{}, fmt.Errorf("lifting variant payload for case %q: %w", variantCase.Name, err)
	}

	return types.ValVariant(variantCase.Name, &payload), nil
}

// liftVariantPayload lifts a single payload value based on its PayloadType.
func liftVariantPayload(flatVal uint64, payloadType PayloadType) (types.Val, error) {
	// Handle PrimitiveType payloads
	if prim, ok := payloadType.(*PrimitiveType); ok {
		switch prim.Name {
		case "bool":
			return types.ValBool(flatVal != 0), nil
		case "s8":
			return types.ValS8(int8(flatVal)), nil
		case "u8":
			return types.ValU8(uint8(flatVal)), nil
		case "s16":
			return types.ValS16(int16(flatVal)), nil
		case "u16":
			return types.ValU16(uint16(flatVal)), nil
		case "s32":
			return types.ValS32(int32(flatVal)), nil
		case "u32":
			return types.ValU32(uint32(flatVal)), nil
		case "s64":
			return types.ValS64(int64(flatVal)), nil
		case "u64":
			return types.ValU64(flatVal), nil
		case "f32":
			return types.ValF32(math.Float32frombits(uint32(flatVal))), nil
		case "f64":
			return types.ValF64(math.Float64frombits(flatVal)), nil
		case "char":
			return types.ValChar(rune(flatVal)), nil
		default:
			// For unknown primitive types, default to s32
			return types.ValS32(int32(flatVal)), nil
		}
	}

	// For non-primitive types, default to s32 (simplified handling)
	// A full implementation would recursively handle composite types
	return types.ValS32(int32(flatVal)), nil
}

// elementSizeForKind returns the size in bytes for a ValKind per the Canonical ABI.
func elementSizeForKind(kind types.ValKind) uint32 {
	switch kind {
	case types.ValKindS8, types.ValKindU8, types.ValKindBool:
		return 1
	case types.ValKindS16, types.ValKindU16:
		return 2
	case types.ValKindS32, types.ValKindU32, types.ValKindF32, types.ValKindChar:
		return 4
	case types.ValKindS64, types.ValKindU64, types.ValKindF64:
		return 8
	default:
		return 4 // Default to 4 for unknown types
	}
}

// alignmentForKind returns the alignment in bytes for a ValKind per the Canonical ABI.
func alignmentForKind(kind types.ValKind) uint32 {
	switch kind {
	case types.ValKindS8, types.ValKindU8, types.ValKindBool:
		return 1
	case types.ValKindS16, types.ValKindU16:
		return 2
	case types.ValKindS32, types.ValKindU32, types.ValKindF32, types.ValKindChar:
		return 4
	case types.ValKindS64, types.ValKindU64, types.ValKindF64:
		return 8
	default:
		return 4 // Default to 4 for unknown types
	}
}

// sizeOfVal returns the size in bytes needed to store a Val in memory.
// For composite types, this returns an estimate suitable for result/variant lowering.
func sizeOfVal(v types.Val) uint32 {
	switch v.Kind() {
	case types.ValKindS8, types.ValKindU8, types.ValKindBool:
		return 1
	case types.ValKindS16, types.ValKindU16:
		return 2
	case types.ValKindS32, types.ValKindU32, types.ValKindF32, types.ValKindChar, types.ValKindOwn, types.ValKindBorrow:
		return 4
	case types.ValKindS64, types.ValKindU64, types.ValKindF64:
		return 8
	case types.ValKindString, types.ValKindList:
		return 8 // ptr + len
	case types.ValKindResult:
		// discriminant (4 bytes aligned) + max payload size
		_, okVal, errVal := v.Result()
		var payloadSize uint32 = 0
		if okVal != nil {
			payloadSize = sizeOfVal(*okVal)
		}
		if errVal != nil {
			if s := sizeOfVal(*errVal); s > payloadSize {
				payloadSize = s
			}
		}
		return 4 + payloadSize
	case types.ValKindVariant:
		// discriminant (4 bytes) + payload
		_, payload := v.Variant()
		if payload != nil {
			return 4 + sizeOfVal(*payload)
		}
		return 4
	case types.ValKindOption:
		// discriminant (4 bytes) + payload
		payload := v.Option()
		if payload != nil {
			return 4 + sizeOfVal(*payload)
		}
		return 4
	default:
		return 4 // Default to 4 for unknown types
	}
}

// writeListElement writes a Val to memory at the given offset.
// Returns an error if the write fails or the element type is not supported.
func writeListElement(mem api.Memory, offset uint32, elem types.Val) error {
	switch elem.Kind() {
	case types.ValKindS8:
		if !mem.WriteByteAt(offset, byte(elem.S8())) {
			return fmt.Errorf("failed to write s8 at offset %d", offset)
		}
	case types.ValKindU8:
		if !mem.WriteByteAt(offset, elem.U8()) {
			return fmt.Errorf("failed to write u8 at offset %d", offset)
		}
	case types.ValKindBool:
		var b byte
		if elem.Bool() {
			b = 1
		}
		if !mem.WriteByteAt(offset, b) {
			return fmt.Errorf("failed to write bool at offset %d", offset)
		}
	case types.ValKindS16:
		if !mem.WriteUint16Le(offset, uint16(elem.S16())) {
			return fmt.Errorf("failed to write s16 at offset %d", offset)
		}
	case types.ValKindU16:
		if !mem.WriteUint16Le(offset, elem.U16()) {
			return fmt.Errorf("failed to write u16 at offset %d", offset)
		}
	case types.ValKindS32:
		if !mem.WriteUint32Le(offset, uint32(elem.S32())) {
			return fmt.Errorf("failed to write s32 at offset %d", offset)
		}
	case types.ValKindU32:
		if !mem.WriteUint32Le(offset, elem.U32()) {
			return fmt.Errorf("failed to write u32 at offset %d", offset)
		}
	case types.ValKindChar:
		if !mem.WriteUint32Le(offset, uint32(elem.Char())) {
			return fmt.Errorf("failed to write char at offset %d", offset)
		}
	case types.ValKindF32:
		bits := math.Float32bits(elem.F32())
		if !mem.WriteUint32Le(offset, bits) {
			return fmt.Errorf("failed to write f32 at offset %d", offset)
		}
	case types.ValKindS64:
		if !mem.WriteUint64Le(offset, uint64(elem.S64())) {
			return fmt.Errorf("failed to write s64 at offset %d", offset)
		}
	case types.ValKindU64:
		if !mem.WriteUint64Le(offset, elem.U64()) {
			return fmt.Errorf("failed to write u64 at offset %d", offset)
		}
	case types.ValKindF64:
		bits := math.Float64bits(elem.F64())
		if !mem.WriteUint64Le(offset, bits) {
			return fmt.Errorf("failed to write f64 at offset %d", offset)
		}
	default:
		return fmt.Errorf("unsupported list element type: %s", elem.Kind())
	}
	return nil
}
