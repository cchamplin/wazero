// internal/component/binary/import_test.go

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeImportName_Plain(t *testing.T) {
	// 0x00 prefix = plain name without version suffix
	data := []byte{
		0x00,                           // plain name
		0x08,                           // length
		't', 'e', 's', 't', '-', 'a', 'p', 'i',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test-api", name)
}

func TestDecodeImportName_WithVersion(t *testing.T) {
	// 0x01 prefix = name with version suffix
	data := []byte{
		0x01,                           // with version suffix
		0x0a,                           // length = 10
		't', 'e', 's', 't', '@', '1', '.', '2', '.', '3',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test@1.2.3", name)
}

func TestDecodeExternDesc_Func(t *testing.T) {
	// 0x01 = func, then type index
	data := []byte{0x01, 0x05}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescFunc, desc.Kind)
	require.Equal(t, uint32(5), desc.TypeIdx)
}

func TestDecodeExternDesc_CoreModule(t *testing.T) {
	// 0x00 0x11 = core module, then core type index
	data := []byte{0x00, 0x11, 0x02}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescCoreModule, desc.Kind)
	require.Equal(t, uint32(2), desc.CoreTypeIdx)
}

func TestDecodeExternDesc_Instance(t *testing.T) {
	// 0x05 = instance, then type index
	data := []byte{0x05, 0x03}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescInstance, desc.Kind)
	require.Equal(t, uint32(3), desc.TypeIdx)
}

func TestDecodeExternDesc_Component(t *testing.T) {
	// 0x04 = component, then type index
	data := []byte{0x04, 0x07}

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescComponent, desc.Kind)
	require.Equal(t, uint32(7), desc.TypeIdx)
}

func TestDecodeImportSection(t *testing.T) {
	// Section with 2 imports
	data := []byte{
		0x02,                                // count: 2
		// Import 1: func
		0x00, 0x04, 't', 'e', 's', 't',      // name (plain)
		0x01, 0x00,                          // func type 0
		// Import 2: instance
		0x00, 0x05, 'o', 't', 'h', 'e', 'r', // name (plain)
		0x05, 0x01,                          // instance type 1
	}

	c := &component.Component{}
	err := decodeImportSection(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Imports))
	require.Equal(t, "test", c.Imports[0].Name)
	require.Equal(t, component.ImportExternDescFunc, c.Imports[0].ExternDesc.Kind)
	require.Equal(t, "other", c.Imports[1].Name)
	require.Equal(t, component.ImportExternDescInstance, c.Imports[1].ExternDesc.Kind)
}

// TestDecodeExternDesc_Value tests value imports with valuebound.
func TestDecodeExternDesc_Value(t *testing.T) {
	// 0x02 = value import, followed by valtype (type index 0 in LEB128)
	data := []byte{0x02, 0x00} // kind=value, valtype=type index 0

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescValue, desc.Kind)
	require.NotNil(t, desc.ValType)
	require.False(t, desc.ValType.IsPrimitive)
	require.Equal(t, uint32(0), desc.ValType.TypeIdx)
}

// TestDecodeExternDesc_ValuePrimitive tests value imports with primitive types.
func TestDecodeExternDesc_ValuePrimitive(t *testing.T) {
	// 0x02 = value import, followed by primitive valtype (0x7f = i32)
	data := []byte{0x02, 0x7f} // kind=value, valtype=i32

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescValue, desc.Kind)
	require.NotNil(t, desc.ValType)
	require.True(t, desc.ValType.IsPrimitive)
	require.Equal(t, byte(0x7f), desc.ValType.Primitive)
}

// TestDecodeExternDesc_Type tests type imports with typebound.
func TestDecodeExternDesc_Type(t *testing.T) {
	// 0x03 = type import, 0x00 = sub bound, 0x05 = type index 5
	data := []byte{0x03, 0x00, 0x05} // kind=type, bound=sub, typeIdx=5

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescType, desc.Kind)
	require.Equal(t, component.TypeBoundSub, desc.TypeBoundKind)
	require.NotNil(t, desc.TypeBoundIdx)
	require.Equal(t, uint32(5), *desc.TypeBoundIdx)
}

// TestDecodeExternDesc_TypeEq tests type imports with eq bound.
func TestDecodeExternDesc_TypeEq(t *testing.T) {
	// 0x03 = type import, 0x01 = eq bound, 0x0A = type index 10
	data := []byte{0x03, 0x01, 0x0A} // kind=type, bound=eq, typeIdx=10

	desc, err := decodeExternDesc(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescType, desc.Kind)
	require.Equal(t, component.TypeBoundEq, desc.TypeBoundKind)
	require.NotNil(t, desc.TypeBoundIdx)
	require.Equal(t, uint32(10), *desc.TypeBoundIdx)
}

