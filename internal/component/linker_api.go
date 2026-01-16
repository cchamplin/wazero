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
	_ api.ComponentLinker         = (*ComponentLinkerWrapper)(nil)
	_ api.ComponentInstanceBuilder = (*ComponentInstanceBuilderWrapper)(nil)
	_ api.Component               = (*ComponentWrapper)(nil)
	_ api.ComponentFunc           = (*ComponentFuncWrapper)(nil)
)

// ComponentLinkerWrapper wraps the internal Linker to implement api.ComponentLinker.
type ComponentLinkerWrapper struct {
	internalapi.WazeroOnlyType
	linker *Linker
}

// NewComponentLinker creates a new ComponentLinker that implements api.ComponentLinker.
func NewComponentLinker() *ComponentLinkerWrapper {
	return &ComponentLinkerWrapper{
		linker: NewLinker(),
	}
}

// DefineFunc defines a host function that can satisfy component imports.
func (l *ComponentLinkerWrapper) DefineFunc(namespace, name string, fn any) error {
	// Convert the Go function to our internal HostFunc format.
	// For now, we accept HostFunc directly; a fuller implementation
	// would introspect fn's signature and create a wrapper.
	if hf, ok := fn.(HostFunc); ok {
		return l.linker.DefineFunc(namespace, name, nil, hf)
	}
	// For non-HostFunc, wrap it (simplified - just stores a placeholder)
	wrapper := func(ctx context.Context, args []Val) ([]Val, error) {
		// Placeholder - full implementation would call fn with converted args
		return nil, nil
	}
	return l.linker.DefineFunc(namespace, name, nil, wrapper)
}

// DefineInstance starts building an instance definition with multiple exports.
func (l *ComponentLinkerWrapper) DefineInstance(namespace string) api.ComponentInstanceBuilder {
	return &ComponentInstanceBuilderWrapper{
		builder: l.linker.DefineInstance(namespace),
	}
}

// DefineResource defines a resource type with its destructor.
func (l *ComponentLinkerWrapper) DefineResource(namespace, name string, dtor func(rep uint32)) error {
	return l.linker.DefineResource(namespace, name, dtor)
}

// Instantiate creates a component instance with resolved imports.
func (l *ComponentLinkerWrapper) Instantiate(ctx context.Context, compiled api.CompiledComponent) (api.Component, error) {
	// Get the internal compiled component
	cc, ok := compiled.(*CompiledComponent)
	if !ok {
		return nil, fmt.Errorf("invalid compiled component type: expected *CompiledComponent")
	}

	// Instantiate using the internal linker
	inst, err := l.linker.Instantiate(ctx, cc.Internal())
	if err != nil {
		return nil, err
	}

	return &ComponentWrapper{instance: inst}, nil
}

// ComponentInstanceBuilderWrapper wraps InstanceBuilder to implement api.ComponentInstanceBuilder.
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
	// Not yet implemented - return nil for now
	return nil
}

// Close releases resources associated with this component instance.
func (c *ComponentWrapper) Close(ctx context.Context) error {
	// No cleanup needed for now
	return nil
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
		switch v := p.(type) {
		case int32:
			vals[i] = ValS32(v)
		case uint32:
			vals[i] = ValU32(v)
		case int64:
			vals[i] = ValS64(v)
		case uint64:
			vals[i] = ValU64(v)
		default:
			// Unsupported type - leave as zero
		}
	}

	results, err := f.fn.Call(ctx, vals...)
	if err != nil {
		return nil, err
	}

	// Convert results back to any
	out := make([]any, len(results))
	for i, r := range results {
		switch r.Kind() {
		case ValKindS32:
			out[i] = r.S32()
		case ValKindU32:
			out[i] = r.U32()
		case ValKindS64:
			out[i] = r.S64()
		case ValKindU64:
			out[i] = r.U64()
		default:
			out[i] = nil
		}
	}

	return out, nil
}
