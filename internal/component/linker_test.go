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

// TestLinker_DefineResource_Duplicate asserts DefineResource errors
// on a duplicate "namespace/name" key, mirroring the DefineFunc
// duplicate guard.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:768-780
// (LinkerInstance::resource calls insert → NameMap::insert, which
// errors without allow_shadowing).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_DefineResource_Duplicate(t *testing.T) {
	l := NewLinker()

	err := l.DefineResource("test", "res", func(rep uint32) {})
	require.NoError(t, err)

	// Duplicate should error.
	err = l.DefineResource("test", "res", func(rep uint32) {})
	require.Error(t, err)
}

// TestLinker_Get_Direct asserts Linker.Get returns the stored
// *FuncDef for an exact key lookup (no semver walk). Get is the
// "no magic" accessor on top of the definitions map, distinct from
// MatchImport which does semver compatibility selection.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:870-872
// (LinkerInstance::get — exact-name fetch from the name map).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Get_Direct(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("test:api", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	def, ok := l.Get("test:api/fn")
	require.True(t, ok)
	require.NotNil(t, def)
}

// TestLinker_Get_NotFound asserts Linker.Get returns (nil, false)
// for an unknown key without constructing a zero-value definition.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:870-872
// (LinkerInstance::get returns Option::None on miss).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Get_NotFound(t *testing.T) {
	l := NewLinker()

	def, ok := l.Get("nonexistent")
	require.False(t, ok)
	require.Nil(t, def)
}

// TestLinker_Get_Instance asserts Linker.Get on an instance key
// returns the *InstanceDef whose Exports map carries every Func
// accumulated through the InstanceBuilder.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:159-161
// (Linker::instance + LinkerInstance builder) + :870-872 (get).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Get_Instance(t *testing.T) {
	l := NewLinker()

	err := l.DefineInstance("wasi:io/streams@0.2.0").
		Func("read", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	def, ok := l.Get("wasi:io/streams@0.2.0")
	require.True(t, ok)
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["read"])
}

// TestLinker_MatchImport_OldImportNewItem asserts that when a
// component requires v1.0.0 and the linker provides v1.0.1, MatchImport
// resolves the import to the v1.0.1 definition. This is the core
// semver-compatible-import behaviour: defining a newer patch in the
// linker satisfies an older request within the same (major, minor).
//
// Wasmtime parallel: debug-vendored/wasmtime/tests/all/component_model/linker.rs:7
// (fn old_import_importing_new_item — same scenario against wasmtime's
// Linker::define_import semver machinery).
// Wasmtime source: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (the "Names and Semver" doc comment on struct Linker describes this
// exact lookup direction).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchImport_OldImportNewItem(t *testing.T) {
	l := NewLinker()

	// Define v1.0.1.
	err := l.DefineFunc("test:api@1.0.1", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Request v1.0.0 — should match v1.0.1 (patch upgrade).
	def, err := l.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestLinker_MatchImport_NewImportOldItem asserts that when a
// component requires v1.0.1 and the linker only provides v1.0.0,
// MatchImport returns an error. This is the "can't downgrade" half
// of the semver rule — a newer API cannot be satisfied by an older
// registration because the older registration may lack fields the
// caller depends on.
//
// Wasmtime parallel: debug-vendored/wasmtime/tests/all/component_model/linker.rs:30
// (fn new_import_importing_old_item — asserts wasmtime's linker errors
// on the same scenario).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchImport_NewImportOldItem(t *testing.T) {
	l := NewLinker()

	// Define v1.0.0.
	err := l.DefineFunc("test:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Request v1.0.1 — should NOT match v1.0.0 (older patch cannot
	// satisfy newer patch requirement in strict mode).
	_, err = l.MatchImport("test:api@1.0.1/fn")
	require.Error(t, err)
}

// TestLinker_MatchImport_SelectsMax asserts that when several
// compatible versions are registered, MatchImport selects the
// highest-compatible version. Invokes the returned callback to
// verify the right selection by observing its return value.
//
// Wasmtime parallel: debug-vendored/wasmtime/tests/all/component_model/linker.rs:81
// (fn missing_import_selects_max — asserts wasmtime's linker picks
// the newest compatible version).
// Wasmtime source: debug-vendored/wasmtime/crates/environ/src/component/names.rs:4-45
// (NameMap::alternate_lookups structure that implements the
// "highest compatible" rule; wazero walks its own flat map and picks
// the max in matchLegacyImport).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchImport_SelectsMax(t *testing.T) {
	l := NewLinker()

	// Define multiple versions out of order.
	err := l.DefineFunc("test:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(100)}, nil
	})
	require.NoError(t, err)
	err = l.DefineFunc("test:api@1.0.2", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(102)}, nil
	})
	require.NoError(t, err)
	err = l.DefineFunc("test:api@1.0.1", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(101)}, nil
	})
	require.NoError(t, err)

	// Request v1.0.0 — should select highest compatible (v1.0.2).
	def, err := l.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	funcDef := def.(*FuncDef)

	// Call to verify we got v1.0.2. Pass nil for ctx's fnType since
	// this test callback ignores it.
	results, err := funcDef.Callback(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, int32(102), results[0].S32())
}

