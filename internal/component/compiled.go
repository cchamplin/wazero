// internal/component/compiled.go
package component

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// Compile-time check that CompiledComponent implements api.CompiledComponent.
var _ api.CompiledComponent = (*CompiledComponent)(nil)

// CompiledModuleCloser is an interface for compiled modules that can be closed.
// This avoids importing wazero package directly to prevent import cycles.
type CompiledModuleCloser interface {
	api.Closer
}

// CompiledComponent wraps a parsed Component with pre-compiled core modules.
type CompiledComponent struct {
	internalapi.WazeroOnlyType

	component       *Component
	compiledModules []CompiledModuleCloser
	runtime         any // Stores the runtime for later use (avoids import cycle)
}

// NewCompiledComponent creates a new CompiledComponent.
func NewCompiledComponent(c *Component, compiledModules []CompiledModuleCloser, rt any) *CompiledComponent {
	return &CompiledComponent{
		component:       c,
		compiledModules: compiledModules,
		runtime:         rt,
	}
}

// Imports returns all imports required by this component.
func (c *CompiledComponent) Imports() []api.ComponentImport {
	result := make([]api.ComponentImport, len(c.component.Imports))
	for i, imp := range c.component.Imports {
		result[i] = api.ComponentImport{
			Name: imp.Name,
			Kind: convertImportKind(imp.ExternDesc),
		}
	}
	return result
}

// Exports returns all exports provided by this component.
func (c *CompiledComponent) Exports() []api.ComponentExport {
	result := make([]api.ComponentExport, len(c.component.Exports))
	for i, exp := range c.component.Exports {
		result[i] = api.ComponentExport{
			Name: exp.Name,
			Kind: convertExportKind(exp.Kind),
		}
	}
	return result
}

// Close releases resources associated with this compiled component.
func (c *CompiledComponent) Close(ctx context.Context) error {
	var firstErr error
	for _, cm := range c.compiledModules {
		if cm != nil {
			if err := cm.Close(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Internal returns the parsed component for linker use.
func (c *CompiledComponent) Internal() *Component {
	return c.component
}

// CompiledModules returns the pre-compiled core modules.
func (c *CompiledComponent) CompiledModules() []CompiledModuleCloser {
	return c.compiledModules
}

// Runtime returns the runtime used to compile the modules.
// Returns as any to avoid import cycles.
func (c *CompiledComponent) Runtime() any {
	return c.runtime
}

func convertImportKind(desc ImportExternDesc) api.ComponentExportKind {
	switch desc.Kind {
	case ImportExternDescFunc:
		return api.ComponentExportKindFunc
	case ImportExternDescValue:
		return api.ComponentExportKindValue
	case ImportExternDescType:
		return api.ComponentExportKindType
	case ImportExternDescInstance:
		return api.ComponentExportKindInstance
	case ImportExternDescComponent:
		// Component imports are semantically similar to instance imports
		// in the public API, as both represent composite entities.
		return api.ComponentExportKindInstance
	case ImportExternDescCoreModule:
		// Core module imports map to instance kind since modules are
		// instantiated entities in the component model.
		return api.ComponentExportKindInstance
	default:
		return api.ComponentExportKindFunc
	}
}

func convertExportKind(kind ExportKind) api.ComponentExportKind {
	switch kind {
	case ExportKindFunc:
		return api.ComponentExportKindFunc
	case ExportKindValue:
		return api.ComponentExportKindValue
	case ExportKindType:
		return api.ComponentExportKindType
	case ExportKindComponent:
		// Component exports are semantically similar to instance exports
		// in the public API, as both represent composite entities.
		return api.ComponentExportKindInstance
	case ExportKindInstance:
		return api.ComponentExportKindInstance
	default:
		return api.ComponentExportKindFunc
	}
}
