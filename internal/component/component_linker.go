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

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/abi"
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

	// Create a store-wide ResourceStore and wire it into the instance.
	// All nested instances created during this Instantiate call share
	// this store, enabling cross-instance (sibling) resource resolution.
	store := runtime.NewResourceStore()
	inst.rt.Store = store
	store.RegisterInstance(inst.rt.ID, inst)

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

	_ = instanceToImport

	// Step 12 — Instantiate core modules with wired host exports.
	if err := l.instantiateCoreModules(ctx, inst, c, compiled.CompiledModules(),
		canonLowers, canonResources, funcAliases); err != nil {
		return nil, fmt.Errorf("Instantiate: core modules: %w", err)
	}

	// Step 13 — Execute start function.
	if err := l.executeStartFunction(ctx, inst, c); err != nil {
		return nil, fmt.Errorf("Instantiate: start function: %w", err)
	}

	// Step 14 — Wire exports.
	if err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace); err != nil {
		return nil, fmt.Errorf("Instantiate: wire exports: %w", err)
	}
	return inst, nil
}

// --- Task C8-b: core-module host wiring + export wiring ----------------
//
// Spec: definitions.py canon_lift / canon_lower sections for the closure
// semantics, plus the Instantiator pipeline described in wasmtime
// crates/wasmtime/src/runtime/component/instance.rs:710-900 for the
// overall shape of core-module instantiation with per-import host
// modules.
//
// The Session-1 rebuild departs from pre-Session-0's experimental
// ImportResolver approach (git show 36a29b13^:internal/component/
// component_linker.go:906-1050) in favour of the simpler model shared
// with wasmtime: build one synthetic host module per core-import-module
// name via HostModuleInstantiator, then instantiate the core module with
// those host modules already registered under the expected names.

// instantiateCoreModules walks c.CoreInstances and instantiates each
// inline or instantiate-kind entry in order. For each Instantiate-kind
// core instance we build one host module per import-module-name referenced
// in coreInst.Args, register it with the runtime via
// HostModuleInstantiator, then instantiate the core module itself via
// CoreModuleInstantiator. For Inline-kind core instances we register a
// single host module under a synthetic name — the wrapper pattern from
// pre-Session-0 createInlineInstanceModule.
//
// For components with no core instances (the two existing skeleton
// tests) this is a no-op and returns nil.
//
// Real wiring of cross-instance aliases and inline-export resolution
// lands with the C8-c end-to-end test (real core module from the
// binary decoder); C8-b lands the scaffolding.
func (l *ComponentLinker) instantiateCoreModules(
	ctx context.Context,
	inst *Instance,
	c *Component,
	compiledModules []CompiledModuleCloser,
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]*Alias,
) error {
	if len(c.CoreInstances) == 0 {
		return nil
	}
	hmi, _ := l.runtime.(HostModuleInstantiator)
	cmi, _ := l.runtime.(CoreModuleInstantiator)
	if cmi == nil {
		return fmt.Errorf(
			"instantiateCoreModules: runtime does not implement CoreModuleInstantiator (type %T); component has %d core instance(s)",
			l.runtime, len(c.CoreInstances))
	}
	for ciIdx := range c.CoreInstances {
		ci := &c.CoreInstances[ciIdx]
		switch ci.Kind {
		case CoreInstanceExprInstantiate:
			if hmi == nil {
				return fmt.Errorf(
					"instantiateCoreModules: core instance %d needs HostModuleInstantiator; runtime type %T does not implement it",
					ciIdx, l.runtime)
			}
			// Build one synthetic host module per distinct import module
			// name the core module will resolve when instantiated. The
			// CoreInstantiateArg.Name IS the import-module-name.
			seen := make(map[string]bool)
			for _, arg := range ci.Args {
				if seen[arg.Name] {
					continue
				}
				seen[arg.Name] = true
				hm, err := l.buildCoreHostModule(ctx, inst, c, arg.Name, arg.InstanceIdx,
					canonLowers, canonResources, funcAliases)
				if err != nil {
					return fmt.Errorf("core instance %d host module %q: %w", ciIdx, arg.Name, err)
				}
				if hm == nil {
					continue
				}
				if _, err := hmi.InstantiateHostModule(ctx, arg.Name, hm); err != nil {
					return fmt.Errorf("core instance %d register host module %q: %w", ciIdx, arg.Name, err)
				}
			}
			if int(ci.ModuleIdx) >= len(compiledModules) {
				return fmt.Errorf("core instance %d: module index %d out of range (have %d)",
					ciIdx, ci.ModuleIdx, len(compiledModules))
			}
			compiled := compiledModules[ci.ModuleIdx]
			if compiled == nil {
				return fmt.Errorf("core instance %d: module %d not compiled", ciIdx, ci.ModuleIdx)
			}
			mod, err := cmi.InstantiateCoreModule(ctx, compiled)
			if err != nil {
				return fmt.Errorf("core instance %d: instantiate module %d: %w", ciIdx, ci.ModuleIdx, err)
			}
			for len(inst.coreInstances) <= ciIdx {
				inst.coreInstances = append(inst.coreInstances, nil)
			}
			inst.coreInstances[ciIdx] = mod
		case CoreInstanceExprInline:
			// Inline-kind core instances are synthetic re-export bundles.
			// They do not produce an api.Module directly — they are
			// consumed as arg.InstanceIdx targets by Instantiate-kind
			// siblings and resolved inside buildCoreHostModule. Leave a
			// nil slot in coreInstances so later idx lookups stay aligned.
			for len(inst.coreInstances) <= ciIdx {
				inst.coreInstances = append(inst.coreInstances, nil)
			}
		default:
			return fmt.Errorf("core instance %d: unknown kind %v", ciIdx, ci.Kind)
		}
	}
	return nil
}

