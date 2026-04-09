package abi

import (
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// helpers to construct a LiftContext with a concrete resource type for
// testing liftOwnHandle / liftBorrowHandle gap fixes.

// makeLiftCtx creates a LiftContext whose Instance (ID 0) has a single
// *runtime.ResourceType at ResourceTypes[0], a ComponentTypes with one
// Concrete ResourceTable entry at index 0 pointing to Instance 0
// / Resource 0, and an optional BorrowScope.
func makeLiftCtx(withBorrowScope bool) (*LiftContext, *runtime.ResourceType) {
	rt := &runtime.ResourceType{}
	inst := runtime.NewComponentInstance(0, nil)
	inst.ResourceTypes = []*runtime.ResourceType{rt}

	ct := &types.ComponentTypes{
		ResourceTables: []types.TypeResourceTable{
			{
				Concrete: true,
				Instance: 0,
				Resource: 0,
			},
		},
	}

	ctx := &LiftContext{
		Types:    ct,
		Instance: inst,
	}
	if withBorrowScope {
		ctx.BorrowScope = runtime.NewBorrowScope(inst.Table)
	}
	return ctx, rt
}

// ownType returns a ValType for own<resource-0>.
func ownType() types.ValType {
	return types.ValType{Kind: types.TypeKindOwn, Index: 0}
}

// borrowType returns a ValType for borrow<resource-0>.
func borrowType() types.ValType {
	return types.ValType{Kind: types.TypeKindBorrow, Index: 0}
}

// TestLiftOwnHandleGap1TrapNotOwn verifies that liftOwnHandle traps
// when the handle is a borrow (not own).
//
// Spec: definitions.py:1338 — trap_if(not h.own)
func TestLiftOwnHandleGap1TrapNotOwn(t *testing.T) {
	ctx, rt := makeLiftCtx(false)

	// Insert a BORROW handle (own=false).
	_, err := ctx.Instance.Table.NewResourceHandle(42, false, rt)
	require.NoError(t, err)

	// handleIdx 0 is a borrow — liftOwnHandle must trap.
	_, err = liftOwnHandle(ctx, ownType(), 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not an own")
}

// TestLiftOwnHandleGap2TrapNumLends verifies that liftOwnHandle traps
// when the handle has outstanding lends.
//
// Spec: definitions.py:1337 — trap_if(h.num_lends != 0)
func TestLiftOwnHandleGap2TrapNumLends(t *testing.T) {
	ctx, rt := makeLiftCtx(false)

	// Insert an OWN handle.
	h, err := ctx.Instance.Table.NewResourceHandle(42, true, rt)
	require.NoError(t, err)

	// Manually increment lends to simulate outstanding borrow.
	err = ctx.Instance.Table.IncrementLends(h)
	require.NoError(t, err)

	// liftOwnHandle must trap because NumLends != 0.
	_, err = liftOwnHandle(ctx, ownType(), h.Index())
	require.Error(t, err)
	require.Contains(t, err.Error(), "outstanding lends")
}

// TestLiftOwnHandleGap3GenerationBridging verifies that liftOwnHandle
// uses GetByIndex to correctly resolve the generation-tagged handle
// from the Wasm-side u32 index (not a raw cast).
//
// Scenario: allocate handle h1, remove it, allocate h2 at same slot.
// liftOwnHandle(h2.Index()) must resolve to h2 (new generation), not h1.
func TestLiftOwnHandleGap3GenerationBridging(t *testing.T) {
	ctx, rt := makeLiftCtx(false)

	// Allocate h1, then remove it (frees the slot).
	h1, err := ctx.Instance.Table.NewResourceHandle(100, true, rt)
	require.NoError(t, err)
	_, err = ctx.Instance.Table.Remove(h1)
	require.NoError(t, err)

	// Allocate h2 at the same slot (reuse) with different rep.
	h2, err := ctx.Instance.Table.NewResourceHandle(200, true, rt)
	require.NoError(t, err)
	require.Equal(t, h1.Index(), h2.Index()) // same slot
	require.True(t, h2 != h1)               // different generation

	// liftOwnHandle must succeed and return the NEW rep (200).
	val, err := liftOwnHandle(ctx, ownType(), h2.Index())
	require.NoError(t, err)
	require.Equal(t, types.ValKindOwn, val.Kind())
	require.Equal(t, uint32(200), val.Own())
}

// TestLiftOwnHandleGap4ReturnsRep verifies that liftOwnHandle returns
// h.rep (the resource representation) rather than the Wasm handle index.
//
// Spec: definitions.py:1339 — return h.rep
func TestLiftOwnHandleGap4ReturnsRep(t *testing.T) {
	ctx, rt := makeLiftCtx(false)

	// Insert own handle with rep=999.
	_, err := ctx.Instance.Table.NewResourceHandle(999, true, rt)
	require.NoError(t, err)

	// handleIdx 0 — the first allocated slot.
	val, err := liftOwnHandle(ctx, ownType(), 0)
	require.NoError(t, err)
	require.Equal(t, types.ValKindOwn, val.Kind())
	// Must be the REP (999), not the handle index (0).
	require.Equal(t, uint32(999), val.Own())
}

// TestLiftBorrowHandleGap3GenerationBridging verifies that liftBorrowHandle
// uses GetByIndex to correctly resolve the generation-tagged handle.
func TestLiftBorrowHandleGap3GenerationBridging(t *testing.T) {
	ctx, rt := makeLiftCtx(true)

	// Allocate h1, remove it, allocate h2 at same slot with different rep.
	h1, err := ctx.Instance.Table.NewResourceHandle(100, true, rt)
	require.NoError(t, err)
	_, err = ctx.Instance.Table.Remove(h1)
	require.NoError(t, err)

	h2, err := ctx.Instance.Table.NewResourceHandle(200, true, rt)
	require.NoError(t, err)
	require.Equal(t, h1.Index(), h2.Index()) // same slot
	require.True(t, h2 != h1)               // different generation

	// liftBorrowHandle must succeed and return the NEW rep (200).
	val, err := liftBorrowHandle(ctx, borrowType(), h2.Index())
	require.NoError(t, err)
	require.Equal(t, types.ValKindBorrow, val.Kind())
	require.Equal(t, uint32(200), val.Borrow())
}

// TestLiftBorrowHandleGap4ReturnsRep verifies that liftBorrowHandle returns
// h.rep (the resource representation) rather than the Wasm handle index.
//
// Spec: definitions.py:1347 — return h.rep
func TestLiftBorrowHandleGap4ReturnsRep(t *testing.T) {
	ctx, rt := makeLiftCtx(true)

	// Insert own handle with rep=777.
	_, err := ctx.Instance.Table.NewResourceHandle(777, true, rt)
	require.NoError(t, err)

	val, err := liftBorrowHandle(ctx, borrowType(), 0)
	require.NoError(t, err)
	require.Equal(t, types.ValKindBorrow, val.Kind())
	// Must be the REP (777), not the handle index (0).
	require.Equal(t, uint32(777), val.Borrow())
}

// TestLiftBorrowHandleNoBorrowScope verifies that liftBorrowHandle traps
// when no borrow scope is active.
//
// Spec: definitions.py:1342 — assert(isinstance(cx.borrow_scope, Subtask))
func TestLiftBorrowHandleNoBorrowScope(t *testing.T) {
	ctx, rt := makeLiftCtx(false) // no borrow scope

	_, err := ctx.Instance.Table.NewResourceHandle(42, true, rt)
	require.NoError(t, err)

	_, err = liftBorrowHandle(ctx, borrowType(), 0)
	require.Error(t, err)
	if !strings.Contains(err.Error(), "borrow scope") {
		t.Fatalf("expected 'borrow scope' in error, got: %s", err.Error())
	}
}

// TestLiftBorrowHandleNoDoubleIncrement verifies that liftBorrowHandle
// increments NumLends exactly once. The old code called IncrementLends
// explicitly AND then called BorrowScope.AddLender which also calls
// IncrementLends — double-incrementing NumLends.
//
// Spec: definitions.py:1346 — cx.borrow_scope.add_lender(h)
// AddLender internally increments NumLends.
func TestLiftBorrowHandleNoDoubleIncrement(t *testing.T) {
	ctx, rt := makeLiftCtx(true)

	h, err := ctx.Instance.Table.NewResourceHandle(42, true, rt)
	require.NoError(t, err)

	_, err = liftBorrowHandle(ctx, borrowType(), h.Index())
	require.NoError(t, err)

	// Retrieve the entry and check NumLends is exactly 1.
	entry, err := ctx.Instance.Table.Get(h)
	require.NoError(t, err)
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	require.True(t, ok)
	require.Equal(t, uint32(1), resEntry.NumLends)
}
