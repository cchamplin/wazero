// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: resource generation-counter tests validate that
// wazero's generation-tagged Handle type preserves the spec's observable
// index-keyed table semantics (definitions.py:303-315 class Table).
//
// The spec's Table uses plain integer indices with no generation counter.
// wazero adds generation counting as a safety mechanism to detect
// use-after-free when slots are reused via the free list. These tests
// verify that the generation-bridging (GetByIndex returning the current
// generation) produces correct observable behavior: new allocations at
// reused slots work, stale internal handles are rejected, and
// Instance.ResourceRep correctly resolves through GetByIndex.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestResourceGeneration validates wazero's generation-bridging preserves
// the spec's observable behavior for index-keyed table operations.
//
// Spec: definitions.py:303-315 (class Table) — add/get/remove with
// index-keyed free-list reuse.
// No counterpart (justified): the spec Table has no generation counter;
// generation counting is a wazero implementation detail for use-after-free
// safety. These tests verify that the bridging is transparent to spec
// semantics.
func TestResourceGeneration(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: GenerationIncrementsOnReuse
	// ------------------------------------------------------------------
	// Allocate a handle, remove it, allocate again at the same slot;
	// verify the internal generation counter incremented.
	//
	// Spec: definitions.py:303-315 (class Table) — free-list reuse.
	//   After remove(i), add(e) reuses slot i.
	// No counterpart (justified): generation counting is a wazero
	//   implementation detail. The spec reuses the same index; wazero
	//   additionally increments the generation to detect stale handles.
	t.Run("GenerationIncrementsOnReuse", func(t *testing.T) {
		table := runtime.NewTable()
		rt := &runtime.ResourceType{}

		// Allocate handle at slot 0, generation 0.
		h1, err := table.NewResourceHandle(uint32(10), true, rt)
		require.NoError(t, err)
		require.Equal(t, uint32(0), h1.Index())
		require.Equal(t, uint32(0), h1.Generation())

		// Remove slot 0 — goes onto free list.
		_, err = table.Remove(h1)
		require.NoError(t, err)

		// Allocate again — reuses slot 0, generation increments to 1.
		h2, err := table.NewResourceHandle(uint32(20), true, rt)
		require.NoError(t, err)
		require.Equal(t, uint32(0), h2.Index(), "should reuse slot 0")
		require.Equal(t, uint32(1), h2.Generation(), "generation should increment on reuse")

		// The new handle works.
		entry, err := table.GetResourceHandle(h2)
		require.NoError(t, err)
		require.Equal(t, uint32(20), entry.Rep)
	})

	// ------------------------------------------------------------------
	// Case 2: StaleHandleRejected
	// ------------------------------------------------------------------
	// After slot reuse, an old generation-tagged Handle fails Get.
	// This is the core safety invariant of wazero's generation counting.
	//
	// Spec: definitions.py:303-315 (class Table).
	//   The spec's Table.get(i) succeeds as long as array[i] is not None.
	//   wazero adds generation validation so that a handle from a prior
	//   allocation at the same slot is rejected.
	// No counterpart (justified): the spec has no generation concept;
	//   this tests wazero's use-after-free detection.
	t.Run("StaleHandleRejected", func(t *testing.T) {
		table := runtime.NewTable()
		rt := &runtime.ResourceType{}

		// Allocate and remove.
		h1, err := table.NewResourceHandle(uint32(10), true, rt)
		require.NoError(t, err)
		_, err = table.Remove(h1)
		require.NoError(t, err)

		// Reallocate at same slot — new generation.
		h2, err := table.NewResourceHandle(uint32(20), true, rt)
		require.NoError(t, err)
		require.Equal(t, h1.Index(), h2.Index(), "same slot index")
		require.True(t, h2.Generation() > h1.Generation(), "generation should have advanced")

		// Old handle (h1) fails Get — generation mismatch.
		_, err = table.Get(h1)
		require.Error(t, err, "stale handle must be rejected")

		// Old handle (h1) fails GetResourceHandle too.
		_, err = table.GetResourceHandle(h1)
		require.Error(t, err, "stale handle must be rejected via GetResourceHandle")

		// New handle (h2) succeeds.
		entry, err := table.GetResourceHandle(h2)
		require.NoError(t, err)
		require.Equal(t, uint32(20), entry.Rep)
	})

	// ------------------------------------------------------------------
	// Case 3: GetByIndexReturnsCurrentGeneration
	// ------------------------------------------------------------------
	// GetByIndex resolves a u32 slot index to the current-generation
	// Handle. This is the bridge between the spec's index-only world
	// and wazero's generation-tagged handles.
	//
	// Spec: definitions.py:313-316 Table.get(i) — returns array[i].
	//   The spec uses a plain index; wazero's GetByIndex adds generation
	//   tagging to the returned Handle.
	// No counterpart (justified): GetByIndex is a wazero bridging API
	//   that has no spec equivalent. It must return the correct generation
	//   so that subsequent Get/Remove calls succeed.
	t.Run("GetByIndexReturnsCurrentGeneration", func(t *testing.T) {
		table := runtime.NewTable()
		rt := &runtime.ResourceType{}

		// Allocate at slot 0 (gen 0), remove, reallocate (gen 1).
		h1, err := table.NewResourceHandle(uint32(10), true, rt)
		require.NoError(t, err)
		_, err = table.Remove(h1)
		require.NoError(t, err)
		h2, err := table.NewResourceHandle(uint32(20), true, rt)
		require.NoError(t, err)

		// GetByIndex on slot 0 must return the gen-1 handle.
		gotHandle, gotEntry, err := table.GetByIndex(h2.Index())
		require.NoError(t, err)
		require.Equal(t, h2, gotHandle, "GetByIndex must return current-generation handle")
		resEntry, ok := gotEntry.(*runtime.ResourceHandleEntry)
		require.True(t, ok)
		require.Equal(t, uint32(20), resEntry.Rep)

		// The returned handle must be usable with Get.
		entry2, err := table.Get(gotHandle)
		require.NoError(t, err)
		require.Equal(t, gotEntry, entry2)
	})

	// ------------------------------------------------------------------
	// Case 4: ResourceRepUsesGetByIndex
	// ------------------------------------------------------------------
	// Instance.ResourceRep receives a u32 index from the Wasm side and
	// must resolve it via GetByIndex to find the current-generation
	// entry. This test verifies the end-to-end path after slot reuse.
	//
	// Spec: definitions.py:2169-2173 canon_resource_rep.
	//   The spec calls table.get(i) with a plain index.
	//   wazero's ResourceRep calls GetByIndex(handleIdx) internally,
	//   which bridges the u32 to the current generation.
	// No counterpart (justified): the spec's get(i) is direct; wazero
	//   interposes GetByIndex for generation safety.
	t.Run("ResourceRepUsesGetByIndex", func(t *testing.T) {
		inst := component.NewInstance(&component.Component{}, 1, nil)
		rt := &runtime.ResourceType{Impl: inst.Runtime()}
		inst.Runtime().ResourceTypes = append(inst.Runtime().ResourceTypes, rt)

		// Allocate rep=10, get the slot index.
		idx1, err := inst.ResourceNew(types.ResourceIdx(0), uint32(10))
		require.NoError(t, err)

		// Drop it.
		err = inst.ResourceDrop(types.ResourceIdx(0), idx1)
		require.NoError(t, err)

		// Allocate rep=20 — reuses the same slot index.
		idx2, err := inst.ResourceNew(types.ResourceIdx(0), uint32(20))
		require.NoError(t, err)
		require.Equal(t, idx1, idx2, "slot index should be reused")

		// ResourceRep must return the new rep (20), not the old one (10).
		rep, err := inst.ResourceRep(types.ResourceIdx(0), idx2)
		require.NoError(t, err)
		require.Equal(t, uint32(20), rep)

		// Verify GetByIndex returns consistent data.
		h, entry, err := inst.Table().GetByIndex(idx2)
		require.NoError(t, err)
		resEntry := entry.(*runtime.ResourceHandleEntry)
		require.Equal(t, uint32(20), resEntry.Rep)
		// The generation should be > 0 since slot was reused.
		require.True(t, h.Generation() > 0, "generation must have incremented after reuse")
	})

	// ------------------------------------------------------------------
	// Case 5: MultipleGenerationsAtSameSlot
	// ------------------------------------------------------------------
	// Recycle a slot several times, verify each generation works and
	// all prior generations are rejected.
	//
	// Spec: definitions.py:303-315 (class Table) — free-list reuse.
	//   The spec's add/remove cycle reuses indices. wazero's generation
	//   counter must increment on every reuse and prior handles must
	//   become invalid.
	// No counterpart (justified): verifies wazero's multi-generation
	//   safety across many reuse cycles.
	t.Run("MultipleGenerationsAtSameSlot", func(t *testing.T) {
		table := runtime.NewTable()
		rt := &runtime.ResourceType{}

		const numCycles = 5
		var handles [numCycles]runtime.Handle

		for i := 0; i < numCycles; i++ {
			h, err := table.NewResourceHandle(uint32(100+i), true, rt)
			require.NoError(t, err)
			require.Equal(t, uint32(0), h.Index(), "all allocations should use slot 0")
			require.Equal(t, uint32(i), h.Generation(), "generation should match cycle")
			handles[i] = h

			// Verify current handle works.
			entry, err := table.GetResourceHandle(h)
			require.NoError(t, err)
			require.Equal(t, uint32(100+i), entry.Rep)

			// All prior handles at this slot must be rejected.
			for j := 0; j < i; j++ {
				_, err := table.Get(handles[j])
				require.Error(t, err, "handle from generation %d must be rejected at generation %d", j, i)
			}

			// Remove to free slot 0 for next cycle.
			_, err = table.Remove(h)
			require.NoError(t, err)
		}

		// After all cycles, slot 0 is free. Allocate once more.
		hFinal, err := table.NewResourceHandle(uint32(999), true, rt)
		require.NoError(t, err)
		require.Equal(t, uint32(0), hFinal.Index())
		require.Equal(t, uint32(numCycles), hFinal.Generation())
		entry, err := table.GetResourceHandle(hFinal)
		require.NoError(t, err)
		require.Equal(t, uint32(999), entry.Rep)

		// All prior handles still rejected.
		for j := 0; j < numCycles; j++ {
			_, err := table.Get(handles[j])
			require.Error(t, err, "handle from generation %d must be rejected after %d cycles", j, numCycles)
		}
	})
}
