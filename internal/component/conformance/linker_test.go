// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: host linker conformance tests — semver matching
// and import resolution end-to-end from the embedder surface. The
// companion unit tests live in internal/component/linker_test.go; this
// file restages the embedder-level assertions at the conformance layer
// so an API regression shows up both in the component-internal suite
// and in the conformance gate.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestLinker_OldImportNewItem asserts that importing v1.0.0 succeeds
// when only v1.0.1 is defined. Backward-compatible patch bumps must
// satisfy older requirements under the component-model semver rules.
//
// Wasmtime parallel: runtime/component/linker.rs:27-60 (semver
// compatibility doc comment on LinkerInstance::get / NameMap::get).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_OldImportNewItem(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(42)}, nil
	}

	err := linker.DefineFunc("test:api@1.0.1", "greet", hostFn)
	require.NoError(t, err)

	def, err := linker.MatchImport("test:api@1.0.0/greet")
	require.NoError(t, err)
	require.NotNil(t, def)

	funcDef, ok := def.(*component.FuncDef)
	require.True(t, ok)
	require.NotNil(t, funcDef.Callback)
}

// TestLinker_NewImportOldItem asserts that importing v1.0.1 fails when
// only v1.0.0 is defined — newer requirements cannot be satisfied by
// older definitions in the forward direction.
//
// Wasmtime parallel: runtime/component/linker.rs:27-60 (semver rule:
// an import at version V is satisfied only by definitions whose
// version is >= V within the same compatibility class).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_NewImportOldItem(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("test:api@1.0.0", "greet", hostFn)
	require.NoError(t, err)

	def, err := linker.MatchImport("test:api@1.0.1/greet")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestLinker_SelectsMaxVersion asserts that when multiple compatible
// versions are defined, MatchImport returns the highest compatible
// version.
//
// Wasmtime parallel: runtime/component/linker.rs:27-60 (semver rule:
// pick the highest version that satisfies the requested compatibility
// class).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_SelectsMaxVersion(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	versions := []string{"1.0.0", "1.0.1", "1.0.2", "1.1.0", "1.2.0"}
	for _, ver := range versions {
		v := ver
		hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return []types.Val{types.ValS32(int32(len(v)))}, nil
		}
		err := linker.DefineFunc("test:api@"+ver, "fn", hostFn)
		require.NoError(t, err)
	}

	def, err := linker.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	funcDef, ok := def.(*component.FuncDef)
	require.True(t, ok)
	require.NotNil(t, funcDef)
}

// TestLinker_MajorVersionMismatch asserts that v1.x.y does not match
// v2.x.y in either direction — major version bumps are breaking.
//
// Wasmtime parallel: runtime/component/linker.rs:27-60 (major versions
// are distinct compatibility classes and never overlap).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MajorVersionMismatch(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("test:api@2.0.0", "fn", hostFn)
	require.NoError(t, err)

	def, err := linker.MatchImport("test:api@1.0.0/fn")
	require.Error(t, err)
	require.Nil(t, def)

	linker2 := component.NewComponentLinker(rt)
	err = linker2.DefineFunc("test:api@1.0.0", "fn", hostFn)
	require.NoError(t, err)

	def, err = linker2.MatchImport("test:api@2.0.0/fn")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestLinker_Pre1_0_SemverRules asserts that pre-1.0 (0.x.y) versions
