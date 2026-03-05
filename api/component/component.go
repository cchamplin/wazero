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
)

// HostFunc is a host function that can be called from a component.
// It receives a context and a slice of Val arguments, and returns
// a slice of Val results or an error.
type HostFunc = internalcomponent.HostFunc

// Val represents a dynamically-typed component model value.
type Val = internalcomponent.Val

// ValKind identifies the type of a Val.
type ValKind = internalcomponent.ValKind

// ValKind constants for all component model value types.
const (
	ValKindBool    = internalcomponent.ValKindBool
	ValKindS8      = internalcomponent.ValKindS8
	ValKindU8      = internalcomponent.ValKindU8
	ValKindS16     = internalcomponent.ValKindS16
	ValKindU16     = internalcomponent.ValKindU16
	ValKindS32     = internalcomponent.ValKindS32
	ValKindU32     = internalcomponent.ValKindU32
	ValKindS64     = internalcomponent.ValKindS64
	ValKindU64     = internalcomponent.ValKindU64
	ValKindF32     = internalcomponent.ValKindF32
	ValKindF64     = internalcomponent.ValKindF64
	ValKindChar    = internalcomponent.ValKindChar
	ValKindString  = internalcomponent.ValKindString
	ValKindList    = internalcomponent.ValKindList
	ValKindRecord  = internalcomponent.ValKindRecord
	ValKindTuple   = internalcomponent.ValKindTuple
	ValKindVariant = internalcomponent.ValKindVariant
	ValKindEnum    = internalcomponent.ValKindEnum
	ValKindOption  = internalcomponent.ValKindOption
	ValKindResult  = internalcomponent.ValKindResult
	ValKindFlags   = internalcomponent.ValKindFlags
	ValKindOwn     = internalcomponent.ValKindOwn
	ValKindBorrow  = internalcomponent.ValKindBorrow
)

// Val constructors create Val instances of specific types.
var (
	ValBool        = internalcomponent.ValBool
	ValS8          = internalcomponent.ValS8
	ValU8          = internalcomponent.ValU8
	ValS16         = internalcomponent.ValS16
	ValU16         = internalcomponent.ValU16
	ValS32         = internalcomponent.ValS32
	ValU32         = internalcomponent.ValU32
	ValS64         = internalcomponent.ValS64
	ValU64         = internalcomponent.ValU64
	ValF32         = internalcomponent.ValF32
	ValF64         = internalcomponent.ValF64
	ValChar        = internalcomponent.ValChar
	ValString      = internalcomponent.ValString
	ValRecord      = internalcomponent.ValRecord
	ValList        = internalcomponent.ValList
	ValTuple       = internalcomponent.ValTuple
	ValVariant     = internalcomponent.ValVariant
	ValEnum        = internalcomponent.ValEnum
	ValOption      = internalcomponent.ValOption
	ValResultOk    = internalcomponent.ValResultOk
	ValResultError = internalcomponent.ValResultError
	ValFlags       = internalcomponent.ValFlags
	ValOwn         = internalcomponent.ValOwn
	ValBorrow      = internalcomponent.ValBorrow
)

// ResourceTable manages resource handles with generation counting.
// It implements the Component Model's handle table semantics.
type ResourceTable = internalcomponent.ResourceTable

// NewResourceTable creates an empty resource table.
var NewResourceTable = internalcomponent.NewResourceTable

// WithResourceTable returns a new context with the given ResourceTable stored.
// The table can later be retrieved using ResourceTableFromContext on the
// internal side, or passed to component functions via the context.
func WithResourceTable(ctx context.Context, table *ResourceTable) context.Context {
	return internalcomponent.WithResourceTable(ctx, table)
}
