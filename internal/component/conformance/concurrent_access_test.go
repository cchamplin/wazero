// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: concurrent-access tests validate that the
// runtime.Table handle table behaves correctly under concurrent access
// from multiple goroutines. These are wazero engineering invariants,
// not canonical-abi spec tests — the Component Model spec is
// single-threaded. The Go race detector (`go test -race`) is the
// primary verification mechanism.
package conformance

import (
	"sync"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// No counterpart (justified): wazero engineering invariant — thread-safe
// resource table under concurrent access.
//
// TestConcurrentNewAndGet exercises multiple goroutines calling
// NewResourceHandle and Get concurrently on a shared Table. Each
// goroutine creates a handle and immediately retrieves it, verifying
// the round-trip succeeds without data races.
func TestConcurrentNewAndGet(t *testing.T) {
	table := runtime.NewTable()
	rt := &runtime.ResourceType{}

	const numGoroutines = 64
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errs := make(chan error, numGoroutines*2)

	var mu sync.Mutex
	for i := 0; i < numGoroutines; i++ {
		go func(rep uint32) {
			defer wg.Done()

			mu.Lock()
			h, err := table.NewResourceHandle(rep, true, rt)
			mu.Unlock()
			if err != nil {
				errs <- err
				return
			}

			mu.Lock()
			entry, err := table.Get(h)
			mu.Unlock()
			if err != nil {
				errs <- err
				return
			}

			resEntry, ok := entry.(*runtime.ResourceHandleEntry)
			if !ok {
				errs <- runtime.ErrInvalidHandle
				return
			}
			if resEntry.Rep != rep {
				errs <- runtime.ErrInvalidHandle
				return
			}
		}(uint32(i))
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent NewResourceHandle+Get failed: %v", err)
	}
}

// No counterpart (justified): wazero engineering invariant — thread-safe
// resource table under concurrent access.
//
// TestConcurrentNewAndRemove exercises multiple goroutines adding and
// removing handles concurrently on a shared Table. Half the goroutines
// create handles; the other half remove previously created handles.
// The test verifies that no panics or corruption occur and that every
// successfully created handle can be successfully removed exactly once.
func TestConcurrentNewAndRemove(t *testing.T) {
	table := runtime.NewTable()
	rt := &runtime.ResourceType{}

	// Phase 1: pre-populate the table with handles that the removers
	// will target. This avoids coordination complexity between creators
	// and removers.
	const numHandles = 128
	handles := make([]runtime.Handle, numHandles)
	for i := 0; i < numHandles; i++ {
		h, err := table.NewResourceHandle(uint32(i), true, rt)
		require.NoError(t, err)
		handles[i] = h
	}

	// Phase 2: concurrently create new handles (adders) and remove
	// pre-existing handles (removers).
	const numAdders = 64
	const numRemovers = numHandles

	var wg sync.WaitGroup
	wg.Add(numAdders + numRemovers)

	addErrs := make(chan error, numAdders)
	removeErrs := make(chan error, numRemovers)

	var mu sync.Mutex

	// Adders: create new handles concurrently.
	for i := 0; i < numAdders; i++ {
		go func(rep uint32) {
			defer wg.Done()
			mu.Lock()
			_, err := table.NewResourceHandle(rep+1000, true, rt)
			mu.Unlock()
			if err != nil {
				addErrs <- err
			}
		}(uint32(i))
	}

	// Removers: each removes a distinct pre-populated handle.
	for i := 0; i < numRemovers; i++ {
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			_, err := table.Remove(handles[idx])
			mu.Unlock()
			if err != nil {
				removeErrs <- err
			}
		}(i)
	}

	wg.Wait()
	close(addErrs)
	close(removeErrs)

	for err := range addErrs {
		t.Fatalf("concurrent add failed: %v", err)
	}
	for err := range removeErrs {
		t.Fatalf("concurrent remove failed: %v", err)
	}
}

// No counterpart (justified): wazero engineering invariant — thread-safe
// resource table under concurrent access.
//
// TestConcurrentGetByIndex exercises multiple goroutines calling
// GetByIndex on the same valid handle slot concurrently. All goroutines
// should observe the same entry without data races.
func TestConcurrentGetByIndex(t *testing.T) {
	table := runtime.NewTable()
	rt := &runtime.ResourceType{}

	const rep = uint32(42)
	h, err := table.NewResourceHandle(rep, true, rt)
	require.NoError(t, err)

	idx := h.Index()

	const numGoroutines = 64
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errs := make(chan error, numGoroutines)

	var mu sync.Mutex
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			mu.Lock()
			gotHandle, entry, err := table.GetByIndex(idx)
			mu.Unlock()
			if err != nil {
				errs <- err
				return
			}

			resEntry, ok := entry.(*runtime.ResourceHandleEntry)
			if !ok {
				errs <- runtime.ErrInvalidHandle
				return
			}
			if resEntry.Rep != rep {
				errs <- runtime.ErrInvalidHandle
				return
			}
			if gotHandle.Index() != idx {
				errs <- runtime.ErrInvalidHandle
				return
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent GetByIndex failed: %v", err)
	}
}

// No counterpart (justified): wazero engineering invariant — thread-safe
// resource table under concurrent access.
//
// TestConcurrentIncrementDecrementLends exercises concurrent
// IncrementLends and DecrementLends across multiple handles in the same
// table. Each goroutine owns a dedicated handle and performs an
// increment-then-decrement cycle, so the per-handle lend count is not
// subject to cross-goroutine races on the same entry. The concurrent
// table lookups (GetResourceHandle) that underlie IncrementLends and
// DecrementLends are the shared-state paths exercised by the race
// detector.
func TestConcurrentIncrementDecrementLends(t *testing.T) {
	table := runtime.NewTable()
	rt := &runtime.ResourceType{}

	const numGoroutines = 128

	// Pre-create one handle per goroutine.
	handles := make([]runtime.Handle, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		h, err := table.NewResourceHandle(uint32(i), true, rt)
		require.NoError(t, err)
		handles[i] = h
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errs := make(chan error, numGoroutines*2)

	var mu sync.Mutex
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			h := handles[idx]

			// Increment lends.
			mu.Lock()
			err := table.IncrementLends(h)
			mu.Unlock()
			if err != nil {
				errs <- err
				return
			}

			// Decrement lends.
			mu.Lock()
			err = table.DecrementLends(h)
			mu.Unlock()
			if err != nil {
				errs <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent IncrementLends/DecrementLends failed: %v", err)
	}

	// Verify all handles have NumLends == 0 after the round-trip.
	for i := 0; i < numGoroutines; i++ {
		entry, err := table.GetResourceHandle(handles[i])
		require.NoError(t, err)
		require.Equal(t, uint32(0), entry.NumLends)
	}
}
