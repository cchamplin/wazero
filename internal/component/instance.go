// internal/component/instance.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// Instance represents an instantiated component.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc
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
	name      string
	funcType  *FuncType
	coreFunc  api.Function
	canonical *CanonicalDef
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
// For Phase 1, this only supports primitive types (especially s32).
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Phase 1: Primitive flat ABI only
	// Convert component Vals to core wasm values
	coreParams := make([]uint64, len(params))
	for i, p := range params {
		switch p.Kind() {
		case ValKindS32:
			coreParams[i] = uint64(uint32(p.S32()))
		case ValKindU32:
			coreParams[i] = uint64(p.U32())
		case ValKindS64:
			coreParams[i] = uint64(p.S64())
		case ValKindU64:
			coreParams[i] = p.U64()
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
	// Phase 1: Assume single s32 result
	results := make([]Val, len(coreResults))
	for i, r := range coreResults {
		results[i] = ValS32(int32(r))
	}

	return results, nil
}
