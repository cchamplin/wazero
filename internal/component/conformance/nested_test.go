// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: nested-component / instance-export tests.
// Restored from Session 0 compile-fix stubs. Adapted to the current
// Linker HostFunc signature (no FuncType parameter).
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Nested Component Tests
// =============================================================================

// TestNested_TwoLevelInstantiation tests that an outer component can
// instantiate an inner component that uses a host function.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (LinkerInstance::func_new + nested
//	component instantiation with shared host functions).
//
// Spec: Component-model nested component instantiation.
func TestNested_TwoLevelInstantiation(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFnCalled := false
	hostValue := int32(0)

	err := linker.DefineFunc("host:math@1.0.0", "double", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		hostFnCalled = true
		if len(args) > 0 {
			hostValue = args[0].S32()
		}
		return []types.Val{types.ValS32(hostValue * 2)}, nil
	})
	require.NoError(t, err)

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

	ctx := context.Background()
	outerInst, err := linker.Instantiate(ctx, outerComponent)
	require.NoError(t, err)
	require.NotNil(t, outerInst)

	innerInst, err := linker.Instantiate(ctx, innerComponent)
	require.NoError(t, err)
	require.NotNil(t, innerInst)

	def, err := linker.MatchImport("host:math@1.0.0/double")
	require.NoError(t, err)
	require.NotNil(t, def)

	funcDef, ok := def.(*component.FuncDef)
	require.True(t, ok)

	result, err := funcDef.Callback(ctx, nil, []types.Val{types.ValS32(21)})
	require.NoError(t, err)
	require.True(t, hostFnCalled)
	require.Equal(t, int32(21), hostValue)
	require.Equal(t, 1, len(result))
	require.Equal(t, int32(42), result[0].S32())
}

// TestNested_MaxDepth tests that reasonable nesting depth (10 levels) works.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:1148 (Instantiate with deeply nested component
//	definitions).
//
// Spec: Component-model nested component depth.
func TestNested_MaxDepth(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	baseCallCount := 0

	err := linker.DefineFunc("base:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		baseCallCount++
		return nil, nil
	})
	require.NoError(t, err)

	const maxDepth = 10
	components := make([]*component.Component, maxDepth)

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
		if i < maxDepth-1 {
			comp.Components = []*component.Component{components[i+1]}
		}
		components[i] = comp
	}

	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, components[0])
	require.NoError(t, err)
	require.NotNil(t, inst)

	def, err := linker.MatchImport("base:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	funcDef := def.(*component.FuncDef)
	_, err = funcDef.Callback(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, baseCallCount)
}

// TestNested_IndependentLinkers tests that separate linkers maintain
// independent state.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (separate LinkerInstance trees maintain independent
//	definition maps).
//
// Spec: Component-model linker independence.
func TestNested_IndependentLinkers(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker1 := component.NewComponentLinker(rt)
	linker2 := component.NewComponentLinker(rt)

	fn1Called := false
	fn2Called := false

	err := linker1.DefineFunc("test:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		fn1Called = true
		return []types.Val{types.ValS32(1)}, nil
	})
	require.NoError(t, err)

	err = linker2.DefineFunc("test:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		fn2Called = true
		return []types.Val{types.ValS32(2)}, nil
	})
	require.NoError(t, err)

	def1, err := linker1.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)

	def2, err := linker2.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)

	ctx := context.Background()

	funcDef1 := def1.(*component.FuncDef)
	result1, err := funcDef1.Callback(ctx, nil, nil)
	require.NoError(t, err)
	require.True(t, fn1Called)
	require.False(t, fn2Called)
	require.Equal(t, int32(1), result1[0].S32())

	fn1Called = false

	funcDef2 := def2.(*component.FuncDef)
	result2, err := funcDef2.Callback(ctx, nil, nil)
	require.NoError(t, err)
	require.False(t, fn1Called)
	require.True(t, fn2Called)
	require.Equal(t, int32(2), result2[0].S32())
}

