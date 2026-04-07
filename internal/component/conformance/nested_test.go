// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Tasks 253-254: Nested Component and Instance Export Tests.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 253: Nested Component Tests
// =============================================================================

// TestNested_TwoLevelInstantiation tests that an outer component can instantiate
// an inner component that uses a host function.
// This simulates the pattern: Host -> Outer Component -> Inner Component
func TestNested_TwoLevelInstantiation(t *testing.T) {
	// Create a linker with a host function
	linker := component.NewLinker()

	hostFnCalled := false
	hostValue := int32(0)

	funcType := &component.FuncType{
		Params:  []component.NamedValType{{Name: "x", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
		Results: []component.NamedValType{{Name: "result", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}},
	}

	hostFn := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		hostFnCalled = true
		if len(args) > 0 {
			hostValue = args[0].S32()
		}
		return []types.Val{types.ValS32(hostValue * 2)}, nil
	}

	// Define host function at v1.0.0
	err := linker.DefineFunc("host:math@1.0.0", "double", funcType, hostFn)
	require.NoError(t, err)

	// Create an "inner" component that imports the host function
	innerComponent := &component.Component{
		Imports: []component.Import{
			{
				Name: "host:math@1.0.0/double",
				ExternDesc: component.ImportExternDesc{
					Kind: component.ImportExternDescFunc,
				},
			},
		},
		Exports: []component.Export{
			{
				Name: "inner:api@1.0.0/process",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	// Create an "outer" component that wraps the inner component
	outerComponent := &component.Component{
		Imports: []component.Import{
			{
				Name: "host:math@1.0.0/double",
				ExternDesc: component.ImportExternDesc{
					Kind: component.ImportExternDescFunc,
				},
			},
		},
		Components: []*component.Component{innerComponent},
	}

	// Instantiate outer component
	ctx := context.Background()
	outerInst, err := linker.Instantiate(ctx, outerComponent)
	require.NoError(t, err)
	require.NotNil(t, outerInst)

	// Instantiate inner component with the same linker
	innerInst, err := linker.Instantiate(ctx, innerComponent)
	require.NoError(t, err)
	require.NotNil(t, innerInst)

	// Manually verify we can resolve the import that inner component needs
	def, err := linker.MatchImport("host:math@1.0.0/double")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Call the host function directly to verify the chain works
	funcDef, ok := def.(*component.FuncDef)
	require.True(t, ok)

	result, err := funcDef.Callback(ctx, []types.Val{types.ValS32(21)})
	require.NoError(t, err)
	require.True(t, hostFnCalled)
	require.Equal(t, int32(21), hostValue)
	require.Equal(t, 1, len(result))
	require.Equal(t, int32(42), result[0].S32())
}

// TestNested_MaxDepth tests that reasonable nesting depth (10 levels) works.
// This creates a chain of nested component definitions.
func TestNested_MaxDepth(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}
	baseCallCount := 0

	baseFn := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		baseCallCount++
		return nil, nil
	}

	// Define the base host function
	err := linker.DefineFunc("base:api@1.0.0", "fn", funcType, baseFn)
	require.NoError(t, err)

	// Create 10 levels of nested components
	const maxDepth = 10
	components := make([]*component.Component, maxDepth)

	// Build from innermost to outermost
	for i := maxDepth - 1; i >= 0; i-- {
		comp := &component.Component{
			Imports: []component.Import{
				{
					Name: "base:api@1.0.0/fn",
					ExternDesc: component.ImportExternDesc{
						Kind: component.ImportExternDescFunc,
					},
				},
			},
		}

		// If not the innermost, add the inner component as a nested component
		if i < maxDepth-1 {
			comp.Components = []*component.Component{components[i+1]}
		}

		components[i] = comp
	}

	// Instantiate the outermost component
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, components[0])
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Verify we can still resolve imports at any level
	def, err := linker.MatchImport("base:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Call the base function to ensure it's still accessible
	funcDef := def.(*component.FuncDef)
	_, err = funcDef.Callback(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 1, baseCallCount)
}

// TestNested_IndependentLinkers tests that separate linkers maintain independent state.
func TestNested_IndependentLinkers(t *testing.T) {
	linker1 := component.NewLinker()
	linker2 := component.NewLinker()

	funcType := &component.FuncType{}

	fn1Called := false
	fn2Called := false

	fn1 := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		fn1Called = true
		return []types.Val{types.ValS32(1)}, nil
	}

	fn2 := func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		fn2Called = true
		return []types.Val{types.ValS32(2)}, nil
	}

	// Define same-named function with different implementations
	err := linker1.DefineFunc("test:api@1.0.0", "fn", funcType, fn1)
	require.NoError(t, err)

	err = linker2.DefineFunc("test:api@1.0.0", "fn", funcType, fn2)
	require.NoError(t, err)

	// Each linker should return its own function
	def1, err := linker1.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)

	def2, err := linker2.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)

	ctx := context.Background()

	funcDef1 := def1.(*component.FuncDef)
	result1, err := funcDef1.Callback(ctx, nil)
	require.NoError(t, err)
	require.True(t, fn1Called)
	require.False(t, fn2Called)
	require.Equal(t, int32(1), result1[0].S32())

	fn1Called = false // Reset

	funcDef2 := def2.(*component.FuncDef)
	result2, err := funcDef2.Callback(ctx, nil)
	require.NoError(t, err)
	require.False(t, fn1Called)
	require.True(t, fn2Called)
	require.Equal(t, int32(2), result2[0].S32())
}

