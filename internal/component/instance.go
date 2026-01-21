// internal/component/instance.go

package component

import (
	"context"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/tetratelabs/wazero/api"
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
	resourceTable *ResourceTable          // Table for tracking resource handles
	destructors   map[uint32]func(any)    // Destructor functions by resource type index
	callContext   *CallContext            // Current call context for borrow tracking

	// mayLeaveDisabled tracks whether the component cannot call out.
	// Set to true during lowering and post-return to prevent reentrance.
	// Per Canonical ABI spec, may_leave defaults to true (so this defaults to false).
	mayLeaveDisabled bool

	// activeCallDepth tracks the number of active calls into this instance.
	// Used by call_might_be_recursive to detect reentrance.
	activeCallDepth int32

	// Value index space for start function support
	values         []Val
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
	Impl func(ctx context.Context, args []Val) ([]Val, error)
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
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Set up call context and subtask for resource tracking
	callCtx := NewCallContext()
	var subtask *Subtask
	var borrowScope *BorrowScope

	// Initialize resource table if needed
	if f.instance != nil {
		if f.instance.resourceTable == nil {
			f.instance.resourceTable = NewResourceTable()
		}
		// Create subtask which owns the borrow scope
		subtask = NewSubtask(f.instance.resourceTable)
		borrowScope = subtask.BorrowScope()

		// Defer subtask cleanup for error paths
		// This ensures the borrow scope is released even if we return early
		defer func() {
			if subtask != nil && subtask.State() == SubtaskStatePending {
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

	// Create TypeResolver for dynamic type resolution
	var resolver *TypeResolver
	if f.component != nil {
		resolver = NewTypeResolver(f.component)
	}

	// Convert component Vals to core wasm values
	// Records are flattened into their constituent fields
	var coreParams []uint64

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

	for i, p := range params {
		switch p.Kind() {
		case ValKindS32:
			coreParams = append(coreParams, uint64(uint32(p.S32())))
		case ValKindU32:
			coreParams = append(coreParams, uint64(p.U32()))
		case ValKindS64:
			coreParams = append(coreParams, uint64(p.S64()))
		case ValKindU64:
			coreParams = append(coreParams, p.U64())
		case ValKindRecord:
			// Flatten record fields in alphabetical order (component model spec)
			rec := p.Record()
			// Get field names and sort them for deterministic order
			fieldNames := make([]string, 0, len(rec))
			for name := range rec {
				fieldNames = append(fieldNames, name)
			}
			sort.Strings(fieldNames)

			for _, name := range fieldNames {
				field := rec[name]
				switch field.Kind() {
				case ValKindS32:
					coreParams = append(coreParams, uint64(uint32(field.S32())))
				case ValKindU32:
					coreParams = append(coreParams, uint64(field.U32()))
				case ValKindS64:
					coreParams = append(coreParams, uint64(field.S64()))
				case ValKindU64:
					coreParams = append(coreParams, field.U64())
				default:
					return nil, fmt.Errorf("unsupported record field type: %s", field.Kind())
				}
			}
		case ValKindOption:
			// Flatten option to (discriminant: i32, payload: i32)
			// discriminant: 0 = None, 1 = Some
			opt := p.Option()
			if opt == nil {
				// None: discriminant = 0, payload = 0 (unused)
				coreParams = append(coreParams, 0, 0)
			} else {
				// Some: discriminant = 1, payload = value
				switch opt.Kind() {
				case ValKindS32:
					coreParams = append(coreParams, 1, uint64(uint32(opt.S32())))
				case ValKindU32:
					coreParams = append(coreParams, 1, uint64(opt.U32()))
				case ValKindS64:
					coreParams = append(coreParams, 1, uint64(opt.S64()))
				case ValKindU64:
					coreParams = append(coreParams, 1, opt.U64())
				default:
					return nil, fmt.Errorf("unsupported option payload type: %s", opt.Kind())
				}
			}
		case ValKindList:
			// Lists flatten to (ptr: i32, len: i32) pointing to linear memory
			// The list data must be written to component memory first
			list := p.List()
			if f.instance == nil || len(f.instance.coreInstances) == 0 {
				return nil, fmt.Errorf("no instance available for list memory allocation")
			}

			// Get the memory from the core module
			// Use the memory specified in canonical options, or default to "memory"
			coreModule := f.instance.coreInstances[0]
			mem := coreModule.Memory()
			if mem == nil {
				return nil, fmt.Errorf("core module has no memory for list data")
			}

			// Calculate element size and alignment based on element type
			// Detect from first element, default to s32/u32 if empty
			var elemSize uint32 = 4
			var alignment uint32 = 4
			if len(list) > 0 {
				elemSize = elementSizeForKind(list[0].Kind())
				alignment = alignmentForKind(list[0].Kind())
			}
			listLen := uint32(len(list))
			listSize := listLen * elemSize

			// Allocate memory using realloc per Canonical ABI spec
			var ptr uint32
			if listSize > 0 {
				if f.reallocFunc == nil {
					return nil, fmt.Errorf("list lowering requires realloc function")
				}
				// realloc(old_ptr, old_size, align, new_size) -> new_ptr
				results, err := f.reallocFunc.Call(ctx, 0, 0, uint64(alignment), uint64(listSize))
				if err != nil {
					return nil, fmt.Errorf("realloc for list failed: %w", err)
				}
				ptr = uint32(results[0])
			}
			// For empty lists (listSize == 0), ptr remains 0, which is valid

			// Write list elements to allocated memory using the helper function
			for j, elem := range list {
				offset := ptr + uint32(j)*elemSize
				if err := writeListElement(mem, offset, elem); err != nil {
					return nil, fmt.Errorf("failed to write list element %d: %w", j, err)
				}
			}

			// Pass (ptr, len) to core function
			coreParams = append(coreParams, uint64(ptr), uint64(listLen))
		case ValKindOwn:
			// own<T> argument: Create a new owned handle from the representation
			// The Val contains a representation value that we turn into a handle
			// When caller passes own<T>, ownership transfers to callee
			rep := p.Own()
			if f.instance == nil || f.instance.resourceTable == nil {
				return nil, fmt.Errorf("lower own param %d: no resource table available", i)
			}
			h := f.instance.resourceTable.New(rep, true)
			coreParams = append(coreParams, uint64(h.Index()))
		case ValKindBorrow:
			// borrow<T> argument: Create a borrowed handle from the representation
			// The Val contains a representation value that we turn into a handle
			// Borrowed handles must be dropped before return
			rep := p.Borrow()
			if f.instance == nil || f.instance.resourceTable == nil {
				return nil, fmt.Errorf("lower borrow param %d: no resource table available", i)
			}
			h := f.instance.resourceTable.New(rep, false) // own=false for borrowed
			// Track borrow in call context for return validation
			callCtx.IncrementBorrows()
			coreParams = append(coreParams, uint64(h.Index()))
		default:
			return nil, fmt.Errorf("unsupported parameter type: %s", p.Kind())
		}
	}

	// === END LOWERING PARAMS - may_leave = true ===
	if f.instance != nil {
		f.instance.SetMayLeave(true)
	}
	loweringComplete = true

	// Call the core function
	coreResults, err := f.coreFunc.Call(ctx, coreParams...)
	if err != nil {
		return nil, err
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
				result := []Val{ValOwn(handleVal)}
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
			result := []Val{ValOwn(0)}
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
				result := []Val{ValBorrow(handleVal)}
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
			result := []Val{ValBorrow(0)}
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
		if !resultTypeRef.IsPrimitive && f.component != nil && resultTypeRef.TypeIdx < uint32(len(f.component.Types)) {
			// Result is a defined type - look up the actual type definition
			typeDef := &f.component.Types[resultTypeRef.TypeIdx]
			if typeDef.Option != nil && len(coreResults) == 2 {
				// Option type: first result is discriminant, second is payload
				discriminant := coreResults[0]
				if discriminant == 0 {
					// None
					if err := callCtx.ValidateReturn(); err != nil {
						return nil, err
					}
					result := []Val{ValOption(nil)}
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
				result := []Val{ValOption(&payload)}
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
				result := []Val{ValRecord(rec)}
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
						result := []Val{ValResultOk(&payload)}
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
					result := []Val{ValResultOk(nil)}
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
					result := []Val{ValResultError(&errVal)}
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
				result := []Val{ValResultError(nil)}
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
				result := []Val{ValString(str)}
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
				result := []Val{val}
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
		results := make([]Val, len(coreResults))
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
	results := make([]Val, len(coreResults))
	for i, r := range coreResults {
		results[i] = ValS32(int32(r))
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
func (f *ExportedFunc) liftPrimitiveVal(coreVal uint64, typeRef ValTypeRef) Val {
	if typeRef.IsPrimitive {
		switch typeRef.Primitive {
		case 0x7f: // bool
			if coreVal != 0 {
				return ValBool(true)
			}
			return ValBool(false)
		case 0x7e: // s8
			return ValS8(int8(coreVal))
		case 0x7d: // u8
			return ValU8(uint8(coreVal))
		case 0x7c: // s16
			return ValS16(int16(coreVal))
		case 0x7b: // u16
			return ValU16(uint16(coreVal))
		case 0x7a: // s32
			return ValS32(int32(coreVal))
		case 0x79: // u32
			return ValU32(uint32(coreVal))
		case 0x78: // s64
			return ValS64(int64(coreVal))
		case 0x77: // u64
			return ValU64(coreVal)
		case 0x76: // f32
			return ValF32(math.Float32frombits(uint32(coreVal)))
		case 0x75: // f64
			return ValF64(math.Float64frombits(coreVal))
		case 0x74: // char
			return ValChar(rune(coreVal))
		}
	}
	// Default fallback to s32
	return ValS32(int32(coreVal))
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
func (f *ExportedFunc) liftRecord(recordDef *RecordTypeDef, coreResults []uint64) map[string]Val {
	rec := make(map[string]Val)

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
			rec[name] = ValS32(int32(coreResults[coreIdx]))
		}
		coreIdx++
	}

	return rec
}

// liftResolvedType lifts core values to a component Val using a resolved type.
// This is the type-resolver-driven path for lifting complex types.
func (f *ExportedFunc) liftResolvedType(resolvedType types.ValType, coreResults []uint64, subtask *Subtask, callCtx *CallContext) ([]Val, error) {
	switch t := resolvedType.(type) {
	case types.Record:
		if len(coreResults) < len(t.Fields) {
			return nil, fmt.Errorf("not enough core results for record: have %d, need %d", len(coreResults), len(t.Fields))
		}
		rec := make(map[string]Val)
		for i, field := range t.Fields {
			rec[field.Name] = f.liftResolvedPrimitiveVal(coreResults[i], field.Type)
		}
		if err := callCtx.ValidateReturn(); err != nil {
			return nil, err
		}
		result := []Val{ValRecord(rec)}
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
			result := []Val{ValOption(nil)}
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
		result := []Val{ValOption(&payload)}
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
				result := []Val{ValResultOk(&payload)}
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
			result := []Val{ValResultOk(nil)}
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
			result := []Val{ValResultError(&errVal)}
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
		result := []Val{ValResultError(nil)}
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
		result := []Val{ValString(str)}
		// Complete subtask before return
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
func (f *ExportedFunc) liftResolvedPrimitiveVal(coreVal uint64, valType types.ValType) Val {
	switch valType.(type) {
	case types.Bool:
		if coreVal != 0 {
			return ValBool(true)
		}
		return ValBool(false)
	case types.S8:
		return ValS8(int8(coreVal))
	case types.U8:
		return ValU8(uint8(coreVal))
	case types.S16:
		return ValS16(int16(coreVal))
	case types.U16:
		return ValU16(uint16(coreVal))
	case types.S32:
		return ValS32(int32(coreVal))
	case types.U32:
		return ValU32(uint32(coreVal))
	case types.S64:
		return ValS64(int64(coreVal))
	case types.U64:
		return ValU64(coreVal)
	case types.F32:
		return ValF32(math.Float32frombits(uint32(coreVal)))
	case types.F64:
		return ValF64(math.Float64frombits(coreVal))
	case types.Char:
		return ValChar(rune(coreVal))
	default:
		// Default fallback to s32
		return ValS32(int32(coreVal))
	}
}

// liftOwn transfers ownership of a resource out of the component.
// It removes the handle from the table and returns the representation value.
// Traps if the handle has active borrows (NumLends > 0).
// Traps if the handle is not owned (i.e., it's a borrowed handle).
func (f *ExportedFunc) liftOwn(handleIdx uint32, borrowScope *BorrowScope) (any, error) {
	if f.instance == nil || f.instance.resourceTable == nil {
		return nil, fmt.Errorf("lift_own: no resource table available")
	}

	// Construct handle from index - try to find the valid generation
	h := Handle(handleIdx)

	// Get the entry first to check ownership
	entry, err := f.instance.resourceTable.Get(h)
	if err != nil {
		// Try to find the entry by scanning generations
		for gen := uint32(1); gen < 1000; gen++ {
			h = MakeHandle(handleIdx, gen)
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
// It tracks the lend in the BorrowScope to prevent ownership transfer while borrowed.
func (f *ExportedFunc) liftBorrow(handleIdx uint32, borrowScope *BorrowScope) (any, error) {
	if f.instance == nil || f.instance.resourceTable == nil {
		return nil, fmt.Errorf("lift_borrow: no resource table available")
	}

	// Construct handle from index
	h := Handle(handleIdx)

	// Try to get the entry
	entry, err := f.instance.resourceTable.Get(h)
	if err != nil {
		// Try to find the entry by scanning generations
		for gen := uint32(1); gen < 1000; gen++ {
			h = MakeHandle(handleIdx, gen)
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
		i.resourceTable = NewResourceTable()
	}
	h := i.resourceTable.New(rep, true)
	return uint32(h), nil
}

// ResourceRep implements canon resource.rep.
// Extracts the representation from a handle without removing it.
func (i *Instance) ResourceRep(handleIdx uint32) (any, error) {
	if i.resourceTable == nil {
		return nil, fmt.Errorf("resource.rep: %w", ErrInvalidHandle)
	}
	h := Handle(handleIdx)
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
		return fmt.Errorf("resource.drop: %w", ErrInvalidHandle)
	}
	h := Handle(handleIdx)
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
func (i *Instance) SetCallContext(ctx *CallContext) {
	i.callContext = ctx
}

// CallContext returns the current call context.
func (i *Instance) CallContext() *CallContext {
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
func (i *Instance) AddValue(v Val) uint32 {
	if i.values == nil {
		i.values = make([]Val, 0)
		i.valuesConsumed = make([]bool, 0)
	}
	idx := uint32(len(i.values))
	i.values = append(i.values, v)
	i.valuesConsumed = append(i.valuesConsumed, false)
	return idx
}

// GetValue retrieves a value from the value index space.
func (i *Instance) GetValue(idx uint32) (Val, error) {
	if idx >= uint32(len(i.values)) {
		return Val{}, fmt.Errorf("value index %d out of range (have %d)", idx, len(i.values))
	}
	return i.values[idx], nil
}

// ConsumeValue retrieves and marks a value as consumed.
// Returns error if value already consumed or index out of range.
func (i *Instance) ConsumeValue(idx uint32) (Val, error) {
	if idx >= uint32(len(i.values)) {
		return Val{}, fmt.Errorf("value index %d out of range (have %d)", idx, len(i.values))
	}
	if i.valuesConsumed[idx] {
		return Val{}, fmt.Errorf("value %d already consumed", idx)
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
func liftEnum(discriminant uint64, enumType *EnumType) (Val, error) {
	idx := int(discriminant)
	if idx < 0 || idx >= len(enumType.Cases) {
		return Val{}, fmt.Errorf("invalid enum discriminant %d for type with %d cases",
			discriminant, len(enumType.Cases))
	}
	return ValEnum(enumType.Cases[idx]), nil
}

// liftFlags converts a bitvector to a flags Val.
func liftFlags(bitvector uint64, flagsType *FlagsType) (Val, error) {
	flags := make(map[string]bool)
	for i, name := range flagsType.Flags {
		if bitvector&(1<<i) != 0 {
			flags[name] = true
		}
	}
	return ValFlags(flags), nil
}

// liftVariant converts flat representation to a variant Val.
// The flat representation consists of a discriminant followed by the payload values.
// This is the inverse of lowerVariantToFlat in canon_lower.go.
func liftVariant(flat []uint64, variantType *VariantType) (Val, error) {
	if len(flat) < 1 {
		return Val{}, fmt.Errorf("variant requires at least discriminant")
	}

	disc := int(flat[0])
	if disc < 0 || disc >= len(variantType.Cases) {
		return Val{}, fmt.Errorf("invalid variant discriminant %d for type with %d cases",
			disc, len(variantType.Cases))
	}

	variantCase := variantType.Cases[disc]

	// If the case has no payload type, return variant with nil payload
	if variantCase.Type == nil {
		return ValVariant(variantCase.Name, nil), nil
	}

	// Case has a payload - we need to lift it from the remaining flat values
	if len(flat) < 2 {
		return Val{}, fmt.Errorf("variant case %q requires payload but none provided", variantCase.Name)
	}

	// Lift the payload based on its type
	payload, err := liftVariantPayload(flat[1], variantCase.Type)
	if err != nil {
		return Val{}, fmt.Errorf("lifting variant payload for case %q: %w", variantCase.Name, err)
	}

	return ValVariant(variantCase.Name, &payload), nil
}

// liftVariantPayload lifts a single payload value based on its PayloadType.
func liftVariantPayload(flatVal uint64, payloadType PayloadType) (Val, error) {
	// Handle PrimitiveType payloads
	if prim, ok := payloadType.(*PrimitiveType); ok {
		switch prim.Name {
		case "bool":
			return ValBool(flatVal != 0), nil
		case "s8":
			return ValS8(int8(flatVal)), nil
		case "u8":
			return ValU8(uint8(flatVal)), nil
		case "s16":
			return ValS16(int16(flatVal)), nil
		case "u16":
			return ValU16(uint16(flatVal)), nil
		case "s32":
			return ValS32(int32(flatVal)), nil
		case "u32":
			return ValU32(uint32(flatVal)), nil
		case "s64":
			return ValS64(int64(flatVal)), nil
		case "u64":
			return ValU64(flatVal), nil
		case "f32":
			return ValF32(math.Float32frombits(uint32(flatVal))), nil
		case "f64":
			return ValF64(math.Float64frombits(flatVal)), nil
		case "char":
			return ValChar(rune(flatVal)), nil
		default:
			// For unknown primitive types, default to s32
			return ValS32(int32(flatVal)), nil
		}
	}

	// For non-primitive types, default to s32 (simplified handling)
	// A full implementation would recursively handle composite types
	return ValS32(int32(flatVal)), nil
}

// elementSizeForKind returns the size in bytes for a ValKind per the Canonical ABI.
func elementSizeForKind(kind ValKind) uint32 {
	switch kind {
	case ValKindS8, ValKindU8, ValKindBool:
		return 1
	case ValKindS16, ValKindU16:
		return 2
	case ValKindS32, ValKindU32, ValKindF32, ValKindChar:
		return 4
	case ValKindS64, ValKindU64, ValKindF64:
		return 8
	default:
		return 4 // Default to 4 for unknown types
	}
}

// alignmentForKind returns the alignment in bytes for a ValKind per the Canonical ABI.
func alignmentForKind(kind ValKind) uint32 {
	switch kind {
	case ValKindS8, ValKindU8, ValKindBool:
		return 1
	case ValKindS16, ValKindU16:
		return 2
	case ValKindS32, ValKindU32, ValKindF32, ValKindChar:
		return 4
	case ValKindS64, ValKindU64, ValKindF64:
		return 8
	default:
		return 4 // Default to 4 for unknown types
	}
}

// sizeOfVal returns the size in bytes needed to store a Val in memory.
// For composite types, this returns an estimate suitable for result/variant lowering.
func sizeOfVal(v Val) uint32 {
	switch v.Kind() {
	case ValKindS8, ValKindU8, ValKindBool:
		return 1
	case ValKindS16, ValKindU16:
		return 2
	case ValKindS32, ValKindU32, ValKindF32, ValKindChar, ValKindOwn, ValKindBorrow:
		return 4
	case ValKindS64, ValKindU64, ValKindF64:
		return 8
	case ValKindString, ValKindList:
		return 8 // ptr + len
	case ValKindResult:
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
	case ValKindVariant:
		// discriminant (4 bytes) + payload
		_, payload := v.Variant()
		if payload != nil {
			return 4 + sizeOfVal(*payload)
		}
		return 4
	case ValKindOption:
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
func writeListElement(mem api.Memory, offset uint32, elem Val) error {
	switch elem.Kind() {
	case ValKindS8:
		if !mem.WriteByteAt(offset, byte(elem.S8())) {
			return fmt.Errorf("failed to write s8 at offset %d", offset)
		}
	case ValKindU8:
		if !mem.WriteByteAt(offset, elem.U8()) {
			return fmt.Errorf("failed to write u8 at offset %d", offset)
		}
	case ValKindBool:
		var b byte
		if elem.Bool() {
			b = 1
		}
		if !mem.WriteByteAt(offset, b) {
			return fmt.Errorf("failed to write bool at offset %d", offset)
		}
	case ValKindS16:
		if !mem.WriteUint16Le(offset, uint16(elem.S16())) {
			return fmt.Errorf("failed to write s16 at offset %d", offset)
		}
	case ValKindU16:
		if !mem.WriteUint16Le(offset, elem.U16()) {
			return fmt.Errorf("failed to write u16 at offset %d", offset)
		}
	case ValKindS32:
		if !mem.WriteUint32Le(offset, uint32(elem.S32())) {
			return fmt.Errorf("failed to write s32 at offset %d", offset)
		}
	case ValKindU32:
		if !mem.WriteUint32Le(offset, elem.U32()) {
			return fmt.Errorf("failed to write u32 at offset %d", offset)
		}
	case ValKindChar:
		if !mem.WriteUint32Le(offset, uint32(elem.Char())) {
			return fmt.Errorf("failed to write char at offset %d", offset)
		}
	case ValKindF32:
		bits := math.Float32bits(elem.F32())
		if !mem.WriteUint32Le(offset, bits) {
			return fmt.Errorf("failed to write f32 at offset %d", offset)
		}
	case ValKindS64:
		if !mem.WriteUint64Le(offset, uint64(elem.S64())) {
			return fmt.Errorf("failed to write s64 at offset %d", offset)
		}
	case ValKindU64:
		if !mem.WriteUint64Le(offset, elem.U64()) {
			return fmt.Errorf("failed to write u64 at offset %d", offset)
		}
	case ValKindF64:
		bits := math.Float64bits(elem.F64())
		if !mem.WriteUint64Le(offset, bits) {
			return fmt.Errorf("failed to write f64 at offset %d", offset)
		}
	default:
		return fmt.Errorf("unsupported list element type: %s", elem.Kind())
	}
	return nil
}
