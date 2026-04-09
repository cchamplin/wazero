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

	"github.com/tetratelabs/wazero/internal/component/runtime"
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
	runtime         any // wazero.Runtime - stored as any to avoid import cycle
	definitions     map[string]Definition
	relaxedSemver   bool
	instanceCounter uint32
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

// Instantiate creates a component instance from a compiled component.
// Spec: definitions.py:256-273 ComponentInstance + canon.lift/lower
// closure creation at :1978-2040 and :2064-2130.
// Wasmtime parallel: runtime/component/instance.rs:710-833 Instantiator.
//
// Session 1 rebuild: this is the 14-step pipeline from the design doc
// (design lines 829-913). Each helper method gets its own task in the
// implementation plan. This skeleton wires steps 1-3 and returns the
// partially-constructed Instance. Steps 4-14 land in Tasks C6-C11.
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
	_ = ctx
	if compiled == nil {
		return nil, fmt.Errorf("Instantiate: compiled is nil")
	}
	c := compiled.Internal()
	if c == nil {
		return nil, fmt.Errorf("Instantiate: compiled.Internal() is nil")
	}

	// Step 1 — Allocate instance + runtime.ComponentInstance.
	inst := newInstance(c, l.nextInstanceID(), nil)

	// Step 2 — Bind resource type declarations to runtime identities.
	// Matches wasmtime Instantiator::resource at instance.rs:912-931.
	// Session 1 Decision 2.
	if err := l.bindResourceTypes(inst, c); err != nil {
		return nil, fmt.Errorf("Instantiate: bind resource types: %w", err)
	}

	// Step 3 — Build index spaces from aliases (funcSpace, memSpace).
	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	l.buildCoreIndexSpaces(c, funcSpace, memSpace)
	_ = funcSpace
	_ = memSpace

	// Step 4 — Resolve and type-check imports.
	tc := NewTypeChecker(c)
	resolvedImports := make(map[string]Definition)
	instanceToImport := make(map[uint32]string)
	if err := l.resolveAndCheckImports(c, tc, resolvedImports, instanceToImport); err != nil {
		return nil, fmt.Errorf("Instantiate: resolve imports: %w", err)
	}

	// Step 5 — Populate value index space from value imports.
	l.populateValueImports(inst, c, resolvedImports)

	// Step 6 — Align instance index space with instance imports.
	l.alignInstanceImports(inst, c)

	// Step 7 — Build component function index space from canon.lift
	// declarations + resolved function imports.
	l.buildComponentFuncs(inst, c, resolvedImports)

	// Step 8 — Build type index space for nested instantiation arg
	// resolution.
	l.buildTypeSpace(inst, c)

	// Step 9 — Process nested component instances.
	componentInstDefs, err := l.processNestedInstances(ctx, inst, c)
	if err != nil {
		return nil, fmt.Errorf("Instantiate: nested instances: %w", err)
	}

	// Step 10 — Build canon lower / canon resource info maps from CanonicalDef.
	canonLowers, canonResources := l.buildCanonMaps(c)

	// Step 11 — Build function alias map for inline instance resolution.
	funcAliases := l.buildFuncAliases(c)

	// Steps 12-14 land in subsequent tasks (C8..C11).
	_ = instanceToImport
	_ = componentInstDefs
	_ = canonLowers
	_ = canonResources
	_ = funcAliases
	return inst, nil
}

// processNestedInstances handles nested component instantiation. The
// full restoration of instantiateNestedComponent lives in Checkpoint D
// (Task D2). Session 1 Stage 3 returns an empty placeholder map and
// traps at the first nested ParsedComponentInstance with a precise
// Checkpoint D pointer.
//
// Note: ParsedComponentInstance.Kind is ComponentInstanceExprKind with
// Instantiate/Inline variants (there is no "alias" kind — aliases are a
// separate Component.Aliases section). Both variants are deferred.
func (l *ComponentLinker) processNestedInstances(ctx context.Context, inst *Instance, c *Component) (map[uint32]*InstanceDef, error) {
	_ = ctx
	_ = inst
	componentInstDefs := make(map[uint32]*InstanceDef)
	if len(c.ComponentInstances) > 0 {
		ci := &c.ComponentInstances[0]
		return nil, fmt.Errorf(
			"nested component instantiation: rebuild in progress (Session 1 Checkpoint D Task D2). ParsedComponentInstance kind %v at index 0",
			ci.Kind)
	}
	return componentInstDefs, nil
}

