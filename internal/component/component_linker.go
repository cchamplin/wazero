// internal/component/component_linker.go
package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// MaxFlatResults is the maximum number of flattened result values
// that can be returned directly (for synchronous calls).
// Beyond this, results spill to memory via a return pointer.
const MaxFlatResults = 1

// canonLowerInfo stores information about each canon lower operation
// so we can create the corresponding core function with the correct signature.
type canonLowerInfo struct {
	compFuncIdx uint32            // Component function being lowered
	options     *CanonicalOptions // Memory, realloc, encoding
	coreFuncIdx uint32            // Resulting core function index
}

// canonResourceInfo stores information about a canonical resource operation
// (resource.new, resource.drop, resource.rep).
type canonResourceInfo struct {
	kind        CanonKind // ResourceNew, ResourceDrop, or ResourceRep
	typeIdx     uint32    // Resource type index
	coreFuncIdx uint32    // Resulting core function index
}

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

// DefineValue adds a value definition for value imports.
func (l *ComponentLinker) DefineValue(namespace, name string, value Val) error {
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

// SkipValidation disables validation for this instance definition.
// Use this when providing a partial implementation of a WASI interface.
func (b *ComponentInstanceBuilder) SkipValidation() *ComponentInstanceBuilder {
	b.skipValidation = true
	return b
}

// Build finalizes the instance definition.
// Validation is enabled by default to catch missing required exports.
// Use SkipValidation() to disable for partial implementations.
func (b *ComponentInstanceBuilder) Build() error {
	if _, exists := b.linker.definitions[b.namespace]; exists {
		return fmt.Errorf("definition already exists: %s", b.namespace)
	}
	b.linker.definitions[b.namespace] = &InstanceDef{Exports: b.exports, SkipValidation: b.skipValidation}
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

	// Step 1: Validate imports with type checking and build instance-to-import mapping
	typeChecker := NewTypeChecker(c)
	resolvedImports := make(map[string]Definition)
	// instanceToImport maps component instance indices to import names.
	// Instance imports (ImportExternDescInstance) contribute to the component instance index space.
	instanceToImport := make(map[uint32]string)
	compInstanceIdx := uint32(0)

	for _, imp := range c.Imports {
		def, err := l.MatchImport(imp.Name)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", imp.Name, err)
		}

		// TYPE CHECK: Validate definition matches expected type
		if err := typeChecker.CheckDefinition(&imp.ExternDesc, imp.Name, def); err != nil {
			return nil, fmt.Errorf("import %q type mismatch: %w", imp.Name, err)
		}

		resolvedImports[imp.Name] = def

		// Instance imports create entries in the component instance index space
		if imp.ExternDesc.Kind == ImportExternDescInstance {
			instanceToImport[compInstanceIdx] = imp.Name
			compInstanceIdx++
		}
	}

	// Process value imports to populate value index space
	for _, imp := range c.Imports {
		if imp.ExternDesc.Kind == ImportExternDescValue {
			def := resolvedImports[imp.Name]
			valueDef, ok := def.(*ImportedValueDef)
			if !ok {
				return nil, fmt.Errorf("import %q: expected value, got %T", imp.Name, def)
			}
			inst.AddValue(valueDef.Value)
		}
	}

	// Instance imports occupy slots in the component instance index space.
	// Add placeholder entries so that subsequent component instance indices
	// are correctly aligned (e.g., if there are 19 imported instances,
	// the first component instance should be at index 19, not 0).
	for _, imp := range c.Imports {
		if imp.ExternDesc.Kind == ImportExternDescInstance {
			inst.AddInstanceToSpace(nil)
		}
	}

	// Build component function index space from aliases and canon lift operations.
	// This must happen before processing nested component instances, because
	// resolveFromParentScope needs componentFuncs to be populated when resolving
	// function arguments for nested component instantiation.
	inst.componentFuncs = make(map[uint32]ComponentFunc)
	l.buildComponentFuncs(inst, c, resolvedImports, instanceToImport)

	// Build the type index space so that resolveFromParentScope can find types
	// referenced by nested component instantiation arguments.
	l.buildTypeSpace(inst, c)

	// Process nested component instances.
	// Track the mapping from instance-space index to component instance definition
	// so we can wire shim exports later during export processing.
	componentInstDefs := make(map[uint32]*ComponentInstance)
	for i := range c.ComponentInstances {
		compInst := &c.ComponentInstances[i]
		if compInst.Kind == ComponentInstanceExprInstantiate {
			nestedInst, err := l.instantiateNestedComponent(ctx, inst, compInst, c)
			if err != nil {
				return nil, fmt.Errorf("component instance %d: %w", i, err)
			}
			idx := inst.AddInstanceToSpace(nestedInst)
			componentInstDefs[idx] = compInst
		}
		// Handle inline component instances if needed (future enhancement)
		compInstanceIdx++
	}

	// Track canon lower operations by their resulting core function index.
	// Each canon lower produces a core function that wraps a component function.
	// We need to track this so inline instances that export canon lower functions
	// can create proper host modules with correct signatures.
	// The ComponentFuncIdx field stores the assigned core function index for Lower
	// and resource operations (assigned during binary decoding).
	canonLowers := make(map[uint32]canonLowerInfo)
	canonResources := make(map[uint32]canonResourceInfo)
	for _, canon := range c.Canonicals {
		switch canon.Kind {
		case CanonKindLower:
			// canon.ComponentFuncIdx contains the assigned core function index
			canonLowers[canon.ComponentFuncIdx] = canonLowerInfo{
				compFuncIdx: canon.FuncIdx,
				options:     &canon.Options,
				coreFuncIdx: canon.ComponentFuncIdx,
			}
		case CanonKindResourceNew, CanonKindResourceDrop, CanonKindResourceRep:
			// Track resource operations by their assigned core function index
			canonResources[canon.ComponentFuncIdx] = canonResourceInfo{
				kind:        canon.Kind,
				typeIdx:     canon.TypeIdx,
				coreFuncIdx: canon.ComponentFuncIdx,
			}
			// CanonKindLift produces component funcs, not core funcs
		}
	}

	// Build function alias map for inline instance resolution
	funcAliases := make(map[uint32]struct {
		instanceIdx uint32
		exportName  string
	})
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortFunc {
			funcAliases[alias.Idx] = struct {
				instanceIdx uint32
				exportName  string
			}{alias.InstanceIdx, alias.ExportName}
		}
	}

	// Step 2: Instantiate core modules
	// We also wire up memory sharing incrementally after each module that has memory is created.
	// This is necessary because later modules may call functions (like _initialize) that need
	// memory access before all modules are instantiated.
	if len(c.CoreInstances) > 0 {
		// Explicit core instance descriptors present -- use them.
		for i, coreInst := range c.CoreInstances {
			switch coreInst.Kind {
			case CoreInstanceExprInstantiate:
				module, err := l.instantiateCoreModule(ctx, inst, c, &coreInst, compiledModules, i, resolvedImports, canonLowers, canonResources, funcAliases)
				if err != nil {
					return nil, fmt.Errorf("instantiate core instance %d: %w", i, err)
				}
				inst.coreInstances = append(inst.coreInstances, module)

				// If this module has memory, immediately wire up memory sharing with other modules.
				// This ensures that any subsequent module instantiation that triggers function calls
				// (like _initialize) will have access to memory.
				if modInst, ok := module.(*wasm.ModuleInstance); ok && modInst.MemoryInstance != nil {
					if err := l.wireMemorySharing(inst, c); err != nil {
						return nil, fmt.Errorf("wire memory sharing after instance %d: %w", i, err)
					}
				}
			case CoreInstanceExprInline:
				inst.coreInstances = append(inst.coreInstances, nil)
			}
		}
	} else if len(compiledModules) > 0 {
		// No explicit core instance descriptors but compiled modules exist.
		// Directly instantiate each compiled module (simpler component format).
		mi, ok := l.runtime.(CoreModuleInstantiator)
		if !ok {
			return nil, fmt.Errorf("runtime does not support module instantiation (type: %T)", l.runtime)
		}
		for i, cm := range compiledModules {
			if cm == nil {
				return nil, fmt.Errorf("module %d not compiled", i)
			}
			module, err := mi.InstantiateCoreModule(ctx, cm)
			if err != nil {
				return nil, fmt.Errorf("instantiate core module %d: %w", i, err)
			}
			inst.coreInstances = append(inst.coreInstances, module)
		}
	}

	// Step 2.5: Execute start function if defined
	if err := l.executeStartFunction(ctx, inst, c); err != nil {
		return nil, fmt.Errorf("start function: %w", err)
	}

	// Step 3: Wire exports
	for _, exp := range c.Exports {
		switch exp.Kind {
		case ExportKindFunc:
			exportedFunc, err := l.wireExportedFunc(inst, c, &exp, funcSpace, memSpace)
			if err != nil {
				return nil, fmt.Errorf("wire export %q: %w", exp.Name, err)
			}
			inst.exports[exp.Name] = exportedFunc

		case ExportKindInstance:
			// Look up instance from instance index space
			exportedInst := inst.GetInstanceFromSpace(exp.Idx)
			if exportedInst != nil {
				// If this instance came from a component instance (shim pattern),
				// wire its exports by tracing through the shim's import/export
				// mapping back to the parent's canon lifts.
				if compInstDef, ok := componentInstDefs[exp.Idx]; ok {
					if err := l.wireNestedComponentExports(inst, c, exportedInst, compInstDef, funcSpace, memSpace); err != nil {
						return nil, fmt.Errorf("wire nested exports for %q: %w", exp.Name, err)
					}
				}
				inst.AddExportedInstance(exp.Name, exportedInst)
			}
		}
	}

	return inst, nil
}

