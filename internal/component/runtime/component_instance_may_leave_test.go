// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "testing"

// TestIsMayLeaveIsStandaloneBoolean asserts the spec's may_leave flag is
// independent of enterCount. The prior wazero implementation ANDed the two,
// which caused canon.lower and canon.resource.new/drop to trap while on
// the call stack even though the spec permits them.
//
// Spec: definitions.py:260 (class ComponentInstance: may_leave: bool).
// Spec: definitions.py:1955 (lower_flat_values sets may_leave=False).
// Spec: definitions.py:1973 (lower_flat_values restores may_leave=True).
// Spec: definitions.py:2065 (canon_lower: trap_if(not caller_task.inst.may_leave)).
// Wasmtime parallel: runtime/vm/component/concurrent_disabled.rs:159 may_enter().
func TestIsMayLeaveIsStandaloneBoolean(t *testing.T) {
	inst := NewComponentInstance(0, nil)

	// Fresh instance: MayLeave defaults true, enterCount = 0, IsMayLeave() = true.
	if !inst.IsMayLeave() {
		t.Fatalf("fresh instance IsMayLeave() = false, want true")
	}

	// Enter the instance. enterCount = 1, MayLeave still true.
	// Under the spec, IsMayLeave() must still be true because the two
	// fields are orthogonal.
	inst.Enter()
	if !inst.IsMayLeave() {
		t.Fatalf("entered instance IsMayLeave() = false, want true (enterCount is orthogonal to may_leave)")
	}

	// Now explicitly set MayLeave = false. IsMayLeave() must be false.
	inst.MayLeave = false
	if inst.IsMayLeave() {
		t.Fatalf("MayLeave=false IsMayLeave() = true, want false")
	}

	// Restore MayLeave = true. IsMayLeave() must be true regardless of
	// enterCount.
	inst.MayLeave = true
	if !inst.IsMayLeave() {
		t.Fatalf("restored MayLeave IsMayLeave() = false, want true")
	}

	// Leave. enterCount = 0, MayLeave still true, IsMayLeave() still true.
	inst.Leave()
	if !inst.IsMayLeave() {
		t.Fatalf("left instance IsMayLeave() = false, want true")
	}
}
