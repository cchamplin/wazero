// internal/component/edge_case_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestTypeIndexOutOfRange verifies error handling when type index is invalid.
//
// The TypeChecker resolves import TypeIdx via Component.ResolveTypeDef.
// When TypeIdx exceeds len(c.TypeDefs), the checker must return an error
// instead of panicking with an out-of-range index.
//
// Spec: wasmtime matching.rs:51-162 — function import type resolution
// uses component_any_type_at(type_idx) which traps on out-of-range.
// Wasmtime parallel: crates/environ/src/component/translate.rs:796-801.
func TestTypeIndexOutOfRange(t *testing.T) {
	// Build a component with no type definitions but an import
	// referencing TypeIdx = 99 (out of range).
	c := &Component{
		Types:              &types.ComponentTypes{},
		TypeDefs:           nil, // empty — index 99 is out of range
		FuncIdxToCanonical: make(map[uint32]uint32),
		Imports: []Import{{
			Name: "ns/f",
			ExternDesc: ImportExternDesc{
				Kind:    ImportExternDescFunc,
				TypeIdx: 99,
			},
		}},
	}
	compiled := NewCompiledComponent(c, nil, nil)
	l := NewComponentLinker(nil)
	err := l.DefineFunc("ns", "f", func(_ context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return args, nil
	})
	require.NoError(t, err)

	_, err = l.Instantiate(context.Background(), compiled)
	require.Error(t, err, "expected error for out-of-range type index")
	require.Contains(t, err.Error(), "out of range")
}

// TestDeepNesting verifies that 5+ levels of nesting work correctly.
//
// No counterpart (justified): wazero engineering invariant — verifies deep
// component nesting doesn't cause stack overflow.
func TestDeepNesting(t *testing.T) {
	// Create 5-level hierarchy: level1 -> level2 -> level3 -> level4 -> level5
	level1 := newInstance(&Component{}, 1, nil)
	level1Type := &TypeDef{Kind: TypeDefKindFunc}
	level1.AddTypeToSpace(level1Type)

	level2 := newInstance(&Component{}, 2, nil)
	level1.AddChild(level2)

	level3 := newInstance(&Component{}, 3, nil)
	level2.AddChild(level3)

	level4 := newInstance(&Component{}, 4, nil)
	level3.AddChild(level4)

	level5 := newInstance(&Component{}, 5, nil)
	level4.AddChild(level5)

	// From level5, try outer alias to level1 (depth=4)
	alias := &Alias{
		Kind:       AliasKindOuter,
		Sort:       SortType,
		OuterCount: 4,
		OuterIndex: 0,
	}

	resolved, err := ResolveOuterAlias(level5, alias)
	require.NoError(t, err)

	resolvedType, ok := resolved.(*TypeDef)
	require.True(t, ok)
	require.Same(t, level1Type, resolvedType)
}

// TestLinkerEmptyDefinitions verifies that an empty linker returns proper errors.
//
// No counterpart (justified): wazero API invariant — empty linker definitions
// produce valid empty instance.
func TestLinkerEmptyDefinitions(t *testing.T) {
	l := NewLinker()

	// Try to match an import with no definitions
	_, err := l.MatchImport("wasi:test/thing@1.0.0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no compatible definition")
}

// TestInstanceGetExportedInstanceNotFound verifies nil return for missing export.
//
// No counterpart (justified): wazero API invariant — missing exported instance
// returns nil.
func TestInstanceGetExportedInstanceNotFound(t *testing.T) {
	inst := newInstance(&Component{}, 0, nil)

	// No exported instances
	result := inst.GetExportedInstance("nonexistent")
	require.Nil(t, result)
}

// TestValueIndexSpaceOverflow verifies value operations with many values.
//
// Spec: definitions.py:256-273 (value index space). Tests boundary behavior
// at index space limits.
func TestValueIndexSpaceOverflow(t *testing.T) {
	inst := newInstance(&Component{}, 0, nil)

	// Add several values
	for i := 0; i < 100; i++ {
		inst.AddValue(types.ValS32(int32(i)))
	}

	// All values should be retrievable
	for i := uint32(0); i < 100; i++ {
		val, err := inst.GetValue(i)
		require.NoError(t, err)
		require.Equal(t, int32(i), val.S32())
	}

	// Index 100 should fail
	_, err := inst.GetValue(100)
	require.Error(t, err)
}

// TestGetAncestorBeyondRoot verifies nil return when depth exceeds hierarchy.
//
// Spec: definitions.py:290-299 (reflexive ancestors). Tests ancestor traversal
// past the root returns nil.
func TestGetAncestorBeyondRoot(t *testing.T) {
	root := newInstance(&Component{}, 0, nil)
	child := newInstance(&Component{}, 1, root)
	root.AddChild(child)

	// depth=1 gets root
	ancestor := child.GetAncestor(1)
	require.Same(t, root, ancestor)

	// depth=2 exceeds hierarchy, should return nil
	beyondRoot := child.GetAncestor(2)
	require.Nil(t, beyondRoot)
}
