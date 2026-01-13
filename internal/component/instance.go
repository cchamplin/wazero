// internal/component/instance.go

package component

import (
	"context"
	"fmt"
	"sort"

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
	component *Component // reference to parent component for type lookups
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
// Supports primitive types and records (flattened to their fields).
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
	// Convert component Vals to core wasm values
	// Records are flattened into their constituent fields
	var coreParams []uint64
	for _, p := range params {
		switch p.Kind() {
		case ValKindS32:
			coreParams = append(coreParams, uint64(uint32(p.S32())))
		case ValKindU32:
			coreParams = append(coreParams, uint64(p.U32()))
		case ValKindS64:
			coreParams = append(coreParams, uint64(p.S64()))
		case ValKindU64:
			coreParams = append(coreParams, p.U64())
		case ValKindRecord:
			// Flatten record fields in alphabetical order (component model spec)
			rec := p.Record()
			// Get field names and sort them for deterministic order
			fieldNames := make([]string, 0, len(rec))
			for name := range rec {
				fieldNames = append(fieldNames, name)
			}
			sort.Strings(fieldNames)

			for _, name := range fieldNames {
				field := rec[name]
				switch field.Kind() {
				case ValKindS32:
					coreParams = append(coreParams, uint64(uint32(field.S32())))
				case ValKindU32:
					coreParams = append(coreParams, uint64(field.U32()))
				case ValKindS64:
					coreParams = append(coreParams, uint64(field.S64()))
				case ValKindU64:
					coreParams = append(coreParams, field.U64())
				default:
					return nil, fmt.Errorf("unsupported record field type: %s", field.Kind())
				}
			}
		case ValKindOption:
			// Flatten option to (discriminant: i32, payload: i32)
			// discriminant: 0 = None, 1 = Some
			opt := p.Option()
			if opt == nil {
				// None: discriminant = 0, payload = 0 (unused)
				coreParams = append(coreParams, 0, 0)
			} else {
				// Some: discriminant = 1, payload = value
				switch opt.Kind() {
				case ValKindS32:
					coreParams = append(coreParams, 1, uint64(uint32(opt.S32())))
				case ValKindU32:
					coreParams = append(coreParams, 1, uint64(opt.U32()))
				case ValKindS64:
					coreParams = append(coreParams, 1, uint64(opt.S64()))
				case ValKindU64:
					coreParams = append(coreParams, 1, opt.U64())
				default:
					return nil, fmt.Errorf("unsupported option payload type: %s", opt.Kind())
				}
			}
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
	// Check if the result type is a record or option by examining the function type
	if f.funcType != nil && len(f.funcType.Results) == 1 {
		resultType := f.funcType.Results[0].ValType
		if !resultType.IsPrimitive && f.component != nil && resultType.TypeIdx < uint32(len(f.component.Types)) {
			// Result is a defined type - look up the actual type definition
			typeDef := &f.component.Types[resultType.TypeIdx]
			if typeDef.Option != nil && len(coreResults) == 2 {
				// Option type: first result is discriminant, second is payload
				discriminant := coreResults[0]
				if discriminant == 0 {
					// None
					return []Val{ValOption(nil)}, nil
				}
				// Some: currently assumes s32 payload
				// TODO: In a full implementation, the payload type should be
				// determined from the option type definition.
				payload := ValS32(int32(coreResults[1]))
				return []Val{ValOption(&payload)}, nil
			} else if typeDef.Record != nil && len(coreResults) == 2 {
				// Record type: For records, the core function returns multiple flat values
				// that need to be reconstructed into a record
				// TODO: Currently hardcodes field names "x" and "y". In a full implementation,
				// field names should be looked up from the record type definition using
				// resultType.TypeIdx to resolve the actual type and its field names.
				// TODO: Currently assumes all record fields are s32. In a full implementation,
				// field types should be determined from the type definition.
				rec := map[string]Val{
					"x": ValS32(int32(coreResults[0])),
					"y": ValS32(int32(coreResults[1])),
				}
				return []Val{ValRecord(rec)}, nil
			}
		}
	}

	// Default: treat results as primitives (s32)
	// TODO: Currently assumes all primitive results are s32. In a full implementation,
	// the result type should be determined from f.funcType.Results.
	results := make([]Val, len(coreResults))
	for i, r := range coreResults {
		results[i] = ValS32(int32(r))
	}

	return results, nil
}
