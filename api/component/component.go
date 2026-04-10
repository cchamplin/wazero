// Package component provides types for the WebAssembly Component Model.
//
// The Component Model extends WebAssembly with higher-level types (records,
// variants, lists, options, results, resources, etc.) and a composition
// mechanism that allows components to import and export typed interfaces.
//
// This package exports the dynamic value type [Val] and host function
// signature [HostFunc] needed when defining host functions for component
// imports and when calling component-exported functions via
// [api.ComponentFunc.Call] / [api.ComponentFunc.CallAndPostReturn].
// Use the Val constructors (ValS32, ValString, ValRecord, etc.) to build
// arguments and the accessor methods (S32, StringVal, Record, etc.) to
// read results.
//
// # Defining host functions
//
// Use [HostFunc] and [Val] when providing host implementations via
// [api.ComponentLinker.DefineFunc] or [api.ComponentInstanceBuilder.Func]:
//
//	linker := rt.NewComponentLinker()
//	err := linker.DefineInstance("my:app/math").
//		Func("add", component.HostFunc(func(ctx context.Context, _ *component.TypeFunc, args []component.Val) ([]component.Val, error) {
//			a, b := args[0].S32(), args[1].S32()
//			return []component.Val{component.ValS32(a + b)}, nil
//		})).
//		Build()
//
// # Val types
//
// Val is a dynamically-typed value that can represent any component model
// type. Use the constructor functions (ValBool, ValS32, ValString,
// ValRecord, ValList, ValOption, ValResultOk, etc.) to create values,
// and the accessor methods (Bool, S32, StringVal, Record, List, Option,
// Result, etc.) to read them.
//
// See the [examples/component-host-functions] example for a complete
// demonstration.
package component