// executeStartFunction executes the component's start function if defined.
// Called after core module instantiation, before wiring exports.
func (l *ComponentLinker) executeStartFunction(ctx context.Context, inst *Instance, c *Component) error {
	if c.Start == nil {
		return nil // No start function
	}

	// Get the start function
	startFunc, ok := inst.componentFuncs[c.Start.FuncIdx]
	if !ok {
		return fmt.Errorf("start function %d not found", c.Start.FuncIdx)
	}
	if startFunc.Impl == nil {
		return fmt.Errorf("start function %d has no implementation", c.Start.FuncIdx)
	}

	// Gather value arguments and mark as consumed
	args := make([]Val, len(c.Start.ArgValueIdx))
	for i, argIdx := range c.Start.ArgValueIdx {
		val, err := inst.ConsumeValue(argIdx)
		if err != nil {
			return fmt.Errorf("start arg %d: %w", i, err)
		}
		args[i] = val
	}

	// Call start function
	results, err := startFunc.Impl(ctx, args)
	if err != nil {
		return fmt.Errorf("start function failed: %w", err)
	}

	// Verify result count matches declaration
	if uint32(len(results)) != c.Start.ResultCount {
		return fmt.Errorf("start function returned %d values, expected %d",
			len(results), c.Start.ResultCount)
	}

	// Append results to value index space
	for _, r := range results {
		inst.AddValue(r)
	}

	return nil
}

// buildComponentFuncs populates the componentFuncs map with component-level functions.
// These come from two sources:
// 1. Component-level aliases (AliasKindExport with SortFunc) that reference imported instance exports
// 2. Canon lift operations that create component functions from core functions
//
// The component function index space is built during binary decoding and tracked via
// NextFuncIdx. This method populates the runtime implementations for those indices.
//
// Note: The Alias.Idx field is only populated for core export aliases. For component-level
// function aliases, we need to track the component function index based on the order of
// operations that consume indices from the component function space.
func (l *ComponentLinker) buildComponentFuncs(inst *Instance, c *Component, resolvedImports map[string]Definition, instanceToImport map[uint32]string) {
	// First pass: Process canon lift operations.
	// Each canon lift creates a component function from a core function.
	// The ComponentFuncIdx is assigned during binary decoding and is authoritative.
	for _, canon := range c.Canonicals {
		if canon.Kind == CanonKindLift {
			// Canon lift creates a component function.
			// The implementation will be wired up later when we have the core function.
			// For now, we just record that this function index exists.
			// The actual implementation is in wireExportedFunc which handles lifting.
			// Note: We don't populate Impl here because canon lift wraps a core function,
			// not a host function. The ExportedFunc.Call method handles the lifting.

			// Get the function type if available
			var funcType *FuncType
			if int(canon.TypeIdx) < len(c.Types) && c.Types[canon.TypeIdx].Kind == TypeDefKindFunc {
				funcType = c.Types[canon.TypeIdx].Func
			}
			inst.componentFuncs[canon.ComponentFuncIdx] = ComponentFunc{
				Type: funcType,
				Impl: nil, // Core function lifting is handled by ExportedFunc
			}
		}
	}

	// Second pass: Process component-level aliases that reference imported instance exports.
	// For these aliases, we need to determine the component function index based on the
	// Idx field which represents the target index this alias creates.
	// Note: Currently the decoder only sets Idx for core export aliases. For component-level
	// function aliases, the Idx field represents the component function index assigned during
	// binary decoding (see alias.go decodeAliasSection).
	for _, alias := range c.Aliases {
		if alias.Kind == AliasKindExport && alias.Sort == SortFunc {
			// This aliases a function from a component instance (typically an imported instance).
			// Look up which import this instance refers to.
			importName, ok := instanceToImport[alias.InstanceIdx]
			if !ok {
				// Instance might be a ComponentInstance definition, not an import.
				// For now, we only handle imported instances.
				continue
			}

			// Get the resolved import definition
			def, ok := resolvedImports[importName]
			if !ok {
				continue
			}

			// The import should be an InstanceDef with function exports
			instDef, ok := def.(*InstanceDef)
			if !ok {
				continue
			}

			// Look up the specific function export by name
			exportDef, ok := instDef.Exports[alias.ExportName]
			if !ok {
				continue
			}

			// The export should be a FuncDef
			funcDef, ok := exportDef.(*FuncDef)
			if !ok {
				continue
			}

			// Try to get the function type from the component's type system.
			// This is needed when the host uses FuncNoType() and doesn't provide type info.
			funcType := funcDef.Type
			if funcType == nil {
				funcType = l.lookupFuncTypeFromImport(c, alias.InstanceIdx, alias.ExportName)
			}

			// Add to component functions.
			// For component-level aliases with SortFunc, alias.Idx should be the
			// component function index assigned during decoding.
			// Note: The decoder needs to be updated to track this properly.
			// For now, we use alias.Idx as the component function index.
			inst.componentFuncs[alias.Idx] = ComponentFunc{
				Type: funcType,
				Impl: funcDef.Callback,
			}
		}
	}
}

// lookupFuncTypeFromImport looks up the function type from the component's import type definitions.
// This is used when the host defines a function without type information (using FuncNoType).
func (l *ComponentLinker) lookupFuncTypeFromImport(c *Component, instanceIdx uint32, exportName string) *FuncType {

	// Find the import that creates this instance
	compInstanceIdx := uint32(0)
	for _, imp := range c.Imports {
		if imp.ExternDesc.Kind == ImportExternDescInstance {
			if compInstanceIdx == instanceIdx {
				// Found the import - now look up its type.
				// The import's TypeIdx references a type in the component's type index space.
				// We must use TypeIdxToStoredIdx to map from the component type index to
				// the stored index in c.Types, because type aliases consume component type
				// indices but don't add entries to c.Types.
				typeIdx := imp.ExternDesc.TypeIdx
				storedIdx := typeIdx
				if c.TypeIdxToStoredIdx != nil {
					if si, ok := c.TypeIdxToStoredIdx[typeIdx]; ok {
						storedIdx = si
					}
				}

				if int(storedIdx) < len(c.Types) {
					typeDef := &c.Types[storedIdx]
					if typeDef.Instance != nil {
						return l.lookupFuncInInstanceTypeWithOuter(typeDef.Instance, exportName, c)
					}
				}

				// Fallback: scan all instance types for one that has this export
				for i := range c.Types {
					typeDef := &c.Types[i]
					if typeDef.Instance != nil {
						if ft := l.lookupFuncInInstanceTypeWithOuter(typeDef.Instance, exportName, c); ft != nil {
							return ft
						}
					}
				}
				return nil
			}
			compInstanceIdx++
		}
	}
	return nil
}

// lookupFuncInInstanceType looks up a function type by export name within an instance type.
// It resolves nested type references using the instance type's local type namespace.
// This version doesn't have access to outer types, so it cannot resolve type aliases.
func (l *ComponentLinker) lookupFuncInInstanceType(inst *InstanceTypeDef, exportName string) *FuncType {
	return l.lookupFuncInInstanceTypeWithOuter(inst, exportName, nil)
}

// lookupFuncInInstanceTypeWithOuter looks up a function type by export name within an instance type.
// It resolves nested type references using the instance type's local type namespace and outer types.
// The component parameter provides both the Types array and the TypeIdxToStoredIdx mapping needed
// to resolve outer type aliases correctly.
func (l *ComponentLinker) lookupFuncInInstanceTypeWithOuter(inst *InstanceTypeDef, exportName string, c *Component) *FuncType {
	// Build the local type index space for this instance type
	localTypes := buildLocalTypeIndex(inst, c)

	for _, decl := range inst.Declarations {
		if decl.Kind == InstanceDeclKindExport && decl.Export != nil {
			if decl.Export.Name == exportName && decl.Export.Kind == ExportKindFunc {
				// For function exports, Idx is the type index in the local type space
				funcTypeIdx := decl.Export.Idx
				// Look up the type in the local type space
				td := localTypes[funcTypeIdx]
				if td != nil && td.Kind == TypeDefKindFunc && td.Func != nil {
					// Resolve the FuncType's parameter and result types using local types
					return resolveFuncType(td.Func, localTypes)
				}
				return nil
			}
		}
	}
	return nil
}

// buildLocalTypeIndex builds a map from type index to TypeDef for an instance type.
// In instance type definitions, each declaration that adds to the type index space
// (type declarations and type aliases) increments the type index counter.
// The component parameter provides the outer Types array and TypeIdxToStoredIdx mapping
// needed to resolve outer type aliases correctly. It may be nil if no outer context is available.
func buildLocalTypeIndex(inst *InstanceTypeDef, c *Component) map[uint32]*TypeDef {
	localTypes := make(map[uint32]*TypeDef)
	typeIdx := uint32(0)

	for _, decl := range inst.Declarations {
		// Only type declarations and type aliases contribute to the type index space
		switch decl.Kind {
		case InstanceDeclKindType:
			if decl.Type != nil {
				localTypes[typeIdx] = decl.Type
			}
			typeIdx++
		case InstanceDeclKindAlias:
			// Aliases with Sort=Type also contribute to the type index space
			if decl.Alias != nil && decl.Alias.Sort == SortType {
				// Try to resolve the alias using outer types
				if c != nil && decl.Alias.Kind == AliasKindOuter {
					// Outer alias references a type from an enclosing scope.
					// OuterIndex is a component type index, which may include
					// alias entries that are not in c.Types. Use ResolveTypeIdx
					// to handle both direct types and alias chains.
					if decl.Alias.OuterCount == 1 {
						if td := c.ResolveTypeIdx(decl.Alias.OuterIndex); td != nil {
							localTypes[typeIdx] = td
						}
					}
				}
				typeIdx++
			}
		case InstanceDeclKindExport:
			// Type exports also consume a type index!
			if decl.Export != nil && decl.Export.Kind == ExportKindType {
				// For eq bounds, look up the local type first
				if td, ok := localTypes[decl.Export.Idx]; ok {
					localTypes[typeIdx] = td
				}
				typeIdx++
			}
			// CoreType declarations don't contribute to the type index space
		}
	}
	return localTypes
}

