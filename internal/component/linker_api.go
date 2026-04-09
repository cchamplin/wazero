// internal/component/linker_api.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// Compile-time checks that our types implement the api interfaces.
var (
	_ api.ComponentLinker          = (*ComponentLinkerWrapper)(nil)
	_ api.ComponentInstanceBuilder = (*ComponentInstanceBuilderWrapper)(nil)
	_ api.Component                = (*ComponentWrapper)(nil)
	_ api.Component                = (*ComponentInstanceWrapper)(nil)
	_ api.ComponentFunc            = (*ComponentFuncWrapper)(nil)
)

// ComponentLinkerWrapper wraps the internal ComponentLinker to implement api.ComponentLinker.
type ComponentLinkerWrapper struct {
	internalapi.WazeroOnlyType
	linker *ComponentLinker
}

// NewComponentLinkerWrapper creates a new wrapper that implements api.ComponentLinker.
// The runtime parameter should be a wazero.Runtime instance for core module instantiation.
func NewComponentLinkerWrapper(rt any) *ComponentLinkerWrapper {
	return &ComponentLinkerWrapper{
		linker: NewComponentLinker(rt),
	}
}

// DefineFunc defines a host function that can satisfy component imports.
// Direct pass-through under the wasmtime func_new model; the host
// has no type to declare.
func (l *ComponentLinkerWrapper) DefineFunc(namespace, name string, fn api.HostFunc) error {
	return l.linker.DefineFunc(namespace, name, HostFunc(fn))
}

// DefineInstance starts building an instance definition with multiple exports.
func (l *ComponentLinkerWrapper) DefineInstance(namespace string) api.ComponentInstanceBuilder {
	return &ComponentInstanceBuilderWrapper2{
		builder: l.linker.DefineInstance(namespace),
	}
}

// DefineResource defines a resource type with its destructor.
func (l *ComponentLinkerWrapper) DefineResource(namespace, name string, dtor func(rep uint32)) error {
	return l.linker.DefineResource(namespace, name, dtor)
}

// MergeFrom copies all definitions from a basic Linker into this ComponentLinkerWrapper.
// This allows WASI interfaces registered on a basic Linker to be used with
// a ComponentLinkerWrapper that has runtime integration for core module instantiation.
func (l *ComponentLinkerWrapper) MergeFrom(linker *Linker) {
	l.linker.MergeFrom(linker)
}

// SetRelaxedSemverMatching enables or disables relaxed semver matching.
func (l *ComponentLinkerWrapper) SetRelaxedSemverMatching(relaxed bool) {
	l.linker.SetRelaxedSemverMatching(relaxed)
}

// Instantiate creates a component instance with resolved imports.
func (l *ComponentLinkerWrapper) Instantiate(ctx context.Context, compiled api.CompiledComponent) (api.Component, error) {
	// Get the internal compiled component
	cc, ok := compiled.(*CompiledComponent)
	if !ok {
		return nil, fmt.Errorf("invalid compiled component type: expected *CompiledComponent")
	}

	// Instantiate using the internal ComponentLinker (which has runtime integration)
	inst, err := l.linker.Instantiate(ctx, cc)
	if err != nil {
		return nil, err
	}

	return &ComponentWrapper{instance: inst}, nil
}

// ComponentInstanceBuilderWrapper wraps InstanceBuilder to implement api.ComponentInstanceBuilder.
// NOTE: This is kept for backward compatibility with code using the basic Linker.
type ComponentInstanceBuilderWrapper struct {
	internalapi.WazeroOnlyType
	builder *InstanceBuilder
}

// Func adds a function export to the instance being built.
// Direct pass-through under the wasmtime func_new model.
func (b *ComponentInstanceBuilderWrapper) Func(name string, fn api.HostFunc) api.ComponentInstanceBuilder {
	b.builder.Func(name, HostFunc(fn))
	return b
}

// Resource adds a resource type definition to the instance.
func (b *ComponentInstanceBuilderWrapper) Resource(name string, dtor func(rep uint32)) api.ComponentInstanceBuilder {
	b.builder.Resource(name, dtor)
	return b
}

// SkipValidation disables type checking for this instance definition.
func (b *ComponentInstanceBuilderWrapper) SkipValidation() api.ComponentInstanceBuilder {
	b.builder.SkipValidation()
	return b
}

// Build finalizes the instance definition.
func (b *ComponentInstanceBuilderWrapper) Build() error {
	return b.builder.Build()
}

