// internal/component/linker_api_test.go
//
// Tests for the api-adapter shims (ComponentWrapper,
// ComponentInstanceWrapper, ComponentFuncWrapper) that expose the
// internal *component.Instance surface through the public api.Component
// interface. These are wazero-specific embedder shims with no direct
// canonical-abi counterpart; see the per-test citation blocks.

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// closeMockModule is a minimal api.Module implementation for exercising
// ComponentWrapper.Close. It tracks whether Close was called and can
// return a configured error on the first Close; the second Close is a
// no-op to mirror production api.Module idempotency.
type closeMockModule struct {
	internalapi.WazeroOnlyType
	closed   bool
	closeErr error
}

func (m *closeMockModule) String() string                       { return "closeMock" }
func (m *closeMockModule) Name() string                         { return "closeMock" }
func (m *closeMockModule) Memory() api.Memory                   { return nil }
func (m *closeMockModule) ExportedFunction(string) api.Function { return nil }
func (m *closeMockModule) ExportedFunctionDefinitions() map[string]api.FunctionDefinition {
	return nil
}
func (m *closeMockModule) ExportedMemory(string) api.Memory { return nil }
func (m *closeMockModule) ExportedMemoryDefinitions() map[string]api.MemoryDefinition {
	return nil
}
func (m *closeMockModule) ExportedGlobal(string) api.Global { return nil }
func (m *closeMockModule) CloseWithExitCode(_ context.Context, _ uint32) error {
	return m.Close(context.Background())
}
func (m *closeMockModule) IsClosed() bool { return m.closed }
func (m *closeMockModule) Close(context.Context) error {
	if m.closed {
		return nil
	}
	m.closed = true
	return m.closeErr
}

// TestComponentInstanceWrapper verifies that ComponentInstanceWrapper
// proxies ExportedFunction lookups to the underlying Instance.exports
// map and returns nil for names that are absent.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs:156
// (pub fn get_func — name-based lookup over the component's export
// items, returning Option<Func>).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise the host linker/instance api-adapter layer; these wrappers
// are wazero embedder surface outside the canonical ABI scope.
func TestComponentInstanceWrapper(t *testing.T) {
	// Use NewInstance so the embedded *runtime.ComponentInstance is
	// populated (Session 1 Decision 3 / Task B4). The test then
	// replaces the exports map with a single "greet" entry; funcType
	// can remain nil because the wrapper never dereferences it.
	inst := NewInstance(nil, 0, nil)
	inst.exports["greet"] = &ExportedFunc{name: "greet"}

	wrapper := &ComponentInstanceWrapper{instance: inst}

	// Should find the function.
	fn := wrapper.ExportedFunction("greet")
	if fn == nil {
		t.Error("ExportedFunction should find 'greet'")
	}

	// Should return nil for missing.
	missing := wrapper.ExportedFunction("not-found")
	if missing != nil {
		t.Error("ExportedFunction should return nil for missing")
	}
}

// TestComponentInstanceWrapper_ExportedInstance verifies that
// ComponentInstanceWrapper.ExportedInstance proxies to
// Instance.GetExportedInstance, returns a wrapper around the nested
// instance whose own ExportedFunction lookup works, and returns nil
// for missing nested-instance names.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs:290
// (pub fn get_export — nested instance + name lookup returning
// (ComponentItem, ComponentExportIndex)).
// No counterpart (justified): canonical-abi run_tests.py does not
// exercise host-embedding instance lookups; the api-adapter layer is
// wazero-specific.
func TestComponentInstanceWrapper_ExportedInstance(t *testing.T) {
	// Build a nested instance with a single "nested-func" export.
	nestedInst := NewInstance(nil, 1, nil)
	nestedInst.exports["nested-func"] = &ExportedFunc{name: "nested-func"}

	// Build a parent instance that exposes the nested instance as
	// "child" via the exportedInstances map.
	parentInst := NewInstance(nil, 0, nil)
	parentInst.exportedInstances = map[string]*Instance{
		"child": nestedInst,
	}

	wrapper := &ComponentInstanceWrapper{instance: parentInst}

	// Should find the nested instance.
	child := wrapper.ExportedInstance("child")
	if child == nil {
		t.Fatal("ExportedInstance should find 'child'")
	}

	// The nested wrapper should resolve its own exported function.
	fn := child.ExportedFunction("nested-func")
	if fn == nil {
		t.Error("nested instance should have 'nested-func'")
	}

	// Should return nil for missing nested instance.
	missingChild := wrapper.ExportedInstance("not-found")
	if missingChild != nil {
		t.Error("ExportedInstance should return nil for missing")
	}
}

