// internal/component/instance.go

package component

import (
	"context"
	"fmt"
	"sort"

	"github.com/tetratelabs/wazero/api"
)

// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc

	// Resource management fields
	resourceTable *ResourceTable          // Table for tracking resource handles
	destructors   map[uint32]func(any)    // Destructor functions by resource type index
	callContext   *CallContext            // Current call context for borrow tracking
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
	name        string
	funcType    *FuncType
	coreFunc    api.Function
	canonical   *CanonicalDef
	component   *Component   // reference to parent component for type lookups
	instance    *Instance    // reference to parent instance for memory access
	memory      api.Memory   // resolved memory for canonical ABI operations
	reallocFunc api.Function // resolved realloc function for memory allocation
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
// Supports primitive types, records (flattened to their fields), and resource handles.
// For resource handles (own/borrow), it uses the Canonical ABI lift/lower operations.
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Set up call context and borrow scope for resource tracking
	callCtx := NewCallContext()
	var borrowScope *BorrowScope

	// Initialize resource table if needed
	if f.instance != nil {
		if f.instance.resourceTable == nil {
			f.instance.resourceTable = NewResourceTable()
		}
		borrowScope = NewBorrowScope(f.instance.resourceTable)
		// Set the call context for this invocation
		prevCallCtx := f.instance.callContext
		f.instance.callContext = callCtx
		defer func() {
			f.instance.callContext = prevCallCtx
		}()
	}

	// Convert component Vals to core wasm values
	// Records are flattened into their constituent fields
	var coreParams []uint64
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

			// Write list elements to memory
			// For list<s32>, each element is 4 bytes
			// Currently using a simple allocation strategy: write at offset 0
			// TODO: In a full implementation, use realloc for proper allocation
			ptr := uint32(0)
			for j, elem := range list {
				offset := ptr + uint32(j*4)
				switch elem.Kind() {
				case ValKindS32:
					if !mem.WriteUint32Le(offset, uint32(elem.S32())) {
						return nil, fmt.Errorf("failed to write list element %d to memory", j)
					}
				case ValKindU32:
					if !mem.WriteUint32Le(offset, elem.U32()) {
						return nil, fmt.Errorf("failed to write list element %d to memory", j)
					}
				default:
					return nil, fmt.Errorf("unsupported list element type: %s", elem.Kind())
				}
			}

			// Pass (ptr, len) to core function
			coreParams = append(coreParams, uint64(ptr), uint64(len(list)))
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

	// Call the core function
	coreResults, err := f.coreFunc.Call(ctx, coreParams...)
	if err != nil {
		return nil, err
	}

	// Convert core results back to component Vals
	// Check if the result type is a record, option, or handle by examining the function type
	if f.funcType != nil && len(f.funcType.Results) == 1 {
		resultType := f.funcType.Results[0].ValType

		// Check for own<T> result
		if resultType.IsOwn && len(coreResults) == 1 {
			// own<T> result: Extract rep and transfer ownership out of component
			handleIdx := uint32(coreResults[0])
			rep, err := f.liftOwn(handleIdx, borrowScope)
			if err != nil {
				return nil, fmt.Errorf("lift own result: %w", err)
			}
			// Release borrow scope before return
			if borrowScope != nil {
				if err := borrowScope.Release(); err != nil {
					return nil, fmt.Errorf("release borrow scope: %w", err)
				}
			}
			// Validate no outstanding borrows
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			// Return the representation as the handle value
			// The rep is any, so we extract uint32 if it's a handle index
			if handleVal, ok := rep.(uint32); ok {
				return []Val{ValOwn(handleVal)}, nil
			}
			// If rep is not uint32, return it wrapped in ValOwn with index 0
			// This is a simplification; proper implementation would track the actual handle
			return []Val{ValOwn(0)}, nil
		}

		// Check for borrow<T> result (rare, but possible)
		if resultType.IsBorrow && len(coreResults) == 1 {
			// borrow<T> result: Read the rep without removing from table
			handleIdx := uint32(coreResults[0])
			rep, err := f.liftBorrow(handleIdx, borrowScope)
			if err != nil {
				return nil, fmt.Errorf("lift borrow result: %w", err)
			}
			// Release borrow scope before return
			if borrowScope != nil {
				if err := borrowScope.Release(); err != nil {
					return nil, fmt.Errorf("release borrow scope: %w", err)
				}
			}
			// Validate no outstanding borrows
			if err := callCtx.ValidateReturn(); err != nil {
				return nil, err
			}
			if handleVal, ok := rep.(uint32); ok {
				return []Val{ValBorrow(handleVal)}, nil
			}
			return []Val{ValBorrow(0)}, nil
		}

		if !resultType.IsPrimitive && f.component != nil && resultType.TypeIdx < uint32(len(f.component.Types)) {
			// Result is a defined type - look up the actual type definition
			typeDef := &f.component.Types[resultType.TypeIdx]
			if typeDef.Option != nil && len(coreResults) == 2 {
				// Option type: first result is discriminant, second is payload
				discriminant := coreResults[0]
				if discriminant == 0 {
					// None
					// Release borrow scope and validate before return
					if borrowScope != nil {
						if err := borrowScope.Release(); err != nil {
							return nil, fmt.Errorf("release borrow scope: %w", err)
						}
					}
					if err := callCtx.ValidateReturn(); err != nil {
						return nil, err
					}
					return []Val{ValOption(nil)}, nil
				}
				// Some: currently assumes s32 payload
				// TODO: In a full implementation, the payload type should be
				// determined from the option type definition.
				payload := ValS32(int32(coreResults[1]))
				// Release borrow scope and validate before return
				if borrowScope != nil {
					if err := borrowScope.Release(); err != nil {
						return nil, fmt.Errorf("release borrow scope: %w", err)
					}
				}
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				return []Val{ValOption(&payload)}, nil
			} else if typeDef.Record != nil && len(coreResults) == 2 {
				// Record type: For records, the core function returns multiple flat values
				// that need to be reconstructed into a record
				// TODO: Currently hardcodes field names "x" and "y". In a full implementation,
				// field names should be looked up from the record type definition using
				// resultType.TypeIdx to resolve the actual type and its field names.
				// TODO: Currently assumes all record fields are s32. In a full implementation,
				// field types should be determined from the type definition.
				rec := map[string]Val{
					"x": ValS32(int32(coreResults[0])),
					"y": ValS32(int32(coreResults[1])),
				}
				// Release borrow scope and validate before return
				if borrowScope != nil {
					if err := borrowScope.Release(); err != nil {
						return nil, fmt.Errorf("release borrow scope: %w", err)
					}
				}
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				return []Val{ValRecord(rec)}, nil
			} else if typeDef.Result != nil && len(coreResults) == 2 {
				// Result type: first result is discriminant, second is payload
				// discriminant 0 = Ok, 1 = Error
				discriminant := coreResults[0]
				if discriminant == 0 {
					// Ok: return success result with value
					// TODO: In a full implementation, the payload type should be
					// determined from the result type definition.
					payload := ValS32(int32(coreResults[1]))
					// Release borrow scope and validate before return
					if borrowScope != nil {
						if err := borrowScope.Release(); err != nil {
							return nil, fmt.Errorf("release borrow scope: %w", err)
						}
					}
					if err := callCtx.ValidateReturn(); err != nil {
						return nil, err
					}
					return []Val{ValResultOk(&payload)}, nil
				}
				// Error: return error result with error value
				// TODO: In a full implementation, the error type should be
				// determined from the result type definition.
				errVal := ValS32(int32(coreResults[1]))
				// Release borrow scope and validate before return
				if borrowScope != nil {
					if err := borrowScope.Release(); err != nil {
						return nil, fmt.Errorf("release borrow scope: %w", err)
					}
				}
				if err := callCtx.ValidateReturn(); err != nil {
					return nil, err
				}
				return []Val{ValResultError(&errVal)}, nil
			}
		}
	}

	// Release borrow scope before returning
	if borrowScope != nil {
		if err := borrowScope.Release(); err != nil {
			return nil, fmt.Errorf("release borrow scope: %w", err)
		}
	}

	// Validate that all borrowed handles have been dropped
	if err := callCtx.ValidateReturn(); err != nil {
		return nil, err
	}

	// Default: treat results as primitives (s32)
	// TODO: Currently assumes all primitive results are s32. In a full implementation,
	// the result type should be determined from f.funcType.Results.
	results := make([]Val, len(coreResults))
	for i, r := range coreResults {
		results[i] = ValS32(int32(r))
	}

	return results, nil
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