// buildCanonMaps indexes canon.lower / canon.resource.* declarations by
// the core function slot they occupy. The returned maps are consumed by
// Task C8 when wiring core module host imports.
//
// Spec: definitions.py canon.lower / canon.resource.* declaration shapes.
func (l *ComponentLinker) buildCanonMaps(c *Component) (map[uint32]canonLowerInfo, map[uint32]canonResourceInfo) {
	lowers := make(map[uint32]canonLowerInfo)
	resources := make(map[uint32]canonResourceInfo)
	for i := range c.Canonicals {
		canon := &c.Canonicals[i]
		switch canon.Kind {
		case CanonKindLower:
			lowers[canon.CoreFuncIdx] = canonLowerInfo{
				funcIdx: canon.FuncIdx,
				typeIdx: canon.TypeIdx,
				options: canon.Options,
			}
		case CanonKindResourceNew, CanonKindResourceDrop, CanonKindResourceRep:
			resources[canon.CoreFuncIdx] = canonResourceInfo{
				kind:    canon.Kind,
				typeIdx: canon.TypeIdx,
			}
		}
	}
	return lowers, resources
}

// buildFuncAliases indexes function-producing aliases by their alias
// target index (Alias.Idx). Covers core-export aliases with CoreSortFunc
// and component-level export/outer aliases with SortFunc.
func (l *ComponentLinker) buildFuncAliases(c *Component) map[uint32]*Alias {
	aliases := make(map[uint32]*Alias)
	for i := range c.Aliases {
		a := &c.Aliases[i]
		switch a.Kind {
		case AliasKindCoreExport:
			if a.CoreSort == CoreSortFunc {
				aliases[a.Idx] = a
			}
		case AliasKindExport, AliasKindOuter:
			if a.Sort == SortFunc {
				aliases[a.Idx] = a
			}
		}
	}
	return aliases
}

// canonLowerInfo captures a canon.lower declaration indexed by the core
// function slot it occupies. Consumed by Task C8.
type canonLowerInfo struct {
	funcIdx uint32
	typeIdx uint32
	options CanonicalOptions
}

// canonResourceInfo captures a canon.resource.{new,drop,rep} declaration
// indexed by the core function slot it occupies. Consumed by Task C8.
type canonResourceInfo struct {
	kind    CanonKind
	typeIdx uint32
}

// resolveAndCheckImports walks c.Imports, resolves each from the linker's
// definitions, and type-checks the resolved definition against the
// expected extern-desc type.
//
// Spec: Explainer.md:920-982 (component-model import type matching).
// Wasmtime parallel: runtime/component/matching.rs:51-162.
func (l *ComponentLinker) resolveAndCheckImports(
	c *Component,
	tc *TypeChecker,
	resolvedImports map[string]Definition,
	instanceToImport map[uint32]string,
) error {
	var instanceIdx uint32
	for i := range c.Imports {
		imp := &c.Imports[i]
		def, err := l.MatchImport(imp.Name)
		if err != nil {
			return fmt.Errorf("import %q: %w", imp.Name, err)
		}
		if err := tc.CheckDefinition(&imp.ExternDesc, imp.Name, def); err != nil {
			return fmt.Errorf("import %q: type check: %w", imp.Name, err)
		}
		resolvedImports[imp.Name] = def
		if imp.ExternDesc.Kind == ImportExternDescInstance {
			instanceToImport[instanceIdx] = imp.Name
			instanceIdx++
		}
	}
	return nil
}

// populateValueImports fills inst.values with value imports (constants)
// in import-declaration order.
func (l *ComponentLinker) populateValueImports(inst *Instance, c *Component, resolvedImports map[string]Definition) {
	for i := range c.Imports {
		imp := &c.Imports[i]
		if imp.ExternDesc.Kind != ImportExternDescValue {
			continue
		}
		vd, ok := resolvedImports[imp.Name].(*ImportedValueDef)
		if !ok || vd == nil {
			continue
		}
		inst.values = append(inst.values, vd.Value)
		inst.valuesConsumed = append(inst.valuesConsumed, false)
	}
}

// alignInstanceImports ensures inst.instanceSpace has a slot for every
// instance import. The slot is populated by a later wiring step.
func (l *ComponentLinker) alignInstanceImports(inst *Instance, c *Component) {
	for i := range c.Imports {
		if c.Imports[i].ExternDesc.Kind != ImportExternDescInstance {
			continue
		}
		inst.instanceSpace = append(inst.instanceSpace, nil)
	}
}

