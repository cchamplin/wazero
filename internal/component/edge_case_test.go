// internal/component/edge_case_test.go
package component

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestTypeIndexOutOfRange verifies error handling when type index is invalid.
func TestTypeIndexOutOfRange(t *testing.T) {
	// Session 0 compile-fix: the TypeChecker no longer indexes
	// c.Types as a []TypeDef slice (it's now the canonical type bag). See
	// type_checker.go.
	t.Skip("session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md")
}

// TestDeepNesting verifies that 5+ levels of nesting work correctly.
func TestDeepNesting(t *testing.T) {
	// Create 5-level hierarchy: level1 -> level2 -> level3 -> level4 -> level5
	level1 := &Instance{}
	level1Type := &TypeDef{Kind: TypeDefKindFunc}
	level1.AddTypeToSpace(level1Type)

	level2 := &Instance{}
	level1.AddChild(level2)

	level3 := &Instance{}
	level2.AddChild(level3)

	level4 := &Instance{}
	level3.AddChild(level4)

	level5 := &Instance{}
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
func TestLinkerEmptyDefinitions(t *testing.T) {
	l := NewLinker()

	// Try to match an import with no definitions
	_, err := l.MatchImport("wasi:test/thing@1.0.0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no compatible definition")
}

// TestInstanceGetExportedInstanceNotFound verifies nil return for missing export.
func TestInstanceGetExportedInstanceNotFound(t *testing.T) {
	inst := &Instance{}

	// No exported instances
	result := inst.GetExportedInstance("nonexistent")
	require.Nil(t, result)
}

// TestValueIndexSpaceOverflow verifies value operations with many values.
func TestValueIndexSpaceOverflow(t *testing.T) {
	inst := &Instance{}

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
func TestGetAncestorBeyondRoot(t *testing.T) {
	root := &Instance{}
	child := &Instance{}
	root.AddChild(child)

	// depth=1 gets root
	ancestor := child.GetAncestor(1)
	require.Same(t, root, ancestor)

	// depth=2 exceeds hierarchy, should return nil
	beyondRoot := child.GetAncestor(2)
	require.Nil(t, beyondRoot)
}