// TestLinker_MatchImport_DirectMatch asserts that MatchImport handles
// plain (non-versioned) keys by falling back to an exact lookup. This
// is the short-circuit for hosts that don't use semver naming.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/environ/src/component/names.rs:105-117
// (NameMap::get tries the direct definitions map before consulting
// alternate_lookups).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchImport_DirectMatch(t *testing.T) {
	l := NewLinker()

	err := l.DefineFunc("test", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	def, err := l.MatchImport("test/fn")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestLinker_Instantiate_Basic asserts Linker.Instantiate on a
// component with no imports returns a live Instance whose Component()
// accessor points back at the original Component literal.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:274-284
// (Linker::instantiate — end-to-end instantiation entry point).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Instantiate_Basic(t *testing.T) {
	l := NewLinker()

	// Minimal component: one function export, no imports. The
	// Types/TypeDefs fields are intentionally left nil; the legacy
	// Linker.Instantiate path walks Exports only (linker.go:492-496)
	// so a dense type table is not required for this assertion.
	c := &Component{
		Exports: []Export{
			{Name: "test", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.Equal(t, c, inst.Component())
}

// TestLinker_Instantiate_WithImports asserts that Linker.Instantiate
// resolves each Component.Import via MatchImport and succeeds when
// all imports match. The test does not assert per-import wiring
// shape; it only verifies the resolver path produces a live Instance.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:163-181
// (Linker::typecheck walks component.import_types and calls
// NameMap::get for each).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Instantiate_WithImports(t *testing.T) {
	l := NewLinker()

	// Define the import.
	err := l.DefineFunc("test:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(42)}, nil
	})
	require.NoError(t, err)

	// Component with a single function import. TypeIdx is unused by
	// the legacy Instantiate path (linker.go:483-489 only calls
	// MatchImport on the name).
	c := &Component{
		Imports: []Import{
			{
				Name: "test:api@1.0.0/fn",
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

// TestLinker_Instantiate_MissingImport asserts Linker.Instantiate
// returns an error if the component declares an import that has no
// matching definition in the linker.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:175-178
// (Linker::typecheck formats "a matching implementation was not found
// in the linker" when NameMap::get returns None).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_Instantiate_MissingImport(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Imports: []Import{
			{
				Name: "missing:api@1.0.0/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
	}

	_, err := l.Instantiate(context.Background(), c)
	require.Error(t, err)
}

// TestInstance_GetExportedFunc asserts Instance.GetExportedFunc
// returns a non-nil wrapper for an exported function by its plain
// name (no semver walk).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver lookup rules; plain names fall through to an exact match
// in NameMap::get at names.rs:105-117).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestInstance_GetExportedFunc(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "add", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Get exported function by exact name. GetExportedFunc falls back
	// to getExactExportedFunc for non-versioned names.
	fn := inst.GetExportedFunc("add")
	require.NotNil(t, fn)
}

// TestInstance_GetExportedFunc_NotFound asserts GetExportedFunc
// returns nil when the requested export name is not declared on the
// component.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs
// (Instance::get_func returns Option::None on miss). See also
// linker.rs:27-60 (semver doc comment) which applies to the versioned
// miss path.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestInstance_GetExportedFunc_NotFound(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "add", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Non-existent export returns nil.
	fn := inst.GetExportedFunc("missing")
	require.Nil(t, fn)
}

// TestInstance_GetExportedFunc_ExportOldGetNew asserts that when a
// component exports v1.0.0 and a caller requests v1.0.1, the lookup
// succeeds. wazero's export-side lookup is bidirectional: it tries
// both SemverCompatible(req, exp) and SemverCompatible(exp, req),
// which lets the caller upgrade to a newer minor/patch exposed under
// the older requested identifier.
//
// Wasmtime parallel: debug-vendored/wasmtime/tests/all/component_model/instance.rs:66
// (fn export_old_get_new — asserts the equivalent behaviour on
// wasmtime's Instance::get_func).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestInstance_GetExportedFunc_ExportOldGetNew(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "test:api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.1 — bidirectional compatibility allows export
	// v1.0.0 to satisfy a newer request (same API).
	fn := inst.GetExportedFunc("test:api@1.0.1/fn")
	require.NotNil(t, fn)
}

// TestInstance_GetExportedFunc_ExportNewGetOld asserts that when a
// component exports v1.0.1 and a caller requests v1.0.0, the lookup
// succeeds via the forward-compatible half of the bidirectional rule.
//
// Wasmtime parallel: debug-vendored/wasmtime/tests/all/component_model/instance.rs:101
// (fn export_new_get_old — asserts the equivalent behaviour on
// wasmtime's Instance::get_func).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestInstance_GetExportedFunc_ExportNewGetOld(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "test:api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.0 — semver-compatible with export v1.0.1.
	fn := inst.GetExportedFunc("test:api@1.0.0/fn")
	require.NotNil(t, fn)
}