// buildCoreHostModule assembles the HostModuleExport list for one
// import-module-name of a core module. The source is identified by
// instanceIdx — the CoreInstantiateArg.InstanceIdx pointing at another
// entry in c.CoreInstances (typically an Inline-kind one carrying the
// canon.lower / canon.resource.* / alias exports).
//
// Returns (nil, nil) when the source instance has no exports worth
// registering — the caller skips the InstantiateHostModule call in that
// case.
func (l *ComponentLinker) buildCoreHostModule(
	ctx context.Context,
	inst *Instance,
	c *Component,
	moduleName string,
	sourceInstanceIdx uint32,
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]*Alias,
) ([]HostModuleExport, error) {
	_ = ctx
	_ = moduleName
	if int(sourceInstanceIdx) >= len(c.CoreInstances) {
		return nil, fmt.Errorf("source instance %d out of range (have %d)",
			sourceInstanceIdx, len(c.CoreInstances))
	}
	src := &c.CoreInstances[sourceInstanceIdx]
	if src.Kind != CoreInstanceExprInline {
		// Instantiate-kind source: forward all of its exported functions
		// via SourceModule. Requires the source already instantiated.
		if int(sourceInstanceIdx) >= len(inst.coreInstances) || inst.coreInstances[sourceInstanceIdx] == nil {
			return nil, fmt.Errorf("source core instance %d not yet instantiated", sourceInstanceIdx)
		}
		sourceMod := inst.coreInstances[sourceInstanceIdx]
		var exports []HostModuleExport
		for name := range sourceMod.ExportedFunctionDefinitions() {
			exports = append(exports, HostModuleExport{
				Name:         name,
				SourceModule: sourceMod,
				SourceName:   name,
			})
		}
		return exports, nil
	}

	// Inline-kind: each InlineExport resolves to a canon.lower closure,
	// a canon.resource.* op, or an alias-forward to another core instance.
	var exports []HostModuleExport
	for _, ie := range src.InlineExports {
		if ie.Sort != CoreSortFunc {
			// Table / memory inline exports are registered via the
			// HostModuleInstantiator tables/memories variants, which C8-b
			// does not yet wire (followup in C8-c / Checkpoint D). Skip
			// with a precise error so tests that hit this path trap
			// instead of silently dropping the export.
			return nil, fmt.Errorf(
				"inline export %q sort %v: non-func inline exports not yet wired (Task C8-c)",
				ie.Name, ie.Sort)
		}
		// 1. canon.lower?
		if info, ok := canonLowers[ie.Idx]; ok {
			compFunc, hasFn := inst.GetComponentFunc(info.funcIdx)
			if !hasFn || compFunc.Impl == nil {
				return nil, fmt.Errorf("canon.lower export %q: component func %d has no Impl",
					ie.Name, info.funcIdx)
			}
			// Flatten the component function's param/result tuples
			// into []types.ValType lists the abi layer consumes.
			paramElems := unpackTupleElems(c.Types, compFunc.Type.Params)
			resultElems := unpackTupleElems(c.Types, compFunc.Type.Results)
			// needsRetptr determination is a function of flat arity;
			// precise computation lives in abi.FlattenParams /
			// FlattenResults. For C8-b wiring, mirror the default
			// pre-Session-0 behaviour and let abi.LowerResults handle
			// the retptr path when called with needsRetptr=false —
			// C8-c's end-to-end test exercises precise retptr sizing.
			// Compute precise core-ABI flat types via abi.FlattenParams /
			// FlattenResults so s64/f32/f64/aggregate params get the
			// correct core wasm slot widths. The retptr decision drives
			// the canon.lower closure's stack-shape contract.
			coreParams, coreResults, needsRetptr := abi.CoreSignature(c.Types, paramElems, resultElems)
			fn := l.createCanonLowerFunc(inst, c, info, compFunc, paramElems, resultElems, needsRetptr)
			exports = append(exports, HostModuleExport{
				Name:        ie.Name,
				ParamTypes:  coreParams,
				ResultTypes: coreResults,
				Func:        fn,
			})
			continue
		}
		// 2. canon.resource.*?
		if info, ok := canonResources[ie.Idx]; ok {
			hmExp := l.createResourceOpExport(inst, ie.Name, info)
			if hmExp != nil {
				exports = append(exports, *hmExp)
			}
			continue
		}
		// 3. Function alias forwarding to another core instance.
		if alias, ok := funcAliases[ie.Idx]; ok && alias.Kind == AliasKindCoreExport {
			if int(alias.InstanceIdx) >= len(inst.coreInstances) || inst.coreInstances[alias.InstanceIdx] == nil {
				return nil, fmt.Errorf("alias-forward export %q: source core instance %d not instantiated",
					ie.Name, alias.InstanceIdx)
			}
			sourceMod := inst.coreInstances[alias.InstanceIdx]
			exports = append(exports, HostModuleExport{
				Name:         ie.Name,
				SourceModule: sourceMod,
				SourceName:   alias.ExportName,
			})
			continue
		}
		return nil, fmt.Errorf("inline export %q (idx %d): no canon.lower, canon.resource, or alias source",
			ie.Name, ie.Idx)
	}
	return exports, nil
}

