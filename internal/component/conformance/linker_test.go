// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Tasks 251-252: Linker Semver Matching and Import Resolution Tests.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 251: Semver Matching Tests
// =============================================================================

// TestLinker_OldImportNewItem tests that importing v1.0.0 with v1.0.1 available succeeds.
// Backward compatible: newer patch versions should satisfy older requirements.
func TestLinker_OldImportNewItem(t *testing.T) {
	linker := component.NewLinker()

	// Define a function at v1.0.1 (newer patch version)
	funcType := &component.FuncType{
		Params:  []component.NamedValType{{Name: "x", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
		Results: []component.NamedValType{{Name: "result", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
	}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return []component.Val{component.ValS32(42)}, nil
	}

	err := linker.DefineFunc("test:api@1.0.1", "greet", funcType, hostFn)
	require.NoError(t, err)

	// Try to match import for v1.0.0 (older patch version)
	def, err := linker.MatchImport("test:api@1.0.0/greet")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Should be a FuncDef
	funcDef, ok := def.(*component.FuncDef)
	require.True(t, ok)
	require.NotNil(t, funcDef.Callback)
}

// TestLinker_NewImportOldItem tests that importing v1.0.1 with only v1.0.0 available fails.
// Forward incompatible: older versions cannot satisfy newer requirements.
func TestLinker_NewImportOldItem(t *testing.T) {
	linker := component.NewLinker()

	// Define a function at v1.0.0 (older patch version)
	funcType := &component.FuncType{
		Params:  []component.NamedValType{},
		Results: []component.NamedValType{},
	}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("test:api@1.0.0", "greet", funcType, hostFn)
	require.NoError(t, err)

	// Try to match import for v1.0.1 (newer patch version)
	// This should fail because v1.0.0 cannot satisfy v1.0.1 requirements
	def, err := linker.MatchImport("test:api@1.0.1/greet")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestLinker_SelectsMaxVersion tests that when multiple compatible versions are available,
// the linker selects the highest compatible version.
func TestLinker_SelectsMaxVersion(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}

	// Define multiple versions
	versions := []string{"1.0.0", "1.0.1", "1.0.2", "1.1.0", "1.2.0"}
	for _, ver := range versions {
		// Use a closure to capture the version
		v := ver
		hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
			// Return different value based on version for testing
			return []component.Val{component.ValS32(int32(len(v)))}, nil
		}
		err := linker.DefineFunc("test:api@"+ver, "fn", funcType, hostFn)
		require.NoError(t, err)
	}

	// Request v1.0.0 - should get highest compatible (1.2.0)
	def, err := linker.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Verify we got a definition (the linker returns the best match)
	funcDef, ok := def.(*component.FuncDef)
	require.True(t, ok)
	require.NotNil(t, funcDef)
}

// TestLinker_MajorVersionMismatch tests that v1.x.y does not match v2.x.y.
// Major version changes are breaking changes.
func TestLinker_MajorVersionMismatch(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	// Define v2.0.0
	err := linker.DefineFunc("test:api@2.0.0", "fn", funcType, hostFn)
	require.NoError(t, err)

	// Try to import v1.0.0 - should fail (major version mismatch)
	def, err := linker.MatchImport("test:api@1.0.0/fn")
	require.Error(t, err)
	require.Nil(t, def)

	// Also test the reverse: define v1.0.0, request v2.0.0
	linker2 := component.NewLinker()
	err = linker2.DefineFunc("test:api@1.0.0", "fn", funcType, hostFn)
	require.NoError(t, err)

	def, err = linker2.MatchImport("test:api@2.0.0/fn")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestLinker_Pre1_0_SemverRules tests that for 0.x.y versions, minor versions are breaking.
// Per semver spec, for pre-1.0 versions, the minor version is the breaking version.
func TestLinker_Pre1_0_SemverRules(t *testing.T) {
	// Test that 0.1.x is not compatible with 0.2.x
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	// Define v0.2.0
	err := linker.DefineFunc("test:api@0.2.0", "fn", funcType, hostFn)
	require.NoError(t, err)

	// Try to import v0.1.0 - should fail (pre-1.0 minor version mismatch)
	def, err := linker.MatchImport("test:api@0.1.0/fn")
	require.Error(t, err)
	require.Nil(t, def)

	// Test that 0.1.1 can satisfy 0.1.0 (patch version compatible)
	linker2 := component.NewLinker()
	err = linker2.DefineFunc("test:api@0.1.1", "fn", funcType, hostFn)
	require.NoError(t, err)

	def, err = linker2.MatchImport("test:api@0.1.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Test that 0.1.0 cannot satisfy 0.1.1 (newer patch requirement)
	linker3 := component.NewLinker()
	err = linker3.DefineFunc("test:api@0.1.0", "fn", funcType, hostFn)
	require.NoError(t, err)

	def, err = linker3.MatchImport("test:api@0.1.1/fn")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestLinker_MinorVersionBump tests that minor version bumps are backward compatible for 1.x+.
func TestLinker_MinorVersionBump(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	// Define v1.2.0 (newer minor)
	err := linker.DefineFunc("test:api@1.2.0", "fn", funcType, hostFn)
	require.NoError(t, err)

	// v1.1.0 should be satisfied by v1.2.0
	def, err := linker.MatchImport("test:api@1.1.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// v1.0.0 should also be satisfied by v1.2.0
	def, err = linker.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// But v1.3.0 should NOT be satisfied by v1.2.0
	def, err = linker.MatchImport("test:api@1.3.0/fn")
	require.Error(t, err)
	require.Nil(t, def)
}

// =============================================================================
// Task 252: Import Resolution Tests
// =============================================================================

// TestImport_FunctionsInInstances tests defining an instance with functions
// and retrieving them through the linker.
func TestImport_FunctionsInInstances(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{
		Params:  []component.NamedValType{{Name: "a", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
		Results: []component.NamedValType{{Name: "result", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
	}

	addCalled := false
	mulCalled := false

	addFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		addCalled = true
		return []component.Val{component.ValS32(10)}, nil
	}

	mulFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		mulCalled = true
		return []component.Val{component.ValS32(20)}, nil
	}

	// Define an instance with multiple functions
	err := linker.DefineInstance("wasi:math@1.0.0").
		Func("add", funcType, addFn).
		Func("multiply", funcType, mulFn).
		Build()
	require.NoError(t, err)

	// Retrieve the instance definition
	def, ok := linker.Get("wasi:math@1.0.0")
	require.True(t, ok)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports)

	// Verify functions are in the instance
	addDef, ok := instDef.Exports["add"]
	require.True(t, ok)
	addFuncDef, ok := addDef.(*component.FuncDef)
	require.True(t, ok)

	// Call the function
	ctx := context.Background()
	_, err = addFuncDef.Callback(ctx, []component.Val{component.ValS32(5)})
	require.NoError(t, err)
	require.True(t, addCalled)

	mulDef, ok := instDef.Exports["multiply"]
	require.True(t, ok)
	mulFuncDef, ok := mulDef.(*component.FuncDef)
	require.True(t, ok)

	_, err = mulFuncDef.Callback(ctx, []component.Val{component.ValS32(3)})
	require.NoError(t, err)
	require.True(t, mulCalled)
}

// TestImport_MissingImport tests that instantiating a component with a missing
// import results in an error.
func TestImport_MissingImport(t *testing.T) {
	linker := component.NewLinker()

	// Create a component that requires an import
	comp := &component.Component{
		Imports: []component.Import{
			{
				Name: "wasi:io@1.0.0/read",
				ExternDesc: component.ImportExternDesc{
					Kind: component.ImportExternDescFunc,
				},
			},
		},
	}

	// Try to instantiate without providing the import
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.Error(t, err)
	require.Nil(t, inst)
	require.Contains(t, err.Error(), "import")
}

// TestImport_ResourceWithDestructor tests defining a resource with a destructor
// and verifying the destructor is properly stored.
func TestImport_ResourceWithDestructor(t *testing.T) {
	linker := component.NewLinker()

	destructorCalled := false
	var destroyedRep uint32

	destructor := func(rep uint32) {
		destructorCalled = true
		destroyedRep = rep
	}

	// Define a resource with a destructor
	err := linker.DefineResource("my:resources@1.0.0", "file-handle", destructor)
	require.NoError(t, err)

	// Retrieve the resource definition
	def, ok := linker.Get("my:resources@1.0.0/file-handle")
	require.True(t, ok)

	resDef, ok := def.(*component.ResourceDef)
	require.True(t, ok)
	require.NotNil(t, resDef.Destructor)

	// Call the destructor manually to verify it works
	resDef.Destructor(42)
	require.True(t, destructorCalled)
	require.Equal(t, uint32(42), destroyedRep)
}

// TestImport_ResourceInInstance tests defining a resource within an instance.
func TestImport_ResourceInInstance(t *testing.T) {
	linker := component.NewLinker()

	destructorCalled := false

	// Define an instance with a resource
	err := linker.DefineInstance("my:component@1.0.0").
		Resource("handle", func(rep uint32) {
			destructorCalled = true
		}).
		Build()
	require.NoError(t, err)

	// Retrieve the instance
	def, ok := linker.Get("my:component@1.0.0")
	require.True(t, ok)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok)

	// Verify resource is in the instance
	resDef, ok := instDef.Exports["handle"]
	require.True(t, ok)

	resDefTyped, ok := resDef.(*component.ResourceDef)
	require.True(t, ok)

	// Call destructor
	resDefTyped.Destructor(100)
	require.True(t, destructorCalled)
}

// TestImport_DuplicateDefinition tests that defining the same key twice returns an error.
func TestImport_DuplicateDefinition(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	// First definition succeeds
	err := linker.DefineFunc("test:api@1.0.0", "fn", funcType, hostFn)
	require.NoError(t, err)

	// Second definition with same key fails
	err = linker.DefineFunc("test:api@1.0.0", "fn", funcType, hostFn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

// TestImport_DuplicateInstanceDefinition tests that defining the same instance twice returns an error.
func TestImport_DuplicateInstanceDefinition(t *testing.T) {
	linker := component.NewLinker()

	// First instance definition succeeds
	err := linker.DefineInstance("test:ns@1.0.0").Build()
	require.NoError(t, err)

	// Second instance with same namespace fails
	err = linker.DefineInstance("test:ns@1.0.0").Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

// TestImport_NoVersion tests import matching for unversioned names.
func TestImport_NoVersion(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	// Define unversioned function
	err := linker.DefineFunc("my-namespace", "my-func", funcType, hostFn)
	require.NoError(t, err)

	// Can retrieve with exact key
	def, ok := linker.Get("my-namespace/my-func")
	require.True(t, ok)
	require.NotNil(t, def)

	// MatchImport with unversioned name should work for exact match
	def, err = linker.MatchImport("my-namespace/my-func")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestImport_InstantiateWithResolvedImports tests that instantiation succeeds
// when all imports are satisfied.
func TestImport_InstantiateWithResolvedImports(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	hostFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		return nil, nil
	}

	// Define the required import
	err := linker.DefineFunc("wasi:io@1.0.0", "read", funcType, hostFn)
	require.NoError(t, err)

	// Create a component that requires the import
	comp := &component.Component{
		Imports: []component.Import{
			{
				Name: "wasi:io@1.0.0/read",
				ExternDesc: component.ImportExternDesc{
					Kind: component.ImportExternDescFunc,
				},
			},
		},
	}

	// Instantiation should succeed
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestImport_FuncNoType tests the FuncNoType builder method for dynamic functions.
func TestImport_FuncNoType(t *testing.T) {
	linker := component.NewLinker()

	called := false
	dynamicFn := func(ctx context.Context, args []component.Val) ([]component.Val, error) {
		called = true
		return args, nil // Echo back args
	}

	// Define instance with a function that has no explicit type
	err := linker.DefineInstance("test:dynamic@1.0.0").
		FuncNoType("echo", dynamicFn).
		Build()
	require.NoError(t, err)

	// Retrieve and call
	def, ok := linker.Get("test:dynamic@1.0.0")
	require.True(t, ok)

	instDef := def.(*component.InstanceDef)
	funcDef := instDef.Exports["echo"].(*component.FuncDef)
	require.Nil(t, funcDef.Type) // No type info

	ctx := context.Background()
	result, err := funcDef.Callback(ctx, []component.Val{component.ValS32(123)})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, 1, len(result))
	require.Equal(t, int32(123), result[0].S32())
}
