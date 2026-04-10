// internal/component/instance_pre_test.go
//
// Tests for InstancePre: pre-computed import resolution pattern.
// Task 21: factor import resolution out of Instantiate so it can be
// cached and reused across multiple instantiations.
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstantiatePreSuccess verifies that InstantiatePre succeeds and
// returns a non-nil InstancePre for a trivial component with no imports.
func TestInstantiatePreSuccess(t *testing.T) {
	compiled := buildEmptyCompiledComponent(t)

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	pre, err := l.InstantiatePre(compiled)
	require.NoError(t, err)
	require.NotNil(t, pre)
}

// TestInstantiatePreNilCompiled verifies that InstantiatePre returns an
// error when compiled is nil.
func TestInstantiatePreNilCompiled(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	pre, err := l.InstantiatePre(nil)
	require.Error(t, err)
	require.Nil(t, pre)
}

// TestInstancePreInstantiateCreatesWorkingInstance verifies that
// InstancePre.Instantiate creates a working instance equivalent to
// the direct Instantiate path.
func TestInstancePreInstantiateCreatesWorkingInstance(t *testing.T) {
	compiled := buildEmptyCompiledComponent(t)

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	pre, err := l.InstantiatePre(compiled)
	require.NoError(t, err)

	inst, err := pre.Instantiate(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NotNil(t, inst.Runtime())
}

// TestInstancePreInstantiateWithTypedImport verifies that InstancePre
// correctly resolves and type-checks a typed function import, matching
// the behavior of the direct Instantiate path.
func TestInstancePreInstantiateWithTypedImport(t *testing.T) {
	compiled := buildComponentWithOneTypedImport(t, "ns", "f")

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	err := l.DefineFunc("ns", "f", func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return args, nil
	})
	require.NoError(t, err)

	pre, err := l.InstantiatePre(compiled)
	require.NoError(t, err)

	inst, err := pre.Instantiate(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inst)

	cf, ok := inst.GetComponentFunc(0)
	require.True(t, ok)
	require.NotNil(t, cf.Type)
	require.NotNil(t, cf.Impl)
}

// TestInstancePreMultipleInstantiatesCreateDistinctInstances verifies
// that calling Instantiate multiple times on the same InstancePre
// creates distinct instances with independent state.
func TestInstancePreMultipleInstantiatesCreateDistinctInstances(t *testing.T) {
	compiled := buildEmptyCompiledComponent(t)

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	pre, err := l.InstantiatePre(compiled)
	require.NoError(t, err)

	ctx := context.Background()
	inst1, err := pre.Instantiate(ctx)
	require.NoError(t, err)
	require.NotNil(t, inst1)

	inst2, err := pre.Instantiate(ctx)
	require.NoError(t, err)
	require.NotNil(t, inst2)

	// Distinct instances must have different instance IDs.
	require.NotEqual(t, inst1.Runtime().ID, inst2.Runtime().ID)

	// Distinct instances must be different pointers.
	require.True(t, inst1 != inst2)
}

// TestInstancePreComponent verifies that InstancePre.Component returns
// the compiled component that was used to create the InstancePre.
func TestInstancePreComponent(t *testing.T) {
	compiled := buildEmptyCompiledComponent(t)

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	pre, err := l.InstantiatePre(compiled)
	require.NoError(t, err)

	require.Equal(t, compiled, pre.Component())
}

// TestInstantiatePreMissingImport verifies that InstantiatePre fails
// with an appropriate error when a required import is not defined.
func TestInstantiatePreMissingImport(t *testing.T) {
	compiled := buildComponentWithOneTypedImport(t, "ns", "f")

	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l := NewComponentLinker(rt)
	// Do NOT define the import.
	pre, err := l.InstantiatePre(compiled)
	require.Error(t, err)
	require.Nil(t, pre)
}

// TestInstantiateViaPreMatchesDirectPath verifies that the refactored
// Instantiate (which now delegates to InstantiatePre) produces the same
// result as calling InstantiatePre + Instantiate explicitly.
func TestInstantiateViaPreMatchesDirectPath(t *testing.T) {
	compiled := buildComponentWithOneTypedImport(t, "ns", "f")

	hostFn := func(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
		return args, nil
	}

	// Direct Instantiate path.
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	l1 := NewComponentLinker(rt)
	require.NoError(t, l1.DefineFunc("ns", "f", hostFn))
	directInst, err := l1.Instantiate(context.Background(), compiled)
	require.NoError(t, err)
	require.NotNil(t, directInst)

	// Explicit InstantiatePre + Instantiate path.
	l2 := NewComponentLinker(rt)
	require.NoError(t, l2.DefineFunc("ns", "f", hostFn))
	pre, err := l2.InstantiatePre(compiled)
	require.NoError(t, err)
	preInst, err := pre.Instantiate(context.Background())
	require.NoError(t, err)
	require.NotNil(t, preInst)

	// Both should have resolved the component function at index 0
	// with a non-nil type and implementation.
	cf1, ok1 := directInst.GetComponentFunc(0)
	cf2, ok2 := preInst.GetComponentFunc(0)
	require.True(t, ok1)
	require.True(t, ok2)
	require.NotNil(t, cf1.Type)
	require.NotNil(t, cf2.Type)
	require.NotNil(t, cf1.Impl)
	require.NotNil(t, cf2.Impl)
}
