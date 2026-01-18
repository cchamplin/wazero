// internal/component/component_linker.go
package component

import (
	"context"
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// ComponentLinker resolves component imports and instantiates components
// with full runtime integration for core module instantiation.
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
// When enabled, pre-1.0 versions (0.x.y) match any patch version within
// the same minor version (e.g., 0.2.0 matches 0.2.3).
// By default, strict matching is used where available.Patch >= required.Patch.
func (l *ComponentLinker) SetRelaxedSemverMatching(relaxed bool) {
	l.relaxedSemver = relaxed
}

// RelaxedSemverMatching returns whether relaxed semver matching is enabled.
func (l *ComponentLinker) RelaxedSemverMatching() bool {
	return l.relaxedSemver
}

// DefineFunc adds a host function definition.
func (l *ComponentLinker) DefineFunc(namespace, name string, fn HostFunc) error {
	key := namespace + "/" + name
	if _, exists := l.definitions[key]; exists {
		return fmt.Errorf("definition already exists: %s", key)
	}
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

// ComponentInstanceBuilder builds an instance definition for ComponentLinker.
type ComponentInstanceBuilder struct {
	linker    *ComponentLinker
	namespace string
	exports   map[string]Definition
}

// DefineInstance starts building an instance definition.
func (l *ComponentLinker) DefineInstance(namespace string) *ComponentInstanceBuilder {
	return &ComponentInstanceBuilder{
		linker:    l,
		namespace: namespace,
		exports:   make(map[string]Definition),
	}
}

// Func adds a function export.
func (b *ComponentInstanceBuilder) Func(name string, fn HostFunc) *ComponentInstanceBuilder {
	b.exports[name] = &FuncDef{Callback: fn}
	return b
}

// Resource adds a resource export.
func (b *ComponentInstanceBuilder) Resource(name string, destructor func(rep uint32)) *ComponentInstanceBuilder {
	b.exports[name] = &ResourceDef{Destructor: destructor}
	return b
}

// Build finalizes the instance definition.
func (b *ComponentInstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports}
	return nil
}

// Instantiate creates a component instance with resolved imports.
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
	c := compiled.Internal()
	compiledModules := compiled.CompiledModules()

	inst := &Instance{
		component:     c,
		coreInstances: make([]api.Module, 0),
		exports:       make(map[string]*ExportedFunc),
		resourceTable: NewResourceTable(),
	}

	// Build index spaces from aliases.
	// Each alias has an Idx field that represents the target index in the
	// appropriate core index space (func, memory, etc.). This Idx is assigned
	// during binary decoding and accounts for all operations that consume
	// indices in these spaces (aliases, canon lower, canon resource.*, etc.).
	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()

	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport {
			switch alias.CoreSort {
			case CoreSortFunc:
				funcSpace.AddAlias(alias.Idx, alias.InstanceIdx, alias.ExportName)
			case CoreSortMemory:
				memSpace.AddAlias(alias.Idx, alias.InstanceIdx, alias.ExportName)
			}
		}
	}

	// Step 1: Validate imports
	resolvedImports := make(map[string]Definition)
	for _, imp := range c.Imports {
		def, err := l.MatchImport(imp.Name)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", imp.Name, err)
		}
		resolvedImports[imp.Name] = def
	}

	// Step 2: Instantiate core modules
	for i, coreInst := range c.CoreInstances {
		switch coreInst.Kind {
		case CoreInstanceExprInstantiate:
			module, err := l.instantiateCoreModule(ctx, inst, c, &coreInst, compiledModules, i, resolvedImports)
			if err != nil {
				return nil, fmt.Errorf("instantiate core instance %d: %w", i, err)
			}
			inst.coreInstances = append(inst.coreInstances, module)
		case CoreInstanceExprInline:
			inst.coreInstances = append(inst.coreInstances, nil)
		}
	}

	// Step 3: Wire exports
	for _, exp := range c.Exports {
		if exp.Kind == ExportKindFunc {
			exportedFunc, err := l.wireExportedFunc(inst, c, &exp, funcSpace, memSpace)
			if err != nil {
				return nil, fmt.Errorf("wire export %q: %w", exp.Name, err)
			}
			inst.exports[exp.Name] = exportedFunc
		}
	}

	return inst, nil
}


