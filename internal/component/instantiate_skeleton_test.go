// internal/component/instantiate_skeleton_test.go
//
// Session 1 Task C5: skeleton test for the Instantiate rebuild.
//
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md Task C5
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstantiateSkeleton asserts ComponentLinker.Instantiate returns a
// non-nil Instance with a populated *runtime.ComponentInstance for a
// trivial component (no imports, no core modules, no resources).
//
// Spec: definitions.py:256-273 ComponentInstance shape.
// Wasmtime parallel: runtime/component/instance.rs:743 Instantiator::new.
func TestInstantiateSkeleton(t *testing.T) {
	compiled := buildEmptyCompiledComponent(t)

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	inst, err := l.Instantiate(context.Background(), compiled)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NotNil(t, inst.Runtime())
}

// TestInstantiateWithTypedImport asserts Instantiate resolves a typed
// function import from the linker registry and type-checks it through
// the TypeChecker path.
//
// Spec: definitions.py import type matching (component-model import
// subtyping rules); Explainer.md:920-982.
// Wasmtime parallel: runtime/component/matching.rs:51-162
// (function import matching).
func TestInstantiateWithTypedImport(t *testing.T) {
	compiled := buildComponentWithOneTypedImport(t, "ns", "f")
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	// Register the import with a HostFunc (no type at registration —
	// wasmtime func_new model; Task C3 corrective).
	err := l.DefineFunc("ns", "f", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return args, nil
	})
	require.NoError(t, err)

	inst, err := l.Instantiate(context.Background(), compiled)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Verify the component function index space received the resolved
	// import, and its Type was populated from the component's import
	// declaration (not from the shared FuncDef, which is nil per Task C3
	// corrective).
	cf, ok := inst.GetComponentFunc(0)
	require.True(t, ok)
	require.NotNil(t, cf.Type)
	require.NotNil(t, cf.Impl)
}

// buildComponentWithOneTypedImport constructs a *Component directly with
// one typed function import (s32) -> s32. The binary package cannot be
// imported here (test import cycle); the struct is assembled by hand.
func buildComponentWithOneTypedImport(t *testing.T, ns, name string) *CompiledComponent {
	t.Helper()
	b := types.NewComponentTypesBuilder()
	paramsTuple := b.InternTuple([]types.ValType{types.S32})
	resultsTuple := b.InternTuple([]types.ValType{types.S32})
	funcIdx := b.InternFunc(false, []string{"x"}, paramsTuple, resultsTuple)
	ct := b.Finish()

	c := &Component{
		Types:              ct,
		FuncIdxToCanonical: make(map[uint32]uint32),
	}
	// One TypeDef slot referring to the interned func type.
	c.TypeDefs = []TypeDef{{Kind: TypeDefKindFunc, Func: funcIdx}}
	c.NextTypeIdx = 1
	// One function import referring to type index 0.
	c.Imports = []Import{{
		Name: ns + "/" + name,
		ExternDesc: ImportExternDesc{
			Kind:    ImportExternDescFunc,
			TypeIdx: 0,
		},
	}}
	c.NextFuncIdx = 1
	return NewCompiledComponent(c, nil, nil)
}

// buildEmptyCompiledComponent constructs the minimal valid CompiledComponent
// for instantiation skeleton tests: a Component with an empty
// ComponentTypes bag, no sections, no core modules, no resources.
// The binary package cannot be imported here without a test-cycle, so the
// struct is assembled directly.
func buildEmptyCompiledComponent(t *testing.T) *CompiledComponent {
	t.Helper()
	c := &Component{
		Types:              &types.ComponentTypes{},
		FuncIdxToCanonical: make(map[uint32]uint32),
	}
	return NewCompiledComponent(c, nil, nil)
}
