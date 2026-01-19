// internal/component/instantiate.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// ModuleInstantiator is an interface for instantiating modules.
// This avoids importing wazero package directly to prevent import cycles.
type ModuleInstantiator interface {
	// InstantiateModuleFromData compiles and instantiates a module from raw binary data.
	InstantiateModuleFromData(ctx context.Context, data []byte) (api.Module, error)
}

// RuntimeInstantiator is an interface that matches wazero.Runtime's module methods.
// This allows tests to wrap a runtime without importing wazero in production code.
type RuntimeInstantiator interface {
	Instantiate(ctx context.Context, binary []byte) (api.Module, error)
}

// runtimeAdapter wraps a RuntimeInstantiator to implement ModuleInstantiator.
type runtimeAdapter struct {
	rt RuntimeInstantiator
}

// NewRuntimeInstantiator creates a ModuleInstantiator from a RuntimeInstantiator.
// This is useful for tests that need to pass a wazero.Runtime to Instantiate.
func NewRuntimeInstantiator(rt RuntimeInstantiator) ModuleInstantiator {
	return &runtimeAdapter{rt: rt}
}

// InstantiateModuleFromData implements ModuleInstantiator.
func (r *runtimeAdapter) InstantiateModuleFromData(ctx context.Context, data []byte) (api.Module, error) {
	return r.rt.Instantiate(ctx, data)
}

// Instantiate creates an Instance from a parsed Component.
// It instantiates all embedded core modules and wires up exports.
func Instantiate(ctx context.Context, instantiator ModuleInstantiator, c *Component) (*Instance, error) {
	inst := &Instance{
		component:     c,
		coreInstances: make([]api.Module, len(c.CoreModules)),
		exports:       make(map[string]*ExportedFunc),
	}

	// Instantiate each core module
	for i := range c.CoreModules {
		if i >= len(c.CoreModuleData) {
			return nil, fmt.Errorf("missing module data for core module %d", i)
		}

		modInst, err := instantiator.InstantiateModuleFromData(ctx, c.CoreModuleData[i])
		if err != nil {
			return nil, fmt.Errorf("instantiate core module %d: %w", i, err)
		}

		inst.coreInstances[i] = modInst
	}

	// Build core function and memory index spaces from aliases
	funcSpace := make(map[uint32]string) // core func idx -> export name
	memSpace := make(map[uint32]string)  // core memory idx -> export name
	for _, alias := range c.Aliases {
		if alias.Kind == AliasKindCoreExport {
			switch alias.CoreSort {
			case CoreSortFunc:
				funcSpace[alias.Idx] = alias.ExportName
			case CoreSortMemory:
				memSpace[alias.Idx] = alias.ExportName
			}
		}
	}

	// Wire up exports based on canonical definitions
	for _, exp := range c.Exports {
		if exp.Kind != ExportKindFunc {
			continue
		}

		// exp.Idx is the component function index, not the canonical array index.
		// We need to look up the canonical index using the FuncIdxToCanonical map.
		canonIdx, ok := c.FuncIdxToCanonical[exp.Idx]
		if !ok {
			continue
		}
		if canonIdx >= uint32(len(c.Canonicals)) {
			continue
		}
		canon := &c.Canonicals[canonIdx]

		if canon.Kind != CanonKindLift {
			continue
		}

		// Find the core function
		if len(inst.coreInstances) == 0 {
			continue
		}
		coreModule := inst.coreInstances[0]

		// Resolve the core function by index using the function space
		var coreFunc api.Function
		if exportName, ok := funcSpace[canon.CoreFuncIdx]; ok {
			coreFunc = coreModule.ExportedFunction(exportName)
		} else {
			// Fallback: try to find any exported function (for simple components)
			coreFuncs := coreModule.ExportedFunctionDefinitions()
			for name := range coreFuncs {
				coreFunc = coreModule.ExportedFunction(name)
				break
			}
		}

		if coreFunc == nil {
			continue
		}

		// Resolve memory from canonical options
		var memory api.Memory
		if canon.Options.MemoryIdx != nil {
			if exportName, ok := memSpace[*canon.Options.MemoryIdx]; ok {
				memory = coreModule.ExportedMemory(exportName)
			} else {
				memory = coreModule.Memory()
			}
		}

		// Resolve realloc function from canonical options
		var reallocFunc api.Function
		if canon.Options.ReallocIdx != nil {
			if exportName, ok := funcSpace[*canon.Options.ReallocIdx]; ok {
				reallocFunc = coreModule.ExportedFunction(exportName)
			}
		}

		// Find the function type
		var funcType *FuncType
		if canon.TypeIdx < uint32(len(c.Types)) {
			td := &c.Types[canon.TypeIdx]
			if td.Kind == TypeDefKindFunc {
				funcType = td.Func
			}
		}

		inst.exports[exp.Name] = &ExportedFunc{
			name:        exp.Name,
			funcType:    funcType,
			coreFunc:    coreFunc,
			canonical:   canon,
			component:   c,
			instance:    inst,
			memory:      memory,
			reallocFunc: reallocFunc,
		}
	}

	return inst, nil
}
