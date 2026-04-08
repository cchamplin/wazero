// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/testdata"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestDecodeComponent_Preamble(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedErr error
	}{
		{
			name:        "empty",
			input:       []byte{},
			expectedErr: ErrInvalidMagic,
		},
		{
			name:        "magic only",
			input:       Magic[:],
			expectedErr: ErrUnexpectedEOF,
		},
		{
			name:        "wrong magic",
			input:       []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00},
			expectedErr: ErrInvalidMagic,
		},
		{
			name:        "wrong version",
			input:       append(Magic[:], 0x01, 0x00, 0x00, 0x00, 0x01, 0x00),
			expectedErr: ErrInvalidVersion,
		},
		{
			name:        "core module layer",
			input:       append(append(Magic[:], Version[:]...), LayerModule[:]...),
			expectedErr: ErrInvalidLayer,
		},
		{
			name:        "valid empty component",
			input:       append(append(Magic[:], Version[:]...), LayerComponent[:]...),
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeComponent(tc.input)
			if tc.expectedErr != nil {
				require.Equal(t, tc.expectedErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDecodeComponent_EmptyComponent(t *testing.T) {
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestDecodeComponent_EmptyFixture(t *testing.T) {
	// Use the embedded test fixture
	c, err := DecodeComponent(testdata.EmptyComponent)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestDecodeComponent_SectionHeader(t *testing.T) {
	// Component with one empty core-custom section
	// magic + version + layer + section(id=0, size=0)
	input := append(
		append(append(Magic[:], Version[:]...), LayerComponent[:]...),
		0x00, // section ID: core-custom
		0x00, // section size: 0 (LEB128)
	)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestDecodeComponent_CoreModule(t *testing.T) {
	// Build a component with an embedded minimal core module
	// Component preamble: magic(4) + version(2) + layer(2) = 8 bytes
	// Section: id(1) + size(LEB128) + content

	// Minimal valid core module: magic + version = 8 bytes
	coreModule := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x01, 0x00, 0x00, 0x00, // version
	}

	// Build component
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDCoreModule)) // section ID = 1
	input = append(input, byte(len(coreModule)))     // section size (fits in 1 byte)
	input = append(input, coreModule...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.CoreModules))
	require.NotNil(t, c.CoreModules[0])
}

func TestDecodeComponent_TypeSection(t *testing.T) {
	// Build a component with a type section containing one function type
	// Function type: (func (param "a" s32) (param "b" s32) (result s32))

	typeSection := []byte{
		0x01,      // 1 type definition
		0x40,      // sync functype
		0x02,      // 2 params
		0x01, 'a', // param name "a" (length 1)
		0x7a,      // s32
		0x01, 'b', // param name "b" (length 1)
		0x7a, // s32
		0x00, // single result
		0x7a, // s32
	}

	// Build component
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDType))    // section ID = 7
	input = append(input, byte(len(typeSection))) // section size
	input = append(input, typeSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Types)

	// The function type is interned into c.Types.Funcs[0]. It has a
	// two-element parameter tuple (s32, s32) and a single-element result
	// tuple (s32). The param names are preserved on the TypeFunc.
	ct := c.Types
	require.Equal(t, 1, len(ct.Funcs))
	fn := ct.Funcs[0]
	require.Equal(t, []string{"a", "b"}, fn.ParamNames)
	require.Equal(t, types.TypeKindTuple, fn.Params.Kind)
	paramTuple := ct.Tuples[fn.Params.Index]
	require.Equal(t, 2, len(paramTuple.Types))
	require.Equal(t, types.S32, paramTuple.Types[0])
	require.Equal(t, types.S32, paramTuple.Types[1])
	require.Equal(t, types.TypeKindTuple, fn.Results.Kind)
	resultTuple := ct.Tuples[fn.Results.Index]
	require.Equal(t, 1, len(resultTuple.Types))
	require.Equal(t, types.S32, resultTuple.Types[0])
}

