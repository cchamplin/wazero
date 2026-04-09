// Package component provides types for the WebAssembly Component Model.
//
// The Component Model extends WebAssembly with higher-level types (records,
// variants, lists, options, results, resources, etc.) and a composition
// mechanism that allows components to import and export typed interfaces.
//
// This package exports the dynamic value type [Val] and host function
// signature [HostFunc] needed when defining host functions for component
// imports. For calling component-exported functions, Go primitives (int32,
// string, map[string]any, []any, etc.) can be passed directly to
// [api.ComponentFunc.Call] and are converted automatically.
//
// # Defining host functions
//
// Use [HostFunc] and [Val] when providing host implementations via
// [api.ComponentLinker.DefineFunc] or [api.ComponentInstanceBuilder.Func]:
//
//	linker := rt.NewComponentLinker()
//	err := linker.DefineInstance("my:app/math").
//		Func("add", component.HostFunc(func(ctx context.Context, args []component.Val) ([]component.Val, error) {
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

// WithResourceTable returns a new context with the given ResourceTable stored.
// The table can later be retrieved using ResourceTableFromContext on the
// internal side, or passed to component functions via the context.
func WithResourceTable(ctx context.Context, table *ResourceTable) context.Context {
	return internalcomponent.WithResourceTable(ctx, table)
}
