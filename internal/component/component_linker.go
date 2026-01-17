// internal/component/component_linker.go
package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// ComponentLinker resolves component imports and instantiates components
// with full runtime integration for core module instantiation.
type ComponentLinker struct {
	runtime     any // wazero.Runtime - stored as any to avoid import cycle
	definitions map[string]Definition
}

// NewComponentLinker creates a new component linker with access to a runtime.
// The runtime parameter should be a wazero.Runtime instance.
func NewComponentLinker(rt any) *ComponentLinker {
	return &ComponentLinker{
		runtime:     rt,
		definitions: make(map[string]Definition),
	}
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

	// Build index spaces from aliases
	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()

	for i, alias := range c.Aliases {
		if alias.Kind == AliasKindCoreExport {
			switch alias.CoreSort {
			case CoreSortFunc:
				funcSpace.AddAlias(uint32(i), alias.InstanceIdx, alias.ExportName)
			case CoreSortMemory:
				memSpace.AddAlias(uint32(i), alias.InstanceIdx, alias.ExportName)
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
			module, err := l.instantiateCoreModule(ctx, inst, c, &coreInst, compiledModules, i)
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
func (l *ComponentLinker) buildImportResolver(inst *Instance, c *Component, coreInst *CoreInstance) experimental.ImportResolver {
	// Build a map from import module name to source instance index
	importMap := make(map[string]uint32)
	for _, arg := range coreInst.Args {
		importMap[arg.Name] = arg.InstanceIdx
	}

	return func(moduleName string) api.Module {
		// Look up the instance index for this import module name
		instanceIdx, ok := importMap[moduleName]
		if !ok {
			return nil
		}

		// Check if we have a direct core instance
		if int(instanceIdx) < len(inst.coreInstances) {
			coreInstance := inst.coreInstances[instanceIdx]
			if coreInstance != nil {
				return coreInstance
			}
		}

		// The instance might be an inline instance - we need to trace through
		// the inline exports to find the actual source.
		// An inline instance exports items from the alias index space.
		if int(instanceIdx) < len(c.CoreInstances) {
			srcCoreInst := &c.CoreInstances[instanceIdx]
			if srcCoreInst.Kind == CoreInstanceExprInline {
				// For inline instances, find the actual source instance
				// through the inline exports.
				// Each inline export references an alias, and we need to
				// find what core instance the alias points to.
				return l.resolveInlineInstanceSource(inst, c, srcCoreInst)
			}
		}

		return nil
	}
}

// resolveInlineInstanceSource creates a virtual module wrapper that re-exports
// functions from real core instances with renamed export names.
// This handles inline instances like: (core instance (export "" (func 0)))
// where func 0 is an alias to a real core instance's export.
//
// NOTE: This is a complex feature that requires creating a wrapper module
// that remaps export names. The current implementation has limitations when
// the importing module's engine tries to resolve the function references.
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

	// Track the core function index space (populated by aliases)
	// Each alias with CoreSortFunc adds to this space
	funcAliases := make(map[uint32]exportSource)
	funcIdx := uint32(0)
	for _, alias := range c.Aliases {
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortFunc {
			funcAliases[funcIdx] = exportSource{alias.InstanceIdx, alias.ExportName}
			funcIdx++
		}
	}

	// Map inline exports to their sources
	for _, exp := range inlineInst.InlineExports {
		if exp.Sort != CoreSortFunc {
			continue
		}
		if src, ok := funcAliases[exp.Idx]; ok {
			exportMapping[exp.Name] = src
		}
	}

	if len(exportMapping) == 0 {
		return nil
	}

	// Find the primary source instance (typically all exports come from one instance)
	var primarySourceIdx uint32
	var primarySource *wasm.ModuleInstance
	for _, src := range exportMapping {
		if int(src.instanceIdx) < len(inst.coreInstances) {
			srcMod := inst.coreInstances[src.instanceIdx]
			if srcMod == nil {
				continue
			}
			// Cast to *wasm.ModuleInstance
			if mi, ok := srcMod.(*wasm.ModuleInstance); ok {
				primarySourceIdx = src.instanceIdx
				primarySource = mi
				break
			}
		}
	}

	if primarySource == nil {
		return nil
	}

	// Create a wrapper module instance that has remapped exports
	// The wrapper shares the engine, source, etc. with the original
	// but has a new Exports map with the renamed entries
	wrapper := &wasm.ModuleInstance{
		ModuleName:       fmt.Sprintf("inline_wrapper_%p", inlineInst),
		Exports:          make(map[string]*wasm.Export),
		Globals:          primarySource.Globals,
		MemoryInstance:   primarySource.MemoryInstance,
		Tables:           primarySource.Tables,
		Engine:           primarySource.Engine,
		TypeIDs:          primarySource.TypeIDs,
		DataInstances:    primarySource.DataInstances,
		ElementInstances: primarySource.ElementInstances,
		Source:           primarySource.Source,
	}

	// Build the remapped exports
	for newName, src := range exportMapping {
		var srcMod *wasm.ModuleInstance
		if src.instanceIdx == primarySourceIdx {
			srcMod = primarySource
		} else if int(src.instanceIdx) < len(inst.coreInstances) {
			if mi, ok := inst.coreInstances[src.instanceIdx].(*wasm.ModuleInstance); ok {
				srcMod = mi
			}
		}

		if srcMod == nil {
			continue
		}

		// Find the original export in the source module
		if origExp, ok := srcMod.Exports[src.exportName]; ok {
			// Create a new export with the new name but same index and type
			wrapper.Exports[newName] = &wasm.Export{
				Type:  origExp.Type,
				Name:  newName,
				Index: origExp.Index,
			}
		}
	}

	return wrapper
}

func (l *ComponentLinker) instantiateCoreModule(
	ctx context.Context,
	inst *Instance,
	c *Component,
	coreInst *CoreInstance,
	compiledModules []CompiledModuleCloser,
	instanceIdx int,
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
	// previously instantiated core instances
	if len(coreInst.Args) > 0 {
		resolver := l.buildImportResolver(inst, c, coreInst)
		ctx = experimental.WithImportResolver(ctx, resolver)
	}

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
	canonIdx := exp.Idx
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
	linker := &Linker{definitions: l.definitions}
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
