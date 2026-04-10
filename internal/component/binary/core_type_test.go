// internal/component/binary/core_type_test.go
package binary

import (
	"testing"
)

func TestDecodeCoreTypeSection(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01,       // count = 1
		0x60,       // func type
		0x02, 0x7f, 0x7f, // 2 params: i32, i32
		0x01, 0x7f, // 1 result: i32
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.CoreTypes) != 1 {
		t.Fatalf("expected 1 core type, got %d", len(c.CoreTypes))
	}

	if c.CoreTypes[0].Func == nil {
		t.Fatal("expected core func type")
	}

	if len(c.CoreTypes[0].Func.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(c.CoreTypes[0].Func.Params))
	}
}

func TestDecodeCoreModuleType(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01,       // count = 1
		0x50,       // module type
		0x02,       // 2 declarations
		// Import declaration
		0x00,                     // import
		0x04, 't', 'e', 's', 't', // module name
		0x03, 'f', 'o', 'o', // import name
		0x00, // func kind
		0x00, // type index
		// Export declaration
		0x03,                   // export
		0x03, 'b', 'a', 'r', // export name
		0x00, // func kind
		0x00, // type index
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.CoreTypes[0].Module == nil {
		t.Fatal("expected core module type")
	}

	if len(c.CoreTypes[0].Module.Imports) != 1 {
		t.Errorf("expected 1 import, got %d", len(c.CoreTypes[0].Module.Imports))
	}

	if len(c.CoreTypes[0].Module.Exports) != 1 {
		t.Errorf("expected 1 export, got %d", len(c.CoreTypes[0].Module.Exports))
	}

	// Validate import details
	imp := c.CoreTypes[0].Module.Imports[0]
	if imp.Module != "test" {
		t.Errorf("expected import module 'test', got %q", imp.Module)
	}
	if imp.Name != "foo" {
		t.Errorf("expected import name 'foo', got %q", imp.Name)
	}
	if imp.Kind != 0x00 {
		t.Errorf("expected import kind 0x00, got 0x%02x", imp.Kind)
	}

	// Validate export details
	exp := c.CoreTypes[0].Module.Exports[0]
	if exp.Name != "bar" {
		t.Errorf("expected export name 'bar', got %q", exp.Name)
	}
	if exp.Kind != 0x00 {
		t.Errorf("expected export kind 0x00, got 0x%02x", exp.Kind)
	}
}

func TestDecodeCoreModuleType_TypeDeclaration(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01, // count = 1
		0x50, // module type
		0x01, // 1 declaration
		// Type declaration
		0x01,             // type
		0x60,             // func type
		0x01, 0x7f,       // 1 param: i32
		0x01, 0x7f,       // 1 result: i32
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.CoreTypes[0].Module == nil {
		t.Fatal("expected core module type")
	}
}

func TestDecodeCoreModuleType_OuterAlias(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01, // count = 1
		0x50, // module type
		0x01, // 1 declaration
		// Outer alias declaration
		0x02, // alias
		0x10, // core sort: type
		0x01, // alias target: outer
		0x01, // outer count
		0x00, // index
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.CoreTypes[0].Module == nil {
		t.Fatal("expected core module type")
	}
}

func TestDecodeCoreModuleType_UnknownDeclaration(t *testing.T) {
	data := buildComponentWithSection(SectionIDCoreType, []byte{
		0x01, // count = 1
		0x50, // module type
		0x01, // 1 declaration
		// Unknown declaration
		0xFF,
	})

	_, err := DecodeComponent(data)
	if err == nil {
		t.Fatal("expected error for unknown declaration kind")
	}
}

func buildComponentWithSection(sectionID SectionID, content []byte) []byte {
	result := make([]byte, 0, 20+len(content))
	result = append(result, Magic[:]...)
	result = append(result, Version[:]...)
	result = append(result, LayerComponent[:]...)
	result = append(result, byte(sectionID))
	result = append(result, byte(len(content)))
	result = append(result, content...)
	return result
}