// executeStartFunction runs the component's start function, if any.
// The start function is called with arguments consumed from the value
// index space, and its results are appended back to the value index space.
//
// Spec: Explainer.md start function (lines 2436-2476).
// Spec: definitions.py ComponentInstance start-function handling.
// Wasmtime parallel: runtime/component/instance.rs start function execution.
func (l *ComponentLinker) executeStartFunction(ctx context.Context, inst *Instance, c *Component) error {
	if c.Start == nil {
		return nil
	}

	// Step 1: Look up the component function.
	startFunc, ok := inst.GetComponentFunc(c.Start.FuncIdx)
	if !ok {
		return fmt.Errorf("start function: component function %d not found", c.Start.FuncIdx)
	}
	if startFunc.Impl == nil {
		return fmt.Errorf("start function: component function %d has nil implementation", c.Start.FuncIdx)
	}

	// Step 2: Resolve argument values by consuming them from the value index space.
	args := make([]types.Val, len(c.Start.ArgValueIdx))
	for i, valIdx := range c.Start.ArgValueIdx {
		v, err := inst.ConsumeValue(valIdx)
		if err != nil {
			return fmt.Errorf("start function: consuming argument %d (value index %d): %w", i, valIdx, err)
		}
		args[i] = v
	}

	// Step 3: Invoke the start function.
	results, err := startFunc.Impl(ctx, startFunc.Type, args)
	if err != nil {
		return fmt.Errorf("start function: invocation failed: %w", err)
	}

	// Step 4: Validate result count and store results in the value index space.
	if uint32(len(results)) != c.Start.ResultCount {
		return fmt.Errorf("start function: expected %d results, got %d", c.Start.ResultCount, len(results))
	}
	for _, r := range results {
		inst.AddValue(r)
	}

	return nil
}

// wireExports populates inst.exports and inst.exportedInstances from
// c.Exports. Each export kind is handled as follows:
//
//   - ExportKindFunc: ExportedFunc whose impl is either the canon.lift
//     closure or the imported-function re-export HostFunc.
//   - ExportKindInstance: wired via wireNestedComponentExports (real
//     nested instances) or wireInlineInstanceExports (inline instances).
//   - ExportKindType: no-op (types resolved at decode/typecheck time).
//   - ExportKindValue: no-op (values in value index space).
//   - ExportKindComponent: no-op (used for nested instantiation).
//
// Spec: definitions.py ComponentInstance export wiring.
// Plan: Task D3 extended this from func-only to all export kinds.
func (l *ComponentLinker) wireExports(
	inst *Instance,
	c *Component,
	componentInstDefs map[uint32]*InstanceDef,
	funcSpace *CoreFuncIndexSpace,
	memSpace *CoreMemoryIndexSpace,
) error {
	for i := range c.Exports {
		exp := &c.Exports[i]
		switch exp.Kind {
		case ExportKindFunc:
			ef, err := l.wireExportedFunc(inst, c, exp, funcSpace, memSpace)
			if err != nil {
				return fmt.Errorf("export %q: %w", exp.Name, err)
			}
			inst.exports[exp.Name] = ef

		case ExportKindInstance:
			// Instance export: exp.Idx is an index into the component
			// instance space. The instance could be a real nested child
			// (non-nil in instanceSpace) or an inline instance (nil in
			// instanceSpace with an InstanceDef in componentInstDefs).
			childInst := inst.GetInstanceFromSpace(exp.Idx)
			if childInst != nil {
				// Real nested instance — wire it directly.
				if err := l.wireNestedComponentExports(inst, childInst, exp.Name); err != nil {
					return fmt.Errorf("export %q: wire nested instance: %w", exp.Name, err)
				}
			} else if componentInstDefs != nil {
				if instDef, ok := componentInstDefs[exp.Idx]; ok {
					// Inline instance — synthesize an Instance from the InstanceDef.
					if err := l.wireInlineInstanceExports(inst, instDef, exp.Name); err != nil {
						return fmt.Errorf("export %q: wire inline instance: %w", exp.Name, err)
					}
				}
				// If no InstanceDef exists either, the instance index is
				// out of range or was not populated. This is not an error
				// for components that declare an instance export for an
				// import-aligned slot — the import alignment reserves
				// nil slots that are not backed by InstanceDefs.
			}

		case ExportKindType:
			// Types are resolved at decode/typecheck time; no runtime
			// wiring needed.

		case ExportKindValue:
			// Values live in the value index space and are accessed via
			// Instance.GetValue; no runtime wiring needed.

		case ExportKindComponent:
			// Component exports are not directly callable at runtime.
			// They participate in nested instantiation via the component
			// index space. No runtime wiring needed.

		default:
			return fmt.Errorf("export %q: export kind %v not yet wired", exp.Name, exp.Kind)
		}
	}
	return nil
}

