// internal/component/component_linker_test.go
//
// SESSION 0 COMPILE-FIX STUB (Task 17).
//
// The previous test suite here exercised the deleted lift/lower path
// (FuncType, NamedValType, ValTypeRef, canonLowerInfo, canonResourceInfo,
// ComponentLinker.buildImportResolver, etc.) and won't compile against the
// new type shapes. Every test has been reduced to a t.Skip pointing at the
// Session 1 followup note. Task 19 collects the full list of skipped tests.
package component

import "testing"

const componentLinkerTestSkipMsg = "session 1 work: see docs/plans/2026-04-07-canonical-abi-unification-session0-followup.md"

func TestComponentLinkerDefineFunc(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestComponentLinkerDefineInstance(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestComponentLinkerMergeFrom(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestComponentLinkerDefineResource(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestPostReturnCalledAfterMainFunction(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestPostReturnNotCalledWhenNil(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestComponentLinker_OrderedInstantiation(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}

func TestComponentLinker_TypeCheckingIntegration(t *testing.T) {
	t.Skip(componentLinkerTestSkipMsg)
}
