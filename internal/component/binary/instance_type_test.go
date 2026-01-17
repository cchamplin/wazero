// internal/component/binary/instance_type_test.go
package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeInstanceType(t *testing.T) {
	// Instance type with one export declaration
	// (instance (export "test" (func (type 0))))
	// Format: 0x42 count [0x04 exportname externdesc]
	// exportname: 0x00 len name
	// externdesc for func: 0x01 typeidx
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x04, 't', 'e', 's', 't', // name "test"
		0x01,       // externdesc kind: func
		0x00,       // type index 0
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(c.Types))
	}

	if c.Types[0].Kind != component.TypeDefKindInstance {
		t.Fatalf("expected instance type, got kind %d", c.Types[0].Kind)
	}

	if c.Types[0].Instance == nil {
		t.Fatal("expected instance type def")
	}

	if len(c.Types[0].Instance.Declarations) != 1 {
		t.Errorf("expected 1 declaration, got %d", len(c.Types[0].Instance.Declarations))
	}
}

func TestDecodeInstanceTypeWithAlias(t *testing.T) {
	// Instance type with alias declaration
	// (instance (alias outer 0 1 (type)))
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
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

	if c.Types[0].Instance.Declarations[0].Kind != component.InstanceDeclKindAlias {
		t.Errorf("expected alias declaration")
	}
}

func TestDecodeInstanceTypeEmpty(t *testing.T) {
	// Instance type with no declarations
	// (instance)
	data := buildComponentWithTypeSection([]byte{
		0x42, // instance type opcode
		0x00, // 0 declarations
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(c.Types))
	}

	if c.Types[0].Kind != component.TypeDefKindInstance {
		t.Fatalf("expected instance type, got kind %d", c.Types[0].Kind)
	}

	if c.Types[0].Instance == nil {
		t.Fatal("expected instance type def")
	}

	if len(c.Types[0].Instance.Declarations) != 0 {
		t.Errorf("expected 0 declarations, got %d", len(c.Types[0].Instance.Declarations))
	}
}

func TestDecodeInstanceTypeMultipleExports(t *testing.T) {
	// Instance type with multiple export declarations
	// (instance (export "foo" (func (type 0))) (export "bar" (type (eq 1))))
	// externdesc for func: 0x01 typeidx
	// externdesc for type: 0x03 bound typeidx (bound 0x01 = eq)
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x02,       // 2 declarations
		// First export: func
		0x04,       // export declaration
		0x00,       // simple name
		0x03, 'f', 'o', 'o', // name "foo"
		0x01,       // externdesc kind: func
		0x00,       // type index 0
		// Second export: type
		0x04,       // export declaration
		0x00,       // simple name
		0x03, 'b', 'a', 'r', // name "bar"
		0x03,       // externdesc kind: type
		0x01,       // type bound: eq
		0x01,       // type index 1
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Types[0].Instance.Declarations) != 2 {
		t.Errorf("expected 2 declarations, got %d", len(c.Types[0].Instance.Declarations))
	}

	// Check first export
	if c.Types[0].Instance.Declarations[0].Kind != component.InstanceDeclKindExport {
		t.Errorf("expected export declaration for first")
	}
	if c.Types[0].Instance.Declarations[0].Export.Name != "foo" {
		t.Errorf("expected name 'foo', got '%s'", c.Types[0].Instance.Declarations[0].Export.Name)
	}
	if c.Types[0].Instance.Declarations[0].Export.Kind != component.ExportKindFunc {
		t.Errorf("expected func kind")
	}

	// Check second export
	if c.Types[0].Instance.Declarations[1].Kind != component.InstanceDeclKindExport {
		t.Errorf("expected export declaration for second")
	}
	if c.Types[0].Instance.Declarations[1].Export.Name != "bar" {
		t.Errorf("expected name 'bar', got '%s'", c.Types[0].Instance.Declarations[1].Export.Name)
	}
	if c.Types[0].Instance.Declarations[1].Export.Kind != component.ExportKindType {
		t.Errorf("expected type kind")
	}
}