// wireExportedFunc builds one ExportedFunc for an ExportKindFunc entry.
// Resolves the canon.lift closure with core-instance-bound memory /
// realloc / post_return, per C8-a review finding #3.
func (l *ComponentLinker) wireExportedFunc(
	inst *Instance,
	c *Component,
	exp *Export,
	funcSpace *CoreFuncIndexSpace,
	memSpace *CoreMemoryIndexSpace,
) (*ExportedFunc, error) {
	// Case A: exp.Idx refers to a component function slot. If the slot
	// holds a pre-populated Impl (imported-function re-export), wrap it
	// directly. Otherwise we expect a canon.lift binding via
	// FuncIdxToCanonical.
	cf, hasCF := inst.GetComponentFunc(exp.Idx)

	// Prefer canon.lift binding when present.
	if canonIdx, ok := c.FuncIdxToCanonical[exp.Idx]; ok && int(canonIdx) < len(c.Canonicals) {
		canon := &c.Canonicals[canonIdx]
		if canon.Kind == CanonKindLift {
			// Resolve the core function / memory / realloc / post_return
			// from the now-instantiated core modules via the index spaces.
			coreFn, err := resolveCoreFuncViaSpace(inst, canon.CoreFuncIdx, funcSpace)
			if err != nil {
				return nil, fmt.Errorf("resolve core func %d: %w", canon.CoreFuncIdx, err)
			}
			var memory api.Memory
			if canon.Options.MemoryIdx != nil {
				memory, err = resolveCoreMemoryViaSpace(inst, *canon.Options.MemoryIdx, memSpace)
				if err != nil {
					return nil, fmt.Errorf("resolve memory %d: %w", *canon.Options.MemoryIdx, err)
				}
			}
			var realloc api.Function
			if canon.Options.ReallocIdx != nil {
				realloc, err = resolveCoreFuncViaSpace(inst, *canon.Options.ReallocIdx, funcSpace)
				if err != nil {
					return nil, fmt.Errorf("resolve realloc %d: %w", *canon.Options.ReallocIdx, err)
				}
			}
			var postReturn api.Function
			if canon.Options.PostReturnIdx != nil {
				postReturn, err = resolveCoreFuncViaSpace(inst, *canon.Options.PostReturnIdx, funcSpace)
				if err != nil {
					return nil, fmt.Errorf("resolve post_return %d: %w", *canon.Options.PostReturnIdx, err)
				}
			}
			td, _, err := c.ResolveTypeDef(canon.TypeIdx)
			if err != nil || td == nil || td.Kind != TypeDefKindFunc {
				return nil, fmt.Errorf("canon.lift type idx %d did not resolve to a func type", canon.TypeIdx)
			}
			ft := td.FuncType(c)
			impl := l.buildCanonLiftFunc(inst, canon, coreFn, ft, memory, realloc, postReturn)
			return &ExportedFunc{
				name:      exp.Name,
				funcType:  ft,
				component: c,
				instance:  inst,
				impl:      impl,
			}, nil
		}
	}

	// Case B: imported-function re-export.
	if hasCF && cf.Impl != nil {
		return &ExportedFunc{
			name:      exp.Name,
			funcType:  cf.Type,
			component: c,
			instance:  inst,
			impl:      cf.Impl,
		}, nil
	}

	return nil, fmt.Errorf("no canonical lift or imported-func source for export %q (idx %d)", exp.Name, exp.Idx)
}

// resolveCoreFuncViaSpace looks up a core function by its component-level
// func index through the CoreFuncIndexSpace, then dereferences the
// resulting (instanceIdx, exportName) against inst.coreInstances.
func resolveCoreFuncViaSpace(inst *Instance, funcIdx uint32, space *CoreFuncIndexSpace) (api.Function, error) {
	instanceIdx, exportName, err := space.Resolve(funcIdx)
	if err != nil {
		return nil, err
	}
	if int(instanceIdx) >= len(inst.coreInstances) || inst.coreInstances[instanceIdx] == nil {
		return nil, fmt.Errorf("core instance %d not instantiated", instanceIdx)
	}
	fn := inst.coreInstances[instanceIdx].ExportedFunction(exportName)
	if fn == nil {
		return nil, fmt.Errorf("core instance %d has no export %q", instanceIdx, exportName)
	}
	return fn, nil
}

// resolveCoreMemoryViaSpace is the memory analog of resolveCoreFuncViaSpace.
func resolveCoreMemoryViaSpace(inst *Instance, memIdx uint32, space *CoreMemoryIndexSpace) (api.Memory, error) {
	instanceIdx, exportName, err := space.Resolve(memIdx)
	if err != nil {
		return nil, err
	}
	if int(instanceIdx) >= len(inst.coreInstances) || inst.coreInstances[instanceIdx] == nil {
		return nil, fmt.Errorf("core instance %d not instantiated", instanceIdx)
	}
	mem := inst.coreInstances[instanceIdx].ExportedMemory(exportName)
	if mem == nil {
		return nil, fmt.Errorf("core instance %d has no memory export %q", instanceIdx, exportName)
	}
	return mem, nil
}

