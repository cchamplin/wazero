// internal/component/instance_pre.go
//
// InstancePre holds a pre-computed import resolution for a compiled component.
// Import resolution and type-checking are done once at InstantiatePre time.
// Multiple instances can then be created cheaply from the same InstancePre.
//
// Reference: wasmtime crates/wasmtime/src/runtime/component/instance.rs InstancePre.
package component

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/internal/component/runtime"
)

// InstancePre holds a pre-computed import resolution for a compiled component.
// Import resolution and type-checking are done once at InstantiatePre time.
// Multiple instances can then be created cheaply from the same InstancePre.
type InstancePre struct {
	linker           *ComponentLinker
	compiled         *CompiledComponent
	resolvedImports  map[string]Definition
	instanceToImport map[uint32]string
	tc               *TypeChecker
}

// InstantiatePre performs import resolution and type-checking against the
// compiled component and caches the result. The returned InstancePre can
// be used to create multiple instances cheaply via InstancePre.Instantiate.
func (l *ComponentLinker) InstantiatePre(compiled *CompiledComponent) (*InstancePre, error) {
	if compiled == nil {
		return nil, fmt.Errorf("InstantiatePre: compiled is nil")
	}
	c := compiled.Internal()
	if c == nil {
		return nil, fmt.Errorf("InstantiatePre: compiled.Internal() is nil")
	}

	resolvedImports, instanceToImport, tc, err := l.resolveImports(c)
	if err != nil {
		return nil, err
	}
	return &InstancePre{
		linker:           l,
		compiled:         compiled,
		resolvedImports:  resolvedImports,
		instanceToImport: instanceToImport,
		tc:               tc,
	}, nil
}

// resolveImports extracts import resolution out of Instantiate so it can
// be cached by InstancePre. It allocates the maps, creates a TypeChecker,
// and delegates to resolveAndCheckImports.
func (l *ComponentLinker) resolveImports(c *Component) (
	resolvedImports map[string]Definition,
	instanceToImport map[uint32]string,
	tc *TypeChecker,
	err error,
) {
	tc = NewTypeChecker(c)
	resolvedImports = make(map[string]Definition)
	instanceToImport = make(map[uint32]string)
	if err = l.resolveAndCheckImports(c, tc, resolvedImports, instanceToImport); err != nil {
		return nil, nil, nil, fmt.Errorf("resolve imports: %w", err)
	}
	return resolvedImports, instanceToImport, tc, nil
}

// Instantiate creates a new component instance using the pre-resolved imports.
// Each call creates a distinct instance with its own state.
func (ip *InstancePre) Instantiate(ctx context.Context) (*Instance, error) {
	l := ip.linker
	c := ip.compiled.Internal()

	// Step 1 -- Allocate instance + runtime.ComponentInstance.
	inst := newInstance(c, l.nextInstanceID(), nil)

	// Create a store-wide ResourceStore and wire it into the instance.
	store := runtime.NewResourceStore()
	inst.rt.Store = store
	store.RegisterInstance(inst.rt.ID, inst)

	// Step 2 -- Bind resource type declarations to runtime identities.
	if err := l.bindResourceTypes(inst, c); err != nil {
		return nil, fmt.Errorf("Instantiate: bind resource types: %w", err)
	}

	// Step 3 -- Build index spaces from aliases (funcSpace, memSpace).
	funcSpace := NewCoreFuncIndexSpace()
	memSpace := NewCoreMemoryIndexSpace()
	l.buildCoreIndexSpaces(c, funcSpace, memSpace)

	// Step 4 is already done (import resolution cached in ip).

	// Step 5 -- Populate value index space from value imports.
	l.populateValueImports(inst, c, ip.resolvedImports)

	// Step 6 -- Align instance index space with instance imports.
	l.alignInstanceImports(inst, c)

	// Step 7 -- Build component function index space from canon.lift
	// declarations + resolved function imports.
	l.buildComponentFuncs(inst, c, ip.resolvedImports)

	// Step 8 -- Build type index space for nested instantiation arg resolution.
	l.buildTypeSpace(inst, c)

	// Step 9 -- Process nested component instances.
	componentInstDefs, err := l.processNestedInstances(ctx, inst, c)
	if err != nil {
		return nil, fmt.Errorf("Instantiate: nested instances: %w", err)
	}

	// Step 10 -- Build canon lower / canon resource info maps.
	canonLowers, canonResources := l.buildCanonMaps(c)

	// Step 11 -- Build function alias map for inline instance resolution.
	funcAliases := l.buildFuncAliases(c)

	_ = ip.instanceToImport

	// Step 12 -- Instantiate core modules with wired host exports.
	if err := l.instantiateCoreModules(ctx, inst, c, ip.compiled.CompiledModules(),
		canonLowers, canonResources, funcAliases); err != nil {
		return nil, fmt.Errorf("Instantiate: core modules: %w", err)
	}

	// Step 12.5 -- Resolve pending guest destructors and reallocs.
	l.resolvePendingDtors(inst, funcSpace)
	l.resolvePendingReallocs(inst, funcSpace)

	// Step 13 -- Execute start function.
	if err := l.executeStartFunction(ctx, inst, c); err != nil {
		return nil, fmt.Errorf("Instantiate: start function: %w", err)
	}

	// Step 14 -- Wire exports.
	if err := l.wireExports(inst, c, componentInstDefs, funcSpace, memSpace); err != nil {
		return nil, fmt.Errorf("Instantiate: wire exports: %w", err)
	}
	return inst, nil
}

// Component returns the compiled component this InstancePre was created from.
func (ip *InstancePre) Component() *CompiledComponent {
	return ip.compiled
}