// resolveFuncType creates a new FuncType with resolved parameter and result types.
// It converts type references to direct types where possible by inlining the TypeDef
// when we can't resolve to a simple own/borrow reference.
func resolveFuncType(ft *FuncType, localTypes map[uint32]*TypeDef) *FuncType {
	if ft == nil {
		return nil
	}

	resolved := &FuncType{
		Params:  make([]NamedValType, len(ft.Params)),
		Results: make([]NamedValType, len(ft.Results)),
	}

	for i, p := range ft.Params {
		resolved.Params[i] = NamedValType{
			Name:    p.Name,
			ValType: resolveValTypeRef(p.ValType, localTypes),
		}
		// Store the resolved TypeDef for complex types
		if td := getResolvedTypeDef(p.ValType, localTypes); td != nil {
			resolved.Params[i].ResolvedType = td
		}
	}

	for i, r := range ft.Results {
		resolved.Results[i] = NamedValType{
			Name:    r.Name,
			ValType: resolveValTypeRef(r.ValType, localTypes),
		}
		// Store the resolved TypeDef for complex types
		if td := getResolvedTypeDef(r.ValType, localTypes); td != nil {
			resolved.Results[i].ResolvedType = td
		}
	}

	return resolved
}

// getResolvedTypeDef returns the TypeDef for a ValTypeRef if it references a local type.
func getResolvedTypeDef(ref ValTypeRef, localTypes map[uint32]*TypeDef) *TypeDef {
	if ref.IsPrimitive || ref.IsOwn || ref.IsBorrow {
		return nil
	}
	return localTypes[ref.TypeIdx]
}

// resolveValTypeRef resolves a ValTypeRef using local type information.
// If the ref points to a local type, we try to expand it.
func resolveValTypeRef(ref ValTypeRef, localTypes map[uint32]*TypeDef) ValTypeRef {
	if ref.IsPrimitive || ref.IsOwn || ref.IsBorrow {
		return ref // Already resolved
	}

	// Look up the type at this index
	if td, ok := localTypes[ref.TypeIdx]; ok {
		if td.Kind == TypeDefKindDefined && td.Handle != nil {
			// The type is a handle (own/borrow) - return that directly
			return *td.Handle
		}
		// For other defined types, keep the reference but set a flag
		// to indicate it was resolved (the TypeDef is available via ResolvedType)
	}

	return ref
}

// resolveToValType converts a NamedValType to a types.ValType.
// It first checks if a ResolvedType is available (for types resolved from instance types),
// then tries to convert primitive/own/borrow types directly from ValTypeRef,
// and falls back to the TypeResolver for component-level types.
func resolveToValType(nvt NamedValType, resolver *TypeResolver) types.ValType {
	// If we have a ResolvedType (from instance type lookup), use it directly
	if nvt.ResolvedType != nil {
		return typeDefToValType(nvt.ResolvedType)
	}

	// For host-defined functions, try to convert primitive/own/borrow types directly
	// This handles cases where the host provides FuncType with ValTypeRef values
	// that don't reference component-level type indices.
	if nvt.ValType.IsPrimitive || nvt.ValType.IsOwn || nvt.ValType.IsBorrow {
		return valTypeRefToValType(nvt.ValType)
	}

	// Fall back to resolver for component-level types
	vt, err := resolver.ResolveValType(nvt.ValType)
	if err == nil {
		return vt
	}
	return nil
}

// typeDefToValType converts a TypeDef to a types.ValType.
func typeDefToValType(td *TypeDef) types.ValType {
	if td == nil {
		return nil
	}

	switch td.Kind {
	case TypeDefKindDefined:
		// Handle different defined types
		if td.Handle != nil {
			if td.Handle.IsPrimitive {
				// Handle primitive type aliases (e.g., filesize = u64)
				return valTypeRefToValType(*td.Handle)
			}
			if td.Handle.IsOwn {
				return types.Own{ResourceIdx: td.Handle.TypeIdx}
			}
			if td.Handle.IsBorrow {
				return types.Borrow{ResourceIdx: td.Handle.TypeIdx}
			}
		}
		if td.Option != nil {
			inner := valTypeRefToValType(td.Option.InnerType)
			return types.Option{Some: inner}
		}
		if td.Result != nil {
			var okType, errType types.ValType
			if td.Result.OkType != nil {
				okType = valTypeRefToValType(*td.Result.OkType)
			}
			if td.Result.ErrType != nil {
				errType = valTypeRefToValType(*td.Result.ErrType)
			}
			return types.Result{Ok: okType, Error: errType}
		}
		if td.Record != nil {
			fields := make([]types.Field, len(td.Record.Fields))
			for i, f := range td.Record.Fields {
				fields[i] = types.Field{
					Name: f.Name,
					Type: valTypeRefToValType(f.ValType),
				}
			}
			return types.Record{Fields: fields}
		}
		if td.Tuple != nil {
			elemTypes := make([]types.ValType, len(td.Tuple.Types))
			for i, t := range td.Tuple.Types {
				elemTypes[i] = valTypeRefToValType(t)
			}
			return types.Tuple{Types: elemTypes}
		}
		if td.List != nil {
			return types.List{Element: valTypeRefToValType(td.List.ElementType)}
		}
		if td.Variant != nil {
			cases := make([]types.Case, len(td.Variant.Cases))
			for i, c := range td.Variant.Cases {
				var caseType types.ValType
				if c.ValType != nil {
					caseType = valTypeRefToValType(*c.ValType)
				}
				cases[i] = types.Case{Name: c.Name, Type: caseType}
			}
			return types.Variant{Cases: cases}
		}
		if td.Flags != nil {
			return types.Flags{Names: td.Flags.Names}
		}
		if td.Enum != nil {
			return types.Enum{Cases: td.Enum.Names}
		}
	case TypeDefKindResource:
		// Resources don't flatten directly, they're accessed via handles
		return nil
	}

	return nil
}