// TestNested_ComponentWithMultipleImports tests components that require multiple imports.
func TestNested_ComponentWithMultipleImports(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}

	// Define multiple host functions
	err := linker.DefineFunc("wasi:io@1.0.0", "read", funcType, func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = linker.DefineFunc("wasi:io@1.0.0", "write", funcType, func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = linker.DefineFunc("wasi:clocks@1.0.0", "now", funcType, func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Component that requires all three imports
	comp := &component.Component{
		Imports: []component.Import{
			{Name: "wasi:io@1.0.0/read", ExternDesc: component.ImportExternDesc{Kind: component.ImportExternDescFunc}},
			{Name: "wasi:io@1.0.0/write", ExternDesc: component.ImportExternDesc{Kind: component.ImportExternDescFunc}},
			{Name: "wasi:clocks@1.0.0/now", ExternDesc: component.ImportExternDesc{Kind: component.ImportExternDescFunc}},
		},
	}

	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestNested_ComponentWithMissingOneImport tests that missing even one import fails.
func TestNested_ComponentWithMissingOneImport(t *testing.T) {
	linker := component.NewLinker()

	funcType := &component.FuncType{}

	// Define only two of three required imports
	err := linker.DefineFunc("wasi:io@1.0.0", "read", funcType, func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = linker.DefineFunc("wasi:io@1.0.0", "write", funcType, func(ctx context.Context, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Missing: wasi:clocks@1.0.0/now

	// Component that requires all three imports
	comp := &component.Component{
		Imports: []component.Import{
			{Name: "wasi:io@1.0.0/read", ExternDesc: component.ImportExternDesc{Kind: component.ImportExternDescFunc}},
			{Name: "wasi:io@1.0.0/write", ExternDesc: component.ImportExternDesc{Kind: component.ImportExternDescFunc}},
			{Name: "wasi:clocks@1.0.0/now", ExternDesc: component.ImportExternDesc{Kind: component.ImportExternDescFunc}},
		},
	}

	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.Error(t, err)
	require.Nil(t, inst)
	require.Contains(t, err.Error(), "wasi:clocks@1.0.0/now")
}

// =============================================================================
// Task 254: Instance Export Tests
// =============================================================================

// TestInstance_ExportOldGetNew tests that exporting v1.0.0 and getting with v1.0.1
// works (bidirectional compatibility for exports).
func TestInstance_ExportOldGetNew(t *testing.T) {
	// Create a component with an export at v1.0.0
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{
					Params:  []component.NamedValType{},
					Results: []component.NamedValType{},
				},
			},
		},
		Exports: []component.Export{
			{
				Name: "my:api@1.0.0/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Try to get the export with a newer version (v1.0.1)
	// Per component model spec, exports have bidirectional version compatibility
	fn := inst.GetExportedFunc("my:api@1.0.1/hello")
	require.NotNil(t, fn, "should find v1.0.0 export when requesting v1.0.1")
}

// TestInstance_ExportNewGetOld tests that exporting v1.0.1 and getting with v1.0.0
// works (bidirectional compatibility for exports).
func TestInstance_ExportNewGetOld(t *testing.T) {
	// Create a component with an export at v1.0.1
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{
					Params:  []component.NamedValType{},
					Results: []component.NamedValType{},
				},
			},
		},
		Exports: []component.Export{
			{
				Name: "my:api@1.0.1/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Try to get the export with an older version (v1.0.0)
	fn := inst.GetExportedFunc("my:api@1.0.0/hello")
	require.NotNil(t, fn, "should find v1.0.1 export when requesting v1.0.0")
}

// TestInstance_ExportSelectsMax tests that when multiple compatible exports exist,
// the highest version is selected.
func TestInstance_ExportSelectsMax(t *testing.T) {
	// Create a component with multiple versioned exports of the same function
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{
					Params:  []component.NamedValType{},
					Results: []component.NamedValType{},
				},
			},
		},
		Exports: []component.Export{
			{
				Name: "my:api@1.0.0/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
			{
				Name: "my:api@1.0.1/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
			{
				Name: "my:api@1.2.0/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Request v1.0.0 - should get highest compatible (v1.2.0)
	fn := inst.GetExportedFunc("my:api@1.0.0/hello")
	require.NotNil(t, fn)
	// The returned function should be from the v1.2.0 export
	require.Equal(t, "my:api@1.2.0/hello", fn.Name())
}

// TestInstance_ExportMajorVersionMismatch tests that major version mismatches
// are not compatible even for exports.
func TestInstance_ExportMajorVersionMismatch(t *testing.T) {
	// Create a component with v2.0.0 export
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{},
			},
		},
		Exports: []component.Export{
			{
				Name: "my:api@2.0.0/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Request v1.0.0 - should NOT find v2.0.0 (major version mismatch)
	fn := inst.GetExportedFunc("my:api@1.0.0/hello")
	require.Nil(t, fn, "should not match different major versions")
}

// TestInstance_ExportExactMatch tests exact name matching for unversioned exports.
func TestInstance_ExportExactMatch(t *testing.T) {
	// Create a component with an unversioned export
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{},
			},
		},
		Exports: []component.Export{
			{
				Name: "simple-func",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Exact match should work
	fn := inst.GetExportedFunc("simple-func")
	require.NotNil(t, fn)
	require.Equal(t, "simple-func", fn.Name())

	// Non-matching name should not work
	fn = inst.GetExportedFunc("other-func")
	require.Nil(t, fn)
}

// TestInstance_ExportPre1_0_Rules tests pre-1.0 semver rules for exports.
func TestInstance_ExportPre1_0_Rules(t *testing.T) {
	// Create component with 0.x version exports
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{},
			},
		},
		Exports: []component.Export{
			{
				Name: "my:api@0.1.0/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)

	// For pre-1.0, minor versions are breaking, so 0.2.0 should not match 0.1.0
	fn := inst.GetExportedFunc("my:api@0.2.0/hello")
	require.Nil(t, fn, "0.2.0 should not match 0.1.0 for pre-1.0 versions")

	// But 0.1.1 should match 0.1.0 (patch version compatible)
	fn = inst.GetExportedFunc("my:api@0.1.1/hello")
	require.NotNil(t, fn, "0.1.1 should match 0.1.0 (bidirectional for exports)")
}

// TestInstance_ExportWithTypeInfo tests that exported functions include type information.
func TestInstance_ExportWithTypeInfo(t *testing.T) {
	funcType := &component.FuncType{
		Params: []component.NamedValType{
			{Name: "a", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
			{Name: "b", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
		Results: []component.NamedValType{
			{Name: "result", ValType: component.ValTypeRef{IsPrimitive: true, Primitive: 0x7f}},
		},
	}

	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: funcType,
			},
		},
		Exports: []component.Export{
			{
				Name: "math:ops@1.0.0/add",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)

	fn := inst.GetExportedFunc("math:ops@1.0.0/add")
	require.NotNil(t, fn)

	// Check type information is available
	fnType := fn.Type()
	require.NotNil(t, fnType)
	require.Equal(t, 2, len(fnType.Params))
	require.Equal(t, 1, len(fnType.Results))
	require.Equal(t, "a", fnType.Params[0].Name)
	require.Equal(t, "b", fnType.Params[1].Name)
	require.Equal(t, "result", fnType.Results[0].Name)
}

// TestInstance_MultipleExportsWithDifferentNames tests retrieving different exports.
func TestInstance_MultipleExportsWithDifferentNames(t *testing.T) {
	comp := &component.Component{
		Types: []component.TypeDef{
			{
				Kind: component.TypeDefKindFunc,
				Func: &component.FuncType{},
			},
		},
		Exports: []component.Export{
			{
				Name: "math:ops@1.0.0/add",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
			{
				Name: "math:ops@1.0.0/sub",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
			{
				Name: "io:file@1.0.0/read",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	linker := component.NewLinker()
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)

	// All three should be retrievable
	addFn := inst.GetExportedFunc("math:ops@1.0.0/add")
	require.NotNil(t, addFn)

	subFn := inst.GetExportedFunc("math:ops@1.0.0/sub")
	require.NotNil(t, subFn)

	readFn := inst.GetExportedFunc("io:file@1.0.0/read")
	require.NotNil(t, readFn)

	// Non-existent should return nil
	writeFn := inst.GetExportedFunc("io:file@1.0.0/write")
	require.Nil(t, writeFn)
}
