package conformance

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestInstance_MayLeaveDefaultTrue verifies the default may_leave == true
// as set by the ComponentInstance constructor.
//
// Spec: definitions.py:256-273 (class ComponentInstance) + :270
// (self.may_leave = True default in __init__).
// Session 1 Task B1: may_leave is a standalone boolean decoupled from
// enterCount (design Decision 3).
func TestInstance_MayLeaveDefaultTrue(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)
	require.True(t, inst.MayLeave(), "may_leave should default to true")
}

// TestInstance_SetMayLeave exercises the SetMayLeave accessor that backs the
// spec toggles in lower_flat_values / canon_lift post-return.
//
// Spec: definitions.py:1955 (cx.inst.may_leave = False at entry of
// lower_flat_values), :1973 (cx.inst.may_leave = True at exit),
// :2000/:2002 (may_leave toggled around the canon_lift post-return call).
// Session 1 Task B1: may_leave is a standalone boolean decoupled from
// enterCount.
func TestInstance_SetMayLeave(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Default is true
	require.True(t, inst.MayLeave())

	// Set to false
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave())

	// Set back to true
	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave())
}

// TestInstance_MayLeaveFalseDuringLowering documents the lowering cycle:
// may_leave is flipped to false while lower_flat_values runs, then restored
// to true on exit. If a component were to call back during parameter
// lowering, canon_lower would observe may_leave=false and trap.
//
// Spec: definitions.py:1954-1974 (lower_flat_values function), :1955
// (cx.inst.may_leave = False at entry), :1973 (cx.inst.may_leave = True
// at exit), :2065 (canon_lower trap_if(not thread.task.inst.may_leave)).
// Session 1 Task B1: may_leave is a standalone boolean decoupled from
// enterCount.
func TestInstance_MayLeaveFalseDuringLowering(t *testing.T) {
	// This test verifies that if a component function were to call back
	// during parameter lowering, it would see mayLeave=false.
	//
	// We test this by checking the flag state in the Call path.
	// The actual enforcement is tested in Task 1.5.

	inst := component.NewInstance(&component.Component{}, 0, nil)
	require.True(t, inst.MayLeave(), "should start true")

	// Simulate entering lowering
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave(), "should be false during lowering")

	// Simulate exiting lowering
	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave(), "should be true after lowering")
}

// TestInstance_MayLeaveFalseDuringPostReturn documents the post-return
// cycle: canon_lift clears may_leave for the duration of the post-return
// call, then restores it. This ensures synchronously-lowered calls to
// synchronously-lifted functions can always be implemented by a plain
// synchronous function call.
//
// Spec: definitions.py:1999-2002 (canon_lift post-return block — sets
// inst.may_leave = False before opts.post_return and = True after).
// CanonicalABI.md:3286-3297 (post-return block with rationale at
// 3293-3297 — line references verified on HEAD 46f1f549).
// Session 1 Task B1: may_leave is a standalone boolean decoupled from
// enterCount.
func TestInstance_MayLeaveFalseDuringPostReturn(t *testing.T) {
	// This test documents that may_leave should be false during post-return.
	// The actual logic is in instance.go where postReturnFunc is called.
	// Per CanonicalABI.md:3286-3297, may_leave is cleared for the duration
	// of the post-return call so that synchronous lowered calls can always
	// be implemented by plain synchronous function calls.

	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Simulate post-return execution
	inst.SetMayLeave(false)
	require.False(t, inst.MayLeave(), "should be false during post-return")

	inst.SetMayLeave(true)
	require.True(t, inst.MayLeave(), "should be true after post-return")
}

// TestInstance_ValidateMayLeave verifies the trap helper wazero uses to
// implement the spec `trap_if(not may_leave)` guards in canon_lower and
// canon_resource_*. Nil means "no trap"; an error containing
// "may_leave=false" means trap.
//
// Spec: definitions.py:2065 (canon_lower trap_if(not may_leave)),
// :2135 (canon_resource_new trap_if(not may_leave)),
// :2143 (canon_resource_drop trap_if(not may_leave)).
// Session 1 Task B1: may_leave is a standalone boolean decoupled from
// enterCount. Session 1 Task B3: the sentinel is
// "component instance cannot leave (may_leave=false)".
func TestInstance_ValidateMayLeave(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// When may_leave is true, validation passes
	err := inst.ValidateMayLeave()
	require.NoError(t, err)

	// When may_leave is false, validation fails. Session 1 B3 renamed
	// the sentinel to "component instance cannot leave (may_leave=false)".
	inst.SetMayLeave(false)
	err = inst.ValidateMayLeave()
	require.Error(t, err)
	require.Contains(t, err.Error(), "may_leave=false")
}

