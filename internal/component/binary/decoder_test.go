// internal/component/binary/decoder_test.go

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/testdata"
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
		0x7a,      // s32
		0x00,      // single result
		0x7a,      // s32
	}

	// Build component
	input := append(append(Magic[:], Version[:]...), LayerComponent[:]...)
	input = append(input, byte(SectionIDType))    // section ID = 7
	input = append(input, byte(len(typeSection))) // section size
	input = append(input, typeSection...)

	c, err := DecodeComponent(input)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 1, len(c.Types))
	require.Equal(t, component.TypeDefKindFunc, c.Types[0].Kind)
	require.NotNil(t, c.Types[0].Func)
	require.Equal(t, 2, len(c.Types[0].Func.Params))
	require.Equal(t, "a", c.Types[0].Func.Params[0].Name)
}