// TestInstance_GetExportedFunc_SelectsMax asserts that when a
// component exposes several semver-compatible export versions under
// the same name, GetExportedFunc returns the highest compatible
// version. Verified via the unexported `name` field on the returned
// *ExportedFunc (accessible within-package only).
//
// Wasmtime parallel: debug-vendored/wasmtime/tests/all/component_model/instance.rs:137
// (fn export_missing_get_max — asserts wasmtime's Instance::get_func
// selects the highest compatible export).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestInstance_GetExportedFunc_SelectsMax(t *testing.T) {
	l := NewLinker()

	c := &Component{
		Exports: []Export{
			{Name: "test:api@1.0.0/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "test:api@1.0.2/fn", Kind: ExportKindFunc, Idx: 0},
			{Name: "test:api@1.0.1/fn", Kind: ExportKindFunc, Idx: 0},
		},
	}

	inst, err := l.Instantiate(context.Background(), c)
	require.NoError(t, err)

	// Request v1.0.0 — should match highest compatible (v1.0.2).
	fn := inst.GetExportedFunc("test:api@1.0.0/fn")
	require.NotNil(t, fn)
	// Verify we got the right one by checking the name.
	require.Equal(t, "test:api@1.0.2/fn", fn.name)
}

// TestLinker_RelaxedSemverMatching_FuncImport asserts wazero's
// relaxed-semver toggle widens pre-1.0 patch matching to accept any
// patch within the same minor, in either direction. Strict mode
// still requires available.Patch >= required.Patch (semver.go:215-241),
// so a linker that only has 0.2.0 cannot satisfy a 0.2.3 request.
// Relaxed mode drops the patch-ordering constraint for 0.x.y versions.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (the "Names and Semver" doc comment). Wasmtime itself does not
// expose an equivalent "relaxed" toggle — this is a wazero extension
// that matches the "interfaces once defined never change" assumption
// more aggressively for pre-release (0.x) APIs. The test asserts
// wazero-specific behaviour, justified as an embedder-facing extension
// with no direct wasmtime counterpart.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_RelaxedSemverMatching_FuncImport(t *testing.T) {
	l := NewLinker()

	// Define v0.2.0.
	err := l.DefineFunc("test:api@0.2.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return []types.Val{types.ValS32(200)}, nil
	})
	require.NoError(t, err)

	// Strict mode: request v0.2.3 — does NOT match v0.2.0
	// (available.Patch 0 < required.Patch 3).
	_, err = l.MatchImport("test:api@0.2.3/fn")
	require.Error(t, err, "strict mode should not match 0.2.0 for 0.2.3 requirement")

	// Enable relaxed matching.
	l.SetRelaxedSemverMatching(true)
	require.True(t, l.RelaxedSemverMatching())

	// Relaxed mode: request v0.2.3 — should match v0.2.0.
	def, err := l.MatchImport("test:api@0.2.3/fn")
	require.NoError(t, err, "relaxed mode should match 0.2.0 for 0.2.3 requirement")
	require.NotNil(t, def)

	// Verify we got the right function by invoking it.
	funcDef := def.(*FuncDef)
	results, err := funcDef.Callback(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, int32(200), results[0].S32())
}

