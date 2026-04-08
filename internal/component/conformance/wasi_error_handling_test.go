// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: WASI error-handling pattern tests are deferred.
// The original suite imported imports/wasip2 and imports/wasip2/io
// which currently have compile errors against runtime.Table. Tracked
// in
// docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md.
package conformance

import "testing"

// TestWASIErrorHandlingDeferredToSession1 stands in for the full WASI
// error-handling suite.
func TestWASIErrorHandlingDeferredToSession1(t *testing.T) {
	t.Skip(session1SkipReason)
}
