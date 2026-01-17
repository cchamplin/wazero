// internal/component/binary/component_type_test.go
package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeComponentType(t *testing.T) {
	// Component type with an import declaration
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x03,       // import declaration
		0x00,       // simple name
		0x04, 't', 'e', 's', 't', // name "test"
		0x01,       // func extern desc
		0x00,       // type index 0
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(c.Types))
	}

	if c.Types[0].Kind != component.TypeDefKindComponent {
		t.Fatalf("expected component type, got kind %d", c.Types[0].Kind)
	}

	if c.Types[0].Component == nil {
		t.Fatal("expected component type def")
	}
}

func TestDecodeComponentTypeEmpty(t *testing.T) {
	// Component type with no declarations
	data := buildComponentWithTypeSection([]byte{
		0x41, // component type opcode
		0x00, // 0 declarations
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(c.Types))
	}

	if c.Types[0].Kind != component.TypeDefKindComponent {
		t.Fatalf("expected component type, got kind %d", c.Types[0].Kind)
	}

	if c.Types[0].Component == nil {
		t.Fatal("expected component type def")
	}

	if len(c.Types[0].Component.Declarations) != 0 {
		t.Errorf("expected 0 declarations, got %d", len(c.Types[0].Component.Declarations))
	}
}

func TestDecodeComponentTypeWithExport(t *testing.T) {
	// Component type with an export declaration
	// (component (export "foo" (func (type 0))))
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x03, 'f', 'o', 'o', // name "foo"
		0x01,       // externdesc kind: func
		0x00,       // type index 0
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Component.Declarations[0].Kind != component.ComponentDeclKindExport {
		t.Errorf("expected export declaration")
	}
	if c.Types[0].Component.Declarations[0].Export.Name != "foo" {
		t.Errorf("expected name 'foo', got '%s'", c.Types[0].Component.Declarations[0].Export.Name)
	}
}

func TestDecodeComponentTypeWithAlias(t *testing.T) {
	// Component type with alias declaration
	// (component (alias outer 0 1 (type)))
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x02,       // alias declaration
		0x03,       // type sort
		0x02,       // outer alias target
		0x00,       // outer count 0
		0x01,       // outer index 1
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Component.Declarations[0].Kind != component.ComponentDeclKindAlias {
		t.Errorf("expected alias declaration")
	}
}

func TestDecodeComponentTypeWithCoreType(t *testing.T) {
	// Component type with core type declaration
	// (component (core type (func)))
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x00,       // core type declaration
		0x60,       // core func type opcode
		0x00,       // 0 params
		0x00,       // 0 results
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Component.Declarations[0].Kind != component.ComponentDeclKindCoreType {
		t.Errorf("expected core type declaration")
	}
	if c.Types[0].Component.Declarations[0].CoreType == nil {
		t.Fatal("expected core type")
	}
	if c.Types[0].Component.Declarations[0].CoreType.Kind != component.CoreTypeDefKindFunc {
		t.Errorf("expected core func type")
	}
}

func TestDecodeComponentTypeWithNestedType(t *testing.T) {
	// Component type with nested type declaration (a record type)
	// (component (type (record (field "x" s32))))
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x01,       // type declaration
		0x72,       // record type opcode
		0x01,       // 1 field
		0x01, 'x',  // field name "x"
		0x7a,       // s32
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Types[0].Component.Declarations[0].Kind != component.ComponentDeclKindType {
		t.Errorf("expected type declaration")
	}
	if c.Types[0].Component.Declarations[0].Type == nil {
		t.Fatal("expected nested type")
	}
	if c.Types[0].Component.Declarations[0].Type.Kind != component.TypeDefKindDefined {
		t.Errorf("expected defined type kind")
	}
	if c.Types[0].Component.Declarations[0].Type.Record == nil {
		t.Fatal("expected record type def")
	}
}

func TestDecodeComponentTypeMultipleDeclarations(t *testing.T) {
	// Component type with multiple declarations
	// (component (import "a" (func (type 0))) (export "b" (func (type 1))))
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x02,       // 2 declarations
		// Import declaration
		0x03,       // import declaration
		0x00,       // simple name
		0x01, 'a',  // name "a"
		0x01,       // func extern desc
		0x00,       // type index 0
		// Export declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x01, 'b',  // name "b"
		0x01,       // externdesc kind: func
		0x01,       // type index 1
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types[0].Component.Declarations) != 2 {
		t.Errorf("expected 2 declarations, got %d", len(c.Types[0].Component.Declarations))
	}

	// Check import
	if c.Types[0].Component.Declarations[0].Kind != component.ComponentDeclKindImport {
		t.Errorf("expected import declaration for first")
	}
	if c.Types[0].Component.Declarations[0].Import.Name != "a" {
		t.Errorf("expected name 'a', got '%s'", c.Types[0].Component.Declarations[0].Import.Name)
	}

	// Check export
	if c.Types[0].Component.Declarations[1].Kind != component.ComponentDeclKindExport {
		t.Errorf("expected export declaration for second")
	}
	if c.Types[0].Component.Declarations[1].Export.Name != "b" {
		t.Errorf("expected name 'b', got '%s'", c.Types[0].Component.Declarations[1].Export.Name)
	}
}

func TestDecodeComponentTypeImportInstance(t *testing.T) {
	// Component type with import of instance
	// (component (import "inst" (instance (type 0))))
	data := buildComponentWithTypeSection([]byte{
		0x41,       // component type opcode
		0x01,       // 1 declaration
		0x03,       // import declaration
		0x00,       // simple name
		0x04, 'i', 'n', 's', 't', // name "inst"
		0x05,       // instance extern desc
		0x02,       // type index 2
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imp := c.Types[0].Component.Declarations[0].Import
	if imp.Name != "inst" {
		t.Errorf("expected name 'inst', got '%s'", imp.Name)
	}
	if imp.ExternDesc.Kind != component.ImportExternDescInstance {
		t.Errorf("expected instance kind")
	}
	if imp.ExternDesc.TypeIdx != 2 {
		t.Errorf("expected type index 2, got %d", imp.ExternDesc.TypeIdx)
	}
}