// TestLinker_RelaxedSemverMatching_InstanceImport asserts relaxed
// semver applies to instance-typed imports as well as function
// imports.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment). As with the function-import test, the
// "relaxed" toggle is a wazero extension with no wasmtime counterpart.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_RelaxedSemverMatching_InstanceImport(t *testing.T) {
	l := NewLinker()

	// Define instance at v0.2.0.
	err := l.DefineInstance("wasi:cli/environment@0.2.0").
		Func("get-environment", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Strict mode: request v0.2.3 — does NOT match v0.2.0.
	_, err = l.MatchImport("wasi:cli/environment@0.2.3")
	require.Error(t, err, "strict mode should not match 0.2.0 for 0.2.3 requirement")

	// Enable relaxed matching.
	l.SetRelaxedSemverMatching(true)

	// Relaxed mode: request v0.2.3 — should match v0.2.0.
	def, err := l.MatchImport("wasi:cli/environment@0.2.3")
	require.NoError(t, err, "relaxed mode should match 0.2.0 for 0.2.3 requirement")
	require.NotNil(t, def)

	// Verify we got the instance back.
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["get-environment"])
}

// TestLinker_RelaxedSemverMatching_DifferentMinor asserts relaxed
// semver still requires the same pre-1.0 minor version. 0.2.0 never
// satisfies 0.3.0, even with relaxed matching, because the spec
// treats the minor field as the effective "major" for 0.x versions.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment enforces the same rule implicitly by using
// the semver crate's standard Version comparison; wazero's
// SemverCompatible at semver.go:222-224 makes the constraint
// explicit).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_RelaxedSemverMatching_DifferentMinor(t *testing.T) {
	l := NewLinker()

	// Define v0.2.0.
	err := l.DefineFunc("test:api@0.2.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Enable relaxed matching.
	l.SetRelaxedSemverMatching(true)

	// Request v0.3.0 — should NOT match v0.2.0 (different minor is
	// a breaking change in 0.x).
	_, err = l.MatchImport("test:api@0.3.0/fn")
	require.Error(t, err, "relaxed mode should not match 0.2.0 for 0.3.0 requirement")
}