// buildImportResolver creates an import resolver function that maps import module names
// to previously instantiated core instances based on CoreInstantiateArg mappings.
// This enables inter-instance import resolution where later core instances can import
// from earlier ones.
//
// For inline instances (which don't create actual modules), we resolve through the
// inline exports to find the actual source core instance.
//
// Additionally, this resolver handles component-level imports by matching against
// resolvedImports using semver matching. When a component import is matched
// (e.g., wasi:filesystem/preopens@0.2.2), a host module is created that
// wraps the host functions defined in the InstanceDef.
//
// IMPORTANT: Internal core instances (shim modules) take priority over host implementations.
// The component's internal shim modules use canonical operations (canon lower/lift) to
// translate between core module signatures and the component ABI. These shims then call
// the component-level imports (our host implementations) via canonical operations.
// If we bypassed the shims and used host modules directly, we would get signature
// mismatches because our host modules don't match the exact signatures expected by
// core modules (e.g., resource.drop operations have specific canonical signatures).
func (l *ComponentLinker) buildImportResolver(ctx context.Context, inst *Instance, c *Component, coreInst *CoreInstance, resolvedImports map[string]Definition) experimental.ImportResolver {
	// Build a map from import module name to source instance index
	importMap := make(map[string]uint32)
	for _, arg := range coreInst.Args {
		importMap[arg.Name] = arg.InstanceIdx
	}

	// Cache for synthetic modules created from component imports
	syntheticModules := make(map[string]api.Module)

	return func(moduleName string) api.Module {
		// First, check if the component has an internal instance for this import.
		// Internal instances (from inter-core-instance imports or inline instances)
		// take priority because they contain the component's shim code that properly
		// translates between core module signatures and the component ABI using
		// canonical operations (canon lower/lift/resource.drop/etc).
		instanceIdx, ok := importMap[moduleName]
		if ok {
			// Check if we have a direct core instance
			if int(instanceIdx) < len(inst.coreInstances) {
				coreInstance := inst.coreInstances[instanceIdx]
				if coreInstance != nil {
					return coreInstance
				}
			}

			// The instance might be an inline instance - we need to trace through
			// the inline exports to find the actual source.
			if int(instanceIdx) < len(c.CoreInstances) {
				srcCoreInst := &c.CoreInstances[instanceIdx]
				if srcCoreInst.Kind == CoreInstanceExprInline {
					result := l.resolveInlineInstanceSource(inst, c, srcCoreInst)
					if result != nil {
						return result
					}
				}
			}
		}

		// Fall back to host implementations only if no internal instance is available.
		// This handles cases where the component imports directly from host without
		// an internal shim layer.
		def := l.matchComponentImport(moduleName, resolvedImports)
		if def != nil {
			// Check if we already have a cached host module
			if synth, exists := syntheticModules[moduleName]; exists {
				return synth
			}

			// Create a host module from the definition
			hostMod := l.createHostModule(ctx, moduleName, def, inst)
			if hostMod != nil {
				syntheticModules[moduleName] = hostMod
				return hostMod
			}
		}

		return nil
	}
}

