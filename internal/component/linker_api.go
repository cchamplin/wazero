// internal/component/linker_api.go

package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
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
func (l *ComponentLinkerWrapper) DefineFunc(namespace, name string, fn any) error {
	// Convert the Go function to our internal HostFunc format.
	// For now, we accept HostFunc directly; a fuller implementation
	// would introspect fn's signature and create a wrapper.
	if hf, ok := fn.(HostFunc); ok {
		return l.linker.DefineFunc(namespace, name, hf)
	}
	// For non-HostFunc, wrap it (simplified - just stores a placeholder)
	wrapper := func(ctx context.Context, args []Val) ([]Val, error) {
		// Placeholder - full implementation would call fn with converted args
		return nil, nil
	}
	return l.linker.DefineFunc(namespace, name, wrapper)
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
func (b *ComponentInstanceBuilderWrapper) Func(name string, fn any) api.ComponentInstanceBuilder {
	// Convert fn to HostFunc
	if hf, ok := fn.(HostFunc); ok {
		b.builder.Func(name, nil, hf)
	} else {
		// Wrap non-HostFunc (simplified)
		wrapper := func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}
		b.builder.Func(name, nil, wrapper)
	}
	return b
}

// Resource adds a resource type definition to the instance.
func (b *ComponentInstanceBuilderWrapper) Resource(name string, dtor func(rep uint32)) api.ComponentInstanceBuilder {
	b.builder.Resource(name, dtor)
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
func (b *ComponentInstanceBuilderWrapper2) Func(name string, fn any) api.ComponentInstanceBuilder {
	// Convert fn to HostFunc
	if hf, ok := fn.(HostFunc); ok {
		b.builder.Func(name, hf)
	} else {
		// Wrap non-HostFunc (simplified)
		wrapper := func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}
		b.builder.Func(name, wrapper)
	}
	return b
}

// Resource adds a resource type definition to the instance.
func (b *ComponentInstanceBuilderWrapper2) Resource(name string, dtor func(rep uint32)) api.ComponentInstanceBuilder {
	b.builder.Resource(name, dtor)
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
	vals := make([]Val, len(params))
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
func anyToVal(p any) Val {
	switch v := p.(type) {
	case Val:
		// Already a Val, use directly
		return v
	case bool:
		return ValBool(v)
	case int8:
		return ValS8(v)
	case uint8:
		return ValU8(v)
	case int16:
		return ValS16(v)
	case uint16:
		return ValU16(v)
	case int32:
		return ValS32(v)
	case uint32:
		return ValU32(v)
	case int64:
		return ValS64(v)
	case uint64:
		return ValU64(v)
	case float32:
		return ValF32(v)
	case float64:
		return ValF64(v)
	case string:
		return ValString(v)
	// Note: rune is an alias for int32, so we can't have a separate case.
	// Users who want to pass a char should use component.ValChar() directly.
	case map[string]any:
		// Record: convert map[string]any to ValRecord
		fields := make(map[string]Val)
		for k, fv := range v {
			fields[k] = anyToVal(fv)
		}
		return ValRecord(fields)
	case map[string]Val:
		// Record with Val values directly
		return ValRecord(v)
	case []any:
		// List: convert []any to ValList
		elements := make([]Val, len(v))
		for i, e := range v {
			elements[i] = anyToVal(e)
		}
		return ValList(elements)
	case []Val:
		// List with Val values directly
		return ValList(v)
	default:
		// Unsupported type - return zero Val
		return Val{}
	}
}

// valToAny converts a component Val to a Go value.
// Records become map[string]any, lists become []any, etc.
func valToAny(r Val) any {
	switch r.Kind() {
	case ValKindBool:
		return r.Bool()
	case ValKindS8:
		return r.S8()
	case ValKindU8:
		return r.U8()
	case ValKindS16:
		return r.S16()
	case ValKindU16:
		return r.U16()
	case ValKindS32:
		return r.S32()
	case ValKindU32:
		return r.U32()
	case ValKindS64:
		return r.S64()
	case ValKindU64:
		return r.U64()
	case ValKindF32:
		return r.F32()
	case ValKindF64:
		return r.F64()
	case ValKindChar:
		return r.Char()
	case ValKindString:
		return r.StringVal()
	case ValKindRecord:
		// Convert record to map[string]any
		rec := r.Record()
		out := make(map[string]any)
		for k, v := range rec {
			out[k] = valToAny(v)
		}
		return out
	case ValKindList:
		// Convert list to []any
		list := r.List()
		out := make([]any, len(list))
		for i, v := range list {
			out[i] = valToAny(v)
		}
		return out
	case ValKindTuple:
		// Convert tuple to []any
		tuple := r.Tuple()
		out := make([]any, len(tuple))
		for i, v := range tuple {
			out[i] = valToAny(v)
		}
		return out
	case ValKindOption:
		// Convert option to *any (nil for None)
		opt := r.Option()
		if opt == nil {
			return nil
		}
		v := valToAny(*opt)
		return v
	case ValKindResult:
		// Convert result to a struct-like map
		isOk, okVal, errVal := r.Result()
		result := map[string]any{"ok": isOk}
		if isOk && okVal != nil {
			result["value"] = valToAny(*okVal)
		} else if !isOk && errVal != nil {
			result["error"] = valToAny(*errVal)
		}
		return result
	case ValKindVariant:
		// Convert variant to a map with case name and payload
		caseName, payload := r.Variant()
		result := map[string]any{"case": caseName}
		if payload != nil {
			result["payload"] = valToAny(*payload)
		}
		return result
	case ValKindEnum:
		return r.Enum()
	case ValKindFlags:
		return r.Flags()
	case ValKindOwn:
		return r.Own()
	case ValKindBorrow:
		return r.Borrow()
	default:
		return nil
	}
}
