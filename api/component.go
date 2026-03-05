// api/component.go
package api

import (
	"context"

	"github.com/tetratelabs/wazero/internal/internalapi"
)

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
type ComponentFunc interface {
	// Call invokes the function with the given arguments.
	// Arguments and results use the dynamic Val type from internal/component.
	Call(ctx context.Context, params ...any) ([]any, error)

	internalapi.WazeroOnly
}

// ComponentLinker configures imports before instantiating a component.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type ComponentLinker interface {
	// DefineFunc defines a host function that can satisfy component imports.
	DefineFunc(namespace, name string, fn any) error

	// DefineInstance starts building an instance definition with multiple exports.
	DefineInstance(namespace string) ComponentInstanceBuilder

	// DefineResource defines a resource type with its destructor.
	DefineResource(namespace, name string, dtor func(rep uint32)) error

	// Instantiate creates a component instance with resolved imports.
	Instantiate(ctx context.Context, compiled CompiledComponent) (Component, error)

	// SetRelaxedSemverMatching enables or disables relaxed semver matching.
	// When enabled, pre-1.0 versions (0.x.y) match any patch version within
	// the same minor version (e.g., 0.2.0 matches 0.2.3).
	SetRelaxedSemverMatching(relaxed bool)

	internalapi.WazeroOnly
}

// ComponentInstanceBuilder builds an instance definition with multiple exports.
type ComponentInstanceBuilder interface {
	// Func adds a function export to the instance being built.
	Func(name string, fn any) ComponentInstanceBuilder

	// Resource adds a resource type definition to the instance.
	Resource(name string, dtor func(rep uint32)) ComponentInstanceBuilder

	// Build finalizes the instance definition.
	Build() error

	internalapi.WazeroOnly
}