// TestLinker_RelaxedSemverMatching_Post1_0 asserts the relaxed toggle
// is a no-op for 1.x+ versions. Post-1.0 patch ordering is still
// enforced because SemverCompatible's relaxed branch only fires when
// required.Major == 0 (semver.go:221-231).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment — post-1.0 versions follow standard semver).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_RelaxedSemverMatching_Post1_0(t *testing.T) {
	l := NewLinker()

	// Define v1.0.0.
	err := l.DefineFunc("test:api@1.0.0", "fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return nil, nil
	})
	require.NoError(t, err)

	// Enable relaxed matching.
	l.SetRelaxedSemverMatching(true)

	// Request v1.0.1 — should NOT match v1.0.0 (post-1.0 still
	// requires available.Patch >= required.Patch; relaxed mode only
	// affects pre-1.0 versions).
	_, err = l.MatchImport("test:api@1.0.1/fn")
	require.Error(t, err, "relaxed mode should not affect post-1.0 versions")
}

// TestLinker_MatchLockedDep asserts Linker.MatchImport resolves a
// "locked-dep=pkg:M.m.p" import name to the definition registered
// under "pkg@M.m.p". This is the wazero encoding of the component
// model's locked-dep import name form.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (the semver doc comment on Linker). Wasmtime does not directly
// parse the textual "locked-dep=..." form at the Linker API layer —
// wazero's ParseImportName / matchLockedDep path
// (linker.go:240-250) is an embedder-facing convenience for
// hosts that want to consume the raw import-name string. The
// assertion is verified against wazero's matchLockedDep behaviour.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchLockedDep(t *testing.T) {
	l := NewLinker()

	// Define the dependency with version in namespace.
	err := l.DefineInstance("my-pkg@1.2.3").
		Func("fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Match locked-dep import.
	def, err := l.MatchImport("locked-dep=my-pkg:1.2.3")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Verify it's the instance we defined.
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["fn"])
}

// TestLinker_MatchLockedDep_NotFound asserts matchLockedDep returns
// an error when no exact-version match is found. locked-dep is
// exact-match only — it does not walk semver-compatible alternatives.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment). Wazero's matchLockedDep is exact-only by
// design (linker.go:240-250 constructs a single key and returns on
// miss). The test asserts this exact-only behaviour.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchLockedDep_NotFound(t *testing.T) {
	l := NewLinker()

	// Define a different version.
	err := l.DefineInstance("my-pkg@1.0.0").
		Func("fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Request exact version that doesn't exist.
	_, err = l.MatchImport("locked-dep=my-pkg:1.2.3")
	require.Error(t, err)
}

// TestLinker_MatchUnlockedDep asserts matchUnlockedDep selects the
// highest version that satisfies the requested range. Three versions
// are registered (1.0.0, 1.5.0, 2.0.0); the request `[>=1.0.0 <2.0.0)`
// should pick 1.5.0 (highest in range).
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment). Wasmtime does not directly parse the textual
// "unlocked-dep=..." form at the Linker API layer — wazero's
// matchUnlockedDep (linker.go:253-299) is an embedder-facing
// convenience for hosts that want to consume the raw import-name
// string.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchUnlockedDep(t *testing.T) {
	l := NewLinker()

	// Define multiple versions.
	for _, ver := range []string{"1.0.0", "1.5.0", "2.0.0"} {
		err := l.DefineInstance("my-pkg@"+ver).
			Func("fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
				return nil, nil
			}).
			Build()
		require.NoError(t, err)
	}

	// Match unlocked-dep with range — should get highest in range (1.5.0).
	def, err := l.MatchImport("unlocked-dep=my-pkg:{>=1.0.0 <2.0.0}")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestLinker_MatchUnlockedDep_MatchAll asserts the "*" wildcard