// resolveInlineInstanceSource resolves an inline core instance to its underlying source module.
// This handles inline instances like: (core instance (export "" (func 0)))
// where func 0 is an alias to a real core instance's export.
//
// For inline instances where all exports come from a single source module AND the export names
// match the original names, we can return the source module directly. Otherwise, we return nil
// as creating a proper wrapper module that remaps export names requires engine-level support
// that is not currently implemented.
//
// LIMITATION: This implementation only works when:
// 1. All inline exports come from the same source core instance
// 2. The inline export names match the original export names in the source module
//
// Components that re-export functions with different names will not work correctly and
// will return nil, causing instantiation to fail with a clear error rather than panic.
func (l *ComponentLinker) resolveInlineInstanceSource(inst *Instance, c *Component, inlineInst *CoreInstance) api.Module {
	if len(inlineInst.InlineExports) == 0 {
		return nil
	}

	// Build a mapping of export names to source (instanceIdx, exportName)
	// For each inline export, trace through the alias to find the source
	type exportSource struct {
		instanceIdx uint32
		exportName  string
	}
	exportMapping := make(map[string]exportSource)

	// Build maps from target index to alias source.
	// Each alias has an Idx field that represents the target index in the
	// appropriate core index space (func, memory, etc.). This Idx is assigned
	// during binary decoding and accounts for all operations that consume
	// indices in these spaces (aliases, canon lower, canon resource.*, etc.).
	funcAliases := make(map[uint32]exportSource)
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortFunc {
			funcAliases[alias.Idx] = exportSource{alias.InstanceIdx, alias.ExportName}
		}
	}

	// Also track memory aliases
	memAliases := make(map[uint32]exportSource)
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortMemory {
			memAliases[alias.Idx] = exportSource{alias.InstanceIdx, alias.ExportName}
		}
	}

	// Map inline exports to their sources
	var primaryInstanceIdx uint32 = 0xFFFFFFFF
	allSameSource := true
	namesMatch := true

	for _, exp := range inlineInst.InlineExports {
		var src exportSource
		var ok bool
		switch exp.Sort {
		case CoreSortFunc:
			src, ok = funcAliases[exp.Idx]
		case CoreSortMemory:
			src, ok = memAliases[exp.Idx]
		default:
			continue
		}

		if !ok {
			continue
		}

		exportMapping[exp.Name] = src

		// Check if all exports come from the same source instance
		if primaryInstanceIdx == 0xFFFFFFFF {
			primaryInstanceIdx = src.instanceIdx
		} else if src.instanceIdx != primaryInstanceIdx {
			allSameSource = false
		}

		// Check if the export name matches the source name
		if exp.Name != src.exportName {
			namesMatch = false
		}
	}

	if len(exportMapping) == 0 {
		return nil
	}

	// Find the primary source module
	if primaryInstanceIdx == 0xFFFFFFFF || int(primaryInstanceIdx) >= len(inst.coreInstances) {
		return nil
	}

	primarySource := inst.coreInstances[primaryInstanceIdx]
	if primarySource == nil {
		return nil
	}

	// If all exports come from the same source and names match, return the source directly.
	// This is the safe path that doesn't require creating a wrapper.
	if allSameSource && namesMatch {
		return primarySource
	}

	// If all exports come from the same source but names don't match, we need to
	// add the remapped export names to the source module's Exports map.
	// This allows the import resolver to find the exports under the new names.
	if allSameSource {
		// Try to cast to *wasm.ModuleInstance to access the Exports map
		modInst, ok := primarySource.(*wasm.ModuleInstance)
		if !ok {
			// Cannot add remapped exports if we don't have access to the ModuleInstance
			return nil
		}

		// Add remapped exports for any names that don't already exist
		for newName, src := range exportMapping {
			if _, exists := modInst.Exports[newName]; exists {
				// Export with this name already exists, no need to add
				continue
			}

			// Find the original export to get the index and type
			origExport, exists := modInst.Exports[src.exportName]
			if !exists {
				// Original export not found, can't create remapping
				return nil
			}

			// Add a new export entry with the new name pointing to the same function/memory
			// Note: The Index in the export is the function index in the module (imports + locals).
			// For modules without imports, this is the same as the local function index.
			modInst.Exports[newName] = &wasm.Export{
				Type:  origExport.Type,
				Name:  newName,
				Index: origExport.Index,
			}
		}

		return primarySource
	}

	// Cannot safely resolve this inline instance - exports come from different
	// source instances, which would require creating a wrapper module.
	return nil
}

func (l *ComponentLinker) instantiateCoreModule(
	ctx context.Context,
	inst *Instance,
	c *Component,
	coreInst *CoreInstance,
	compiledModules []CompiledModuleCloser,
	instanceIdx int,
	resolvedImports map[string]Definition,
) (api.Module, error) {
	moduleIdx := coreInst.ModuleIdx
	if int(moduleIdx) >= len(compiledModules) {
		return nil, fmt.Errorf("invalid module index: %d", moduleIdx)
	}

	compiled := compiledModules[moduleIdx]
	if compiled == nil {
		return nil, fmt.Errorf("module %d not compiled", moduleIdx)
	}

	// If no runtime provided, we can't instantiate core modules
	if l.runtime == nil {
		return nil, fmt.Errorf("no runtime available for module instantiation")
	}

	// Build an import resolver for this instance to resolve imports from
	// previously instantiated core instances AND component-level imports.
	// We always build a resolver now since we need to handle component imports
	// even when there are no inter-core-instance args.
	resolver := l.buildImportResolver(ctx, inst, c, coreInst, resolvedImports)
	ctx = experimental.WithImportResolver(ctx, resolver)

	// Try to use the runtime to instantiate the module
	// The runtime should implement CoreModuleInstantiator interface
	if mi, ok := l.runtime.(CoreModuleInstantiator); ok {
		return mi.InstantiateCoreModule(ctx, compiled)
	}

	return nil, fmt.Errorf("runtime does not support module instantiation (type: %T)", l.runtime)
}