// valTypeRefToValType converts a ValTypeRef to a types.ValType.
func valTypeRefToValType(ref ValTypeRef) types.ValType {
	if ref.IsPrimitive {
		switch ref.Primitive {
		case 0x7f:
			return types.Bool{}
		case 0x7e:
			return types.S8{}
		case 0x7d:
			return types.U8{}
		case 0x7c:
			return types.S16{}
		case 0x7b:
			return types.U16{}
		case 0x7a:
			return types.S32{}
		case 0x79:
			return types.U32{}
		case 0x78:
			return types.S64{}
		case 0x77:
			return types.U64{}
		case 0x76:
			return types.F32{}
		case 0x75:
			return types.F64{}
		case 0x74:
			return types.Char{}
		case 0x73:
			return types.String{}
		}
	}

	if ref.IsOwn {
		return types.Own{ResourceIdx: ref.TypeIdx}
	}
	if ref.IsBorrow {
		return types.Borrow{ResourceIdx: ref.TypeIdx}
	}

	// For type index references, we can't resolve without context
	// Return u32 as a placeholder for handles
	return types.U32{}
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
func (l *ComponentLinker) buildImportResolver(
	ctx context.Context,
	inst *Instance,
	c *Component,
	coreInst *CoreInstance,
	resolvedImports map[string]Definition,
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]struct {
	instanceIdx uint32
	exportName  string
},
) experimental.ImportResolver {
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
		if ok && os.Getenv("WAZERO_DEBUG_TABLE") != "" {
			fmt.Printf("[import-resolver] looking up moduleName=%q -> instanceIdx=%d (len(coreInstances)=%d)\n",
				moduleName, instanceIdx, len(inst.coreInstances))
			if int(instanceIdx) < len(c.CoreInstances) {
				def := &c.CoreInstances[instanceIdx]
				fmt.Printf("[import-resolver]   CoreInstance kind=%d\n", def.Kind)
			}
		}
		if ok {
			// Check if we have a direct core instance
			if int(instanceIdx) < len(inst.coreInstances) {
				coreInstance := inst.coreInstances[instanceIdx]
				if coreInstance != nil {
					if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
						if modInst, ok := coreInstance.(*wasm.ModuleInstance); ok {
							numFuncs := 0
							if modInst.Source != nil {
								numFuncs = len(modInst.Source.FunctionSection)
							}
							fmt.Printf("[import-resolver]   returning direct coreInstance=%s numFuncs=%d\n",
								modInst.ModuleName, numFuncs)
						}
					}
					return coreInstance
				}
			}

			// The instance might be an inline instance - we need to trace through
			// the inline exports to find the actual source.
			if int(instanceIdx) < len(c.CoreInstances) {
				srcCoreInst := &c.CoreInstances[instanceIdx]
				if srcCoreInst.Kind == CoreInstanceExprInline {
					if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
						fmt.Printf("[import-resolver]   inline instance, calling resolveInlineInstanceSource\n")
					}
					result := l.resolveInlineInstanceSource(inst, c, srcCoreInst)
					if result != nil {
						if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
							if modInst, ok := result.(*wasm.ModuleInstance); ok {
								numFuncs := 0
								if modInst.Source != nil {
									numFuncs = len(modInst.Source.FunctionSection)
								}
								fmt.Printf("[import-resolver]   resolveInlineInstanceSource returned module=%s numFuncs=%d\n",
									modInst.ModuleName, numFuncs)
							}
						}
						return result
					}

					// If resolveInlineInstanceSource returned nil, the exports may come from
					// canon operations (lower/resource) or aliases. Create a composite host module.
					if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
						fmt.Printf("[import-resolver]   resolveInlineInstanceSource returned nil, trying createInlineInstanceModule\n")
					}
					if synth, exists := syntheticModules[moduleName]; exists {
						if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
							fmt.Printf("[import-resolver]   returning cached synthetic module\n")
						}
						return synth
					}
					hostMod := l.createInlineInstanceModule(ctx, inst, c, srcCoreInst, canonLowers, canonResources, funcAliases)
					if hostMod != nil {
						if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
							if modInst, ok := hostMod.(*wasm.ModuleInstance); ok {
								numFuncs := 0
								if modInst.Source != nil {
									numFuncs = len(modInst.Source.FunctionSection)
								}
								fmt.Printf("[import-resolver]   createInlineInstanceModule returned module=%s numFuncs=%d\n",
									modInst.ModuleName, numFuncs)
							} else {
								fmt.Printf("[import-resolver]   createInlineInstanceModule returned non-wasm module type=%T\n", hostMod)
							}
						}
						syntheticModules[moduleName] = hostMod
						return hostMod
					}
					if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
						fmt.Printf("[import-resolver]   createInlineInstanceModule returned nil\n")
					}
				}
			}
		}

		// Fall back to host implementations only if no internal instance is available.
		// This handles cases where the component imports directly from host without
		// an internal shim layer, OR when inline instance resolution fails because
		// exports come from canonical operations (like canon lower).
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

	// Also track table aliases
	tableAliases := make(map[uint32]exportSource)
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortTable {
			tableAliases[alias.Idx] = exportSource{alias.InstanceIdx, alias.ExportName}
		}
	}

	// Map inline exports to their sources
	var primaryInstanceIdx uint32 = 0xFFFFFFFF
	allSameSource := true
	namesMatch := true
	totalExports := 0 // Count of exports we expect to map (func, memory, and table)

	for _, exp := range inlineInst.InlineExports {
		var src exportSource
		var ok bool
		switch exp.Sort {
		case CoreSortFunc:
			totalExports++
			src, ok = funcAliases[exp.Idx]
		case CoreSortMemory:
			totalExports++
			src, ok = memAliases[exp.Idx]
		case CoreSortTable:
			totalExports++
			src, ok = tableAliases[exp.Idx]
		default:
			continue
		}

		if !ok {
			// This export comes from a canonical operation (canon lower, resource.drop, etc.)
			// not from an alias. We cannot resolve it through alias tracing.
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

	// If we couldn't map all exports (some come from canonical operations),
	// we cannot use alias-based resolution. Return nil to allow fallback to
	// host implementations.
	if len(exportMapping) != totalExports {
		return nil
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

	// Check if the inline instance exports any functions. If it only exports memory/tables,
	// we can still use the shortcut path safely because those don't require function resolution.
	hasFuncExports := false
	for _, exp := range inlineInst.InlineExports {
		if exp.Sort == CoreSortFunc {
			hasFuncExports = true
			break
		}
	}

	// Check if the primary source module has imports. If it does, the imported functions
	// might not be resolved yet (because we're still in the process of instantiation).
	// In this case, we cannot use the shortcut path for functions - but memory/table exports are ok.
	if hasFuncExports {
		if modInst, ok := primarySource.(*wasm.ModuleInstance); ok {
			importCount := uint32(0)
			if modInst.Source != nil {
				importCount = modInst.Source.ImportFunctionCount
			}
			if importCount > 0 {
				return nil
			}
		}
	}

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
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]struct {
	instanceIdx uint32
	exportName  string
},
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
	resolver := l.buildImportResolver(ctx, inst, c, coreInst, resolvedImports, canonLowers, canonResources, funcAliases)
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

	// Look up the function type using the type index mapping.
	// The component type index space can include type aliases that don't add entries
	// to c.Types, so we need the mapping to find the correct stored index.
	var funcType *FuncType
	if storedIdx, ok := c.TypeIdxToStoredIdx[canon.TypeIdx]; ok {
		if int(storedIdx) < len(c.Types) && c.Types[storedIdx].Kind == TypeDefKindFunc {
			funcType = c.Types[storedIdx].Func
		}
	} else if int(canon.TypeIdx) < len(c.Types) && c.Types[canon.TypeIdx].Kind == TypeDefKindFunc {
		// Direct lookup fallback for components without the mapping (backward compatibility)
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

	var postReturnFunc api.Function
	if canon.Options.PostReturnIdx != nil {
		postReturnFunc, err = l.resolveCoreFunc(inst, c, *canon.Options.PostReturnIdx, funcSpace)
		if err != nil {
			return nil, fmt.Errorf("resolve post-return: %w", err)
		}
	}

	return &ExportedFunc{
		name:           exp.Name,
		funcType:       funcType,
		coreFunc:       coreFunc,
		canonical:      canon,
		component:      c,
		instance:       inst,
		memory:         memory,
		reallocFunc:    reallocFunc,
		postReturnFunc: postReturnFunc,
	}, nil
}

// wireNestedComponentExports wires the exports of a nested component instance (shim pattern).
// In the component model, interface exports are often wrapped in a shim component that
// imports a canon-lifted function and re-exports it. This method traces through the shim's
// import/export mapping to create proper ExportedFunc objects on the nested instance.
func (l *ComponentLinker) wireNestedComponentExports(
	parent *Instance, parentComp *Component,
	nestedInst *Instance, compInstDef *ComponentInstance,
	funcSpace *CoreFuncIndexSpace, memSpace *CoreMemoryIndexSpace,
) error {
	nestedComp := nestedInst.component
	if nestedComp == nil {
		return nil
	}

	// Build mapping: arg name → parent component function index
	argToParentFunc := make(map[string]uint32)
	for _, arg := range compInstDef.Args {
		if arg.Sort == SortFunc {
			argToParentFunc[arg.Name] = arg.Idx
		}
	}

	// Build mapping: nested func index → import name
	nestedFuncToImport := make(map[uint32]string)
	funcIdx := uint32(0)
	for _, imp := range nestedComp.Imports {
		if imp.ExternDesc.Kind == ImportExternDescFunc {
			nestedFuncToImport[funcIdx] = imp.Name
			funcIdx++
		}
	}

	// Wire each function export
	for _, exp := range nestedComp.Exports {
		if exp.Kind != ExportKindFunc {
			continue
		}

		// Find which import provides this function
		impName, ok := nestedFuncToImport[exp.Idx]
		if !ok {
			continue
		}

		// Find the parent component function index
		parentFuncIdx, ok := argToParentFunc[impName]
		if !ok {
			continue
		}

		// Create ExportedFunc using the parent's canon lift for this function
		syntheticExp := Export{
			Name: exp.Name,
			Kind: ExportKindFunc,
			Idx:  parentFuncIdx,
		}
		exportedFunc, err := l.wireExportedFunc(parent, parentComp, &syntheticExp, funcSpace, memSpace)
		if err != nil {
			return fmt.Errorf("export %q: %w", exp.Name, err)
		}
		nestedInst.exports[exp.Name] = exportedFunc
	}

	return nil
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

// wireMemorySharing wires up memory sharing between core instances after all modules
// are instantiated. Some modules (like the WASI P1 adapter) import memory that aliases
// another module's memory, but the source module may not exist at import resolution time.
//
// This function finds all memory aliases and ensures the importing modules share memory
// with the source modules.
func (l *ComponentLinker) wireMemorySharing(inst *Instance, c *Component) error {
	// Build memory alias map: memIdx -> (instanceIdx, exportName)
	memoryAliases := make(map[uint32]struct {
		instanceIdx uint32
		exportName  string
	})
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortMemory {
			memoryAliases[alias.Idx] = struct {
				instanceIdx uint32
				exportName  string
			}{alias.InstanceIdx, alias.ExportName}
		}
	}

	// Find the primary memory source - trace through inline instances to find the actual module
	var primaryMemoryModule api.Module
	for _, aliasInfo := range memoryAliases {
		// Trace through to find the actual module with memory
		sourceModule := l.traceMemorySource(inst, c, aliasInfo.instanceIdx, aliasInfo.exportName)
		if sourceModule != nil {
			primaryMemoryModule = sourceModule
			break
		}
	}

	if primaryMemoryModule == nil {
		return nil
	}

	// Get the primary memory instance
	primaryMemoryInst, ok := primaryMemoryModule.(*wasm.ModuleInstance)
	if !ok || primaryMemoryInst.MemoryInstance == nil {
		return nil
	}

	// Share memory with all other modules that don't have their own memory
	for _, coreInst := range inst.coreInstances {
		if coreInst == nil {
			continue
		}
		modInst, ok := coreInst.(*wasm.ModuleInstance)
		if !ok {
			continue
		}

		// Skip the primary memory module itself
		if modInst == primaryMemoryInst {
			continue
		}

		hasMemorySection := modInst.Source != nil && modInst.Source.MemorySection != nil

		// If this module has no memory but needs it (imports memory), share the primary memory
		if modInst.MemoryInstance == nil {
			// Check if this module imports memory by looking at its import section
			if modInst.Source != nil {
				memImportFound := false
				for _, imp := range modInst.Source.ImportSection {
					if imp.Type == wasm.ExternTypeMemory {
						memImportFound = true
						modInst.MemoryInstance = primaryMemoryInst.MemoryInstance
						break
					}
				}
				if !memImportFound {
					// If module has no memory and no memory import but doesn't define its own memory,
					// it may still need access to memory (e.g., adapter modules). Share memory unconditionally
					// for modules that don't define their own memory.
					if !hasMemorySection {
						modInst.MemoryInstance = primaryMemoryInst.MemoryInstance
					}
				}
			}
		}
	}

	return nil
}

// traceMemorySource traces through inline instances to find the actual module that exports memory.
func (l *ComponentLinker) traceMemorySource(inst *Instance, c *Component, instanceIdx uint32, exportName string) api.Module {
	// Check if this is a direct core instance with memory
	if int(instanceIdx) < len(inst.coreInstances) {
		if coreInst := inst.coreInstances[instanceIdx]; coreInst != nil {
			if modInst, ok := coreInst.(*wasm.ModuleInstance); ok && modInst.MemoryInstance != nil {
				return coreInst
			}
		}
	}

	// If the instance is nil or has no memory, check if it's an inline instance
	if int(instanceIdx) < len(c.CoreInstances) {
		coreInstDef := &c.CoreInstances[instanceIdx]
		if coreInstDef.Kind == CoreInstanceExprInline {
			// Find the inline export that provides memory
			for _, inlineExp := range coreInstDef.InlineExports {
				if inlineExp.Name == exportName && inlineExp.Sort == CoreSortMemory {
					// Find the alias that provides this memory
					for _, alias := range c.Aliases {
						if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortMemory && alias.Idx == inlineExp.Idx {
							// Recursively trace
							return l.traceMemorySource(inst, c, alias.InstanceIdx, alias.ExportName)
						}
					}
				}
			}
		}
	}

	return nil
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

	// Use the module name from the import - the runtime requires a non-empty name.
	// This name won't conflict since we're providing this module through the import
	// resolver, not registering it in the store for lookup.
	mod, err := hmi.InstantiateHostModule(ctx, moduleName, exports)
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

// createInlineInstanceModule creates a host module from an inline instance
// that has exports coming from multiple sources:
// 1. Canon lower operations - creates proper core functions with correct signatures
// 2. Resource operations (drop/new/rep) - creates resource handling functions
// 3. Aliases - forwards to the source core instance
// 4. Table aliases - shares tables from source core instances
func (l *ComponentLinker) createInlineInstanceModule(
	ctx context.Context,
	inst *Instance,
	c *Component,
	inlineInst *CoreInstance,
	canonLowers map[uint32]canonLowerInfo,
	canonResources map[uint32]canonResourceInfo,
	funcAliases map[uint32]struct {
	instanceIdx uint32
	exportName  string
},
) api.Module {
	// Check if the runtime supports host module instantiation
	hmi, ok := l.runtime.(HostModuleInstantiator)
	if !ok {
		return nil
	}

	// Build table aliases map from component aliases
	tableAliases := make(map[uint32]struct {
		instanceIdx uint32
		exportName  string
	})
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortTable {
			tableAliases[alias.Idx] = struct {
				instanceIdx uint32
				exportName  string
			}{alias.InstanceIdx, alias.ExportName}
		}
	}

	// Build memory aliases map from component aliases
	memoryAliases := make(map[uint32]struct {
		instanceIdx uint32
		exportName  string
	})
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortMemory {
			memoryAliases[alias.Idx] = struct {
				instanceIdx uint32
				exportName  string
			}{alias.InstanceIdx, alias.ExportName}
		}
	}

	var exports []HostModuleExport
	var tableExports []HostModuleTableExport
	var memoryExports []HostModuleMemoryExport

	for _, exp := range inlineInst.InlineExports {
		switch exp.Sort {
		case CoreSortFunc:
			// Try each source type in order: canon lower, resource ops, aliases

			// 1. Check if this is a canon lower function
			if info, ok := canonLowers[exp.Idx]; ok {
				export := l.createCanonLowerExport(ctx, inst, c, exp.Name, info)
				if export != nil {
					exports = append(exports, *export)
				}
				continue
			}

			// 2. Check if this is a resource operation
			if info, ok := canonResources[exp.Idx]; ok {
				export := l.createResourceOpExport(inst, exp.Name, info)
				if export != nil {
					exports = append(exports, *export)
				}
				continue
			}

			// 3. Check if this is an alias - forward to source core instance
			if aliasInfo, ok := funcAliases[exp.Idx]; ok {
				export := l.createAliasExport(inst, c, exp.Name, aliasInfo.instanceIdx, aliasInfo.exportName)
				if export != nil {
					exports = append(exports, *export)
				}
				continue
			}

		case CoreSortTable:
			// Table exports - share from source core instance
			if aliasInfo, ok := tableAliases[exp.Idx]; ok {
				if int(aliasInfo.instanceIdx) < len(inst.coreInstances) {
					sourceModule := inst.coreInstances[aliasInfo.instanceIdx]
					if sourceModule != nil {
						tableExports = append(tableExports, HostModuleTableExport{
							Name:         exp.Name,
							SourceModule: sourceModule,
							SourceName:   aliasInfo.exportName,
						})
					}
				}
			}

		case CoreSortMemory:
			// Memory exports - share from source core instance
			if aliasInfo, ok := memoryAliases[exp.Idx]; ok {
				// For memory aliases, we need to find the actual source module.
				// The aliasInfo.instanceIdx may point to another inline instance, so we need to trace through.
				sourceModule := l.resolveMemorySource(inst, c, aliasInfo.instanceIdx, aliasInfo.exportName)
				if sourceModule != nil {
					memoryExports = append(memoryExports, HostModuleMemoryExport{
						Name:         exp.Name,
						SourceModule: sourceModule,
						SourceName:   aliasInfo.exportName,
					})
				}
			}
		}
	}

	if len(exports) == 0 && len(tableExports) == 0 && len(memoryExports) == 0 {
		return nil
	}

	// Generate a unique module name for this host module
	moduleName := fmt.Sprintf("$inline_%p", inlineInst)

	// If the inline instance has function exports but no memory exports, and there's a memory alias,
	// we should still share memory with the host module so that host functions can access memory.
	// This is important because the host functions (canon lower, alias forwards, etc.) may need
	// to read/write memory even if the inline instance doesn't explicitly export memory.
	if len(memoryExports) == 0 && len(memoryAliases) > 0 {
		// Find the primary memory alias (usually index 0)
		for _, aliasInfo := range memoryAliases {
			sourceModule := l.resolveMemorySource(inst, c, aliasInfo.instanceIdx, aliasInfo.exportName)
			if sourceModule != nil {
				memoryExports = append(memoryExports, HostModuleMemoryExport{
					Name:         "memory", // Standard name
					SourceModule: sourceModule,
					SourceName:   aliasInfo.exportName,
				})
				break // Only need one memory
			}
		}
	}

	// Use the resource-aware method if we have table or memory exports
	if len(tableExports) > 0 || len(memoryExports) > 0 {
		mod, err := hmi.InstantiateHostModuleWithResources(ctx, moduleName, exports, tableExports, memoryExports)
		if err != nil {
			return nil
		}
		return mod
	}

	mod, err := hmi.InstantiateHostModule(ctx, moduleName, exports)
	if err != nil {
		return nil
	}
	return mod
}

// resolveMemorySource finds the actual module that contains the memory for a memory alias.
// Memory aliases may chain through inline instances, so we trace through to find the real source.
func (l *ComponentLinker) resolveMemorySource(inst *Instance, c *Component, instanceIdx uint32, exportName string) api.Module {

	// First, check if we have a direct core instance
	if int(instanceIdx) < len(inst.coreInstances) {
		if coreInst := inst.coreInstances[instanceIdx]; coreInst != nil {
			return coreInst
		}
	}

	// If the instance is nil, it might be an inline instance that we need to trace through
	if int(instanceIdx) < len(c.CoreInstances) {
		srcCoreInst := &c.CoreInstances[instanceIdx]
		if srcCoreInst.Kind == CoreInstanceExprInline {
			// Find the inline export that matches the requested export name
			for _, inlineExp := range srcCoreInst.InlineExports {
				if inlineExp.Name == exportName && inlineExp.Sort == CoreSortMemory {
					// Find the alias that provides this memory index
					for _, alias := range c.Aliases {
						if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortMemory && alias.Idx == inlineExp.Idx {
							// Recursively resolve
							return l.resolveMemorySource(inst, c, alias.InstanceIdx, alias.ExportName)
						}
					}
				}
			}
		}
	}

	return nil
}

// createCanonLowerExport creates a HostModuleExport for a canon lower operation.
func (l *ComponentLinker) createCanonLowerExport(
	ctx context.Context,
	inst *Instance,
	c *Component,
	name string,
	info canonLowerInfo,
) *HostModuleExport {
	// Get the component function being lowered
	compFunc, ok := inst.componentFuncs[info.compFuncIdx]
	if !ok {
		return nil
	}

	// Convert FuncType to types.ValType slices for CoreSignature
	var paramTypes, resultTypes []types.ValType
	if compFunc.Type != nil {
		resolver := NewTypeResolver(c)
		for _, p := range compFunc.Type.Params {
			if vt := resolveToValType(p, resolver); vt != nil {
				paramTypes = append(paramTypes, vt)
			}
		}
		for _, r := range compFunc.Type.Results {
			if vt := resolveToValType(r, resolver); vt != nil {
				resultTypes = append(resultTypes, vt)
			}
		}
	}

	// Get the core signature using the ABI flattening
	params, results, needsRetptr := coreSignature(paramTypes, resultTypes)

	// Create the lowered function
	goFunc := l.createCanonLowerFunc(ctx, inst, c, info, compFunc, needsRetptr)
	if goFunc == nil {
		return nil
	}

	return &HostModuleExport{
		Name:        name,
		ParamTypes:  params,
		ResultTypes: results,
		Func:        goFunc,
	}
}

// createResourceOpExport creates a HostModuleExport for a resource operation.
func (l *ComponentLinker) createResourceOpExport(
	inst *Instance,
	name string,
	info canonResourceInfo,
) *HostModuleExport {
	switch info.kind {
	case CanonKindResourceDrop:
		// resource.drop: (i32) -> ()
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{},
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				handle := uint32(stack[0])
				// Get the resource table and drop the handle
				if inst.resourceTable != nil {
					inst.resourceTable.Remove(Handle(handle))
				}
			}),
		}
	case CanonKindResourceNew:
		// resource.new: (i32) -> i32
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				rep := uint32(stack[0])
				if inst.resourceTable != nil {
					handle := inst.resourceTable.New(rep, true)
					stack[0] = uint64(handle)
				}
			}),
		}
	case CanonKindResourceRep:
		// resource.rep: (i32) -> i32
		return &HostModuleExport{
			Name:        name,
			ParamTypes:  []api.ValueType{api.ValueTypeI32},
			ResultTypes: []api.ValueType{api.ValueTypeI32},
			Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				handle := uint32(stack[0])
				if inst.resourceTable != nil {
					rep, err := inst.resourceTable.Rep(Handle(handle))
					if err == nil {
						stack[0] = uint64(rep)
					}
				}
			}),
		}
	}
	return nil
}

