// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"bytes"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeImportName_Plain(t *testing.T) {
	// 0x00 prefix = plain name without version suffix
	data := []byte{
		0x00, // plain name
		0x08, // length
		't', 'e', 's', 't', '-', 'a', 'p', 'i',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test-api", name)
}

func TestDecodeImportName_WithVersion(t *testing.T) {
	// 0x01 prefix = name with version suffix
	data := []byte{
		0x01, // with version suffix
		0x0a, // length = 10
		't', 'e', 's', 't', '@', '1', '.', '2', '.', '3',
	}

	name, err := decodeImportName(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "test@1.2.3", name)
}

func TestDecodeExternDesc_Func(t *testing.T) {
	// 0x01 = func, then type index
	data := []byte{0x01, 0x05}

	desc, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescFunc, desc.Kind)
	require.Equal(t, uint32(5), desc.TypeIdx)
}

func TestDecodeExternDesc_CoreModule(t *testing.T) {
	// 0x00 0x11 = core module, then core type index
	data := []byte{0x00, 0x11, 0x02}

	desc, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescCoreModule, desc.Kind)
	require.Equal(t, uint32(2), desc.CoreTypeIdx)
}

func TestDecodeExternDesc_Instance(t *testing.T) {
	// 0x05 = instance, then type index
	data := []byte{0x05, 0x03}

	desc, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescInstance, desc.Kind)
	require.Equal(t, uint32(3), desc.TypeIdx)
}

func TestDecodeExternDesc_Component(t *testing.T) {
	// 0x04 = component, then type index
	data := []byte{0x04, 0x07}

	desc, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescComponent, desc.Kind)
	require.Equal(t, uint32(7), desc.TypeIdx)
}

func TestDecodeImportSection(t *testing.T) {
	// Section with 2 imports
	data := []byte{
		0x02, // count: 2
		// Import 1: func
		0x00, 0x04, 't', 'e', 's', 't', // name (plain)
		0x01, 0x00, // func type 0
		// Import 2: instance
		0x00, 0x05, 'o', 't', 'h', 'e', 'r', // name (plain)
		0x05, 0x01, // instance type 1
	}

	dc := newDecodeContext()
	err := decodeImportSection(dc, bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 2, len(dc.c.Imports))
	require.Equal(t, "test", dc.c.Imports[0].Name)
	require.Equal(t, component.ImportExternDescFunc, dc.c.Imports[0].ExternDesc.Kind)
	require.Equal(t, "other", dc.c.Imports[1].Name)
	require.Equal(t, component.ImportExternDescInstance, dc.c.Imports[1].ExternDesc.Kind)
}

// TestDecodeExternDesc_ValuePrimitive tests value imports with primitive
// types. The value bound is a primitive ValType (i32 => U32 scalar).
func TestDecodeExternDesc_ValuePrimitive(t *testing.T) {
	// 0x02 = value import, followed by primitive valtype
	// (0x79 = u32 in the component model binary format).
	data := []byte{0x02, 0x79}

	desc, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, component.ImportExternDescValue, desc.Kind)
	require.Equal(t, types.U32, desc.ValType)
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
			_, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
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

	_, err := decodeExternDesc(newDecodeContext(), bytes.NewReader(data))
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected 0x11 for core module")
}

// TestDecodeImportTypeSubResource verifies that a type import with a
// (sub resource) bound decodes correctly. The typebound body is a
// single 0x01 byte with NO following typeidx.
//
// Spec: Binary.md:236,240 — externdesc 0x03 is a typebound;
// typebound 0x01 is (sub resource), with NO following typeidx.
// Wasmtime: tests/all/component_model/resources.rs:14 uses
// `(import "t" (type $t (sub resource)))`.
func TestDecodeImportTypeSubResource(t *testing.T) {
	importData := []byte{
		0x01,         // count = 1
		0x00,         // plain name prefix
		0x01, 'r',    // name length 1, "r"
		0x03,         // externdesc kind = 0x03 (type import)
		0x01,         // typebound tag = (sub resource), NO following typeidx
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Imports))
	require.Equal(t, "r", c.Imports[0].Name)
	require.Equal(t, component.ImportExternDescType, c.Imports[0].ExternDesc.Kind)
	require.Equal(t, component.TypeBoundSubResource, c.Imports[0].ExternDesc.TypeBoundKind)
	require.Nil(t, c.Imports[0].ExternDesc.TypeBoundIdx)
}