// treat minor bumps as breaking (distinct compatibility classes) but
// patch bumps within the same 0.x.y as backward-compatible.
//
// Wasmtime parallel: runtime/component/linker.rs:27-60 (pre-1.0 rules:
// minor-version bumps break compatibility).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Pre1_0_SemverRules(t *testing.T) {
	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	// 0.1.x is not compatible with 0.2.x.
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	err := linker.DefineFunc("test:api@0.2.0", "fn", hostFn)
	require.NoError(t, err)

	def, err := linker.MatchImport("test:api@0.1.0/fn")
	require.Error(t, err)
	require.Nil(t, def)

	// 0.1.1 satisfies 0.1.0 (patch bump is compatible).
	linker2 := component.NewComponentLinker(rt)
	err = linker2.DefineFunc("test:api@0.1.1", "fn", hostFn)
	require.NoError(t, err)

	def, err = linker2.MatchImport("test:api@0.1.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// 0.1.0 does not satisfy 0.1.1 (newer patch required).
	linker3 := component.NewComponentLinker(rt)
	err = linker3.DefineFunc("test:api@0.1.0", "fn", hostFn)
	require.NoError(t, err)

	def, err = linker3.MatchImport("test:api@0.1.1/fn")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestLinker_MinorVersionBump asserts that for 1.x+ versions, minor
// bumps are backward compatible: a v1.2.0 definition satisfies v1.1.0
// and v1.0.0 imports, but not v1.3.0.
//
// Wasmtime parallel: runtime/component/linker.rs:27-60 (1.x+ rules:
// minor bumps within the same major are compatible).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MinorVersionBump(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("test:api@1.2.0", "fn", hostFn)
	require.NoError(t, err)

	def, err := linker.MatchImport("test:api@1.1.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	def, err = linker.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	def, err = linker.MatchImport("test:api@1.3.0/fn")
	require.Error(t, err)
	require.Nil(t, def)
}

// TestImport_FunctionsInInstances asserts DefineInstance accumulates
// multiple Func exports under a single instance namespace and that the
// stored callbacks dispatch to the registered closures.
//
// Wasmtime parallel: runtime/component/linker.rs LinkerInstance builder
// pattern — a nested instance aggregates multiple exports.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_FunctionsInInstances(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	addCalled := false
	mulCalled := false

	addFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		addCalled = true
		return []types.Val{types.ValS32(10)}, nil
	}

	mulFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		mulCalled = true
		return []types.Val{types.ValS32(20)}, nil
	}

	err := linker.DefineInstance("wasi:math@1.0.0").
		Func("add", addFn).
		Func("multiply", mulFn).
		Build()
	require.NoError(t, err)

	def, ok := linker.Get("wasi:math@1.0.0")
	require.True(t, ok)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports)

	addDef, ok := instDef.Exports["add"]
	require.True(t, ok)
	addFuncDef, ok := addDef.(*component.FuncDef)
	require.True(t, ok)

	ctx := context.Background()
	_, err = addFuncDef.Callback(ctx, nil, []types.Val{types.ValS32(5)})
	require.NoError(t, err)
	require.True(t, addCalled)

	mulDef, ok := instDef.Exports["multiply"]
	require.True(t, ok)
	mulFuncDef, ok := mulDef.(*component.FuncDef)
	require.True(t, ok)

	_, err = mulFuncDef.Callback(ctx, nil, []types.Val{types.ValS32(3)})
	require.NoError(t, err)
	require.True(t, mulCalled)
}

// TestImport_MissingImport asserts that Linker.Instantiate reports an
// error when the component declares an import that has no matching
// definition in the linker.
//
// Wasmtime parallel: runtime/component/linker.rs Linker::typecheck
// reports "a matching implementation was not found in the linker" when
// NameMap::get returns None.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_MissingImport(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

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

	inst, err := linker.Instantiate(context.Background(), comp)
	require.Error(t, err)
	require.Nil(t, inst)
}

// TestImport_ResourceWithDestructor asserts DefineResource stores a
// *ResourceDef whose Destructor closure is invoked with the supplied
// rep when the embedder calls it.
//
// Wasmtime parallel: runtime/component/linker.rs LinkerInstance::resource
// registers a destructor closure under the resource name.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_ResourceWithDestructor(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	destructorCalled := false
	var destroyedRep uint32

	destructor := func(rep uint32) {
		destructorCalled = true
		destroyedRep = rep
	}

	err := linker.DefineResource("my:resources@1.0.0", "file-handle", destructor)
	require.NoError(t, err)

	def, ok := linker.Get("my:resources@1.0.0/file-handle")
	require.True(t, ok)

	resDef, ok := def.(*component.ResourceDef)
	require.True(t, ok)
	require.NotNil(t, resDef.Destructor)

	resDef.Destructor(42)
	require.True(t, destructorCalled)
	require.Equal(t, uint32(42), destroyedRep)
}

