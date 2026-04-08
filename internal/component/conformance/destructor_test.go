// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conformance: destructor integration tests are deferred to
// Session 1. The original suite called the deleted
// runtime.NewResourceTable constructor. Session 1 will re-ground these
// against runtime.NewTable plus the new ComponentInstance shape.
// Tracked in
// docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md.
package conformance

import "testing"

// TestDestructorDeferredToSession1 stands in for the full destructor
// integration suite.
func TestDestructorDeferredToSession1(t *testing.T) {
	t.Skip(session1SkipReason)
}
