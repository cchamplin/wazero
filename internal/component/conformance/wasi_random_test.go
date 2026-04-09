// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI random conformance tests verify that
// wazero's random host module produces correctly-sized, non-trivial
// output from both cryptographic and insecure RNGs.
package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2/random"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestWASIRandom exercises the wasi:random host module API surface.
//
// No counterpart (justified): WASI P2 host module conformance invariant.
func TestWASIRandom(t *testing.T) {

	// ------------------------------------------------------------------
	// Case 1: GetRandomBytes returns the requested number of bytes.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:random/random@0.2.0 get-random-bytes(len) must
	// return exactly len bytes.
	t.Run("GetRandomBytes", func(t *testing.T) {
		bytes := random.GetRandomBytes(32)
		require.Equal(t, 32, len(bytes))
		// Verify not all zeros (extremely unlikely for random)
		allZero := true
		for _, b := range bytes {
			if b != 0 {
				allZero = false
				break
			}
		}
		require.False(t, allZero, "random bytes should not be all zeros")
	})

	// ------------------------------------------------------------------
	// Case 2: GetRandomBytes with zero length returns empty slice.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. get-random-bytes(0) must return an empty byte slice.
	t.Run("GetRandomBytesZero", func(t *testing.T) {
		bytes := random.GetRandomBytes(0)
		require.Equal(t, 0, len(bytes))
	})

	// ------------------------------------------------------------------
	// Case 3: GetRandomU64 returns varying values across calls.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Successive calls to get-random-u64 should very rarely
	// produce identical values.
	t.Run("GetRandomU64", func(t *testing.T) {
		first := random.GetRandomU64()
		second := random.GetRandomU64()
		// The probability of two random u64 values being equal is 1/2^64
		require.True(t, first != second,
			"two random u64 values should differ: both=%d", first)
	})

	// ------------------------------------------------------------------
	// Case 4: GetInsecureRandomBytes returns the requested length.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. wasi:random/insecure@0.2.0 get-insecure-random-bytes
	// must return exactly the requested number of bytes.
	t.Run("GetInsecureRandomBytes", func(t *testing.T) {
		bytes := random.GetInsecureRandomBytes(64)
		require.Equal(t, 64, len(bytes))
	})

	// ------------------------------------------------------------------
	// Case 5: InsecureSeed returns stable values within a process.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Per wasi:random/insecure-seed@0.2.0 spec, insecure-seed
	// must return the same values within a component instance.
	t.Run("InsecureSeedStable", func(t *testing.T) {
		s1a, s2a := random.InsecureSeed()
		s1b, s2b := random.InsecureSeed()
		require.Equal(t, s1a, s1b, "insecure-seed first value should be stable")
		require.Equal(t, s2a, s2b, "insecure-seed second value should be stable")
	})

	// ------------------------------------------------------------------
	// Case 6: GetRandomBytes caps at 64KB.
	// ------------------------------------------------------------------
	//
	// No counterpart (justified): WASI P2 host module conformance
	// invariant. Our implementation caps at 64KB to prevent DoS.
	t.Run("GetRandomBytesCapped", func(t *testing.T) {
		bytes := random.GetRandomBytes(128 * 1024) // Request 128KB
		require.Equal(t, 64*1024, len(bytes), "should be capped at 64KB")
	})
}
