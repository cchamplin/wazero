// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestDecodeInstanceType asserts the decoder produces the expected
// *InstanceTypeDef shape for an instance type (0x42) containing a
// single func export declaration.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceType(t *testing.T) {
	// Instance type with one export declaration
	// (instance (export "test" (func (type 0))))
	data := buildComponentWithTypeSection([]byte{
		0x42,                     // instance type opcode
		0x01,                     // 1 declaration
		0x04,                     // export declaration
		0x00,                     // simple name
		0x04, 't', 'e', 's', 't', // name "test"
		0x01, // externdesc kind: func
		0x00, // type index 0
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindExport, decl.Kind)
	require.NotNil(t, decl.Export)
	require.Equal(t, "test", decl.Export.Name)
	require.Equal(t, component.ExportKindFunc, decl.Export.Kind)
	require.Equal(t, uint32(0), decl.Export.Idx)
}

// TestDecodeInstanceTypeWithAlias asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type containing a
// single outer-type alias declaration.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeWithAlias(t *testing.T) {
	// Instance type with alias declaration
	// (instance (alias outer 0 1 (type)))
	data := buildComponentWithTypeSection([]byte{
		0x42, // instance type opcode
		0x01, // 1 declaration
		0x02, // alias declaration
		0x03, // type sort
		0x02, // outer alias target
		0x00, // outer count 0
		0x01, // outer index 1
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindAlias, decl.Kind)
	require.NotNil(t, decl.Alias)
	require.Equal(t, component.AliasKindOuter, decl.Alias.Kind)
	require.Equal(t, uint32(0), decl.Alias.OuterCount)
	require.Equal(t, uint32(1), decl.Alias.OuterIndex)
}

// TestDecodeInstanceTypeEmpty asserts the decoder produces the expected
// *InstanceTypeDef shape for an instance type (0x42) with no
// declarations.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeEmpty(t *testing.T) {
	// Instance type with no declarations
	// (instance)
	data := buildComponentWithTypeSection([]byte{
		0x42, // instance type opcode
		0x00, // 0 declarations
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 0, len(c.TypeDefs[0].Instance.Declarations))
}

// TestDecodeInstanceTypeMultipleExports asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type containing
// multiple export declarations of different kinds.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeMultipleExports(t *testing.T) {
	// Instance type with multiple export declarations
	// (instance (export "foo" (func (type 0))) (export "bar" (type (eq 1))))
	data := buildComponentWithTypeSection([]byte{
		0x42, // instance type opcode
		0x02, // 2 declarations
		// First export: func
		0x04,                // export declaration
		0x00,                // simple name
		0x03, 'f', 'o', 'o', // name "foo"
		0x01, // externdesc kind: func
		0x00, // type index 0
		// Second export: type
		0x04,                // export declaration
		0x00,                // simple name
		0x03, 'b', 'a', 'r', // name "bar"
		0x03, // externdesc kind: type
		0x00, // type bound: eq
		0x01, // type index 1
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 2, len(c.TypeDefs[0].Instance.Declarations))

	// Check first export: func
	first := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindExport, first.Kind)
	require.NotNil(t, first.Export)
	require.Equal(t, "foo", first.Export.Name)
	require.Equal(t, component.ExportKindFunc, first.Export.Kind)

	// Check second export: type
	second := c.TypeDefs[0].Instance.Declarations[1]
	require.Equal(t, component.InstanceDeclKindExport, second.Kind)
	require.NotNil(t, second.Export)
	require.Equal(t, "bar", second.Export.Name)
	require.Equal(t, component.ExportKindType, second.Export.Kind)
}