// TestDecodeImportTypeEq verifies that a type import with an (eq i)
// bound decodes correctly, reading the trailing typeidx.
//
// Spec: Binary.md:239 — typebound 0x00 i:<typeidx> = (eq i).
// Wasmtime: tests/misc_testsuite/component-model/types.wast:327
// uses `(import "a" (type $t2 (eq $t1)))`.
func TestDecodeImportTypeEq(t *testing.T) {
	// Two imports: first (sub resource) at index 0, then (eq 0) at index 1.
	importData := []byte{
		0x02,                              // count = 2
		0x00, 0x01, 'r', 0x03, 0x01,      // import "r" with (sub resource)
		0x00, 0x01, 't', 0x03, 0x00, 0x00, // import "t" with (eq 0)
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Imports))
	require.Equal(t, "t", c.Imports[1].Name)
	require.Equal(t, component.ImportExternDescType, c.Imports[1].ExternDesc.Kind)
	require.Equal(t, component.TypeBoundEq, c.Imports[1].ExternDesc.TypeBoundKind)
	require.NotNil(t, c.Imports[1].ExternDesc.TypeBoundIdx)
	require.Equal(t, uint32(0), *c.Imports[1].ExternDesc.TypeBoundIdx)
}

// TestDecodeImportTypeBothBoundsSideBySide verifies that the decoder
// does not corrupt state across two consecutive type imports, one
// with each bound variant. This mirrors the wasmtime fixture at
// tests/misc_testsuite/component-model/instance.wast:288-325, which
// uses both `(sub resource)` and `(eq $t)` forms in a single component.
//
// Spec: Binary.md:239-240.
func TestDecodeImportTypeBothBoundsSideBySide(t *testing.T) {
	importData := []byte{
		0x02, // count = 2
		// import 1: "r" with (sub resource)
		0x00, 0x01, 'r', 0x03, 0x01,
		// import 2: "t" with (eq 0) — references import 1's type at index 0
		0x00, 0x01, 't', 0x03, 0x00, 0x00,
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.Equal(t, 2, len(c.Imports))

	// Import 1: (sub resource) — no typeidx.
	require.Equal(t, "r", c.Imports[0].Name)
	require.Equal(t, component.ImportExternDescType, c.Imports[0].ExternDesc.Kind)
	require.Equal(t, component.TypeBoundSubResource, c.Imports[0].ExternDesc.TypeBoundKind)
	require.Nil(t, c.Imports[0].ExternDesc.TypeBoundIdx)

	// Import 2: (eq 0) — references the resource at index 0.
	require.Equal(t, "t", c.Imports[1].Name)
	require.Equal(t, component.ImportExternDescType, c.Imports[1].ExternDesc.Kind)
	require.Equal(t, component.TypeBoundEq, c.Imports[1].ExternDesc.TypeBoundKind)
	require.NotNil(t, c.Imports[1].ExternDesc.TypeBoundIdx)
	require.Equal(t, uint32(0), *c.Imports[1].ExternDesc.TypeBoundIdx)
}

// TestDecodeImportWithValueBound tests decoding value imports with
// primitive value bounds. The value type is resolved to a canonical
// types.ValType (TypeKindU32 for 0x79).
func TestDecodeImportWithValueBound(t *testing.T) {
	importData := []byte{
		0x01,                          // count = 1
		0x00,                          // plain name prefix
		0x05, 'v', 'a', 'l', 'u', 'e', // length=5, name="value"
		0x02, // extern desc = value
		0x79, // valtype = u32 primitive
	}

	data := buildComponentWithSection(SectionIDImport, importData)

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Imports))
	require.Equal(t, component.ImportExternDescValue, c.Imports[0].ExternDesc.Kind)
	require.Equal(t, types.U32, c.Imports[0].ExternDesc.ValType)
}
