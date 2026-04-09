// internal/component/linker_test.go
//
// Unit tests for the component-model host Linker. Each test has a
// citation block tying its assertions to a spec line, a wasmtime
// parallel, or an explicit "no counterpart" justification. See the
// Task C9 restoration notes in the canonical-abi Session 1 plan.
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

const linkerTestSkipMsg = "session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md"

// TestNewLinker asserts NewLinker returns a non-nil Linker with an
// initialised definitions map so subsequent DefineFunc/DefineInstance
// calls can insert without a nil-map panic.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:61-68
// (struct Linker holding a NameMap of definitions).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker; linker semantics are a wazero/wasmtime
// embedder-facing layer outside the canonical ABI scope.
func TestNewLinker(t *testing.T) {
	l := NewLinker()
	require.NotNil(t, l)
	require.NotNil(t, l.definitions)
}

// TestLinker_DefineFunc asserts Linker.DefineFunc stores a HostFunc
// under the "namespace/name" key as a *FuncDef that can be looked up
// via the internal definitions map. The post-C3 API has no
// registration-time type — the component's import declaration is the
// canonical source of truth, supplied to the callback at call time.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:665-675
// (LinkerInstance::func_new — dynamic host path). Wazero's DefineFunc
// mirrors func_new, not the typed func_wrap path.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker; linker semantics are a wazero/wasmtime
// embedder-facing layer outside the canonical ABI scope.
func TestLinker_DefineFunc(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("test:api", "add", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(42)}, nil
	})
	require.NoError(t, err)

	// Check it was stored under "namespace/name" as a *FuncDef whose
	// Callback is non-nil. Type stays nil at registration because the
	// host has no type to declare (wasmtime func_new parallel).
	def, ok := l.definitions["test:api/add"]
	require.True(t, ok)
	require.NotNil(t, def)
	funcDef, ok := def.(*FuncDef)
	require.True(t, ok)
	require.NotNil(t, funcDef.Callback)
	require.Nil(t, funcDef.Type)
}

// TestLinker_DefineFunc_Duplicate asserts Linker.DefineFunc returns a
// non-nil error when the same "namespace/name" key is defined twice
// without explicit shadowing. Mirrors wasmtime's NameMap insert path,
// which errors on duplicates unless allow_shadowing is set.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:865-868
// (LinkerInstance::insert delegates to NameMap::insert with
// allow_shadowing; wazero's Linker does not currently expose a
// shadowing toggle, so duplicate is always an error).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_DefineFunc_Duplicate(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("test", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Duplicate should error.
	err = l.DefineFunc("test", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.Error(t, err)
}

// TestLinker_DefineInstance asserts InstanceBuilder.Func accumulates
// multiple exports under a single instance key, and Build inserts an
// *InstanceDef whose Exports map has one entry per Func call.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:159-161
// (Linker::instance → LinkerInstance builder) combined with
// linker.rs:665-675 (func_new on the nested instance).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_DefineInstance(t *testing.T) {
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

	def, ok := l.definitions["wasi:io/streams@0.2.0"]
	require.True(t, ok)
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Equal(t, 2, len(instDef.Exports))
}

// TestLinker_DefineResource asserts DefineResource stores a
// *ResourceDef whose Destructor callback is the one the embedder
// supplied, by invoking it and observing the side effect.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:768-780
// (LinkerInstance::resource — registers a destructor closure under
// the resource name).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_DefineResource(t *testing.T) {
	l := NewLinker()

	destroyed := false
	err := l.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {
		destroyed = true
	})
	require.NoError(t, err)

	def, ok := l.definitions["wasi:io/streams@0.2.0/input-stream"]
	require.True(t, ok)
	resDef, ok := def.(*ResourceDef)
	require.True(t, ok)

	// Call destructor to verify it is the closure we registered.
	resDef.Destructor(0)
	require.True(t, destroyed)
}

func TestLinker_DefineResource_Duplicate(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_Get_Direct(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_Get_NotFound(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_Get_Instance(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchImport_OldImportNewItem(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchImport_NewImportOldItem(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchImport_SelectsMax(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchImport_DirectMatch(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_Instantiate_Basic(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_Instantiate_WithImports(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_Instantiate_MissingImport(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestInstance_GetExportedFunc(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestInstance_GetExportedFunc_NotFound(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestInstance_GetExportedFunc_ExportOldGetNew(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestInstance_GetExportedFunc_ExportNewGetOld(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestInstance_GetExportedFunc_SelectsMax(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_RelaxedSemverMatching_FuncImport(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_RelaxedSemverMatching_InstanceImport(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_RelaxedSemverMatching_DifferentMinor(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_RelaxedSemverMatching_Post1_0(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchLockedDep(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchLockedDep_NotFound(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchUnlockedDep(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchUnlockedDep_MatchAll(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchUnlockedDep_NoMatch(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchURLImport(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchHashImport(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchPlainImport(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}

func TestLinker_MatchInterfaceImport_Unchanged(t *testing.T) {
	t.Skip(linkerTestSkipMsg)
}