// processNestedInstances walks c.ComponentInstances and processes each
// entry. For Instantiate-kind entries, instantiateNestedComponent is called
// to create a child instance that runs the nested pipeline. For Inline-kind
// entries, exports are resolved from the current scope's index spaces.
//
// The returned map associates each component instance index with an
// InstanceDef that wireExports can consume.
//
// Spec: Component-model nested instantiation (Explainer.md :1020+).
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md
//   (Task D2).
func (l *ComponentLinker) processNestedInstances(ctx context.Context, inst *Instance, c *Component) (map[uint32]*InstanceDef, error) {
	componentInstDefs := make(map[uint32]*InstanceDef)
	for i := range c.ComponentInstances {
		ci := &c.ComponentInstances[i]
		switch ci.Kind {
		case ComponentInstanceExprInstantiate:
			child, err := l.instantiateNestedComponent(ctx, inst, ci, c)
			if err != nil {
				return nil, fmt.Errorf("component instance %d: %w", i, err)
			}
			// Store child in parent's instance space. Instance imports
			// (step 6) have already consumed earlier slots, so append
			// to the end to get the correct component-instance-index.
			slotIdx := inst.AddInstanceToSpace(child)
			// Create an InstanceDef for wireExports.
			componentInstDefs[slotIdx] = instanceToDefinition(child)

		case ComponentInstanceExprInline:
			// Inline instances declare exports from the current scope's
			// index spaces. They don't instantiate a new component.
			instDef := &InstanceDef{Exports: make(map[string]Definition)}
			for _, exp := range ci.InlineExports {
				def, err := l.resolveInlineExport(inst, c, exp)
				if err != nil {
					return nil, fmt.Errorf("component instance %d inline export %q: %w", i, exp.Name, err)
				}
				if def != nil {
					instDef.Exports[exp.Name] = def
				}
			}
			// Inline instances don't produce a child Instance; they
			// produce an InstanceDef that wireExports can consume.
			// Reserve a nil slot in instanceSpace to keep indices aligned
			// via the same abstraction the Instantiate path uses.
			slotIdx := inst.AddInstanceToSpace(nil)
			componentInstDefs[slotIdx] = instDef

		default:
			return nil, fmt.Errorf("component instance %d: unknown kind %v", i, ci.Kind)
		}
	}
	return componentInstDefs, nil
}

// resolveInlineExport resolves a single ComponentInlineExport from the
// current instance's index spaces. The Sort field tells which space to
// look in.
//
// Spec: inline component instances re-export items from the current
// scope (Explainer.md component-instance grammar).
func (l *ComponentLinker) resolveInlineExport(
	inst *Instance,
	_ *Component,
	exp ComponentInlineExport,
) (Definition, error) {
	switch exp.Sort {
	case SortFunc:
		fn, ok := inst.componentFuncs[exp.Idx]
		if !ok {
			return nil, fmt.Errorf("func %d not found in instance", exp.Idx)
		}
		return &FuncDef{Type: fn.Type, Callback: fn.Impl}, nil

	case SortInstance:
		child := inst.GetInstanceFromSpace(exp.Idx)
		if child == nil {
			return nil, fmt.Errorf("instance %d not found in instance space", exp.Idx)
		}
		return instanceToDefinition(child), nil

	case SortType:
		td := inst.GetTypeFromSpace(exp.Idx)
		if td == nil {
			return nil, fmt.Errorf("type %d not found in type space", exp.Idx)
		}
		return &TypeDefDef{TypeDef: td}, nil

	case SortValue:
		v, err := inst.GetValue(exp.Idx)
		if err != nil {
			return nil, fmt.Errorf("value %d: %w", exp.Idx, err)
		}
		return &ImportedValueDef{Value: v}, nil

	case SortComponent:
		comp := inst.GetComponentFromSpace(exp.Idx)
		if comp == nil {
			return nil, fmt.Errorf("component %d not found in component space", exp.Idx)
		}
		return &ComponentDef{Component: comp}, nil

	default:
		return nil, fmt.Errorf("unsupported inline export sort: %s", exp.Sort)
	}
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
		// Register in the store-wide registry for cross-instance lookups.
		if inst.rt.Store != nil {
			inst.rt.Store.Register(inst.rt.ID, uint32(rtIdx), rt)
		}
	}
	return nil
}

