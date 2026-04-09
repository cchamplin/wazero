// internal/component/integration_test.go
//
// Integration tests for the Linker-based instantiation pipeline. These
// exercise DefineFunc/DefineInstance/DefineResource, semver matching,
// and Linker.Instantiate with Component literals.
//
// Restored from Session 0 compile-fix stubs (Task 17). Adapted to the
// current HostFunc signature and Linker API (no FuncType parameter).
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (LinkerInstance::func_new — dynamic host
//	function registration with no caller-declared type).
//
// Spec: Component-model import resolution and instantiation.
func TestIntegration_ComponentWithFuncImport(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("test:host@1.0.0", "double", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		if len(args) > 0 {
			return []types.Val{types.ValS32(args[0].S32() * 2)}, nil
		}
		return []types.Val{types.ValS32(0)}, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "test:host@1.0.0/double",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		Exports: []Export{
			{Name: "compute", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("compute")
	require.NotNil(t, fn)
	require.Equal(t, c, inst.Component())
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (func_new + semver-compatible import matching).
//
// Spec: Component-model semver-compatible import matching.
func TestIntegration_FuncImportSemverMatch(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@1.0.1", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (func_new + semver mismatch path).
//
// Spec: Component-model semver import mismatch error.
func TestIntegration_FuncImportSemverMismatch(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "api@1.0.1/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	_, err = l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (func_new + major version mismatch).
//
// Spec: Component-model major version incompatibility.
func TestIntegration_FuncImportMajorVersionMismatch(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@2.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	_, err = l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (LinkerInstance with module + func_new for instance
//	building).
//
// Spec: Component-model instance import definition.
func TestIntegration_InstanceImport(t *testing.T) {
	l := NewLinker()

	err := l.DefineInstance("wasi:io/streams@0.2.0").
		Func("read", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Func("write", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	def, ok := l.Get("wasi:io/streams@0.2.0")
	require.True(t, ok)
	require.NotNil(t, def)

	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["read"])
	require.NotNil(t, instDef.Exports["write"])
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (LinkerInstance with versioned namespace).
//
// Spec: Component-model instance import with versioning.
func TestIntegration_InstanceImportWithVersioning(t *testing.T) {
	l := NewLinker()

	err := l.DefineInstance("wasi:io/streams@0.2.1").
		Func("read", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	def, ok := l.Get("wasi:io/streams@0.2.1")
	require.True(t, ok)
	require.NotNil(t, def)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 + instance.rs:1148 (full linking scenario
//	with multiple imports and exports).
//
// Spec: Component-model full linking scenario.
func TestIntegration_FullLinkingScenario(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("math@1.0.0", "add", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		if len(args) >= 2 {
			return []types.Val{types.ValS32(args[0].S32() + args[1].S32())}, nil
		}
		return []types.Val{types.ValS32(0)}, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("math@1.0.0", "mul", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		if len(args) >= 2 {
			return []types.Val{types.ValS32(args[0].S32() * args[1].S32())}, nil
		}
		return []types.Val{types.ValS32(0)}, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "math@1.0.0/add",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
			{
				Name: "math@1.0.0/mul",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 1,
				},
			},
		},
		Exports: []Export{
			{Name: "compute", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.Equal(t, c, inst.Component())

	fn := inst.GetExportedFunc("compute")
	require.NotNil(t, fn)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (multiple versioned definitions, highest
//	semver-compatible match selected).
//
// Spec: Component-model semver best-match selection.
func TestIntegration_MultipleVersionedImports(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(100)}, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("api@1.0.2", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(102)}, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("api@1.0.1", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(101)}, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (resource definition in LinkerInstance).
//
// Spec: Component-model resource definition with destructor.
func TestIntegration_ResourceDefinition(t *testing.T) {
	l := NewLinker()

	destructorCalled := false
	var destroyedRep uint32

	err := l.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {
		destructorCalled = true
		destroyedRep = rep
	})
	require.NoError(t, err)

	def, ok := l.Get("wasi:io/streams@0.2.0/input-stream")
	require.True(t, ok)
	require.NotNil(t, def)

	resDef, ok := def.(*ResourceDef)
	require.True(t, ok)
	require.NotNil(t, resDef.Destructor)

	resDef.Destructor(42)
	require.True(t, destructorCalled)
	require.Equal(t, uint32(42), destroyedRep)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (mixed func + resource definitions).
//
// Spec: Component-model mixed import kinds.
func TestIntegration_MixedImports(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@1.0.0", "process", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = l.DefineResource("api@1.0.0", "handle", func(rep uint32) {})
	require.NoError(t, err)

	fnDef, ok := l.Get("api@1.0.0/process")
	require.True(t, ok)
	require.NotNil(t, fnDef)

	resDef, ok := l.Get("api@1.0.0/handle")
	require.True(t, ok)
	require.NotNil(t, resDef)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func with semver-compatible export
//	matching — bidirectional version compatibility for exports).
//
// Spec: Component-model export semver matching.
func TestIntegration_ExportSemverMatching(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	fn := inst.GetExportedFunc("api@1.0.0/fn")
	require.NotNil(t, fn)

	fn = inst.GetExportedFunc("api@1.0.1/fn")
	require.NotNil(t, fn)

	c2 := &Component{
		Exports: []Export{
			{Name: "api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst2, err := l.Instantiate(context.Background(), c2)
	require.NoError(t, err)

	fn = inst2.GetExportedFunc("api@1.0.0/fn")
	require.NotNil(t, fn)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:156-195 (get_func selects highest compatible
//	version among multiple exports).
//
// Spec: Component-model export highest-version selection.
func TestIntegration_ExportSelectsMaxVersion(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "api@1.0.2/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	fn := inst.GetExportedFunc("api@1.0.0/fn")
	require.NotNil(t, fn)
	require.Equal(t, "api@1.0.2/fn", fn.Name())
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:1148 (Instantiate with empty exports list).
//
// Spec: Component-model instantiation with no exports.
func TestIntegration_NoExportsComponent(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Imports: []Import{},
		Exports: []Export{},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("nonexistent")
	require.Nil(t, fn)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/instance.rs:1148 (Instantiate with type definitions).
//
// Spec: Component-model instantiation with type definitions.
func TestIntegration_ComponentWithTypes(t *testing.T) {
	l := NewLinker()

	c := &Component{
		TypeDefs: []TypeDef{
			{Kind: TypeDefKindFunc, Func: 0},
			{Kind: TypeDefKindResource, Resource: 0},
		},
		Exports: []Export{
			{Name: "process", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)

	fn := inst.GetExportedFunc("process")
	require.NotNil(t, fn)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (pre-1.0 semver matching — same minor,
//	higher patch is compatible).
//
// Spec: Component-model pre-1.0 semver matching.
func TestIntegration_Pre1_0_SemverHandling(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@0.2.1", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "api@0.2.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (pre-1.0 semver: different minor versions
//	are incompatible).
//
// Spec: Component-model pre-1.0 minor version incompatibility.
func TestIntegration_Pre1_0_MinorVersionMismatch(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("api@0.3.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "api@0.2.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	_, err = l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (WASI-style namespaced import resolution).
//
// Spec: Component-model WASI-style namespaced imports.
func TestIntegration_WASILikeNamespace(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("wasi:cli/environment@0.2.0", "get-environment", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	err = l.DefineFunc("wasi:cli/stdin@0.2.0", "get-stdin", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	c := &Component{
		Imports: []Import{
			{
				Name: "wasi:cli/environment@0.2.0/get-environment",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
			{
				Name: "wasi:cli/stdin@0.2.0/get-stdin",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs:665-675 (host function callback invocation via
//	MatchImport -> FuncDef.Callback).
//
// Spec: Component-model host function callback.
func TestIntegration_HostFunctionCallback(t *testing.T) {
	l := NewLinker()

	callCount := 0

	err := l.DefineFunc("test@1.0.0", "increment", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		callCount++
		return nil, nil
	})
	require.NoError(t, err)

	def, err := l.MatchImport("test@1.0.0/increment")
	require.NoError(t, err)

	funcDef := def.(*FuncDef)
	require.NotNil(t, funcDef.Callback)

	_, err = funcDef.Callback(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)

	_, err = funcDef.Callback(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
}

// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
//
//	component/linker.rs (LinkerInstance chained func_new calls for instance
//	building).
//
// Spec: Component-model instance builder chaining.
func TestIntegration_InstanceBuilderChaining(t *testing.T) {
	l := NewLinker()

	err := l.DefineInstance("api@1.0.0").
		Func("a", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) { return nil, nil }).
		Func("b", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) { return nil, nil }).
		Func("c", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) { return nil, nil }).
		Build()
	require.NoError(t, err)

	def, ok := l.Get("api@1.0.0")
	require.True(t, ok)

	instDef := def.(*InstanceDef)
	require.Equal(t, 3, len(instDef.Exports))
	require.NotNil(t, instDef.Exports["a"])
	require.NotNil(t, instDef.Exports["b"])
	require.NotNil(t, instDef.Exports["c"])
}