func TestDecodeComponent_CanonSection(t *testing.T) {
	// Build a component with a canon section containing one lift
	canonSection := []byte{
		0x01, // 1 canonical definition
		0x00, // canon.lift
		0x00, // core sort
		0x00, // core:funcidx = 0
		0x00, // opts count = 0
		0x00, // typeidx = 0
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDCanon))
	input = append(input, byte(len(canonSection)))
	input = append(input, canonSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.Canonicals))
	require.Equal(t, component.CanonKindLift, c.Canonicals[0].Kind)
}

func TestDecodeComponent_ExportSection(t *testing.T) {
	exportSection := []byte{
		0x01,                // 1 export
		0x00,                // simple name
		0x03, 'a', 'd', 'd', // name "add"
		0x01, // sort = func
		0x00, // index = 0
		0x00, // no externdesc
	}

	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDExport))
	input = append(input, byte(len(exportSection)))
	input = append(input, exportSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.Exports))
	require.Equal(t, "add", c.Exports[0].Name)
	require.Equal(t, component.ExportKindFunc, c.Exports[0].Kind)
}

func TestDecodeTypeSection_WithResource(t *testing.T) {
	// Build a component with a type section containing a resource type.
	// Resource type format: 0x3f (opcode) + 0x7f (rep type i32) + dtor_flag [dtor_idx]
	//
	// The decoder interns each resource declaration as an Abstract
	// TypeResourceTable entry via the builder; destructor and callback
	// metadata remain on the local ResourceTypeDef returned by
	// decodeResourceDecl (discarded at the DecodeComponent boundary in
	// Session 0).
	tests := []struct {
		name        string
		typeSection []byte
	}{
		{
			name: "resource without destructor",
			typeSection: []byte{
				0x01, // 1 type definition
				0x3f, // resource type opcode
				0x7f, // rep type i32
				0x00, // no destructor
			},
		},
		{
			name: "resource with destructor",
			typeSection: []byte{
				0x01, // 1 type definition
				0x3f, // resource type opcode
				0x7f, // rep type i32
				0x01, // has destructor
				0x05, // destructor at func index 5
			},
		},
		{
			name: "resource with large destructor index",
			typeSection: []byte{
				0x01,       // 1 type definition
				0x3f,       // resource type opcode
				0x7f,       // rep type i32
				0x01,       // has destructor
				0x80, 0x01, // destructor at func index 128 (LEB128)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			// Build component
			input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
			input = append(input, byte(SectionIDType))       // section ID = 7
			input = append(input, byte(len(tc.typeSection))) // section size
			input = append(input, tc.typeSection...)

			c, err := DecodeComponent(input)
			require.NoError(t, err)
			require.NotNil(t, c)
			require.NotNil(t, c.Types)
			require.Equal(t, 1, len(c.Types.ResourceTables))
			require.False(t, c.Types.ResourceTables[0].Concrete)
		})
	}
}

func TestDecodeTypeSection_WithResourceAndFunc(t *testing.T) {
	// Test a type section with both a resource and a function type.
	// The resource declaration interns an Abstract TypeResourceTable
	// entry at scope slot 0; the function signature's own<0> reference
	// resolves to that entry via the scope lookup.
	typeSection := []byte{
		0x02, // 2 type definitions

		// Type 0: resource without destructor
		0x3f, // resource type opcode
		0x7f, // rep type i32
		0x00, // no destructor

		// Type 1: function (param "handle" own<0>) -> ()
		0x40,                               // sync functype
		0x01,                               // 1 param
		0x06, 'h', 'a', 'n', 'd', 'l', 'e', // param name "handle"
		0x69, 0x00, // own<type_0> (own handle to resource at index 0)
		0x01, // named results (vec)
		0x00, // 0 results
	}

	// Build component
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDType))    // section ID = 7
	input = append(input, byte(len(typeSection))) // section size
	input = append(input, typeSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Types)
	ct := c.Types

	// Verify resource table entry.
	require.Equal(t, 1, len(ct.ResourceTables))
	require.False(t, ct.ResourceTables[0].Concrete)

	// Verify the function signature resolved the own<> handle.
	require.Equal(t, 1, len(ct.Funcs))
	fn := ct.Funcs[0]
	require.Equal(t, []string{"handle"}, fn.ParamNames)
	paramTuple := ct.Tuples[fn.Params.Index]
	require.Equal(t, 1, len(paramTuple.Types))
	require.Equal(t, types.TypeKindOwn, paramTuple.Types[0].Kind)
	require.Equal(t, uint32(0), paramTuple.Types[0].Index)
}