// ComponentInstanceBuilderWrapper2 wraps ComponentInstanceBuilder to implement api.ComponentInstanceBuilder.
// This is the wrapper used by ComponentLinkerWrapper which has runtime integration.
type ComponentInstanceBuilderWrapper2 struct {
	internalapi.WazeroOnlyType
	builder *ComponentInstanceBuilder
}

// Func adds a function export to the instance being built.
// Direct pass-through under the wasmtime func_new model.
func (b *ComponentInstanceBuilderWrapper2) Func(name string, fn api.HostFunc) api.ComponentInstanceBuilder {
	b.builder.Func(name, HostFunc(fn))
	return b
}

// Resource adds a resource type definition to the instance.
func (b *ComponentInstanceBuilderWrapper2) Resource(name string, dtor func(rep uint32)) api.ComponentInstanceBuilder {
	b.builder.Resource(name, dtor)
	return b
}

// SkipValidation disables type checking for this instance definition.
func (b *ComponentInstanceBuilderWrapper2) SkipValidation() api.ComponentInstanceBuilder {
	b.builder.SkipValidation()
	return b
}

// Build finalizes the instance definition.
func (b *ComponentInstanceBuilderWrapper2) Build() error {
	return b.builder.Build()
}

// ComponentWrapper wraps Instance to implement api.Component.
type ComponentWrapper struct {
	internalapi.WazeroOnlyType
	instance *Instance
}

// ExportedFunction returns the exported function with the given name.
func (c *ComponentWrapper) ExportedFunction(name string) api.ComponentFunc {
	f := c.instance.GetExportedFunc(name)
	if f == nil {
		return nil
	}
	return &ComponentFuncWrapper{fn: f}
}

// ExportedInstance returns a nested exported instance.
func (c *ComponentWrapper) ExportedInstance(name string) api.Component {
	if c.instance == nil {
		return nil
	}
	nested := c.instance.GetExportedInstance(name)
	if nested == nil {
		return nil
	}
	return &ComponentInstanceWrapper{instance: nested}
}

// Close releases resources associated with this component instance.
// It closes all core module instances created during instantiation.
// Safe to call multiple times; subsequent calls are no-ops.
func (c *ComponentWrapper) Close(ctx context.Context) error {
	if c.instance == nil {
		return nil
	}

	var firstErr error
	for _, mod := range c.instance.coreInstances {
		if mod == nil {
			continue
		}
		if err := mod.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.instance = nil
	return firstErr
}

// ComponentFuncWrapper wraps ExportedFunc to implement api.ComponentFunc.
type ComponentFuncWrapper struct {
	internalapi.WazeroOnlyType
	fn *ExportedFunc
}

// Call invokes the function with the given arguments.
func (f *ComponentFuncWrapper) Call(ctx context.Context, params ...any) ([]any, error) {
	// Convert params to Val
	vals := make([]types.Val, len(params))
	for i, p := range params {
		vals[i] = anyToVal(p)
	}

	results, err := f.fn.Call(ctx, vals...)
	if err != nil {
		return nil, err
	}

	// Convert results back to any
	out := make([]any, len(results))
	for i, r := range results {
		out[i] = valToAny(r)
	}

	return out, nil
}

// ComponentInstanceWrapper wraps an exported component instance for API access.
type ComponentInstanceWrapper struct {
	internalapi.WazeroOnlyType
	instance *Instance
}

// ExportedFunction returns an exported function by name.
func (w *ComponentInstanceWrapper) ExportedFunction(name string) api.ComponentFunc {
	if w.instance == nil {
		return nil
	}
	fn, ok := w.instance.exports[name]
	if !ok || fn == nil {
		return nil
	}
	return &ComponentFuncWrapper{fn: fn}
}

// ExportedInstance returns a nested exported instance by name.
func (w *ComponentInstanceWrapper) ExportedInstance(name string) api.Component {
	if w.instance == nil {
		return nil
	}
	nested := w.instance.GetExportedInstance(name)
	if nested == nil {
		return nil
	}
	return &ComponentInstanceWrapper{instance: nested}
}

// Close releases resources associated with this component instance.
func (w *ComponentInstanceWrapper) Close(ctx context.Context) error {
	// No cleanup needed for nested instances
	return nil
}

// anyToVal converts a Go value to a component Val.
// Supports primitives, maps (records), slices (lists/tuples), and Val directly.
func anyToVal(p any) types.Val {
	switch v := p.(type) {
	case types.Val:
		// Already a Val, use directly
		return v
	case bool:
		return types.ValBool(v)
	case int8:
		return types.ValS8(v)
	case uint8:
		return types.ValU8(v)
	case int16:
		return types.ValS16(v)
	case uint16:
		return types.ValU16(v)
	case int32:
		return types.ValS32(v)
	case uint32:
		return types.ValU32(v)
	case int64:
		return types.ValS64(v)
	case uint64:
		return types.ValU64(v)
	case float32:
		return types.ValF32(v)
	case float64:
		return types.ValF64(v)
	case string:
		return types.ValString(v)
	// Note: rune is an alias for int32, so we can't have a separate case.
	// Users who want to pass a char should use types.ValChar() directly.
	case map[string]any:
		// Record: convert map[string]any to ValRecord
		fields := make(map[string]types.Val)
		for k, fv := range v {
			fields[k] = anyToVal(fv)
		}
		return types.ValRecord(fields)
	case map[string]types.Val:
		// Record with Val values directly
		return types.ValRecord(v)
	case []any:
		// List: convert []any to ValList
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = anyToVal(e)
		}
		return types.ValList(elements)
	case []types.Val:
		// List with Val values directly
		return types.ValList(v)
	case []uint8:
		// List<u8>: convert typed byte slice to ValList of U8
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValU8(e)
		}
		return types.ValList(elements)
	case []int8:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValS8(e)
		}
		return types.ValList(elements)
	case []uint16:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValU16(e)
		}
		return types.ValList(elements)
	case []int16:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValS16(e)
		}
		return types.ValList(elements)
	case []uint32:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValU32(e)
		}
		return types.ValList(elements)
	case []int32:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValS32(e)
		}
		return types.ValList(elements)
	case []uint64:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValU64(e)
		}
		return types.ValList(elements)
	case []int64:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValS64(e)
		}
		return types.ValList(elements)
	case []float32:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValF32(e)
		}
		return types.ValList(elements)
	case []float64:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValF64(e)
		}
		return types.ValList(elements)
	case []string:
		elements := make([]types.Val, len(v))
		for i, e := range v {
			elements[i] = types.ValString(e)
		}
		return types.ValList(elements)
	case nil:
		// nil represents option::none
		return types.ValOption(nil)
	default:
		// Unsupported type - return zero Val
		return types.Val{}
	}
}