// buildCoreIndexSpaces walks c.Aliases and populates the runtime view of
// the core function and core memory index spaces from AliasKindCoreExport
// entries. The decoder has already assigned each alias its index slot in
// the appropriate space (binary/alias.go:107-133); this routine simply
// records the (instanceIdx, exportName) pair under that slot so
// resolveCoreFuncViaSpace / resolveCoreMemoryViaSpace can dereference a
// canon.lift CoreFuncIdx (or memory/realloc/post_return idx) into a live
// api.Function / api.Memory after core modules instantiate.
//
// Spec: Binary.md alias section + index-space densification rules.
// Wasmtime parallel: runtime/component/instance.rs Instantiator stitches
// the same (alias -> exported core item) edges before invoking host
// imports.
func (l *ComponentLinker) buildCoreIndexSpaces(c *Component, funcSpace *CoreFuncIndexSpace, memSpace *CoreMemoryIndexSpace) {
	for i := range c.Aliases {
		a := &c.Aliases[i]
		if a.Kind != AliasKindCoreExport {
			continue
		}
		switch a.CoreSort {
		case CoreSortFunc:
			funcSpace.AddAlias(a.Idx, a.InstanceIdx, a.ExportName)
		case CoreSortMemory:
			memSpace.AddAlias(a.Idx, a.InstanceIdx, a.ExportName)
		}
	}
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

// -- Task C8-a: canon.lift / canon.lower / canon.resource.* closures ----
//
// Spec: definitions.py:1978-2173.
// Wasmtime parallel:
//   runtime/component/func.rs Func::call / call_raw (lines 232-706)
//   runtime/component/func/host.rs DynamicHostFn::call (lines 640-694)
//   runtime/component/func/options.rs LiftContext/LowerContext analog.
//
// This dispatch lands the standalone closures and a standalone
// canon.lower closure unit test. Pipeline wiring (buildCoreHostModule,
// instantiateCoreModules, wireExports, executeStartFunction,
// Instantiate steps 12-14, ExportedFunc.Call rewrite) is deferred to
// Task C8-b; the end-to-end integration test to Task C8-c.

// buildAbiOptions constructs an abi.Options from a CanonicalDef's
// CanonicalOptions together with the per-instance memory and realloc
// function. The impedance mismatch between CanonicalOptions.MemoryIdx
// (*uint32, nilable) and abi.Options.MemoryIdx (uint32, value) is
// handled here: when canon.Options.MemoryIdx is nil we pass 0, which
// is the convention used by the abi.Context constructors.
//
// The memory / realloc api.Function values themselves travel through
// the LiftContext / LowerContext memory + realloc fields directly;
// abi.Options only carries the indirection indices for reference.
//
// Spec: definitions.py:1978-1990 canon_lift uses CanonicalOptions fields
// memory / realloc / post_return directly; wasmtime
// runtime/component/func/options.rs has the parallel Options struct.
// C8-b cleanup (review item 4): dropped unused memory/realloc parameters.
// Options only carries indirection indices; the live api.Memory/api.Function
// values travel via LiftContext/LowerContext.Memory / .Realloc set by callers.
func buildAbiOptions(canon *CanonicalDef) abi.Options {
	var memIdx uint32
	if canon != nil && canon.Options.MemoryIdx != nil {
		memIdx = *canon.Options.MemoryIdx
	}
	var realloIdx *uint32
	var postIdx *uint32
	var enc abi.StringEncoding
	if canon != nil {
		realloIdx = canon.Options.ReallocIdx
		postIdx = canon.Options.PostReturnIdx
		switch canon.Options.StringEncoding {
		case StringEncodingUTF8:
			enc = abi.StringEncodingUTF8
		case StringEncodingUTF16:
			enc = abi.StringEncodingUTF16
		case StringEncodingLatin1UTF16:
			enc = abi.StringEncodingLatin1UTF16
		}
	}
	return abi.Options{
		StringEncoding: enc,
		MemoryIdx:      memIdx,
		ReallocIdx:     realloIdx,
		PostReturnIdx:  postIdx,
	}
}

// reallocAdapter wraps a core-wasm realloc api.Function into the Go
// closure shape abi.LowerContext.Realloc expects. Returns nil when
// realloc is nil (the caller then traps if the lower path needs realloc
// — see abi/lower.go LowerParams retptr path).
func reallocAdapter(realloc api.Function) func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
	if realloc == nil {
		return nil
	}
	return func(oldPtr, oldSize, alignv, newSize uint32) (uint32, error) {
		res, err := realloc.Call(context.Background(),
			uint64(oldPtr), uint64(oldSize), uint64(alignv), uint64(newSize))
		if err != nil {
			return 0, err
		}
		if len(res) == 0 {
			return 0, fmt.Errorf("realloc returned no result")
		}
		return uint32(res[0]), nil
	}
}

// unpackTupleElems returns the element ValTypes of a tuple ValType by
// indexing into the canonical type bag. Returns nil when v is not a
// tuple (e.g. a scalar or a non-aggregate kind). Used by the canon.lift
// / canon.lower closures to materialise the TypeFunc.Params /
// TypeFunc.Results tuples into flat []types.ValType lists for
// LiftParams / LowerParams / LiftResults / LowerResults.
func unpackTupleElems(bag *types.ComponentTypes, v types.ValType) []types.ValType {
	if bag == nil || v.Kind != types.TypeKindTuple {
		return nil
	}
	if int(v.Index) >= len(bag.Tuples) {
		return nil
	}
	return bag.Tuples[v.Index].Types
}