func (l *ComponentLinker) wireExportedFunc(
	inst *Instance,
	c *Component,
	exp *Export,
	funcSpace *CoreFuncIndexSpace,
	memSpace *CoreMemoryIndexSpace,
) (*ExportedFunc, error) {
	// exp.Idx is the component function index, not the canonical array index.
	// We need to look up the canonical index using the FuncIdxToCanonical map.
	funcIdx := exp.Idx
	canonIdx, ok := c.FuncIdxToCanonical[funcIdx]
	if !ok {
		return nil, fmt.Errorf("no canonical for function index: %d", funcIdx)
	}
	if int(canonIdx) >= len(c.Canonicals) {
		return nil, fmt.Errorf("invalid canonical index: %d", canonIdx)
	}

	canon := &c.Canonicals[canonIdx]
	if canon.Kind != CanonKindLift {
		return nil, fmt.Errorf("export %q canonical is not a lift", exp.Name)
	}

	coreFunc, err := l.resolveCoreFunc(inst, c, canon.CoreFuncIdx, funcSpace)
	if err != nil {
		return nil, fmt.Errorf("resolve core func %d: %w", canon.CoreFuncIdx, err)
	}

	var funcType *FuncType
	if int(canon.TypeIdx) < len(c.Types) && c.Types[canon.TypeIdx].Kind == TypeDefKindFunc {
		funcType = c.Types[canon.TypeIdx].Func
	}

	var memory api.Memory
	if canon.Options.MemoryIdx != nil {
		memory, err = l.resolveCoreMemory(inst, c, *canon.Options.MemoryIdx, memSpace)
		if err != nil {
			return nil, fmt.Errorf("resolve memory: %w", err)
		}
	}

	var reallocFunc api.Function
	if canon.Options.ReallocIdx != nil {
		reallocFunc, err = l.resolveCoreFunc(inst, c, *canon.Options.ReallocIdx, funcSpace)
		if err != nil {
			return nil, fmt.Errorf("resolve realloc: %w", err)
		}
	}

	return &ExportedFunc{
		name:        exp.Name,
		funcType:    funcType,
		coreFunc:    coreFunc,
		canonical:   canon,
		component:   c,
		instance:    inst,
		memory:      memory,
		reallocFunc: reallocFunc,
	}, nil
}

func (l *ComponentLinker) resolveCoreFunc(inst *Instance, c *Component, funcIdx uint32, funcSpace *CoreFuncIndexSpace) (api.Function, error) {
	instanceIdx, exportName, err := funcSpace.Resolve(funcIdx)
	if err != nil {
		// Fallback: first core instance
		if len(inst.coreInstances) == 0 {
			return nil, fmt.Errorf("no core instances available")
		}
		coreInst := inst.coreInstances[0]
		if coreInst == nil {
			return nil, fmt.Errorf("core instance not instantiated")
		}
		for name := range coreInst.ExportedFunctionDefinitions() {
			return coreInst.ExportedFunction(name), nil
		}
		return nil, fmt.Errorf("no exported functions")
	}

	if int(instanceIdx) >= len(inst.coreInstances) {
		return nil, fmt.Errorf("core instance %d out of range", instanceIdx)
	}

	coreInst := inst.coreInstances[instanceIdx]
	if coreInst == nil {
		return nil, fmt.Errorf("core instance %d not instantiated", instanceIdx)
	}

	fn := coreInst.ExportedFunction(exportName)
	if fn == nil {
		return nil, fmt.Errorf("function %q not found", exportName)
	}

	return fn, nil
}

func (l *ComponentLinker) resolveCoreMemory(inst *Instance, c *Component, memIdx uint32, memSpace *CoreMemoryIndexSpace) (api.Memory, error) {
	instanceIdx, exportName, err := memSpace.Resolve(memIdx)
	if err != nil {
		// Fallback
		if len(inst.coreInstances) == 0 {
			return nil, fmt.Errorf("no core instances available")
		}
		coreInst := inst.coreInstances[0]
		if coreInst == nil {
			return nil, fmt.Errorf("core instance not instantiated")
		}
		mem := coreInst.Memory()
		if mem == nil {
			return nil, fmt.Errorf("no default memory")
		}
		return mem, nil
	}

	if int(instanceIdx) >= len(inst.coreInstances) {
		return nil, fmt.Errorf("core instance %d out of range", instanceIdx)
	}

	coreInst := inst.coreInstances[instanceIdx]
	if coreInst == nil {
		return nil, fmt.Errorf("core instance %d not instantiated", instanceIdx)
	}

	mem := coreInst.ExportedMemory(exportName)
	if mem == nil {
		return nil, fmt.Errorf("memory %q not found", exportName)
	}

	return mem, nil
}