// createAliasExport creates a HostModuleExport that forwards to a function from another core instance.
// The source function is resolved lazily at call time to handle forward references where the
// source core instance hasn't been instantiated yet.
func (l *ComponentLinker) createAliasExport(
	inst *Instance,
	c *Component,
	name string,
	sourceInstanceIdx uint32,
	sourceExportName string,
) *HostModuleExport {
	if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
		fmt.Printf("[createAliasExport] name=%q sourceInstanceIdx=%d sourceExportName=%q len(coreInstances)=%d\n",
			name, sourceInstanceIdx, sourceExportName, len(inst.coreInstances))
	}
	// Try to get param/result types from the source if available now
	var paramTypes, resultTypes []api.ValueType

	if int(sourceInstanceIdx) < len(inst.coreInstances) {
		sourceInst := inst.coreInstances[sourceInstanceIdx]
		if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
			fmt.Printf("[createAliasExport]   sourceInst at idx %d is %v (nil=%t)\n",
				sourceInstanceIdx, sourceInst, sourceInst == nil)
		}
		if sourceInst != nil {
			defs := sourceInst.ExportedFunctionDefinitions()
			if def := defs[sourceExportName]; def != nil {
				paramTypes = def.ParamTypes()
				resultTypes = def.ResultTypes()
			}
		}
	} else if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
		fmt.Printf("[createAliasExport]   sourceInstanceIdx %d >= len(coreInstances) %d\n",
			sourceInstanceIdx, len(inst.coreInstances))
	}

	// If we couldn't get types from the instantiated module, try to get them
	// from the compiled CoreModule definition. This handles forward references.
	if paramTypes == nil && c != nil {
		// Find the core instance definition to get the module index
		if int(sourceInstanceIdx) < len(c.CoreInstances) {
			coreInstDef := &c.CoreInstances[sourceInstanceIdx]
			if coreInstDef.Kind == CoreInstanceExprInstantiate {
				moduleIdx := coreInstDef.ModuleIdx
				if int(moduleIdx) < len(c.CoreModules) {
					coreModule := c.CoreModules[moduleIdx]
					if coreModule != nil {
						// Find the export in the core module
						for _, exp := range coreModule.ExportSection {
							if exp.Name == sourceExportName && exp.Type == wasm.ExternTypeFunc {
								// Get the function type
								funcIdx := exp.Index
								if funcIdx < coreModule.ImportFunctionCount {
									// Imported function - get type from imports
									importIdx := uint32(0)
									for i := range coreModule.ImportSection {
										imp := &coreModule.ImportSection[i]
										if imp.Type == wasm.ExternTypeFunc {
											if importIdx == funcIdx {
												typeIdx := imp.DescFunc
												if int(typeIdx) < len(coreModule.TypeSection) {
													ft := &coreModule.TypeSection[typeIdx]
													paramTypes = valTypesToAPITypes(ft.Params)
													resultTypes = valTypesToAPITypes(ft.Results)
												}
												break
											}
											importIdx++
										}
									}
								} else {
									// Local function
									localIdx := funcIdx - coreModule.ImportFunctionCount
									if int(localIdx) < len(coreModule.FunctionSection) {
										typeIdx := coreModule.FunctionSection[localIdx]
										if int(typeIdx) < len(coreModule.TypeSection) {
											ft := &coreModule.TypeSection[typeIdx]
											paramTypes = valTypesToAPITypes(ft.Params)
											resultTypes = valTypesToAPITypes(ft.Results)
										}
									}
								}
								break
							}
						}
					}
				}
			} else if coreInstDef.Kind == CoreInstanceExprInline {
				// For inline instances, we need to trace through to find the actual source
				// Look through the inline exports to find the source function
				for _, inlineExp := range coreInstDef.InlineExports {
					if inlineExp.Name == sourceExportName && inlineExp.Sort == CoreSortFunc {
						// The inlineExp.Idx is a core function index in the component's function index space
						// We need to trace this back to its source to get the type
						funcIdx := inlineExp.Idx

						// Check if this is a canon lower operation
						for i := range c.Canonicals {
							canon := &c.Canonicals[i]
							if canon.Kind == CanonKindLower && canon.ComponentFuncIdx == funcIdx {
								// Get the component function type
								if compFunc, ok := inst.componentFuncs[canon.FuncIdx]; ok && compFunc.Type != nil {
									// Convert FuncType to types.ValType slices for CoreSignature
									var compParamTypes, compResultTypes []types.ValType
									resolver := NewTypeResolver(c)
									for _, p := range compFunc.Type.Params {
										if vt := resolveToValType(p, resolver); vt != nil {
											compParamTypes = append(compParamTypes, vt)
										}
									}
									for _, r := range compFunc.Type.Results {
										if vt := resolveToValType(r, resolver); vt != nil {
											compResultTypes = append(compResultTypes, vt)
										}
									}
									// Get the core signature using the ABI flattening
									paramTypes, resultTypes, _ = coreSignature(compParamTypes, compResultTypes)
								}
								break
							}
						}
						break
					}
				}
			}
		}
	}

	// If still nil, use empty (this shouldn't happen with well-formed components)
	if paramTypes == nil {
		paramTypes = []api.ValueType{}
		resultTypes = []api.ValueType{}
	}

	// If the source module is already instantiated, use function forwarding.
	// This allows the import resolution to trace to the actual source module,
	// preserving the correct moduleCtxPtr for table initialization.
	if int(sourceInstanceIdx) < len(inst.coreInstances) {
		sourceInst := inst.coreInstances[sourceInstanceIdx]
		if sourceInst != nil {
			if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
				fmt.Printf("[createAliasExport]   FORWARDING name=%q to sourceInst (available at idx %d)\n",
					name, sourceInstanceIdx)
			}
			return &HostModuleExport{
				Name:         name,
				ParamTypes:   paramTypes,
				ResultTypes:  resultTypes,
				SourceModule: sourceInst,
				SourceName:   sourceExportName,
			}
		}
	}

	if os.Getenv("WAZERO_DEBUG_TABLE") != "" {
		fmt.Printf("[createAliasExport]   LAZY WRAPPER name=%q (sourceInst not available yet)\n", name)
	}
	// Fall back to lazy resolution wrapper for forward references
	// (when the source instance hasn't been instantiated yet)
	return &HostModuleExport{
		Name:        name,
		ParamTypes:  paramTypes,
		ResultTypes: resultTypes,
		Func: api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			// Resolve the source function lazily
			if int(sourceInstanceIdx) >= len(inst.coreInstances) {
				// Source instance not available - this shouldn't happen at call time
				return
			}
			sourceInst := inst.coreInstances[sourceInstanceIdx]
			if sourceInst == nil {
				return
			}
			sourceFunc := sourceInst.ExportedFunction(sourceExportName)
			if sourceFunc == nil {
				return
			}

			// Get the actual param count from the function definition
			defs := sourceInst.ExportedFunctionDefinitions()
			def := defs[sourceExportName]
			if def == nil {
				return
			}
			actualParamCount := len(def.ParamTypes())

			// Call the source function with the correct number of params
			results, err := sourceFunc.Call(ctx, stack[:actualParamCount]...)
			if err != nil {
				return
			}
			// Copy results back to stack
			for i, r := range results {
				stack[i] = r
			}
		}),
	}
}

