// internal/component/linker_api_test.go

package component

import (
	"testing"
)

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
