// This file defines the public interfaces for the WebAssembly Component Model.
//
// The component model lifecycle is:
//  1. Compile a component binary with [wazero.Runtime.CompileComponent]
//  2. Optionally inspect imports/exports on the [CompiledComponent]
//  3. Create a [ComponentLinker] with [wazero.Runtime.NewComponentLinker]
//  4. Define host functions and resources on the linker
//  5. Instantiate with [ComponentLinker.Instantiate] to get a [Component]
//  6. Call exported functions via [Component.ExportedFunction]
//
// For components with no imports, use [wazero.Runtime.InstantiateComponent]
// as a shortcut that skips steps 3-4.

package api

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// HostFunc is the canonical host function callback for component
// imports. Mirrors wasmtime's func_new dynamic host path
// (debug-vendored/wasmtime/.../runtime/component/linker.rs:665-675).
//
// fnType is the component's declared import type, supplied by the
// runtime at call time. The host has no type to declare at
// registration; the component's import IS the source of truth. Most
// callers ignore fnType with `_`.
//
// See [github.com/tetratelabs/wazero/api/component.HostFunc] for the
// re-exported alias used by host modules.
type HostFunc = func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error)

// ComponentExportKind classifies component exports.
type ComponentExportKind byte

const (
	ComponentExportKindFunc     ComponentExportKind = 0x00
	ComponentExportKindValue    ComponentExportKind = 0x01
	ComponentExportKindType     ComponentExportKind = 0x02
	ComponentExportKindInstance ComponentExportKind = 0x04
	ComponentExportKindTable    ComponentExportKind = 0x05
	ComponentExportKindMemory   ComponentExportKind = 0x06
	ComponentExportKindGlobal   ComponentExportKind = 0x07
)

// ComponentImport describes an import required by a component.
type ComponentImport struct {
	Name string
	Kind ComponentExportKind
}

// ComponentExport describes an export provided by a component.
type ComponentExport struct {
	Name string
	Kind ComponentExportKind
}

// CompiledComponent is a parsed and pre-compiled component.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type CompiledComponent interface {
	// Imports returns all imports required by this component.
	Imports() []ComponentImport

	// Exports returns all exports provided by this component.
	Exports() []ComponentExport

	// Close releases resources associated with this compiled component.
	Close(context.Context) error

	internalapi.WazeroOnly
}

// Component is an instantiated component ready for execution.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type Component interface {
	// ExportedFunction returns the exported function with the given name,
	// or nil if not found.
	ExportedFunction(name string) ComponentFunc

	// ExportedInstance returns a nested exported instance, or nil if not found.
	ExportedInstance(name string) Component

	// Close releases resources associated with this component instance.
	Close(context.Context) error

	internalapi.WazeroOnly
}

// ComponentFunc is an exported function from an instantiated component.
//
// See [examples/component-basic] for a simple call example and
// [examples/component-types] for complex type handling.
type ComponentFunc interface {
	// Call invokes the function with the given arguments and returns results.
	// Does NOT run post-return — caller must call PostReturn after reading results.
	// Panics if Call is invoked again before PostReturn.
	//
	// Arguments and results use [types.Val] — the component model's
	// dynamically-typed value. Use the Val constructors from the
	// api/component package (ValS32, ValString, ValRecord, etc.) to
	// build arguments, and the accessor methods (S32, StringVal, Record,
	// etc.) to read results.
	Call(ctx context.Context, params ...types.Val) ([]types.Val, error)

	// PostReturn runs the post-return cleanup function. Must be called after Call.
	// For functions without a post-return phase this is a no-op.
	PostReturn(ctx context.Context) error

	// CallAndPostReturn is a convenience that calls Call + PostReturn in one shot.
	// This is the recommended default for most callers. The two-phase
	// protocol (Call + PostReturn) is for advanced use cases where the
	// caller needs to read results before cleanup.
	CallAndPostReturn(ctx context.Context, params ...types.Val) ([]types.Val, error)

	internalapi.WazeroOnly
}

// ComponentLinker configures imports before instantiating a component.
//
// Use this to define host functions, instances, and resources that satisfy
// a component's imports. For WASI Preview 2 support, use
// [imports/wasip2.MergeInto] to register all WASI P2 interfaces at once.
//
// See [examples/component-host-functions] and [examples/component-wasip2]
// for usage examples.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type ComponentLinker interface {
	// DefineFunc defines a host function that can satisfy a component import.
	// fn is the canonical typed [HostFunc]. The component-declared import
	// type is supplied to fn as its second argument at call time; the host
	// has no type to declare. Mirrors wasmtime LinkerInstance::func_new
	// (linker.rs:665-675).
	DefineFunc(namespace, name string, fn HostFunc) error

	// DefineInstance starts building an instance definition with multiple
	// exports. This is the primary way to provide host implementations for
	// component imports that are organized as interfaces (e.g., "my:app/math").
	//
	//	linker.DefineInstance("my:app/math").
	//		Func("add", myAddFunc).
	//		Func("multiply", myMulFunc).
	//		Build()
	DefineInstance(namespace string) ComponentInstanceBuilder

	// DefineResource defines a resource type with its destructor.
	// The destructor is called when the resource handle is dropped.
	DefineResource(namespace, name string, dtor func(rep uint32)) error

	// Instantiate creates a component instance with all imports resolved.
	// Returns an error if any required import is not defined.
	Instantiate(ctx context.Context, compiled CompiledComponent) (Component, error)

	// SetRelaxedSemverMatching enables or disables relaxed semver matching.
	// When enabled, pre-1.0 versions (0.x.y) match any patch version within
	// the same minor version (e.g., 0.2.0 matches 0.2.3).
	SetRelaxedSemverMatching(relaxed bool)

	internalapi.WazeroOnly
}

// ComponentInstanceBuilder builds an instance definition with multiple exports.
type ComponentInstanceBuilder interface {
	// Func adds a function export to the instance being built. fn is
	// the canonical typed [HostFunc]; the host has no type to declare
	// (the component's import is the source of truth).
	Func(name string, fn HostFunc) ComponentInstanceBuilder

	// Resource adds a resource type definition to the instance.
	Resource(name string, dtor func(rep uint32)) ComponentInstanceBuilder

	// SkipValidation disables type checking for this instance definition.
	// Use this when the type checker's index mapping may be misaligned
	// for components with many type aliases across multiple instances.
	SkipValidation() ComponentInstanceBuilder

	// Build finalizes the instance definition.
	Build() error

	internalapi.WazeroOnly
}
