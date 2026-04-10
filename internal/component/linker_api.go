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
	_ api.InstancePre              = (*InstancePreWrapper)(nil)
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

// DefineUnknownImportsAsTraps causes any unresolved imports to be
// automatically satisfied by trap stubs during instantiation.
func (l *ComponentLinkerWrapper) DefineUnknownImportsAsTraps() {
	l.linker.DefineUnknownImportsAsTraps()
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

// InstantiatePre performs import resolution and type-checking, returning
// an InstancePre that can create multiple instances cheaply.
func (l *ComponentLinkerWrapper) InstantiatePre(compiled api.CompiledComponent) (api.InstancePre, error) {
	cc, ok := compiled.(*CompiledComponent)
	if !ok {
		return nil, fmt.Errorf("invalid compiled component type: expected *CompiledComponent")
	}

	pre, err := l.linker.InstantiatePre(cc)
	if err != nil {
		return nil, err
	}

	return &InstancePreWrapper{inner: pre}, nil
}

// InstancePreWrapper wraps the internal InstancePre to implement api.InstancePre.
type InstancePreWrapper struct {
	internalapi.WazeroOnlyType
	inner *InstancePre
}

// Instantiate creates a new component instance using the pre-resolved imports.
func (w *InstancePreWrapper) Instantiate(ctx context.Context) (api.Component, error) {
	inst, err := w.inner.Instantiate(ctx)
	if err != nil {
		return nil, err
	}
	return &ComponentWrapper{instance: inst}, nil
}

// Component returns the compiled component this InstancePre was created from.
func (w *InstancePreWrapper) Component() api.CompiledComponent {
	return w.inner.Component()
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

// Instance adds a nested instance export. Not supported on the basic
// Linker wrapper — returns self, silently ignoring the nested instance.
// This type is never instantiated in practice (DefineInstance always returns
// ComponentInstanceBuilderWrapper2), so the no-op is safe.
func (b *ComponentInstanceBuilderWrapper) Instance(name string) api.ComponentInstanceBuilder {
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

// Instance adds a nested instance export to the instance being built.
func (b *ComponentInstanceBuilderWrapper2) Instance(name string) api.ComponentInstanceBuilder {
	return &ComponentInstanceBuilderWrapper2{
		builder: b.builder.Instance(name),
	}
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

// ExportedFunctions returns a map of all exported functions keyed by name.
// The returned map is a copy; modifications do not affect the component.
func (c *ComponentWrapper) ExportedFunctions() map[string]api.ComponentFunc {
	if c.instance == nil {
		return nil
	}
	result := make(map[string]api.ComponentFunc)
	for name, fn := range c.instance.exports {
		if fn != nil {
			result[name] = &ComponentFuncWrapper{fn: fn}
		}
	}
	return result
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
// Does NOT run post-return — caller must call PostReturn after reading results.
// Panics if Call is invoked again before PostReturn.
func (f *ComponentFuncWrapper) Call(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	return f.fn.Call(ctx, params...)
}

// PostReturn runs the post-return cleanup function. Must be called after Call.
func (f *ComponentFuncWrapper) PostReturn(ctx context.Context) error {
	return f.fn.PostReturn(ctx)
}

// CallAndPostReturn is a convenience that calls Call + PostReturn in one shot.
func (f *ComponentFuncWrapper) CallAndPostReturn(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	return f.fn.CallAndPostReturn(ctx, params...)
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

// ExportedFunctions returns a map of all exported functions keyed by name.
// The returned map is a copy; modifications do not affect the component.
func (w *ComponentInstanceWrapper) ExportedFunctions() map[string]api.ComponentFunc {
	if w.instance == nil {
		return nil
	}
	result := make(map[string]api.ComponentFunc)
	for name, fn := range w.instance.exports {
		if fn != nil {
			result[name] = &ComponentFuncWrapper{fn: fn}
		}
	}
	return result
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