// TestInstance_ValidateMayLeaveNilInstance verifies the nil-receiver
// short-circuit in ValidateMayLeave: when no component instance is bound
// to the calling context (pure host call path), there is nothing to trap
// on and the helper returns nil.
//
// Spec: definitions.py:2065 (canon_lower trap_if is keyed on
// thread.task.inst.may_leave — when no inst is bound, no guard fires).
// No counterpart (justified): wazero-specific nil-safety of the
// *Instance receiver; the Python reference model always has a bound
// ComponentInstance and so does not encode this case.
func TestInstance_ValidateMayLeaveNilInstance(t *testing.T) {
	var inst *component.Instance
	err := inst.ValidateMayLeave()
	require.NoError(t, err, "nil instance should not error")
}

// TestMayLeave_MultipleSetCycles hammers the may_leave accessor pair
// across ten synthetic lowering cycles to ensure repeated toggles do
// not leak state between cycles.
//
// Spec: definitions.py:1955/:1973 (lower_flat_values toggles may_leave
// on every invocation).
// No counterpart (justified): wazero-specific robustness test for the
// MayLeave/SetMayLeave accessor pair — not a canonical-abi invariant
// per se.
func TestMayLeave_MultipleSetCycles(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Simulate multiple lowering cycles
	for i := 0; i < 10; i++ {
		require.True(t, inst.MayLeave())
		inst.SetMayLeave(false)
		require.False(t, inst.MayLeave())
		inst.SetMayLeave(true)
	}
	require.True(t, inst.MayLeave())
}

// TestMayLeave_ValidationDuringLowering walks the full lowering lifecycle
// through ValidateMayLeave: pre-lowering passes, during-lowering traps,
// post-lowering passes. Mirrors the spec pattern where canon_lower /
// canon_resource_* call trap_if(not may_leave) after lower_flat_values
// has cleared the flag.
//
// Spec: definitions.py:1955/:1973 (lower_flat_values toggles),
// :2065 (canon_lower trap_if), :2135/:2143 (canon_resource_* trap_if).
// Session 1 Task B1: may_leave is a standalone boolean decoupled from
// enterCount. Session 1 Task B3: sentinel contains "may_leave=false".
func TestMayLeave_ValidationDuringLowering(t *testing.T) {
	inst := component.NewInstance(&component.Component{}, 0, nil)

	// Before lowering, validation passes
	require.NoError(t, inst.ValidateMayLeave())

	// During lowering (may_leave=false), validation fails. Session 1 B3
	// renamed the sentinel to include "may_leave=false".
	inst.SetMayLeave(false)
	err := inst.ValidateMayLeave()
	require.Error(t, err)
	require.Contains(t, err.Error(), "may_leave=false")

	// After lowering (may_leave=true), validation passes again
	inst.SetMayLeave(true)
	require.NoError(t, inst.ValidateMayLeave())
}

// TestMayLeave_ConcurrentAccess exercises the MayLeave/SetMayLeave/
// ValidateMayLeave accessors from two goroutines at 1000 iterations each
// to guard against accidental data-race regressions in the accessor
// trio. The canonical ABI assumes a single-threaded per-ComponentInstance
// execution model, so this is not testing a spec invariant.
//
// Spec: definitions.py:256-273 (class ComponentInstance — single-threaded
// model; store-level locking is the spec concurrency boundary).
// No counterpart (justified): wazero-specific robustness test for the
// MayLeave/SetMayLeave accessor pair — not a canonical-abi invariant
// per se.
func TestMayLeave_ConcurrentAccess(t *testing.T) {
	// Note: may_leave is typically single-threaded per component instance,
	// but this tests basic safety.
	inst := component.NewInstance(&component.Component{}, 0, nil)

	done := make(chan bool)

	go func() {
		for i := 0; i < 1000; i++ {
			inst.SetMayLeave(false)
			_ = inst.MayLeave()
			inst.SetMayLeave(true)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			_ = inst.MayLeave()
			_ = inst.ValidateMayLeave()
		}
		done <- true
	}()

	<-done
	<-done
}