// buildMemoryIndexSpace builds a CoreMemoryIndexSpace from the component's aliases.
// This is used to resolve memory indices to core instance/export pairs at runtime.
func buildMemoryIndexSpace(c *Component) *CoreMemoryIndexSpace {
	memSpace := NewCoreMemoryIndexSpace()
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortMemory {
			memSpace.AddAlias(alias.Idx, alias.InstanceIdx, alias.ExportName)
		}
	}
	return memSpace
}

// buildFuncIndexSpace builds a CoreFuncIndexSpace from the component's aliases.
// This is used to resolve function indices (like realloc) to core instance/export pairs at runtime.
func buildFuncIndexSpace(c *Component) *CoreFuncIndexSpace {
	funcSpace := NewCoreFuncIndexSpace()
	for i := range c.Aliases {
		alias := &c.Aliases[i]
		if alias.Kind == AliasKindCoreExport && alias.CoreSort == CoreSortFunc {
			funcSpace.AddAlias(alias.Idx, alias.InstanceIdx, alias.ExportName)
		}
	}
	return funcSpace
}

// createCanonLowerFunc creates a GoModuleFunc that implements a canon lower operation.
// It takes core wasm arguments, lifts them to component values, calls the component
// function, and lowers the results back to core wasm values.
func (l *ComponentLinker) createCanonLowerFunc(
	ctx context.Context,
	inst *Instance,
	c *Component,
	info canonLowerInfo,
	compFunc ComponentFunc,
	needsRetptr bool,
) api.GoModuleFunc {
	if compFunc.Impl == nil {
		return nil
	}

	return api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		// Resolve memory from canon options if specified, otherwise fallback to mod.Memory()
		// This is critical for inline instances where the host module doesn't have its own memory.
		// The memory index in canon options points to a core memory alias which resolves to
		// the actual core instance's memory (e.g., the Go module's memory).
		var memory api.Memory
		var reallocFunc api.Function
		if info.options != nil && info.options.MemoryIdx != nil {
			memIdx := *info.options.MemoryIdx
			// Use memory space to resolve the memory index to instance/export
			if memSpace := buildMemoryIndexSpace(c); memSpace != nil {
				if instanceIdx, exportName, err := memSpace.Resolve(memIdx); err == nil {
					if int(instanceIdx) < len(inst.coreInstances) {
						if coreInst := inst.coreInstances[instanceIdx]; coreInst != nil {
							memory = coreInst.ExportedMemory(exportName)
						}
					}
				}
			}
		}
		// Fallback to mod.Memory() if canon options didn't specify memory or resolution failed
		if memory == nil {
			memory = mod.Memory()
		}

		// Resolve realloc function from canon options if specified
		// This is needed for lowering lists and strings to linear memory
		if info.options != nil && info.options.ReallocIdx != nil {
			reallocIdx := *info.options.ReallocIdx
			if funcSpace := buildFuncIndexSpace(c); funcSpace != nil {
				if instanceIdx, exportName, err := funcSpace.Resolve(reallocIdx); err == nil {
					if int(instanceIdx) < len(inst.coreInstances) {
						if coreInst := inst.coreInstances[instanceIdx]; coreInst != nil {
							reallocFunc = coreInst.ExportedFunction(exportName)
						}
					}
				}
			}
		}

		// Track stack position
		stackIdx := 0

		// Handle retptr if needed
		// Per Canonical ABI spec, retptr is appended AFTER all params
		var retptr uint32
		if needsRetptr {
			// retptr is the last parameter in the stack
			retptr = uint32(stack[len(stack)-1])
			// Don't modify stackIdx - params start from index 0
		}

		// Lift core args to component values
		args := make([]Val, 0)
		if compFunc.Type != nil {
			for _, paramDef := range compFunc.Type.Params {
				val, consumed := liftFromStack(stack[stackIdx:], paramDef.ValType, paramDef.ResolvedType, memory)
				args = append(args, val)
				stackIdx += consumed
			}
		}

		// Call the component function
		results, err := compFunc.Impl(ctx, args)
		if err != nil {
			// Handle error - for now just return without modifying stack
			// In a full implementation this would trap
			return
		}

		// Lower results to core values
		if needsRetptr && memory != nil {
			// Write results to memory at retptr
			if err := writeResultsToMemory(ctx, memory, reallocFunc, retptr, results, compFunc.Type); err != nil {
				return
			}
		} else {
			// Write results to stack
			resultIdx := 0
			for _, result := range results {
				written := lowerToStack(stack[resultIdx:], result)
				resultIdx += written
			}
		}
	})
}