func TestDecodeInstanceTypeWithCoreType(t *testing.T) {
	// Instance type with core type declaration
	// (instance (core type (func)))
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
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

	if c.Types[0].Instance.Declarations[0].Kind != component.InstanceDeclKindCoreType {
		t.Errorf("expected core type declaration")
	}
	if c.Types[0].Instance.Declarations[0].CoreType == nil {
		t.Fatal("expected core type")
	}
	if c.Types[0].Instance.Declarations[0].CoreType.Kind != component.CoreTypeDefKindFunc {
		t.Errorf("expected core func type")
	}
}

func TestDecodeInstanceTypeWithNestedType(t *testing.T) {
	// Instance type with nested type declaration (a record type)
	// (instance (type (record (field "x" s32))))
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
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

	if c.Types[0].Instance.Declarations[0].Kind != component.InstanceDeclKindType {
		t.Errorf("expected type declaration")
	}
	if c.Types[0].Instance.Declarations[0].Type == nil {
		t.Fatal("expected nested type")
	}
	if c.Types[0].Instance.Declarations[0].Type.Kind != component.TypeDefKindDefined {
		t.Errorf("expected defined type kind")
	}
	if c.Types[0].Instance.Declarations[0].Type.Record == nil {
		t.Fatal("expected record type def")
	}
}

func TestDecodeInstanceTypeCoreModuleExport(t *testing.T) {
	// Instance type with export of core module
	// (instance (export "mod" (core module (type 0))))
	// externdesc for core module: 0x00 0x11 typeidx
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x03, 'm', 'o', 'd', // name "mod"
		0x00,       // externdesc kind: core
		0x11,       // core module prefix
		0x00,       // type index 0
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	export := c.Types[0].Instance.Declarations[0].Export
	if export.Name != "mod" {
		t.Errorf("expected name 'mod', got '%s'", export.Name)
	}
}

func TestDecodeInstanceTypeInstanceExport(t *testing.T) {
	// Instance type with export of instance
	// (instance (export "inst" (instance (type 2))))
	// externdesc for instance: 0x05 typeidx
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x04, 'i', 'n', 's', 't', // name "inst"
		0x05,       // externdesc kind: instance
		0x02,       // type index 2
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	export := c.Types[0].Instance.Declarations[0].Export
	if export.Name != "inst" {
		t.Errorf("expected name 'inst', got '%s'", export.Name)
	}
	if export.Kind != component.ExportKindInstance {
		t.Errorf("expected instance kind")
	}
	if export.Idx != 2 {
		t.Errorf("expected index 2, got %d", export.Idx)
	}
}

func TestDecodeInstanceTypeComponentExport(t *testing.T) {
	// Instance type with export of component
	// (instance (export "comp" (component (type 3))))
	// externdesc for component: 0x04 typeidx
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x04, 'c', 'o', 'm', 'p', // name "comp"
		0x04,       // externdesc kind: component
		0x03,       // type index 3
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	export := c.Types[0].Instance.Declarations[0].Export
	if export.Name != "comp" {
		t.Errorf("expected name 'comp', got '%s'", export.Name)
	}
	if export.Kind != component.ExportKindComponent {
		t.Errorf("expected component kind")
	}
	if export.Idx != 3 {
		t.Errorf("expected index 3, got %d", export.Idx)
	}
}

func TestDecodeInstanceTypeValueExport(t *testing.T) {
	// Instance type with export of value
	// (instance (export "val" (value s32)))
	// externdesc for value: 0x02 valtype
	data := buildComponentWithTypeSection([]byte{
		0x42,       // instance type opcode
		0x01,       // 1 declaration
		0x04,       // export declaration
		0x00,       // simple name
		0x03, 'v', 'a', 'l', // name "val"
		0x02,       // externdesc kind: value
		0x7a,       // valtype: s32
	})

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	export := c.Types[0].Instance.Declarations[0].Export
	if export.Name != "val" {
		t.Errorf("expected name 'val', got '%s'", export.Name)
	}
	if export.Kind != component.ExportKindValue {
		t.Errorf("expected value kind")
	}
}