import (
	"context"

	internalcomponent "github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// HostFunc is a host function that can be called from a component.
// It receives a context, the component-declared function type
// (supplied by the runtime at call time), and a slice of Val
// arguments, and returns a slice of Val results or an error.
//
// Mirrors wasmtime's func_new dynamic host path
// (debug-vendored/wasmtime/.../runtime/component/linker.rs:665-675).
type HostFunc = internalcomponent.HostFunc

// Val represents a dynamically-typed component model value.
type Val = types.Val

// TypeFunc is the component-level function type handed to [HostFunc]
// callbacks at call time. It mirrors wasmtime's cx.types[ty] lookup
// (debug-vendored/wasmtime/.../runtime/component/func/host.rs:640-694).
type TypeFunc = types.TypeFunc

// ValKind identifies the type of a Val.
type ValKind = types.ValKind

// ValKind constants for all component model value types.
const (
	ValKindBool    = types.ValKindBool
	ValKindS8      = types.ValKindS8
	ValKindU8      = types.ValKindU8
	ValKindS16     = types.ValKindS16
	ValKindU16     = types.ValKindU16
	ValKindS32     = types.ValKindS32
	ValKindU32     = types.ValKindU32
	ValKindS64     = types.ValKindS64
	ValKindU64     = types.ValKindU64
	ValKindF32     = types.ValKindF32
	ValKindF64     = types.ValKindF64
	ValKindChar    = types.ValKindChar
	ValKindString  = types.ValKindString
	ValKindList    = types.ValKindList
	ValKindRecord  = types.ValKindRecord
	ValKindTuple   = types.ValKindTuple
	ValKindVariant = types.ValKindVariant
	ValKindEnum    = types.ValKindEnum
	ValKindOption  = types.ValKindOption
	ValKindResult  = types.ValKindResult
	ValKindFlags   = types.ValKindFlags
	ValKindOwn     = types.ValKindOwn
	ValKindBorrow  = types.ValKindBorrow
)

// Val constructors create Val instances of specific types.
var (
	ValBool        = types.ValBool
	ValS8          = types.ValS8
	ValU8          = types.ValU8
	ValS16         = types.ValS16
	ValU16         = types.ValU16
	ValS32         = types.ValS32
	ValU32         = types.ValU32
	ValS64         = types.ValS64
	ValU64         = types.ValU64
	ValF32         = types.ValF32
	ValF64         = types.ValF64
	ValChar        = types.ValChar
	ValString      = types.ValString
	ValRecord      = types.ValRecord
	ValList        = types.ValList
	ValTuple       = types.ValTuple
	ValVariant     = types.ValVariant
	ValEnum        = types.ValEnum
	ValOption      = types.ValOption
	ValResultOk    = types.ValResultOk
	ValResultError = types.ValResultError
	ValFlags       = types.ValFlags
	ValOwn         = types.ValOwn
	ValBorrow      = types.ValBorrow
)

// ResourceTable manages resource handles with generation counting.
// It implements the Component Model's handle table semantics.
type ResourceTable = runtime.Table

// NewResourceTable creates an empty resource table.
var NewResourceTable = runtime.NewTable

// Handle is a 64-bit resource handle: upper 32 bits = generation, lower 32 = index.
// Generation counting prevents use-after-free when slots are reused.
type Handle = runtime.Handle

// MakeHandle constructs a handle from an index and generation.
var MakeHandle = runtime.MakeHandle

// WithResourceTable returns a new context with the given ResourceTable stored.
// The table can later be retrieved using ResourceTableFromContext on the
// internal side, or passed to component functions via the context.
func WithResourceTable(ctx context.Context, table *ResourceTable) context.Context {
	return internalcomponent.WithResourceTable(ctx, table)
}

// ResourceType identifies a resource type with pointer identity.
// Two ResourceTypes are equal iff they refer to the same underlying
// resource declaration from the same component instantiation.
//
// Spec: definitions.py:351-361.
type ResourceType struct {
	inner *runtime.ResourceType
}

// Equal reports whether r and other refer to the same underlying resource
// type declaration from the same component instantiation. Equality is
// pointer identity per the spec's `is` check (definitions.py:1345).
func (r ResourceType) Equal(other ResourceType) bool {
	return r.inner == other.inner
}

// Inner returns the underlying internal *runtime.ResourceType.
// This is intended for use by internal packages that need the raw pointer.
func (r ResourceType) Inner() *runtime.ResourceType {
	return r.inner
}

// NewResourceType wraps an internal *runtime.ResourceType for public use.
func NewResourceType(rt *runtime.ResourceType) ResourceType {
	return ResourceType{inner: rt}
}

// ResourceHandle is a read-only public view of a resource handle entry.
// It wraps an internal ResourceHandleEntry without duplicating fields.
type ResourceHandle struct {
	inner *runtime.ResourceHandleEntry
}

// Type returns the ResourceType that this handle belongs to.
func (h ResourceHandle) Type() ResourceType {
	return ResourceType{inner: h.inner.RT}
}

// Owned reports whether this is an owning handle (true) or a borrow (false).
func (h ResourceHandle) Owned() bool {
	return h.inner.Own
}

// Rep returns the underlying representation value (uint32) for this handle.
func (h ResourceHandle) Rep() uint32 {
	return h.inner.Rep
}

// NumLends returns the number of active borrows from this handle.
func (h ResourceHandle) NumLends() uint32 {
	return h.inner.NumLends
}

// NewResourceHandle wraps an internal *runtime.ResourceHandleEntry for public use.
func NewResourceHandle(entry *runtime.ResourceHandleEntry) ResourceHandle {
	return ResourceHandle{inner: entry}
}

// ResourceNew creates a new owning resource handle in the given table for the
// specified resource type and representation value. It returns the Handle
// that can be used to retrieve, lend, or drop the resource.
//
// This mirrors canon resource.new (CanonicalABI.md:3604-3609).
func ResourceNew(table *ResourceTable, rt ResourceType, rep uint32) (Handle, error) {
	return table.NewResourceHandle(rep, true, rt.inner)
}

// ResourceRep retrieves the representation value (uint32) for the given
// handle. Returns an error if the handle is invalid.
//
// This mirrors canon resource.rep (CanonicalABI.md:3610-3615).
func ResourceRep(table *ResourceTable, h Handle) (uint32, error) {
	return table.Rep(h)
}

// ResourceDrop removes a resource handle from the table and returns the
// entry. For owned handles with active borrows, this returns an error.
//
// The caller is responsible for invoking the destructor (if any) on owned
// handles and for decrementing borrow counts on borrowed handles.
//
// This mirrors canon resource.drop (CanonicalABI.md:3634-3646).
func ResourceDrop(table *ResourceTable, h Handle) (ResourceHandle, error) {
	entry, err := table.Remove(h)
	if err != nil {
		return ResourceHandle{}, err
	}
	return ResourceHandle{inner: entry}, nil
}

// ResourceGet retrieves the resource handle entry for the given handle
// without removing it from the table. Returns an error if the handle is
// invalid or does not refer to a resource entry.
func ResourceGet(table *ResourceTable, h Handle) (ResourceHandle, error) {
	entry, err := table.GetResourceHandle(h)
	if err != nil {
		return ResourceHandle{}, err
	}
	return ResourceHandle{inner: entry}, nil
}