// liftFromStack lifts a value from the core wasm stack based on the component type.
// Returns the Val and the number of stack slots consumed.
func liftFromStack(stack []uint64, typeRef ValTypeRef, resolvedType *TypeDef, memory api.Memory) (Val, int) {
	if typeRef.IsPrimitive {
		switch typeRef.Primitive {
		case 0x7f: // bool
			return ValBool(stack[0] != 0), 1
		case 0x7e: // s8
			return ValS8(int8(stack[0])), 1
		case 0x7d: // u8
			return ValU8(uint8(stack[0])), 1
		case 0x7c: // s16
			return ValS16(int16(stack[0])), 1
		case 0x7b: // u16
			return ValU16(uint16(stack[0])), 1
		case 0x7a: // s32
			return ValS32(int32(stack[0])), 1
		case 0x79: // u32
			return ValU32(uint32(stack[0])), 1
		case 0x78: // s64
			return ValS64(int64(stack[0])), 1
		case 0x77: // u64
			return ValU64(stack[0]), 1
		case 0x76: // f32
			return ValF32(float32(stack[0])), 1
		case 0x75: // f64
			return ValF64(float64(stack[0])), 1
		case 0x74: // char
			return ValChar(rune(stack[0])), 1
		case 0x73: // string
			if memory != nil && len(stack) >= 2 {
				ptr := uint32(stack[0])
				length := uint32(stack[1])
				if data, ok := memory.Read(ptr, length); ok {
					return ValString(string(data)), 2
				}
			}
			return ValString(""), 2
		}
	}

	// Handle own and borrow types
	if typeRef.IsOwn {
		return ValOwn(uint32(stack[0])), 1
	}
	if typeRef.IsBorrow {
		return ValBorrow(uint32(stack[0])), 1
	}

	// Handle complex types using ResolvedType
	if resolvedType != nil && resolvedType.Kind == TypeDefKindDefined {
		// Handle list types
		if resolvedType.List != nil {
			if memory != nil && len(stack) >= 2 {
				ptr := uint32(stack[0])
				length := uint32(stack[1])
				// Check if it's list<u8> (element type is u8 primitive)
				elemType := resolvedType.List.ElementType
				if elemType.IsPrimitive && elemType.Primitive == 0x7d { // u8
					// list<u8> - read bytes from memory
					if data, ok := memory.Read(ptr, length); ok {
						vals := make([]Val, len(data))
						for i, b := range data {
							vals[i] = ValU8(b)
						}
						return ValList(vals), 2
					}
				}
			}
			// Fallback for other list types or memory read failure
			return ValList([]Val{}), 2
		}
	}

	// For non-primitive types, treat as i32 for now
	return ValS32(int32(stack[0])), 1
}

// lowerToStack lowers a component Val to the core wasm stack.
// Returns the number of stack slots written.
func lowerToStack(stack []uint64, val Val) int {
	switch val.Kind() {
	case ValKindBool:
		if val.Bool() {
			stack[0] = 1
		} else {
			stack[0] = 0
		}
		return 1
	case ValKindS8:
		stack[0] = uint64(uint32(int32(val.S8())))
		return 1
	case ValKindU8:
		stack[0] = uint64(val.U8())
		return 1
	case ValKindS16:
		stack[0] = uint64(uint32(int32(val.S16())))
		return 1
	case ValKindU16:
		stack[0] = uint64(val.U16())
		return 1
	case ValKindS32:
		stack[0] = uint64(uint32(val.S32()))
		return 1
	case ValKindU32:
		stack[0] = uint64(val.U32())
		return 1
	case ValKindS64:
		stack[0] = uint64(val.S64())
		return 1
	case ValKindU64:
		stack[0] = val.U64()
		return 1
	case ValKindF32:
		stack[0] = uint64(val.U32()) // F32 bits
		return 1
	case ValKindF64:
		stack[0] = val.U64() // F64 bits
		return 1
	case ValKindChar:
		stack[0] = uint64(val.Char())
		return 1
	case ValKindOwn:
		stack[0] = uint64(val.Own())
		return 1
	case ValKindBorrow:
		stack[0] = uint64(val.Borrow())
		return 1
	default:
		// For unknown types, treat as i32
		return 1
	}
}

