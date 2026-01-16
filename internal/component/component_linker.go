// internal/component/component_linker.go
package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
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

// runtimeInstantiator is an interface for runtime module instantiation.
// This allows us to work with wazero.Runtime without importing it directly.
type runtimeInstantiator interface {
	InstantiateModule(ctx context.Context, compiled api.Closer, config any) (api.Module, error)
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

	// Use reflection-free interface check for the runtime
	// The runtime should implement InstantiateModule method
	moduleName := fmt.Sprintf("component_core_%d", instanceIdx)

	// Try to use the runtime to instantiate the module
	// This requires the runtime to have the InstantiateModule method
	type moduleInstantiator interface {
		InstantiateModule(ctx context.Context, compiled any, config any) (api.Module, error)
	}

	if mi, ok := l.runtime.(moduleInstantiator); ok {
		// Create module config - we pass nil and let the runtime use defaults
		return mi.InstantiateModule(ctx, compiled, nil)
	}

	// Alternative: try a simpler interface that wazero.Runtime implements
	type simpleInstantiator interface {
		InstantiateModule(context.Context, CompiledModuleCloser, any) (api.Module, error)
	}

	if si, ok := l.runtime.(simpleInstantiator); ok {
		return si.InstantiateModule(ctx, compiled, nil)
	}

	return nil, fmt.Errorf("runtime does not support module instantiation (type: %T), module name: %s", l.runtime, moduleName)
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