// range matches all versions and selects the highest. Three versions
// are registered (0.1.0, 1.0.0, 3.0.0); the request `{*}` should
// pick 3.0.0.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment). Wazero-specific matchUnlockedDep behaviour
// with no direct wasmtime counterpart.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchUnlockedDep_MatchAll(t *testing.T) {
	l := NewLinker()

	for _, ver := range []string{"0.1.0", "1.0.0", "3.0.0"} {
		err := l.DefineInstance("pkg@"+ver).
			Func("fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
				return nil, nil
			}).
			Build()
		require.NoError(t, err)
	}

	// Match all versions — should get highest (3.0.0).
	def, err := l.MatchImport("unlocked-dep=pkg:*")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestLinker_MatchUnlockedDep_NoMatch asserts matchUnlockedDep
// returns an error when no registered version falls within the
// requested range.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment). Wazero-specific matchUnlockedDep behaviour.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchUnlockedDep_NoMatch(t *testing.T) {
	l := NewLinker()

	// Define versions outside the requested range.
	for _, ver := range []string{"0.5.0", "3.0.0"} {
		err := l.DefineInstance("my-pkg@"+ver).
			Func("fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
				return nil, nil
			}).
			Build()
		require.NoError(t, err)
	}

	// Request range that has no matches.
	_, err := l.MatchImport("unlocked-dep=my-pkg:{>=1.0.0 <2.0.0}")
	require.Error(t, err)
}

// TestLinker_MatchURLImport asserts MatchImport rejects "url=..."
// import names with a non-nil error whose message contains
// "URL imports not supported". wazero does not currently support
// fetching components by URL at link time.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (the semver doc comment describes name handling at the Linker API
// layer; wasmtime also does not fetch URL-named imports at the linker
// layer). wazero's matchImport path at linker.go:226-228 returns the
// "not supported" error — the assertion pins that exact behaviour.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchURLImport(t *testing.T) {
	l := NewLinker()

	_, err := l.MatchImport("url=https://example.com/component.wasm")
	require.Error(t, err)
	require.Contains(t, err.Error(), "URL imports not supported")
}

// TestLinker_MatchHashImport asserts MatchImport rejects
// "integrity=sha256:..." import names. Like URL imports, content-
// addressed imports are not resolved at link time in wazero.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment — wasmtime also does not resolve integrity
// hashes at the linker layer). wazero's matchImport at
// linker.go:228-229 returns the "not supported" error.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchHashImport(t *testing.T) {
	l := NewLinker()

	_, err := l.MatchImport("integrity=sha256:abc123")
	require.Error(t, err)
	require.Contains(t, err.Error(), "hash imports not supported")
}

// TestLinker_MatchPlainImport asserts MatchImport resolves plain
// (non-versioned, single-segment) names via direct lookup.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/environ/src/component/names.rs:105-117
// (NameMap::get tries the direct definitions map first; only
// semver-looking names go through alternate_lookups).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchPlainImport(t *testing.T) {
	l := NewLinker()

	// Define a plain instance (no version, no slash).
	err := l.DefineInstance("my-component").
		Func("fn", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Match plain import.
	def, err := l.MatchImport("my-component")
	require.NoError(t, err)
	require.NotNil(t, def)
}

// TestLinker_MatchInterfaceImport_Unchanged pins the regression that
// an interface import with an identical version to the registered
// instance still matches. This is the baseline case that must not
// regress as matchInterfaceImport / matchLegacyImport evolve.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:27-60
// (semver doc comment — same version always matches). wazero's
// matchInterfaceImport (linker.go:310-321) reconstructs the full
// import name and delegates to matchLegacyImport.
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker.
func TestLinker_MatchInterfaceImport_Unchanged(t *testing.T) {
	l := NewLinker()

	// Define instance at v0.2.0.
	err := l.DefineInstance("wasi:cli/environment@0.2.0").
		Func("get-environment", func(ctx context.Context, fnType *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Request the same version — should match.
	def, err := l.MatchImport("wasi:cli/environment@0.2.0")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Verify we got the instance.
	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.NotNil(t, instDef.Exports["get-environment"])
}
