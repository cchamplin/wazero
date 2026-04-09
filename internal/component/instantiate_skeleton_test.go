// internal/component/instantiate_skeleton_test.go
//
// Session 1 Task C5: skeleton test for the Instantiate rebuild.
//
// Plan: docs/superpowers/plans/2026-04-08-canonical-abi-session1-plan.md Task C5
package component

import (
	"context"
	"testing"

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

	l := NewComponentLinker(nil)
	inst, err := l.Instantiate(context.Background(), compiled)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NotNil(t, inst.Runtime())
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