// MatchImport finds a definition that satisfies the import name.
func (l *ComponentLinker) MatchImport(importName string) (Definition, error) {
	// Create a temporary Linker to reuse its MatchImport logic
	linker := &Linker{definitions: l.definitions, relaxedSemver: l.relaxedSemver}
	return linker.MatchImport(importName)
}

// Get retrieves a definition by its full key.
// Use this for direct lookups of instances or other definitions by exact key.
func (l *ComponentLinker) Get(key string) (Definition, bool) {
	def, ok := l.definitions[key]
	return def, ok
}

// MergeFrom copies all definitions from a Linker into this ComponentLinker.
// This allows using WASI interfaces registered on a Linker with a ComponentLinker
// that has runtime integration for core module instantiation.
func (l *ComponentLinker) MergeFrom(linker *Linker) {
	for key, def := range linker.definitions {
		l.definitions[key] = def
	}
}

// matchComponentImport tries to find a component-level import that matches the given
// module name using semver matching. This handles cases where a core module imports
// from a component interface like "wasi:filesystem/preopens@0.2.2".
//
// The matching algorithm:
// 1. Try exact match in resolvedImports first
// 2. Try semver-compatible matching in resolvedImports
// 3. If not found, try matching against linker definitions directly
func (l *ComponentLinker) matchComponentImport(moduleName string, resolvedImports map[string]Definition) Definition {
	// Try exact match in resolvedImports first
	if def, ok := resolvedImports[moduleName]; ok {
		return def
	}

	// Parse the requested module name to extract base and version
	// Module names can be like "wasi:filesystem/preopens@0.2.2"
	baseReq, reqVersionStr, hasReqVersion := SplitVersion(moduleName)
	if !hasReqVersion {
		// No version in request, try exact match in linker definitions
		if def, ok := l.definitions[moduleName]; ok {
			return def
		}
		return nil
	}

	reqVersion, err := ParseSemver(reqVersionStr)
	if err != nil {
		return nil
	}

	// Find best compatible match in resolvedImports
	var bestDef Definition
	var bestVersion *Semver

	for importName, def := range resolvedImports {
		// Parse the import name
		baseAvail, availVersionStr, hasAvailVersion := SplitVersion(importName)
		if !hasAvailVersion {
			continue
		}

		// Check if base names match
		if baseReq != baseAvail {
			continue
		}

		availVersion, err := ParseSemver(availVersionStr)
		if err != nil {
			continue
		}

		// Check semver compatibility
		if !SemverCompatible(reqVersion, availVersion, l.relaxedSemver) {
			continue
		}

		// Select highest compatible version
		if bestVersion == nil || semverGreater(availVersion, bestVersion) {
			bestDef = def
			bestVersion = availVersion
		}
	}

	if bestDef != nil {
		return bestDef
	}

	// If not found in resolvedImports, try matching against linker definitions directly
	// This handles cases where the core module imports something that wasn't in the
	// component's declared imports but is available in the linker
	for defName, def := range l.definitions {
		// Parse the definition name
		baseAvail, availVersionStr, hasAvailVersion := SplitVersion(defName)
		if !hasAvailVersion {
			continue
		}

		// Check if base names match
		if baseReq != baseAvail {
			continue
		}

		availVersion, err := ParseSemver(availVersionStr)
		if err != nil {
			continue
		}

		// Check semver compatibility
		if !SemverCompatible(reqVersion, availVersion, l.relaxedSemver) {
			continue
		}

		// Select highest compatible version
		if bestVersion == nil || semverGreater(availVersion, bestVersion) {
			bestDef = def
			bestVersion = availVersion
		}
	}

	return bestDef
}