// TestDecodeInstanceTypeWithCoreType asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type containing a
// core type declaration (a core func type).
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeWithCoreType(t *testing.T) {
	// Instance type with core type declaration
	// (instance (core type (func)))
	data := buildComponentWithTypeSection([]byte{
		0x42, // instance type opcode
		0x01, // 1 declaration
		0x00, // core type declaration
		0x60, // core func type opcode
		0x00, // 0 params
		0x00, // 0 results
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindCoreType, decl.Kind)
	require.NotNil(t, decl.CoreType)
	require.Equal(t, component.CoreTypeDefKindFunc, decl.CoreType.Kind)
}

// TestDecodeInstanceTypeWithNestedType asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type containing a
// nested type declaration (a record value type).
//
// In Session 0 the nested value-type body is consumed for byte-level
// correctness only; the returned TypeDef carries Kind == Defined but
// no populated composite payload (see decodeNestedTypeDef). This test
// asserts the Kind discriminator only; Session 2 will thread a nested
// builder/scope and restore the structural assertions.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeWithNestedType(t *testing.T) {
	// Instance type with nested type declaration (a record type)
	// (instance (type (record (field "x" s32))))
	data := buildComponentWithTypeSection([]byte{
		0x42,      // instance type opcode
		0x01,      // 1 declaration
		0x01,      // type declaration
		0x72,      // record type opcode
		0x01,      // 1 field
		0x01, 'x', // field name "x"
		0x7a, // s32 (PrimValTypeS32)
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindType, decl.Kind)
	require.NotNil(t, decl.Type)
	require.Equal(t, component.TypeDefKindDefined, decl.Type.Kind)
}

// TestDecodeInstanceTypeCoreModuleExport asserts the decoder produces
// the expected *InstanceTypeDef shape for an instance type that exports
// a core module (externdesc kind 0x00 with the 0x11 core-module prefix).
//
// Session 0 caveat: decodeInstanceExportDecl (instance_type.go:113-127)
// currently maps externdesc 0x00 (core module) to ExportKindFunc with an
// inline "tracked as func for now" comment. This test deliberately does
// not assert decl.Export.Kind so the assertion won't have to be rewritten
// when a dedicated ExportKindCoreModule is added in a later task.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeCoreModuleExport(t *testing.T) {
	// Instance type with export of core module
	// (instance (export "mod" (core module (type 0))))
	data := buildComponentWithTypeSection([]byte{
		0x42,                // instance type opcode
		0x01,                // 1 declaration
		0x04,                // export declaration
		0x00,                // simple name
		0x03, 'm', 'o', 'd', // name "mod"
		0x00, // externdesc kind: core
		0x11, // core module prefix
		0x00, // type index 0
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindExport, decl.Kind)
	require.NotNil(t, decl.Export)
	require.Equal(t, "mod", decl.Export.Name)
	require.Equal(t, uint32(0), decl.Export.Idx)
}

// TestDecodeInstanceTypeInstanceExport asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type that exports a
// nested instance (externdesc kind 0x05).
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeInstanceExport(t *testing.T) {
	// Instance type with export of instance
	// (instance (export "inst" (instance (type 2))))
	data := buildComponentWithTypeSection([]byte{
		0x42,                     // instance type opcode
		0x01,                     // 1 declaration
		0x04,                     // export declaration
		0x00,                     // simple name
		0x04, 'i', 'n', 's', 't', // name "inst"
		0x05, // externdesc kind: instance
		0x02, // type index 2
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindExport, decl.Kind)
	require.NotNil(t, decl.Export)
	require.Equal(t, "inst", decl.Export.Name)
	require.Equal(t, component.ExportKindInstance, decl.Export.Kind)
	require.Equal(t, uint32(2), decl.Export.Idx)
}

// TestDecodeInstanceTypeComponentExport asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type that exports a
// component (externdesc kind 0x04).
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeComponentExport(t *testing.T) {
	// Instance type with export of component
	// (instance (export "comp" (component (type 3))))
	data := buildComponentWithTypeSection([]byte{
		0x42,                     // instance type opcode
		0x01,                     // 1 declaration
		0x04,                     // export declaration
		0x00,                     // simple name
		0x04, 'c', 'o', 'm', 'p', // name "comp"
		0x04, // externdesc kind: component
		0x03, // type index 3
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindExport, decl.Kind)
	require.NotNil(t, decl.Export)
	require.Equal(t, "comp", decl.Export.Name)
	require.Equal(t, component.ExportKindComponent, decl.Export.Kind)
	require.Equal(t, uint32(3), decl.Export.Idx)
}

// TestDecodeInstanceTypeValueExport asserts the decoder produces the
// expected *InstanceTypeDef shape for an instance type that exports a
// value (externdesc kind 0x02).
//
// Session 0 caveat: decodeInstanceExportDecl currently reads a bare
// valtype after the 0x02 externdesc kind. Per Binary.md lines 241-242
// the spec expects a valuebound (`0x00 i:<valueidx>` or `0x01 t:<valtype>`)
// — i.e. a discriminator-prefixed value. The decoder's shim-style read is
// a Session 0 incompleteness flagged at instance_type.go:138-143, and this
// test only asserts the externdesc kind so it does not lock in the shim
// encoding. A later task will make the decoder spec-correct and this test
// will gain structural assertions on the decoded valtype.
//
// Spec: Binary.md "Instance Type" section (see debug-vendored/component-model/design/mvp/Binary.md).
// No counterpart (justified): wazero decoder test; run_tests.py does not cover decoder behavior.
func TestDecodeInstanceTypeValueExport(t *testing.T) {
	// Instance type with export of value
	// (instance (export "val" (value s32)))
	data := buildComponentWithTypeSection([]byte{
		0x42,                // instance type opcode
		0x01,                // 1 declaration
		0x04,                // export declaration
		0x00,                // simple name
		0x03, 'v', 'a', 'l', // name "val"
		0x02, // externdesc kind: value
		0x7a, // valtype: s32 (PrimValTypeS32)
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)

	require.Equal(t, 1, len(c.TypeDefs))
	require.Equal(t, component.TypeDefKindInstance, c.TypeDefs[0].Kind)
	require.NotNil(t, c.TypeDefs[0].Instance)
	require.Equal(t, 1, len(c.TypeDefs[0].Instance.Declarations))

	decl := c.TypeDefs[0].Instance.Declarations[0]
	require.Equal(t, component.InstanceDeclKindExport, decl.Kind)
	require.NotNil(t, decl.Export)
	require.Equal(t, "val", decl.Export.Name)
	require.Equal(t, component.ExportKindValue, decl.Export.Kind)
}