// TestDecodeExternDesc_UnknownKind tests that unknown import kinds return errors.
func TestDecodeExternDesc_UnknownKind(t *testing.T) {
	tests := []struct {
		name string
		kind byte
	}{
		{"kind_0x06", 0x06},
		{"kind_0x10", 0x10},
		{"kind_0xFF", 0xFF},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte{tc.kind}
			_, err := decodeExternDesc(bytes.NewReader(data))
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown externdesc kind")
		})
	}
}

// TestDecodeImportName_UnknownPrefix tests that unknown name prefixes return errors.
func TestDecodeImportName_UnknownPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix byte
	}{
		{"prefix_0x02", 0x02},
		{"prefix_0x10", 0x10},
		{"prefix_0xFF", 0xFF},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte{tc.prefix, 0x04, 't', 'e', 's', 't'}
			_, err := decodeImportName(bytes.NewReader(data))
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown import name prefix")
		})
	}
}

// TestDecodeExternDesc_CoreModule_BadPrefix tests core module with wrong prefix.
func TestDecodeExternDesc_CoreModule_BadPrefix(t *testing.T) {
	// Core module expects 0x00 0x11, not 0x00 0x12
	data := []byte{0x00, 0x12, 0x05}

	_, err := decodeExternDesc(bytes.NewReader(data))
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected 0x11 for core module")
}

// TestDecodeImportWithTypeBound tests decoding type imports with type bounds.
func TestDecodeImportWithTypeBound(t *testing.T) {
	importData := []byte{
		0x01,                                     // count = 1
		0x00,                                     // plain name prefix
		0x06, 't', 'e', 's', 't', '/', 'x',       // length=6, name="test/x"
		0x03,                                     // extern desc = type
		0x00,                                     // subtype bound (TypeBoundSub)
		0x00,                                     // type index 0
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(c.Imports))
	}

	if c.Imports[0].ExternDesc.Kind != component.ImportExternDescType {
		t.Errorf("expected type import kind, got %v", c.Imports[0].ExternDesc.Kind)
	}

	if c.Imports[0].ExternDesc.TypeBoundKind != component.TypeBoundSub {
		t.Errorf("expected subtype bound, got %v", c.Imports[0].ExternDesc.TypeBoundKind)
	}

	if c.Imports[0].ExternDesc.TypeBoundIdx == nil {
		t.Fatalf("expected type bound index to be set")
	}

	if *c.Imports[0].ExternDesc.TypeBoundIdx != 0 {
		t.Errorf("expected type bound index 0, got %d", *c.Imports[0].ExternDesc.TypeBoundIdx)
	}
}

// TestDecodeImportWithTypeBoundEq tests decoding type imports with eq bounds.
func TestDecodeImportWithTypeBoundEq(t *testing.T) {
	importData := []byte{
		0x01,                                     // count = 1
		0x00,                                     // plain name prefix
		0x04, 't', 'e', 's', 't',                 // length=4, name="test"
		0x03,                                     // extern desc = type
		0x01,                                     // eq bound (TypeBoundEq)
		0x05,                                     // type index 5
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(c.Imports))
	}

	if c.Imports[0].ExternDesc.TypeBoundKind != component.TypeBoundEq {
		t.Errorf("expected eq bound, got %v", c.Imports[0].ExternDesc.TypeBoundKind)
	}

	if c.Imports[0].ExternDesc.TypeBoundIdx == nil {
		t.Fatalf("expected type bound index to be set")
	}

	if *c.Imports[0].ExternDesc.TypeBoundIdx != 5 {
		t.Errorf("expected type bound index 5, got %d", *c.Imports[0].ExternDesc.TypeBoundIdx)
	}
}

// TestDecodeImportWithValueBound tests decoding value imports with value bounds.
func TestDecodeImportWithValueBound(t *testing.T) {
	importData := []byte{
		0x01,                                     // count = 1
		0x00,                                     // plain name prefix
		0x05, 'v', 'a', 'l', 'u', 'e',            // length=5, name="value"
		0x02,                                     // extern desc = value
		0x7f,                                     // valtype = i32 (primitive)
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(c.Imports))
	}

	if c.Imports[0].ExternDesc.Kind != component.ImportExternDescValue {
		t.Errorf("expected value import kind, got %v", c.Imports[0].ExternDesc.Kind)
	}

	if c.Imports[0].ExternDesc.ValType == nil {
		t.Fatalf("expected value type to be set")
	}

	if !c.Imports[0].ExternDesc.ValType.IsPrimitive {
		t.Errorf("expected primitive value type")
	}

	if c.Imports[0].ExternDesc.ValType.Primitive != 0x7f {
		t.Errorf("expected i32 primitive (0x7f), got 0x%02x", c.Imports[0].ExternDesc.ValType.Primitive)
	}
}