// writeResultsToMemory writes component results to linear memory at the given offset.
// This is used when the function has too many results to return via the stack.
// For list types, it allocates memory via realloc and writes (ptr, len) to retptr.
func writeResultsToMemory(ctx context.Context, memory api.Memory, reallocFunc api.Function, retptr uint32, results []Val, funcType *FuncType) error {
	if memory == nil || funcType == nil {
		return nil
	}

	offset := retptr
	for i, result := range results {
		if i >= len(funcType.Results) {
			break
		}

		switch result.Kind() {
		case ValKindS32, ValKindU32:
			memory.WriteUint32Le(offset, result.U32())
			offset += 4
		case ValKindS64, ValKindU64:
			memory.WriteUint64Le(offset, result.U64())
			offset += 8
		case ValKindF32:
			memory.WriteUint32Le(offset, result.U32())
			offset += 4
		case ValKindF64:
			memory.WriteUint64Le(offset, result.U64())
			offset += 8
		case ValKindBool:
			if result.Bool() {
				memory.WriteByteAt(offset, 1)
			} else {
				memory.WriteByteAt(offset, 0)
			}
			offset += 1
		case ValKindOwn, ValKindBorrow:
			memory.WriteUint32Le(offset, result.U32())
			offset += 4
		case ValKindList:
			// List lowering: allocate memory, write elements, return (ptr, len)
			list := result.List()
			listLen := uint32(len(list))

			// Calculate element size and alignment based on element type
			var elemSize uint32 = 4
			var alignment uint32 = 4
			if len(list) > 0 {
				elemSize = elementSizeForKind(list[0].Kind())
				alignment = alignmentForKind(list[0].Kind())
			}
			listSize := listLen * elemSize

			// Allocate memory using realloc per Canonical ABI spec
			var ptr uint32
			if listSize > 0 {
				if reallocFunc == nil {
					return fmt.Errorf("list lowering requires realloc function")
				}
				// realloc(old_ptr, old_size, align, new_size) -> new_ptr
				allocResults, err := reallocFunc.Call(ctx, 0, 0, uint64(alignment), uint64(listSize))
				if err != nil {
					return fmt.Errorf("realloc for list failed: %w", err)
				}
				ptr = uint32(allocResults[0])
			}
			// For empty lists (listSize == 0), ptr remains 0, which is valid

			// Write list elements to allocated memory
			for j, elem := range list {
				elemOffset := ptr + uint32(j)*elemSize
				if err := writeListElement(memory, elemOffset, elem); err != nil {
					return fmt.Errorf("failed to write list element %d: %w", j, err)
				}
			}

			// Write (ptr, len) tuple to retptr
			memory.WriteUint32Le(offset, ptr)
			offset += 4
			memory.WriteUint32Le(offset, listLen)
			offset += 4
		case ValKindString:
			// String lowering: allocate memory, write bytes, return (ptr, len)
			str := result.StringVal()
			strLen := uint32(len(str))

			var ptr uint32
			if strLen > 0 {
				if reallocFunc == nil {
					return fmt.Errorf("string lowering requires realloc function")
				}
				// realloc(old_ptr, old_size, align, new_size) -> new_ptr
				// UTF-8 strings have alignment 1
				allocResults, err := reallocFunc.Call(ctx, 0, 0, 1, uint64(strLen))
				if err != nil {
					return fmt.Errorf("realloc for string failed: %w", err)
				}
				ptr = uint32(allocResults[0])

				// Write string bytes to memory
				memory.Write(ptr, []byte(str))
			}

			// Write (ptr, len) tuple to retptr
			memory.WriteUint32Le(offset, ptr)
			offset += 4
			memory.WriteUint32Le(offset, strLen)
			offset += 4
		case ValKindResult:
			// Result lowering: write discriminant + payload
			isOk, okVal, errVal := result.Result()
			if isOk {
				// Ok case: discriminant=0
				memory.WriteByteAt(offset, 0)
				offset += 4 // Align to 4 bytes for payload
				if okVal != nil {
					// Recursively write the ok payload
					if err := writeResultsToMemory(ctx, memory, reallocFunc, offset, []Val{*okVal}, nil); err != nil {
						return fmt.Errorf("failed to write result ok payload: %w", err)
					}
					offset += sizeOfVal(*okVal)
				}
			} else {
				// Error case: discriminant=1
				memory.WriteByteAt(offset, 1)
				offset += 4 // Align to 4 bytes for payload
				if errVal != nil {
					// Recursively write the error payload
					if err := writeResultsToMemory(ctx, memory, reallocFunc, offset, []Val{*errVal}, nil); err != nil {
						return fmt.Errorf("failed to write result error payload: %w", err)
					}
					offset += sizeOfVal(*errVal)
				}
			}
		case ValKindVariant:
			// Variant lowering: write discriminant + payload
			caseName, payload := result.Variant()
			// For now, write discriminant as first i32 and payload recursively
			// Note: proper implementation needs type info for discriminant index
			_ = caseName // Would need type info to map name to index
			memory.WriteUint32Le(offset, 0) // Placeholder discriminant
			offset += 4
			if payload != nil {
				if err := writeResultsToMemory(ctx, memory, reallocFunc, offset, []Val{*payload}, nil); err != nil {
					return fmt.Errorf("failed to write variant payload: %w", err)
				}
				offset += sizeOfVal(*payload)
			}
		case ValKindOption:
			// Option lowering: write discriminant + payload
			payload := result.Option()
			if payload == nil {
				// None case: discriminant=0
				memory.WriteByteAt(offset, 0)
				offset += 4 // Align to 4 bytes
			} else {
				// Some case: discriminant=1
				memory.WriteByteAt(offset, 1)
				offset += 4 // Align to 4 bytes for payload
				if err := writeResultsToMemory(ctx, memory, reallocFunc, offset, []Val{*payload}, nil); err != nil {
					return fmt.Errorf("failed to write option payload: %w", err)
				}
				offset += sizeOfVal(*payload)
			}
		default:
			// For other types, write as i32 for now
			memory.WriteUint32Le(offset, result.U32())
			offset += 4
		}
	}
	return nil
}

// coreSignature returns the complete core function signature for a lowered function.
// It computes the flattened parameter and result types according to the Canonical ABI.
// If needsRetptr is true, an i32 param is appended for the return pointer.
func coreSignature(paramTypes, resultTypes []types.ValType) (params, results []api.ValueType, needsRetptr bool) {
	params = flattenParams(paramTypes)
	results, needsRetptr = flattenResults(resultTypes)

	if needsRetptr {
		// Append retptr parameter (per Canonical ABI spec, retptr comes after all other params)
		params = append(params, api.ValueTypeI32)
	}

	return params, results, needsRetptr
}

// flattenParams converts component parameter types to core wasm types.
func flattenParams(params []types.ValType) []api.ValueType {
	var result []api.ValueType
	for _, p := range params {
		result = append(result, flattenValType(p)...)
	}
	return result
}

// flattenResults converts component result types to core wasm types.
// Returns the flattened types and whether a retptr is needed.
func flattenResults(results []types.ValType) ([]api.ValueType, bool) {
	var flat []api.ValueType
	for _, r := range results {
		flat = append(flat, flattenValType(r)...)
	}

	if len(flat) > MaxFlatResults {
		return nil, true // Use retptr
	}
	return flat, false
}

// flattenValType converts a single component type to core wasm types.
// This implements the flattening rules from the Canonical ABI specification.
func flattenValType(t types.ValType) []api.ValueType {
	switch v := t.(type) {
	case types.Bool:
		return []api.ValueType{api.ValueTypeI32}
	case types.S8, types.U8, types.S16, types.U16, types.S32, types.U32:
		return []api.ValueType{api.ValueTypeI32}
	case types.S64, types.U64:
		return []api.ValueType{api.ValueTypeI64}
	case types.F32:
		return []api.ValueType{api.ValueTypeF32}
	case types.F64:
		return []api.ValueType{api.ValueTypeF64}
	case types.Char:
		return []api.ValueType{api.ValueTypeI32} // Unicode scalar value
	case types.String:
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32} // ptr, len
	case types.List:
		return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32} // ptr, len
	case types.Own, types.Borrow:
		return []api.ValueType{api.ValueTypeI32} // Handle index
	case types.Record:
		return flattenRecordType(v)
	case types.Tuple:
		return flattenTupleType(v)
	case types.Option:
		return flattenOptionType(v)
	case types.Result:
		return flattenResultType(v)
	case types.Enum:
		return []api.ValueType{api.ValueTypeI32} // Discriminant
	case types.Flags:
		return flattenFlagsType(v)
	default:
		// For unknown types, assume they fit in i32 as a fallback
		return []api.ValueType{api.ValueTypeI32}
	}
}

// flattenRecordType flattens a record type by flattening each field sequentially.
func flattenRecordType(r types.Record) []api.ValueType {
	var result []api.ValueType
	for _, f := range r.Fields {
		result = append(result, flattenValType(f.Type)...)
	}
	return result
}

// flattenTupleType flattens a tuple type by flattening each element sequentially.
func flattenTupleType(t types.Tuple) []api.ValueType {
	var result []api.ValueType
	for _, elemType := range t.Types {
		result = append(result, flattenValType(elemType)...)
	}
	return result
}

// flattenOptionType flattens an option type.
// Option is sugar for variant { none, some(T) }.
func flattenOptionType(o types.Option) []api.ValueType {
	// Discriminant (none=0, some=1)
	result := []api.ValueType{api.ValueTypeI32}

	// Payload for some case
	if o.Some != nil {
		result = append(result, flattenValType(o.Some)...)
	}

	return result
}

// flattenResultType flattens a result type.
// Result is sugar for variant { ok(T), error(E) }.
func flattenResultType(r types.Result) []api.ValueType {
	// Discriminant (ok=0, error=1)
	result := []api.ValueType{api.ValueTypeI32}

	// Find max payload between ok and error
	var okFlat, errFlat []api.ValueType
	if r.Ok != nil {
		okFlat = flattenValType(r.Ok)
	}
	if r.Error != nil {
		errFlat = flattenValType(r.Error)
	}

	// Take the max payload count
	maxLen := len(okFlat)
	if len(errFlat) > maxLen {
		maxLen = len(errFlat)
	}

	// Join the types at each position
	for i := 0; i < maxLen; i++ {
		var okType, errType api.ValueType
		if i < len(okFlat) {
			okType = okFlat[i]
		}
		if i < len(errFlat) {
			errType = errFlat[i]
		}
		// Use the wider type
		if isWiderValueType(errType, okType) {
			result = append(result, errType)
		} else {
			result = append(result, okType)
		}
	}

	return result
}

// flattenFlagsType flattens a flags type.
// Flags are represented as one or more i32 values depending on the number of flags.
func flattenFlagsType(f types.Flags) []api.ValueType {
	n := len(f.Names)
	if n == 0 {
		return nil
	}
	// Number of i32s needed: ceil(n / 32)
	numI32s := (n + 31) / 32
	result := make([]api.ValueType, numI32s)
	for i := range result {
		result[i] = api.ValueTypeI32
	}
	return result
}

// isWiderValueType returns true if a is a wider type than b.
// Width order: i32 < f32 < i64 < f64
func isWiderValueType(a, b api.ValueType) bool {
	return valueTypeWidth(a) > valueTypeWidth(b)
}

// valueTypeWidth returns the width ordering of a type.
func valueTypeWidth(t api.ValueType) int {
	switch t {
	case api.ValueTypeI32:
		return 1
	case api.ValueTypeF32:
		return 2
	case api.ValueTypeI64:
		return 3
	case api.ValueTypeF64:
		return 4
	default:
		return 0
	}
}

// valTypesToAPITypes converts a slice of wasm.ValueType to api.ValueType.
// Since wasm.ValueType is an alias for api.ValueType, this is just a slice copy.
// Always returns a non-nil slice (empty slice for nil input).
func valTypesToAPITypes(vals []wasm.ValueType) []api.ValueType {
	if len(vals) == 0 {
		return []api.ValueType{}
	}
	result := make([]api.ValueType, len(vals))
	copy(result, vals)
	return result
}
