// Package binary provides constants for WebAssembly Component Model binary format.
package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestComponentMagic(t *testing.T) {
	require.Equal(t, []byte{0x00, 0x61, 0x73, 0x6d}, Magic[:])
}

func TestComponentVersion(t *testing.T) {
	// Pre-standard component version
	require.Equal(t, []byte{0x0d, 0x00}, Version[:])
}

func TestLayerComponent(t *testing.T) {
	require.Equal(t, []byte{0x01, 0x00}, LayerComponent[:])
}

func TestLayerModule(t *testing.T) {
	require.Equal(t, []byte{0x00, 0x00}, LayerModule[:])
}

func TestSectionIDs(t *testing.T) {
	// Verify section IDs match the Component Model spec
	// https://github.com/WebAssembly/component-model/blob/main/design/mvp/Binary.md
	tests := []struct {
		name     string
		id       SectionID
		expected byte
	}{
		{"CoreCustom", SectionIDCoreCustom, 0},
		{"CoreModule", SectionIDCoreModule, 1},
		{"CoreInstance", SectionIDCoreInstance, 2},
		{"CoreType", SectionIDCoreType, 3},
		{"Component", SectionIDComponent, 4},
		{"Instance", SectionIDInstance, 5},
		{"Alias", SectionIDAlias, 6},
		{"Type", SectionIDType, 7},
		{"Canon", SectionIDCanon, 8},
		{"Start", SectionIDStart, 9},
		{"Import", SectionIDImport, 10},
		{"Export", SectionIDExport, 11},
		{"Value", SectionIDValue, 12},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, byte(tc.id))
		})
	}
}

func TestSectionIDName(t *testing.T) {
	require.Equal(t, "core-module", SectionIDCoreModule.String())
	require.Equal(t, "type", SectionIDType.String())
	require.Equal(t, "unknown(255)", SectionID(255).String())
}

func TestIsComponent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "valid component",
			input:    append(append(Magic[:], Version[:]...), LayerComponent[:]...),
			expected: true,
		},
		{
			name:     "core module",
			input:    append(append(Magic[:], 0x01, 0x00, 0x00, 0x00), LayerModule[:]...),
			expected: false,
		},
		{
			name:     "too short",
			input:    Magic[:],
			expected: false,
		},
		{
			name:     "wrong magic",
			input:    []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00},
			expected: false,
		},
		{
			name:     "empty",
			input:    []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			result := IsComponent(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