// TestNested_ComponentWithMultipleImports tests components that require
// multiple imports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (multiple func_new definitions resolved
//	during instantiation).
//
// Spec: Component-model multiple import resolution.
func TestNested_ComponentWithMultipleImports(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	err := linker.DefineFunc("wasi:io@1.0.0", "read", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = linker.DefineFunc("wasi:io@1.0.0", "write", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = linker.DefineFunc("wasi:clocks@1.0.0", "now", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

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

// TestNested_ComponentWithMissingOneImport tests that missing even one
// import fails.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (import resolution failure path).
//
// Spec: Component-model import resolution failure.
func TestNested_ComponentWithMissingOneImport(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	err := linker.DefineFunc("wasi:io@1.0.0", "read", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = linker.DefineFunc("wasi:io@1.0.0", "write", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

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
// Instance Export Tests
// =============================================================================

// TestInstance_ExportOldGetNew tests that exporting v1.0.0 and getting
// with v1.0.1 works (bidirectional compatibility for exports).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func with semver-compatible export
//	lookup).
//
// Spec: Component-model export bidirectional semver compatibility.
func TestInstance_ExportOldGetNew(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{
				Name: "my:api@1.0.0/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("my:api@1.0.1/hello")
	require.NotNil(t, fn, "should find v1.0.0 export when requesting v1.0.1")
}

// TestInstance_ExportNewGetOld tests that exporting v1.0.1 and getting
// with v1.0.0 works (bidirectional compatibility for exports).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func export lookup with
//	bidirectional semver).
//
// Spec: Component-model export bidirectional semver compatibility.
func TestInstance_ExportNewGetOld(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{
				Name: "my:api@1.0.1/hello",
				Kind: component.ExportKindFunc,
				Idx:  0,
			},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("my:api@1.0.0/hello")
	require.NotNil(t, fn, "should find v1.0.1 export when requesting v1.0.0")
}

// TestInstance_ExportSelectsMax tests that when multiple compatible
// exports exist, the highest version is selected.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func selects max compatible
//	version among multiple exports).
//
// Spec: Component-model export highest-version selection.
func TestInstance_ExportSelectsMax(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{Name: "my:api@1.0.0/hello", Kind: component.ExportKindFunc, Idx: 0},
			{Name: "my:api@1.0.1/hello", Kind: component.ExportKindFunc, Idx: 0},
			{Name: "my:api@1.2.0/hello", Kind: component.ExportKindFunc, Idx: 0},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("my:api@1.0.0/hello")
	require.NotNil(t, fn)
	require.Equal(t, "my:api@1.2.0/hello", fn.Name())
}

// TestInstance_ExportMajorVersionMismatch tests that major version
// mismatches are not compatible even for exports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func rejects major version
//	mismatch).
//
// Spec: Component-model export major version incompatibility.
func TestInstance_ExportMajorVersionMismatch(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{Name: "my:api@2.0.0/hello", Kind: component.ExportKindFunc, Idx: 0},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("my:api@1.0.0/hello")
	require.Nil(t, fn, "should not match different major versions")
}

// TestInstance_ExportExactMatch tests exact name matching for
// unversioned exports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func exact name match path).
//
// Spec: Component-model export exact match.
func TestInstance_ExportExactMatch(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{Name: "simple-func", Kind: component.ExportKindFunc, Idx: 0},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("simple-func")
	require.NotNil(t, fn)
	require.Equal(t, "simple-func", fn.Name())

	fn = inst.GetExportedFunc("other-func")
	require.Nil(t, fn)
}

// TestInstance_ExportPre1_0_Rules tests pre-1.0 semver rules for exports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func pre-1.0 minor version
//	incompatibility for exports).
//
// Spec: Component-model pre-1.0 export semver rules.
func TestInstance_ExportPre1_0_Rules(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{Name: "my:api@0.1.0/hello", Kind: component.ExportKindFunc, Idx: 0},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)

	fn := inst.GetExportedFunc("my:api@0.2.0/hello")
	require.Nil(t, fn, "0.2.0 should not match 0.1.0 for pre-1.0 versions")

	fn = inst.GetExportedFunc("my:api@0.1.1/hello")
	require.NotNil(t, fn, "0.1.1 should match 0.1.0 (bidirectional for exports)")
}

// TestInstance_MultipleExportsWithDifferentNames tests retrieving
// different exports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func lookup for multiple distinct
//	export names).
//
// Spec: Component-model multiple export lookup.
func TestInstance_MultipleExportsWithDifferentNames(t *testing.T) {
	comp := &component.Component{
		Exports: []component.Export{
			{Name: "math:ops@1.0.0/add", Kind: component.ExportKindFunc, Idx: 0},
			{Name: "math:ops@1.0.0/sub", Kind: component.ExportKindFunc, Idx: 0},
			{Name: "io:file@1.0.0/read", Kind: component.ExportKindFunc, Idx: 0},
		},
	}

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	ctx := context.Background()
	inst, err := linker.Instantiate(ctx, comp)
	require.NoError(t, err)

	addFn := inst.GetExportedFunc("math:ops@1.0.0/add")
	require.NotNil(t, addFn)

	subFn := inst.GetExportedFunc("math:ops@1.0.0/sub")
	require.NotNil(t, subFn)

	readFn := inst.GetExportedFunc("io:file@1.0.0/read")
	require.NotNil(t, readFn)

	writeFn := inst.GetExportedFunc("io:file@1.0.0/write")
	require.Nil(t, writeFn)
}