// TestComponentInstanceWrapper_NilInstance verifies that
// ComponentInstanceWrapper.ExportedFunction and ExportedInstance are
// nil-safe: wrapping a nil *Instance must not panic, and both lookup
// methods must return nil.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs:156,290
// (get_func / get_export both return Option<_>, naturally yielding None
// when the underlying state is absent).
// No counterpart (justified): canonical-abi run_tests.py does not
// model api-wrapper nil-safety; this is a Go-specific embedder contract.
func TestComponentInstanceWrapper_NilInstance(t *testing.T) {
	wrapper := &ComponentInstanceWrapper{instance: nil}

	// Should return nil for function lookup on nil instance.
	fn := wrapper.ExportedFunction("any")
	if fn != nil {
		t.Error("ExportedFunction on nil instance should return nil")
	}

	// Should return nil for nested-instance lookup on nil instance.
	inst := wrapper.ExportedInstance("any")
	if inst != nil {
		t.Error("ExportedInstance on nil instance should return nil")
	}
}

// TestComponentWrapper_Close_NilInstance verifies that
// ComponentWrapper.Close returns nil when the wrapped instance is
// already nil. Guards the early-return path at linker_api.go:178-180.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:61
// (pub struct Linker — wasmtime drives cleanup through Store lifetime
// rather than an explicit Close method, so no direct analogue exists).
// No counterpart (justified): explicit Component.Close is a wazero
// embedder-layer addition; canonical-abi run_tests.py does not exercise
// host teardown semantics.
func TestComponentWrapper_Close_NilInstance(t *testing.T) {
	wrapper := &ComponentWrapper{instance: nil}
	err := wrapper.Close(context.Background())
	if err != nil {
		t.Errorf("Close on nil instance should return nil, got: %v", err)
	}
}

// TestComponentWrapper_Close_EmptyCoreInstances verifies that
// ComponentWrapper.Close on an instance with no core modules returns
// nil and nils out the wrapper's instance pointer so subsequent calls
// take the nil-instance fast path.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:61
// (pub struct Linker — wasmtime handles resource cleanup via Store
// drop, not an explicit post-instantiation nil-out).
// No counterpart (justified): explicit Component.Close is a wazero
// embedder-layer addition; canonical-abi run_tests.py does not model
// host teardown.
func TestComponentWrapper_Close_EmptyCoreInstances(t *testing.T) {
	inst := NewInstance(nil, 0, nil)
	// NewInstance allocates a non-nil empty slice; the test just
	// verifies Close walks it without error.
	inst.coreInstances = []api.Module{}

	wrapper := &ComponentWrapper{instance: inst}
	err := wrapper.Close(context.Background())
	if err != nil {
		t.Errorf("Close on empty coreInstances should return nil, got: %v", err)
	}
	// After close, instance should be nil.
	if wrapper.instance != nil {
		t.Error("instance should be nil after Close")
	}
}