// createHostModule creates a real host module from a Definition using the runtime.
// This is used when core modules need to import from component-level imports
// that are defined as InstanceDef (containing FuncDef exports) or FuncDef.
func (l *ComponentLinker) createHostModule(ctx context.Context, moduleName string, def Definition, inst *Instance) api.Module {
	// Check if the runtime supports host module instantiation
	hmi, ok := l.runtime.(HostModuleInstantiator)
	if !ok {
		return nil
	}

	var exports []HostModuleExport

	switch d := def.(type) {
	case *InstanceDef:
		for name, export := range d.Exports {
			// For ResourceDef, we need to generate the resource operations
			// (drop, new, rep) with the correct naming convention
			if rdef, ok := export.(*ResourceDef); ok {
				// Generate [resource-drop]<name> function
				exports = append(exports, l.createResourceDropExport(name, rdef, inst))
				// Generate [resource-new]<name> function
				exports = append(exports, l.createResourceNewExport(name, inst))
				// Generate [resource-rep]<name> function
				exports = append(exports, l.createResourceRepExport(name, inst))
			} else {
				// Regular function or other definition
				exp := l.createHostModuleExport(ctx, name, export, inst)
				if exp != nil {
					exports = append(exports, *exp)
				}
			}
		}
	case *FuncDef:
		// Single function - wrap it as a module with one export
		funcName := extractFuncName(moduleName)
		exp := l.createHostModuleExport(ctx, funcName, d, inst)
		if exp != nil {
			exports = append(exports, *exp)
		}
	default:
		return nil
	}

	if len(exports) == 0 {
		return nil
	}

	// Use empty module name to avoid registering in the store
	// We'll provide this module through the import resolver only
	mod, err := hmi.InstantiateHostModule(ctx, "", exports)
	if err != nil {
		return nil
	}
	return mod
}

// createResourceDropExport creates a [resource-drop]<name> export for a resource.
func (l *ComponentLinker) createResourceDropExport(name string, rdef *ResourceDef, inst *Instance) HostModuleExport {
	return HostModuleExport{
		Name:        "[resource-drop]" + name,
		ParamTypes:  []api.ValueType{api.ValueTypeI32},
		ResultTypes: []api.ValueType{},
		Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			handle := uint32(stack[0])
			entry, err := inst.resourceTable.Remove(Handle(handle))
			if err != nil {
				return // Silently ignore invalid handles per spec
			}
			if rdef.Destructor != nil && entry.Rep != nil {
				switch rep := entry.Rep.(type) {
				case uint32:
					rdef.Destructor(rep)
				case int:
					rdef.Destructor(uint32(rep))
				}
			}
		}),
	}
}

// createResourceNewExport creates a [resource-new]<name> export for a resource.
func (l *ComponentLinker) createResourceNewExport(name string, inst *Instance) HostModuleExport {
	return HostModuleExport{
		Name:        "[resource-new]" + name,
		ParamTypes:  []api.ValueType{api.ValueTypeI32},
		ResultTypes: []api.ValueType{api.ValueTypeI32},
		Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			rep := uint32(stack[0])
			handle := inst.resourceTable.New(rep, true)
			stack[0] = uint64(handle)
		}),
	}
}

// createResourceRepExport creates a [resource-rep]<name> export for a resource.
func (l *ComponentLinker) createResourceRepExport(name string, inst *Instance) HostModuleExport {
	return HostModuleExport{
		Name:        "[resource-rep]" + name,
		ParamTypes:  []api.ValueType{api.ValueTypeI32},
		ResultTypes: []api.ValueType{api.ValueTypeI32},
		Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			handle := uint32(stack[0])
			rep, err := inst.resourceTable.Rep(Handle(handle))
			if err != nil {
				stack[0] = 0
				return
			}
			stack[0] = uint64(rep)
		}),
	}
}

// createHostModuleExport creates a single HostModuleExport from a FuncDef.
// ResourceDef is handled separately in createHostModule.
func (l *ComponentLinker) createHostModuleExport(ctx context.Context, name string, def Definition, inst *Instance) *HostModuleExport {
	// Only handle FuncDef - ResourceDef is handled in createHostModule
	funcDef, ok := def.(*FuncDef)
	if !ok {
		return nil
	}

	// Create a lowered function using the CanonLower mechanism
	lowered := CanonLower(funcDef.Callback, funcDef.Type, nil)
	lowered.SetInstance(inst)

	// Determine the parameter and result types from the lowered function's core type
	paramTypes, resultTypes := lowered.CoreSignature()

	return &HostModuleExport{
		Name:        name,
		ParamTypes:  paramTypes,
		ResultTypes: resultTypes,
		Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			// Add resource table to context
			ctx = WithResourceTable(ctx, inst.resourceTable)
			lowered.CallWithStack(ctx, stack)
		}),
	}
}

// extractFuncName extracts the function name from a full import path.
// e.g., "wasi:cli/environment@0.2.0/get-environment" -> "get-environment"
func extractFuncName(importPath string) string {
	lastSlash := strings.LastIndex(importPath, "/")
	if lastSlash == -1 {
		return importPath
	}
	return importPath[lastSlash+1:]
}

