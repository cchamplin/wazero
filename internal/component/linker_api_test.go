// internal/component/linker_api_test.go

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// closeMockModule is a mock of api.Module for testing Close behavior.
// It tracks whether Close was called and can return a configured error.
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

func TestComponentInstanceWrapper(t *testing.T) {
	// Create an instance with an exported function
	inst := &Instance{
		exports: map[string]*ExportedFunc{
			"greet": {
				name: "greet",
				funcType: &FuncType{
					Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}},
				},
			},
		},
	}

	wrapper := &ComponentInstanceWrapper{instance: inst}

	// Should find the function
	fn := wrapper.ExportedFunction("greet")
	if fn == nil {
		t.Error("ExportedFunction should find 'greet'")
	}

	// Should return nil for missing
	missing := wrapper.ExportedFunction("not-found")
	if missing != nil {
		t.Error("ExportedFunction should return nil for missing")
	}
}

func TestComponentInstanceWrapper_ExportedInstance(t *testing.T) {
	// Create a nested instance
	nestedInst := &Instance{
		exports: map[string]*ExportedFunc{
			"nested-func": {
				name: "nested-func",
				funcType: &FuncType{
					Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}},
				},
			},
		},
	}

	// Create parent instance with exported nested instance
	parentInst := &Instance{
		exportedInstances: map[string]*Instance{
			"child": nestedInst,
		},
	}

	wrapper := &ComponentInstanceWrapper{instance: parentInst}

	// Should find the nested instance
	child := wrapper.ExportedInstance("child")
	if child == nil {
		t.Fatal("ExportedInstance should find 'child'")
	}

	// The nested instance should have access to its exported function
	fn := child.ExportedFunction("nested-func")
	if fn == nil {
		t.Error("nested instance should have 'nested-func'")
	}

	// Should return nil for missing nested instance
	missingChild := wrapper.ExportedInstance("not-found")
	if missingChild != nil {
		t.Error("ExportedInstance should return nil for missing")
	}
}

func TestComponentInstanceWrapper_NilInstance(t *testing.T) {
	// Test with nil instance
	wrapper := &ComponentInstanceWrapper{instance: nil}

	// Should return nil for function lookup on nil instance
	fn := wrapper.ExportedFunction("any")
	if fn != nil {
		t.Error("ExportedFunction on nil instance should return nil")
	}

	// Should return nil for instance lookup on nil instance
	inst := wrapper.ExportedInstance("any")
	if inst != nil {
		t.Error("ExportedInstance on nil instance should return nil")
	}
}

func TestComponentWrapper_Close_NilInstance(t *testing.T) {
	// Close on a wrapper with nil instance should be safe and return nil.
	wrapper := &ComponentWrapper{instance: nil}
	err := wrapper.Close(context.Background())
	if err != nil {
		t.Errorf("Close on nil instance should return nil, got: %v", err)
	}
}

func TestComponentWrapper_Close_EmptyCoreInstances(t *testing.T) {
	// Close on an instance with empty coreInstances should succeed.
	inst := &Instance{
		coreInstances: []api.Module{},
	}
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

func TestComponentWrapper_Close_ClosesCoreModules(t *testing.T) {
	mod1 := &closeMockModule{}
	mod2 := &closeMockModule{}
	inst := &Instance{
		coreInstances: []api.Module{mod1, mod2},
	}
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

func TestComponentWrapper_Close_ReturnsFirstError(t *testing.T) {
	expectedErr := errors.New("close error")
	mod1 := &closeMockModule{closeErr: expectedErr}
	mod2 := &closeMockModule{}
	inst := &Instance{
		coreInstances: []api.Module{mod1, mod2},
	}
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

func TestComponentWrapper_Close_DoubleClose(t *testing.T) {
	mod := &closeMockModule{}
	inst := &Instance{
		coreInstances: []api.Module{mod},
	}
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