// TestComponentWrapper_Close_ClosesCoreModules verifies that
// ComponentWrapper.Close iterates coreInstances and invokes Close on
// each core module, returns nil when none error, and nils the
// wrapper's instance pointer.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:61
// (pub struct Linker — wasmtime relies on Store drop to tear down
// contained core instances rather than a per-Component Close method).
// No counterpart (justified): explicit per-component teardown with
// first-error semantics is a wazero embedder-layer concern outside the
// canonical ABI.
func TestComponentWrapper_Close_ClosesCoreModules(t *testing.T) {
	mod1 := &closeMockModule{}
	mod2 := &closeMockModule{}
	inst := NewInstance(nil, 0, nil)
	inst.coreInstances = []api.Module{mod1, mod2}

	wrapper := &ComponentWrapper{instance: inst}

	err := wrapper.Close(context.Background())
	if err != nil {
		t.Errorf("Close should return nil, got: %v", err)
	}
	if !mod1.closed {
		t.Error("first core module should be closed")
	}
	if !mod2.closed {
		t.Error("second core module should be closed")
	}
	if wrapper.instance != nil {
		t.Error("instance should be nil after Close")
	}
}

// TestComponentWrapper_Close_ReturnsFirstError verifies that when one
// of the core modules returns an error from Close, ComponentWrapper
// returns that first error, continues closing remaining modules, and
// still nils out its instance pointer. Guards the first-error-wins
// aggregation at linker_api.go:182-190.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:61
// (pub struct Linker — wasmtime has no analogous per-Component Close
// error propagation; Store drop is infallible).
// No counterpart (justified): first-error-wins multi-close aggregation
// is a wazero embedder convention with no canonical-abi parallel.
func TestComponentWrapper_Close_ReturnsFirstError(t *testing.T) {
	expectedErr := errors.New("close error")
	mod1 := &closeMockModule{closeErr: expectedErr}
	mod2 := &closeMockModule{}
	inst := NewInstance(nil, 0, nil)
	inst.coreInstances = []api.Module{mod1, mod2}

	wrapper := &ComponentWrapper{instance: inst}

	err := wrapper.Close(context.Background())
	if err == nil {
		t.Fatal("Close should return an error")
	}
	if err != expectedErr {
		t.Errorf("Close should return first error, got: %v", err)
	}
	// Both modules should still be closed despite the error.
	if !mod1.closed {
		t.Error("first core module should be closed")
	}
	if !mod2.closed {
		t.Error("second core module should be closed")
	}
	if wrapper.instance != nil {
		t.Error("instance should be nil after Close even with error")
	}
}

// TestComponentWrapper_ExportedFunctions verifies that
// ComponentWrapper.ExportedFunctions returns a copy of the exported
// functions map keyed by name, wrapping each *ExportedFunc in a
// ComponentFuncWrapper.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/
// component/instance.rs (exports iteration on Instance).
// No counterpart (justified): canonical-abi run_tests.py does not exercise
// the host-embedding export iteration API; this is a wazero embedder surface.
func TestComponentWrapper_ExportedFunctions(t *testing.T) {
	inst := NewInstance(nil, 0, nil)
	inst.exports["add"] = &ExportedFunc{name: "add"}
	inst.exports["sub"] = &ExportedFunc{name: "sub"}

	wrapper := &ComponentWrapper{instance: inst}
	fns := wrapper.ExportedFunctions()

	if len(fns) != 2 {
		t.Fatalf("ExportedFunctions should return 2 entries, got %d", len(fns))
	}
	if fns["add"] == nil {
		t.Error("ExportedFunctions should include 'add'")
	}
	if fns["sub"] == nil {
		t.Error("ExportedFunctions should include 'sub'")
	}
	if fns["missing"] != nil {
		t.Error("ExportedFunctions should not include 'missing'")
	}

	// Verify the returned map is a copy: mutations should not affect the original.
	fns["injected"] = nil
	if len(wrapper.ExportedFunctions()) != 2 {
		t.Error("ExportedFunctions must return a copy, not a reference to internal state")
	}
}

// TestComponentWrapper_ExportedFunctions_NilInstance verifies that
// ExportedFunctions returns nil when the wrapper's instance is nil.
func TestComponentWrapper_ExportedFunctions_NilInstance(t *testing.T) {
	wrapper := &ComponentWrapper{instance: nil}
	fns := wrapper.ExportedFunctions()
	if fns != nil {
		t.Error("ExportedFunctions on nil instance should return nil")
	}
}