// buildCanonLiftFunc creates the closure that implements a canon.lift
// component function. Mirrors spec canon_lift at definitions.py:1978-2040
// for synchronous calls.
//
// Spec step mapping:
//   :1979       trap_if(call_might_be_recursive) — via
//               inst.rt.Reentrance.CallMightBeRecursive. The structural
//               variant (Instance.CallMightBeRecursive) requires a
//               caller *Instance which the HostFunc signature does not
//               carry; the tracker-based check captures "is this
//               instance already on the active call stack", which is
//               the spec-relevant condition once the caller is out of
//               band (host or indirect). Both the tracker bookkeeping
//               (EnterInstance/LeaveInstance) and the structural check
//               serve different purposes per Task B4's corrective.
//   :1990/:1955 lower_flat_values on the args — wazero uses
//               abi.LowerParams which expects the caller to toggle
//               may_leave around the aggregate (lower.go:729).
//   :1995       callee core-function invocation.
//   :1997       lift_flat_values on the core results — abi.LiftResults.
//   :2000-2002  post_return with may_leave toggled off for the duration.
//   :738-742    deliver_resolve — close the borrow scope.
//
// Wasmtime parallel: runtime/component/func.rs Func::call / call_raw
// (lines 232-706).
func (l *ComponentLinker) buildCanonLiftFunc(
	inst *Instance,
	canon *CanonicalDef,
	coreFunc api.Function,
	funcType *types.TypeFunc,
	memory api.Memory,
	realloc api.Function,
	postReturn api.Function,
) HostFunc {
	opts := buildAbiOptions(canon)
	paramElems := unpackTupleElems(inst.component.Types, funcType.Params)
	resultElems := unpackTupleElems(inst.component.Types, funcType.Results)

	return func(goCtx context.Context, fnType *types.TypeFunc, args []types.Val) (results []types.Val, retErr error) {
		_ = fnType // captured via funcType at construction time
		// :1979 — structural/tracker reentrance check.
		if inst.rt.Reentrance.CallMightBeRecursive(inst.rt.ID) {
			return nil, errReentrance
		}
		inst.rt.Reentrance.EnterInstance(inst.rt.ID)
		defer inst.rt.Reentrance.LeaveInstance(inst.rt.ID)

		borrow := runtime.NewBorrowScope(inst.rt.Table)
		// C8-b review item 1: always release borrow scope on exit,
		// including error paths. :738-742 deliver_resolve.
		defer func() {
			if rerr := borrow.Release(); rerr != nil && retErr == nil {
				retErr = fmt.Errorf("canon.lift: release borrow scope: %w", rerr)
			}
		}()
		callCtx := runtime.NewCallContext()

		lowerCtx := &abi.LowerContext{
			Memory:      memory,
			Opts:        &opts,
			Realloc:     reallocAdapter(realloc),
			Types:       inst.component.Types,
			Instance:    inst.rt,
			CallContext: callCtx,
		}

		// :1955 may_leave toggle around LowerParams aggregate.
		prevMayLeave := inst.rt.IsMayLeave()
		inst.rt.MayLeave = false
		flatArgs, err := abi.LowerParams(lowerCtx, paramElems, args, abi.MaxFlatParams)
		inst.rt.MayLeave = prevMayLeave
		if err != nil {
			return nil, fmt.Errorf("canon.lift: lower params: %w", err)
		}

		// :1995 callee invocation.
		var flatResults []uint64
		if coreFunc != nil {
			flatResults, err = coreFunc.Call(goCtx, flatArgs...)
			if err != nil {
				return nil, fmt.Errorf("canon.lift: core call: %w", err)
			}
		}

		// :1997 lift_flat_values on the return path.
		liftCtx := &abi.LiftContext{
			Memory:      memory,
			Opts:        &opts,
			Types:       inst.component.Types,
			Instance:    inst.rt,
			BorrowScope: borrow,
		}
		liftedResults, err := abi.LiftResults(liftCtx, resultElems, flatResults, abi.MaxFlatResults)
		if err != nil {
			return nil, fmt.Errorf("canon.lift: lift results: %w", err)
		}
		results = liftedResults

		// :2000-2002 post_return with may_leave toggled off.
		if postReturn != nil {
			inst.rt.MayLeave = false
			_, perr := postReturn.Call(goCtx, flatResults...)
			inst.rt.MayLeave = prevMayLeave
			if perr != nil {
				return nil, fmt.Errorf("canon.lift: post_return: %w", perr)
			}
		}

		// :738-742 deliver_resolve is handled by the deferred borrow release.
		return results, nil
	}
}

