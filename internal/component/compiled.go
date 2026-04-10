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

// CoreModuleInstantiator is an interface for instantiating core modules within a component.
// The wazero.Runtime implements this interface to allow the component linker to
// instantiate core modules with proper import resolution.
type CoreModuleInstantiator interface {
	// InstantiateCoreModule instantiates a compiled core module with the given context.
	// The compiled parameter should be a CompiledModuleCloser (actually *compiledModule).
	// Returns the instantiated module or an error.
	InstantiateCoreModule(ctx context.Context, compiled CompiledModuleCloser) (api.Module, error)
}

// HostModuleExport represents a single function export in a host module.
type HostModuleExport struct {
	Name        string
	ParamTypes  []api.ValueType
	ResultTypes []api.ValueType
	Func        api.GoModuleFunc

	// For function forwarding (aliased functions that should preserve the source's moduleCtxPtr):
	// When SourceModule is set, this export forwards to SourceModule.ExportedFunction(SourceName)
	// and the function reference will use the source module's opaquePtr, not the host module's.
	// This is used for component model inline instances that alias functions from other core instances.
	SourceModule api.Module
	SourceName   string
}

// HostModuleTableExport represents a table export that shares a table from another module.
type HostModuleTableExport struct {
	Name         string     // Export name
	SourceModule api.Module // Module containing the source table
	SourceName   string     // Export name in the source module
}

// HostModuleMemoryExport represents a memory export that shares memory from another module.
type HostModuleMemoryExport struct {
	Name         string     // Export name
	SourceModule api.Module // Module containing the source memory
	SourceName   string     // Export name in the source module
}

// HostModuleInstantiator is an interface for creating host modules dynamically.
// This is used by the component linker to create synthetic modules that wrap
// component-level imports for use by core modules.
type HostModuleInstantiator interface {
	// InstantiateHostModule creates and instantiates a new host module with the given
	// name and function exports. Returns the instantiated module or an error.
	InstantiateHostModule(ctx context.Context, moduleName string, exports []HostModuleExport) (api.Module, error)

	// InstantiateHostModuleWithTables creates and instantiates a new host module with
	// function exports and shared table exports. This is used when an inline instance
	// exports both functions (from canon operations) and tables (aliased from other modules).
	InstantiateHostModuleWithTables(ctx context.Context, moduleName string, exports []HostModuleExport, tableExports []HostModuleTableExport) (api.Module, error)

	// InstantiateHostModuleWithResources creates and instantiates a new host module with
	// function exports, shared table exports, and shared memory exports. This is used when
	// an inline instance exports functions, tables, and/or memory (aliased from other modules).
	InstantiateHostModuleWithResources(ctx context.Context, moduleName string, exports []HostModuleExport, tableExports []HostModuleTableExport, memoryExports []HostModuleMemoryExport) (api.Module, error)
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
		if imp.ExternDesc.Kind == ImportExternDescFunc {
			result[i].FuncType = c.resolveFuncType(imp.ExternDesc.TypeIdx)
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
		if exp.Kind == ExportKindFunc {
			result[i].FuncType = c.resolveExportFuncType(&exp)
		}
	}
	return result
}

// resolveFuncType resolves a type index into a ComponentFuncType.
// Returns nil if the type cannot be resolved (e.g., alias cycle, out of range,
// or the resolved type is not a function type).
func (c *CompiledComponent) resolveFuncType(typeIdx uint32) *api.ComponentFuncType {
	td, _, err := c.component.ResolveTypeDef(typeIdx)
	if err != nil || td.Kind != TypeDefKindFunc {
		return nil
	}
	idx := int(td.Func)
	if idx >= len(c.component.Types.Funcs) {
		return nil
	}
	return api.NewComponentFuncType(&c.component.Types.Funcs[idx], c.component.Types)
}

// resolveExportFuncType resolves the function type for a function export.
// It first tries the export's TypeIdx annotation, then falls back to the
// canonical lift's TypeIdx.
func (c *CompiledComponent) resolveExportFuncType(exp *Export) *api.ComponentFuncType {
	// Try the export's own type annotation first.
	if exp.TypeIdx != nil {
		if ft := c.resolveFuncType(*exp.TypeIdx); ft != nil {
			return ft
		}
	}
	// Fall back to the canonical lift's type.
	if canonIdx, ok := c.component.FuncIdxToCanonical[exp.Idx]; ok {
		if int(canonIdx) < len(c.component.Canonicals) {
			canon := &c.component.Canonicals[canonIdx]
			if canon.Kind == CanonKindLift {
				return c.resolveFuncType(canon.TypeIdx)
			}
		}
	}
	return nil
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
	case ExportKindTable:
		return api.ComponentExportKindTable
	case ExportKindMemory:
		return api.ComponentExportKindMemory
	case ExportKindGlobal:
		return api.ComponentExportKindGlobal
	default:
		return api.ComponentExportKindFunc
	}
}