// buildComponentFuncs populates inst.componentFuncs from canon.lift
// declarations + resolved function imports. Function imports consume
// component-function-index slots in declaration order, starting from 0,
// ahead of canon.lift entries. Each ComponentFunc.Type is the type
// RESOLVED from the component's import declaration via ResolveTypeDef
// (alias-safe) — NOT from the shared *FuncDef, whose Type is nil under
// Task C3's wasmtime func_new model.
func (l *ComponentLinker) buildComponentFuncs(
	inst *Instance,
	c *Component,
	resolvedImports map[string]Definition,
) {
	// Function imports occupy the first N slots of the component function
	// index space in declaration order.
	var funcIdx uint32
	for i := range c.Imports {
		imp := &c.Imports[i]
		if imp.ExternDesc.Kind != ImportExternDescFunc {
			continue
		}
		fd, ok := resolvedImports[imp.Name].(*FuncDef)
		if !ok || fd == nil {
			funcIdx++
			continue
		}
		// Resolve the component's import-declaration type through any
		// alias chain (Task A4 corrective): aliases for imported types
		// must be walked via ResolveTypeDef.
		var resolvedType *types.TypeFunc
		if td, _, err := c.ResolveTypeDef(imp.ExternDesc.TypeIdx); err == nil && td.Kind == TypeDefKindFunc {
			resolvedType = td.FuncType(c)
		}
		inst.componentFuncs[funcIdx] = ComponentFunc{
			Type: resolvedType,
			Impl: fd.Callback,
		}
		funcIdx++
	}

	// Walk canon.lift declarations. Each lift allocates a new component
	// function slot at ComponentFuncIdx.
	for i := range c.Canonicals {
		canon := &c.Canonicals[i]
		if canon.Kind != CanonKindLift {
			continue
		}
		td, _, err := c.ResolveTypeDef(canon.TypeIdx)
		if err != nil || td.Kind != TypeDefKindFunc {
			continue
		}
		inst.componentFuncs[canon.ComponentFuncIdx] = ComponentFunc{
			Type: td.FuncType(c),
			// Impl is filled by wireExports after core modules
			// instantiate (Task C8/C9).
			Impl: nil,
		}
	}
}

// buildTypeSpace populates inst.typeSpace in declaration order from
// c.TypeDefs. Nested instantiations read from this slice when they
// resolve type arguments from the parent scope.
func (l *ComponentLinker) buildTypeSpace(inst *Instance, c *Component) {
	inst.typeSpace = make([]*TypeDef, len(c.TypeDefs))
	for i := range c.TypeDefs {
		inst.typeSpace[i] = &c.TypeDefs[i]
	}
}

// nextInstanceID returns the next monotonic instance ID for this linker.
// Session 1: simple counter on the linker; Session 2 may widen to a
// store-wide ID namespace when cross-instance resource lookup lands.
func (l *ComponentLinker) nextInstanceID() uint32 {
	l.instanceCounter++
	return l.instanceCounter
}

// bindResourceTypes walks c.Types.ResourceTables and mints one
// *runtime.ResourceType per declared resource, storing it in
// inst.rt.ResourceTypes in declaration order. Matches wasmtime
// Instantiator::resource at instance.rs:912-931.
//
// Spec: definitions.py:351-361 ResourceType {dtor, dtor_async, dtor_callback}.
// Wasmtime parallel: runtime/component/resources/ty.rs:68-79 ResourceType::guest.
func (l *ComponentLinker) bindResourceTypes(inst *Instance, c *Component) error {
	if c.Types == nil {
		return nil
	}
	for rtIdx, table := range c.Types.ResourceTables {
		if table.Concrete {
			// Already concrete (cross-component imported resource). Session 1
			// does not overwrite; Session 2 handles cross-component matching.
			continue
		}
		// Locate destructor metadata for this declaration via c.TypeDefs.
		var dtor *uint32
		var dtorAsync bool
		var dtorCallback *uint32
		for i := range c.TypeDefs {
			td := &c.TypeDefs[i]
			if td.Kind == TypeDefKindResource && td.Resource == types.ResourceTableIdx(rtIdx) {
				dtor = td.ResourceDtor
				dtorAsync = td.ResourceDtorAsync
				dtorCallback = td.ResourceDtorCallback
				break
			}
		}
		rt := &runtime.ResourceType{
			Impl:         inst.rt,
			Dtor:         dtor,
			DtorAsync:    dtorAsync,
			DtorCallback: dtorCallback,
		}
		inst.rt.ResourceTypes = append(inst.rt.ResourceTypes, rt)
	}
	return nil
}

// buildCoreIndexSpaces is a one-line stub for Task C5; the full body
// (walking c.Aliases and populating funcSpace/memSpace) lands with the
// rest of the pipeline in Task C6 per
// docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md.
func (l *ComponentLinker) buildCoreIndexSpaces(c *Component, funcSpace *CoreFuncIndexSpace, memSpace *CoreMemoryIndexSpace) {
	_, _, _ = c, funcSpace, memSpace
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
