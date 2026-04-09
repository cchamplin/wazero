// internal/component/component_linker.go
//
// SESSION 0 COMPILE-FIX STUB (Task 17).
//
// The previous implementation of ComponentLinker — including Instantiate,
// every lift/lower helper, the resource-op host module creators, and the
// deleted resolveValTypeRef / resolveToValType / typeDefToValType /
// valTypeRefToValType helpers — has been reduced to panic stubs so the
// top-level internal/component/ package can compile against the new
// types.ValType / types.TypeFunc / runtime.ComponentInstance shapes.
//
// Every body below that depends on the broken lift/lower path panics with a
// precise error pointing at the Session 1 followup note. Session 1 will
// delete these stubs and replace them with direct calls into the rewritten
// internal/component/abi/ package.
//
// Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md
// Work Order: step 15 (compile-fix); V5 caller audit (design lines 1927-1945).
package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/types"
)

// MaxFlatResults is the maximum number of flattened result values that can
// be returned directly (for synchronous calls). Beyond this, results spill
// to memory via a return pointer.
const MaxFlatResults = 1

// ComponentLinker resolves component imports and instantiates components.
//
// Session 0 compile-fix: only the shape and the public API surface are
// preserved. The method bodies that drive instantiation panic with a
// precise Session 1 pointer.
type ComponentLinker struct {
	runtime       any // wazero.Runtime - stored as any to avoid import cycle
	definitions   map[string]Definition
	relaxedSemver bool
}

// NewComponentLinker creates a new component linker with access to a runtime.
// The runtime parameter should be a wazero.Runtime instance.
func NewComponentLinker(rt any) *ComponentLinker {
	return &ComponentLinker{
		runtime:     rt,
		definitions: make(map[string]Definition),
	}
}

// SetRelaxedSemverMatching enables or disables relaxed semver matching.
func (l *ComponentLinker) SetRelaxedSemverMatching(relaxed bool) {
	l.relaxedSemver = relaxed
}

// RelaxedSemverMatching returns whether relaxed semver matching is enabled.
func (l *ComponentLinker) RelaxedSemverMatching() bool {
	return l.relaxedSemver
}

// DefineFunc adds a host function definition. The host has no type
// to declare — the component's import declaration IS the canonical
// type, looked up by the type checker at instantiate time and
// supplied to the host's HostFunc callback at call time.
//
// Mirrors wasmtime LinkerInstance::func_new
// (debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:665-675).
func (l *ComponentLinker) DefineFunc(namespace, name string, fn HostFunc) error {
	if fn == nil {
		return fmt.Errorf("DefineFunc: nil HostFunc for %q.%q", namespace, name)
	}
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	// Type is populated by the type checker at instantiate time from the
	// component's import declaration. It is left nil at registration;
	// reading it before instantiate is a programming error.
	l.definitions[key] = &FuncDef{Callback: fn}
	return nil
}

// DefineResource adds a resource type definition.
func (l *ComponentLinker) DefineResource(namespace, name string, destructor func(rep uint32)) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &ResourceDef{Destructor: destructor}
	return nil
}

// DefineValue adds a value definition for value imports.
func (l *ComponentLinker) DefineValue(namespace, name string, value types.Val) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
	l.definitions[key] = &ImportedValueDef{Value: value}
	return nil
}

// ComponentInstanceBuilder builds an instance definition for ComponentLinker.
type ComponentInstanceBuilder struct {
	linker         *ComponentLinker
	namespace      string
	exports        map[string]Definition
	skipValidation bool
}

// DefineInstance starts building an instance definition.
func (l *ComponentLinker) DefineInstance(namespace string) *ComponentInstanceBuilder {
	return &ComponentInstanceBuilder{
		linker:    l,
		namespace: namespace,
		exports:   make(map[string]Definition),
	}
}

// Func adds a function export. See HostFunc / DefineFunc doc: the
// host has no type to declare under the wasmtime func_new model.
func (b *ComponentInstanceBuilder) Func(name string, fn HostFunc) *ComponentInstanceBuilder {
	b.exports[name] = &FuncDef{Callback: fn}
	return b
}

// Resource adds a resource export.
func (b *ComponentInstanceBuilder) Resource(name string, destructor func(rep uint32)) *ComponentInstanceBuilder {
	b.exports[name] = &ResourceDef{Destructor: destructor}
	return b
}

// SkipValidation disables validation for this instance definition.
// Use this when providing a partial implementation of a WASI interface.
func (b *ComponentInstanceBuilder) SkipValidation() *ComponentInstanceBuilder {
	b.skipValidation = true
	return b
}

// Build finalizes the instance definition.
func (b *ComponentInstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports, SkipValidation: b.skipValidation}
	return nil
}

// Instantiate creates a component instance with resolved imports.
//
// Session 0 compile-fix: body panics. The real implementation is scheduled
// for Session 1 deletion/rewrite once abi/lift and abi/lower land.
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
	_ = ctx
	_ = compiled
	panic("compile-fix stub: see Session 1 followup note — component_linker.go Instantiate scheduled for Session 1 deletion")
}

// MatchImport finds a definition that satisfies the import name.
func (l *ComponentLinker) MatchImport(importName string) (Definition, error) {
	// Reuse the basic Linker's matching logic.
	linker := &Linker{definitions: l.definitions, relaxedSemver: l.relaxedSemver}
	return linker.MatchImport(importName)
}

// Get retrieves a definition by its full key.
func (l *ComponentLinker) Get(key string) (Definition, bool) {
	def, ok := l.definitions[key]
	return def, ok
}

// MergeFrom copies all definitions from a Linker into this ComponentLinker.
func (l *ComponentLinker) MergeFrom(linker *Linker) {
	for key, def := range linker.definitions {
		l.definitions[key] = def
	}
}