func TestDecodeComponent_AddS32Fixture(t *testing.T) {
	// Use the embedded add_s32 test fixture.
	c, err := DecodeComponent(testdata.AddS32Component)
	require.NoError(t, err)
	require.NotNil(t, c)

	// Verify core module was parsed.
	require.Equal(t, 1, len(c.CoreModules))
	require.NotNil(t, c.CoreModules[0])

	// Verify type section.
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Funcs))
	fn := c.Types.Funcs[0]
	require.Equal(t, []string{"a", "b"}, fn.ParamNames)

	// Verify canon section.
	require.Equal(t, 1, len(c.Canonicals))
	require.Equal(t, component.CanonKindLift, c.Canonicals[0].Kind)

	// Verify export section.
	require.Equal(t, 1, len(c.Exports))
	require.Equal(t, "add", c.Exports[0].Name)
	require.Equal(t, component.ExportKindFunc, c.Exports[0].Kind)
}

func TestDecodeComponent_VariantType(t *testing.T) {
	// variant { a, b(s32) }
	data := buildComponentWithTypeSection([]byte{
		0x71,      // variant opcode
		0x02,      // 2 cases
		0x01, 'a', // case "a"
		0x00,      // no payload
		0x00,      // no refines
		0x01, 'b', // case "b"
		0x01, 0x7a, // has payload: s32
		0x00, // no refines
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Variants))
	v := c.Types.Variants[0]
	require.Equal(t, 2, len(v.Cases))
	require.Equal(t, "a", v.Cases[0].Name)
	require.False(t, v.Cases[0].HasPayload)
	require.Equal(t, "b", v.Cases[1].Name)
	require.True(t, v.Cases[1].HasPayload)
	require.Equal(t, types.S32, v.Cases[1].Payload)
}

func TestDecodeComponent_TupleType(t *testing.T) {
	// tuple<s32, s32>
	data := buildComponentWithTypeSection([]byte{
		0x6f, // tuple opcode
		0x02, // 2 elements
		0x7a, // s32
		0x7a, // s32
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Tuples))
	tup := c.Types.Tuples[0]
	require.Equal(t, 2, len(tup.Types))
	require.Equal(t, types.S32, tup.Types[0])
	require.Equal(t, types.S32, tup.Types[1])
}

func TestDecodeComponent_FlagsType(t *testing.T) {
	// flags { read, write }
	data := buildComponentWithTypeSection([]byte{
		0x6e, // flags opcode
		0x02, // 2 flags
		0x04, 'r', 'e', 'a', 'd',
		0x05, 'w', 'r', 'i', 't', 'e',
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Flags))
	require.Equal(t, []string{"read", "write"}, c.Types.Flags[0].Names)
}

func TestDecodeComponent_EnumType(t *testing.T) {
	// enum { red, green, blue }
	data := buildComponentWithTypeSection([]byte{
		0x6d, // enum opcode
		0x03, // 3 cases
		0x03, 'r', 'e', 'd',
		0x05, 'g', 'r', 'e', 'e', 'n',
		0x04, 'b', 'l', 'u', 'e',
	})

	c, err := DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c.Types)
	require.Equal(t, 1, len(c.Types.Enums))
	require.Equal(t, []string{"red", "green", "blue"}, c.Types.Enums[0].Names)
}

// Helper to build a minimal component binary with a type section
func buildComponentWithTypeSection(typeData []byte) []byte {
	typeSectionContent := append([]byte{0x01}, typeData...)
	typeSectionSize := len(typeSectionContent)

	result := make([]byte, 0, 20+len(typeSectionContent))
	result = append(result, Magic[:]...)
	result = append(result, Version[:]...)
	result = append(result, LayerComponent[:]...)
	result = append(result, byte(SectionIDType))
	result = append(result, byte(typeSectionSize))
	result = append(result, typeSectionContent...)

	return result
}