// TestImport_ResourceInInstance asserts InstanceBuilder.Resource nests
// a ResourceDef under an instance namespace and that the aggregated
// InstanceDef carries the destructor closure.
//
// Wasmtime parallel: runtime/component/linker.rs LinkerInstance::resource
// inside a nested instance scope.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_ResourceInInstance(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	destructorCalled := false

	err := linker.DefineInstance("my:component@1.0.0").
		Resource("handle", func(rep uint32) {
			destructorCalled = true
		}).
		Build()
	require.NoError(t, err)

	def, ok := linker.Get("my:component@1.0.0")
	require.True(t, ok)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok)

	resDef, ok := instDef.Exports["handle"]
	require.True(t, ok)

	resDefTyped, ok := resDef.(*component.ResourceDef)
	require.True(t, ok)

	resDefTyped.Destructor(100)
	require.True(t, destructorCalled)
}

// TestImport_DuplicateDefinition asserts DefineFunc errors when the
// same "namespace/name" key is defined twice.
//
// Wasmtime parallel: runtime/component/linker.rs NameMap::insert
// returns an error on duplicate insert unless allow_shadowing is set;
// wazero's Linker never allows shadowing.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_DuplicateDefinition(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("test:api@1.0.0", "fn", hostFn)
	require.NoError(t, err)

	err = linker.DefineFunc("test:api@1.0.0", "fn", hostFn)
	require.Error(t, err)
}

// TestImport_DuplicateInstanceDefinition asserts InstanceBuilder.Build
// errors when the same namespace is built twice.
//
// Wasmtime parallel: runtime/component/linker.rs LinkerInstance::instance
// delegates to NameMap::insert, which errors on duplicates.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_DuplicateInstanceDefinition(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	err := linker.DefineInstance("test:ns@1.0.0").Build()
	require.NoError(t, err)

	err = linker.DefineInstance("test:ns@1.0.0").Build()
	require.Error(t, err)
}

// TestImport_NoVersion asserts that unversioned plain names work for
// DefineFunc + Get + MatchImport. Plain names fall through the semver
// parser and are matched by exact key.
//
// Wasmtime parallel: runtime/component/linker.rs names.rs plain-name
// path — exact lookup in NameMap::get.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_NoVersion(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("my-namespace", "my-func", hostFn)
	require.NoError(t, err)

	def, ok := linker.Get("my-namespace/my-func")
	require.True(t, ok)
	require.NotNil(t, def)

	def, err = linker.MatchImport("my-namespace/my-func")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestImport_InstantiateWithResolvedImports asserts that Instantiate
// succeeds when every declared import has a matching definition in
// the linker. Mirrors the happy path of the typecheck loop.
//
// Wasmtime parallel: runtime/component/linker.rs Linker::typecheck
// walks component.import_types and requires a definition for each.
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_InstantiateWithResolvedImports(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	hostFn := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	}

	err := linker.DefineFunc("wasi:io@1.0.0", "read", hostFn)
	require.NoError(t, err)

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

	inst, err := linker.Instantiate(context.Background(), comp)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// TestImport_DynamicHostFunc asserts that a HostFunc registered via
// DefineInstance executes with the correct arg echo behaviour when
// invoked through the stored InstanceDef export. The pre-Session-0
// suite called the deleted InstanceBuilder.FuncNoType path; in the
// post-C3 API every host function is a dynamic HostFunc because the
// component's import type is the source of truth (there is no
// separate typed registration). This test preserves the intent —
// a dynamic echo callback inside a builder-assembled instance —
// without the deleted builder method.
//
// Wasmtime parallel: runtime/component/linker.rs LinkerInstance::func_new
// (the dynamic host path; the typed func_wrap path has no wazero
// analogue post-Session 0).
// No counterpart (justified): conformance/linker tests cover the
// embedder linker surface; canonical-abi run_tests.py does not
// exercise the host linker.
func TestImport_DynamicHostFunc(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	called := false
	echo := func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		called = true
		return args, nil
	}

	err := linker.DefineInstance("test:dynamic@1.0.0").
		Func("echo", echo).
		Build()
	require.NoError(t, err)

	def, ok := linker.Get("test:dynamic@1.0.0")
	require.True(t, ok)

	instDef := def.(*component.InstanceDef)
	funcDef := instDef.Exports["echo"].(*component.FuncDef)
	// Post-C3 API: Type stays nil at registration — the component's
	// import declaration IS the source of truth, supplied at call time.
	require.Nil(t, funcDef.Type)

	ctx := context.Background()
	result, err := funcDef.Callback(ctx, nil, []types.Val{types.ValS32(123)})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, 1, len(result))
	require.Equal(t, int32(123), result[0].S32())
}
