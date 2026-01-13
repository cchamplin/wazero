// internal/component/instantiate.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Instantiate creates an Instance from a parsed Component.
// It instantiates all embedded core modules and wires up exports.
func Instantiate(ctx context.Context, rt wazero.Runtime, c *Component) (*Instance, error) {
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

		compiled, err := rt.CompileModule(ctx, c.CoreModuleData[i])
		if err != nil {
			return nil, fmt.Errorf("compile core module %d: %w", i, err)
		}

		modInst, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
		if err != nil {
			return nil, fmt.Errorf("instantiate core module %d: %w", i, err)
		}

		inst.coreInstances[i] = modInst
	}

	// Wire up exports based on canonical definitions
	for _, exp := range c.Exports {
		if exp.Kind != ExportKindFunc {
			continue
		}

		// Find the canonical definition for this export
		if exp.Idx >= uint32(len(c.Canonicals)) {
			continue
		}
		canon := &c.Canonicals[exp.Idx]

		if canon.Kind != CanonKindLift {
			continue
		}

		// Find the core function
		if len(inst.coreInstances) == 0 {
			continue
		}
		coreModule := inst.coreInstances[0]

		// Get the core function by index
		// Note: This is simplified; real impl needs to track function index space
		coreFuncs := coreModule.ExportedFunctionDefinitions()
		var coreFunc api.Function
		for name := range coreFuncs {
			// For now, find any exported function
			coreFunc = coreModule.ExportedFunction(name)
			break
		}

		if coreFunc == nil {
			continue
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
			name:      exp.Name,
			funcType:  funcType,
			coreFunc:  coreFunc,
			canonical: canon,
			component: c,
		}
	}

	return inst, nil
}