// TestComponentWrapper_ExportedFunctions_Empty verifies that
// ExportedFunctions returns an empty (non-nil) map for an instance
// with no exports.
func TestComponentWrapper_ExportedFunctions_Empty(t *testing.T) {
	inst := NewInstance(nil, 0, nil)
	wrapper := &ComponentWrapper{instance: inst}
	fns := wrapper.ExportedFunctions()
	if fns == nil {
		t.Error("ExportedFunctions should return non-nil empty map")
	}
	if len(fns) != 0 {
		t.Errorf("ExportedFunctions should return empty map, got %d entries", len(fns))
	}
}

// TestComponentInstanceWrapper_ExportedFunctions verifies that
// ComponentInstanceWrapper.ExportedFunctions returns a copy of the
// nested instance's exported functions.
func TestComponentInstanceWrapper_ExportedFunctions(t *testing.T) {
	inst := NewInstance(nil, 0, nil)
	inst.exports["greet"] = &ExportedFunc{name: "greet"}

	wrapper := &ComponentInstanceWrapper{instance: inst}
	fns := wrapper.ExportedFunctions()

	if len(fns) != 1 {
		t.Fatalf("ExportedFunctions should return 1 entry, got %d", len(fns))
	}
	if fns["greet"] == nil {
		t.Error("ExportedFunctions should include 'greet'")
	}

	// Verify the returned map is a copy.
	fns["injected"] = nil
	if len(wrapper.ExportedFunctions()) != 1 {
		t.Error("ExportedFunctions must return a copy")
	}
}

// TestComponentInstanceWrapper_ExportedFunctions_NilInstance verifies that
// ExportedFunctions returns nil when the wrapper's instance is nil.
func TestComponentInstanceWrapper_ExportedFunctions_NilInstance(t *testing.T) {
	wrapper := &ComponentInstanceWrapper{instance: nil}
	fns := wrapper.ExportedFunctions()
	if fns != nil {
		t.Error("ExportedFunctions on nil instance should return nil")
	}
}

// TestComponentLinkerWrapper_DefineUnknownImportsAsTraps verifies
// that DefineUnknownImportsAsTraps on the wrapper delegates to the
// underlying ComponentLinker.
func TestComponentLinkerWrapper_DefineUnknownImportsAsTraps(t *testing.T) {
	wrapper := NewComponentLinkerWrapper(nil)
	// Should not panic.
	wrapper.DefineUnknownImportsAsTraps()
	// Verify the underlying linker has the flag set.
	if !wrapper.linker.trapUnknownImports {
		t.Error("DefineUnknownImportsAsTraps should set trapUnknownImports on the underlying linker")
	}
}

// TestComponentWrapper_Close_DoubleClose verifies that calling
// ComponentWrapper.Close twice is safe: the first call closes the
// module and nils the instance pointer, and the second call takes the
// nil-instance fast path and returns nil.
//
// Wasmtime parallel: debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs:61
// (pub struct Linker — wasmtime's Store drop is run exactly once; the
// idempotency contract is a wazero-specific affordance).
// No counterpart (justified): idempotent double-Close is a wazero
// embedder-layer robustness guarantee with no canonical-abi analogue.
func TestComponentWrapper_Close_DoubleClose(t *testing.T) {
	mod := &closeMockModule{}
	inst := NewInstance(nil, 0, nil)
	inst.coreInstances = []api.Module{mod}

	wrapper := &ComponentWrapper{instance: inst}

	// First close should succeed.
	err := wrapper.Close(context.Background())
	if err != nil {
		t.Errorf("first Close should return nil, got: %v", err)
	}

	// Second close should be a safe no-op.
	err = wrapper.Close(context.Background())
	if err != nil {
		t.Errorf("second Close should return nil (no-op), got: %v", err)
	}
}