// createCanonLowerFunc produces an api.GoModuleFunc implementing a
// canon.lower core wasm function. Spec: definitions.py:2064-2130.
//
// Spec step mapping:
//   :2065       trap_if(not caller_task.inst.may_leave) — wazero uses
//               the per-instance MayLeave; see instance.go MayLeave.
//   :2068-2070  subtask + borrow scope + lift/lower contexts.
//   :2089       lift_flat_values on the incoming core args —
//               abi.LiftParams.
//   :2095       host callback invocation via compFunc.Impl(goCtx,
//               compFunc.Type, args). Direct HostFunc call per Task C3;
//               do NOT type-assert to *FuncDef.
//   :2104-2113  lower_flat_values on the results with may_leave toggle
//               (:1955/:1973).
//   :2113       deliver_resolve — close the borrow scope.
//
// Wasmtime parallel: runtime/component/func/host.rs DynamicHostFn::call
// at around lines 640-694.
//
// Errors propagate via panic per the wazero GoModuleFunc convention —
// the engine converts panics into traps at the call boundary.
func (l *ComponentLinker) createCanonLowerFunc(
	inst *Instance,
	c *Component,
	info canonLowerInfo,
	compFunc ComponentFunc,
	paramTypes []types.ValType,
	resultTypes []types.ValType,
	needsRetptr bool,
) api.GoModuleFunc {
	// Snapshot options from the canon.lower declaration. Memory and
	// realloc are resolved from the core module at call time because
	// they belong to the core instance the lowered function is
	// called from.
	canonSnapshot := &CanonicalDef{Kind: CanonKindLower, Options: info.options}

	return api.GoModuleFunc(func(goCtx context.Context, mod api.Module, stack []uint64) {
		memory := mod.Memory()
		var realloc api.Function
		if info.options.ReallocIdx != nil {
			// Session 1: core module exports realloc under the conventional
			// name "cabi_realloc". If absent, leave realloc nil; the abi
			// layer traps if the retptr path requires it.
			realloc = mod.ExportedFunction("cabi_realloc")
		}
		opts := buildAbiOptions(canonSnapshot)
		_ = memory // referenced via liftCtx/lowerCtx below

		// :2065 entry trap.
		if !inst.rt.IsMayLeave() {
			panic(errMayNotLeave)
		}

		// :2068-2070 subtask + contexts.
		borrow := runtime.NewBorrowScope(inst.rt.Table)
		// C8-b review item 1: always release borrow scope, even on panic
		// from lift/host/lower paths. :2113 deliver_resolve.
		defer func() {
			if rerr := borrow.Release(); rerr != nil {
				// If we're already unwinding from a panic, surface that
				// first; otherwise raise the release error as a trap.
				if p := recover(); p != nil {
					panic(p)
				}
				panic(fmt.Errorf("canon.lower: release borrow scope: %w", rerr))
			}
		}()
		callCtx := runtime.NewCallContext()

		liftCtx := &abi.LiftContext{
			Memory:      memory,
			Opts:        &opts,
			Types:       c.Types,
			Instance:    inst.rt,
			BorrowScope: borrow,
		}

		// :2089 lift params from the incoming core stack. The flat
		// param width is driven by FlattenParams inside LiftParams.
		// For the retptr path, the trailing i32 on the stack is the
		// retptr — LiftParams will read it via iter(flat[0]) when
		// flatTypes > maxFlat.
		// Determine flat-param arity: the core-wasm stack prefix holds
		// the lifted parameters; for a retptr-free call, stack contains
		// exactly flat-arity params. For the needsRetptr case, the last
		// stack slot is the retptr. We hand LiftParams the appropriate
		// prefix.
		paramStack := stack
		if needsRetptr && len(stack) > 0 {
			paramStack = stack[:len(stack)-1]
		}
		args, err := abi.LiftParams(liftCtx, paramTypes, paramStack, abi.MaxFlatParams)
		if err != nil {
			panic(fmt.Errorf("canon.lower: lift params: %w", err))
		}

		// :2095 callee (host) invocation — direct HostFunc dispatch.
		if compFunc.Impl == nil {
			panic(fmt.Errorf("canon.lower: component func %d has nil Impl", info.funcIdx))
		}
		results, err := compFunc.Impl(goCtx, compFunc.Type, args)
		if err != nil {
			panic(fmt.Errorf("canon.lower: host callback: %w", err))
		}

		// :2104-2113 lower results with may_leave toggle.
		lowerCtx := &abi.LowerContext{
			Memory:      memory,
			Opts:        &opts,
			Realloc:     reallocAdapter(realloc),
			Types:       c.Types,
			Instance:    inst.rt,
			CallContext: callCtx,
		}
		prevMayLeave := inst.rt.IsMayLeave()
		inst.rt.MayLeave = false
		err = abi.LowerResults(lowerCtx, resultTypes, results, stack, needsRetptr, abi.MaxFlatResults)
		inst.rt.MayLeave = prevMayLeave
		if err != nil {
			panic(fmt.Errorf("canon.lower: lower results: %w", err))
		}

		// :2113 deliver_resolve is handled by the deferred borrow release.
	})
}

// createResourceOpExport builds a HostModuleExport wrapping one of
// canon.resource.{new,drop,rep} per spec definitions.py:2134-2173.
// The body delegates to Instance.ResourceNew / ResourceRep /
// ResourceDrop, which carry Task B4 rebuild-in-progress stubs today;
// Task E5 fills them in. No test in this dispatch executes the
// returned exports, so the stub delegation is acceptable — the
// closures compile against the real spec-correct core-wasm
// signatures.
//
// Core wasm signatures (plan lines 3129-3131):
//   resource.new  (param i32) (result i32)  — rep in, handle out
//   resource.drop (param i32)                — handle in, nothing
//   resource.rep  (param i32) (result i32)  — handle in, rep out
func (l *ComponentLinker) createResourceOpExport(
	inst *Instance,
	name string,
	info canonResourceInfo,
) *HostModuleExport {
	// Resolve the resource type-def via alias-safe ResolveTypeDef.
	td, _, err := inst.component.ResolveTypeDef(info.typeIdx)
	if err != nil || td == nil || td.Kind != TypeDefKindResource {
		return &HostModuleExport{
			Name: name,
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				panic(fmt.Errorf("canon.resource.%v: unresolved resource type idx %d", info.kind, info.typeIdx))
			}),
		}
	}
	resourceIdx := types.ResourceIdx(td.Resource)

	switch info.kind {
	case CanonKindResourceNew:
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				rep := uint32(stack[0])
				h, err := inst.ResourceNew(resourceIdx, rep)
				if err != nil {
					panic(err)
				}
				stack[0] = uint64(h)
			}),
		}
	case CanonKindResourceDrop:
		return &HostModuleExport{
			Name:       name,
			ParamTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				h := uint32(stack[0])
				if err := inst.ResourceDrop(resourceIdx, h); err != nil {
					panic(err)
				}
			}),
		}
	case CanonKindResourceRep:
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				h := uint32(stack[0])
				rep, err := inst.ResourceRep(resourceIdx, h)
				if err != nil {
					panic(err)
				}
				stack[0] = uint64(rep)
			}),
		}
	}
	return &HostModuleExport{
		Name: name,
		Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			panic(fmt.Errorf("canon.resource: unknown kind %v", info.kind))
		}),
	}
}