// valToAny converts a component Val to a Go value.
// Records become map[string]any, lists become []any, etc.
func valToAny(r types.Val) any {
	switch r.Kind() {
	case types.ValKindBool:
		return r.Bool()
	case types.ValKindS8:
		return r.S8()
	case types.ValKindU8:
		return r.U8()
	case types.ValKindS16:
		return r.S16()
	case types.ValKindU16:
		return r.U16()
	case types.ValKindS32:
		return r.S32()
	case types.ValKindU32:
		return r.U32()
	case types.ValKindS64:
		return r.S64()
	case types.ValKindU64:
		return r.U64()
	case types.ValKindF32:
		return r.F32()
	case types.ValKindF64:
		return r.F64()
	case types.ValKindChar:
		return r.Char()
	case types.ValKindString:
		return r.StringVal()
	case types.ValKindRecord:
		// Convert record to map[string]any
		rec := r.Record()
		out := make(map[string]any)
		for k, v := range rec {
			out[k] = valToAny(v)
		}
		return out
	case types.ValKindList:
		// Convert list to []any
		list := r.List()
		out := make([]any, len(list))
		for i, v := range list {
			out[i] = valToAny(v)
		}
		return out
	case types.ValKindTuple:
		// Convert tuple to []any
		tuple := r.Tuple()
		out := make([]any, len(tuple))
		for i, v := range tuple {
			out[i] = valToAny(v)
		}
		return out
	case types.ValKindOption:
		// Convert option to *any (nil for None)
		opt := r.Option()
		if opt == nil {
			return nil
		}
		v := valToAny(*opt)
		return v
	case types.ValKindResult:
		// Convert result to a struct-like map
		isOk, okVal, errVal := r.Result()
		result := map[string]any{"ok": isOk}
		if isOk && okVal != nil {
			result["value"] = valToAny(*okVal)
		} else if !isOk && errVal != nil {
			result["error"] = valToAny(*errVal)
		}
		return result
	case types.ValKindVariant:
		// Convert variant to a map with case name and payload
		caseName, payload := r.Variant()
		result := map[string]any{"case": caseName}
		if payload != nil {
			result["payload"] = valToAny(*payload)
		}
		return result
	case types.ValKindEnum:
		return r.Enum()
	case types.ValKindFlags:
		return r.Flags()
	case types.ValKindOwn:
		return r.Own()
	case types.ValKindBorrow:
		return r.Borrow()
	default:
		return nil
	}
}
