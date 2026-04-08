// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: resource-generation-counter tests are deferred
// to Session 1. The original suite called the deleted
// runtime.NewResourceTable constructor. Tracked in
// docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md.
package conformance

import "testing"

// TestResourceGenerationDeferredToSession1 stands in for the full
// resource-generation-counter suite.
func TestResourceGenerationDeferredToSession1(t *testing.T) {
	t.Skip(session1SkipReason)
}
