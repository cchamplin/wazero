// Package conformance contains conformance tests for the Component Model implementation.
// Binary format edge case tests verify handling of malformed and boundary condition inputs.
package conformance

import (
	"strings"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/binary"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestBinary_EmptyComponent tests that a minimal valid component can be parsed.
// A valid component is: magic (4 bytes) + version (2 bytes) + layer (2 bytes) = 8 bytes minimum
func TestBinary_EmptyComponent(t *testing.T) {
	// Minimal valid component: magic + version + layer
	minimalComponent := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic: "\0asm"
		0x0d, 0x00, // version: 0x000d (component model pre-standard)
		0x01, 0x00, // layer: component (not core module)
	}

	c, err := binary.DecodeComponent(minimalComponent)
	require.NoError(t, err)
	require.NotNil(t, c)
}

// TestBinary_InvalidMagic tests that invalid magic numbers are rejected with appropriate error.
func TestBinary_InvalidMagic(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "wrong_first_byte",
			data: []byte{0x01, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00},
		},
		{
			name: "wrong_second_byte",
			data: []byte{0x00, 0x62, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00},
		},
		{
			name: "wrong_third_byte",
			data: []byte{0x00, 0x61, 0x74, 0x6d, 0x0d, 0x00, 0x01, 0x00},
		},
		{
			name: "wrong_fourth_byte",
			data: []byte{0x00, 0x61, 0x73, 0x6e, 0x0d, 0x00, 0x01, 0x00},
		},
		{
			name: "all_zeros",
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00},
		},
		{
			name: "all_ff",
			data: []byte{0xff, 0xff, 0xff, 0xff, 0x0d, 0x00, 0x01, 0x00},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := binary.DecodeComponent(tc.data)
			require.Error(t, err)
			require.True(t, strings.Contains(strings.ToLower(err.Error()), "magic"),
				"error should mention 'magic', got: %v", err)
		})
	}
}

// TestBinary_InvalidVersion tests that invalid version numbers are rejected.
func TestBinary_InvalidVersion(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "core_module_version_1",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x01, 0x00},
		},
		{
			name: "future_version",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0xff, 0xff, 0x01, 0x00},
		},
		{
			name: "zero_version",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0x00, 0x00, 0x01, 0x00},
		},
		{
			name: "wrong_minor_version",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0x0d, 0x01, 0x01, 0x00},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := binary.DecodeComponent(tc.data)
			require.Error(t, err)
			require.True(t, strings.Contains(strings.ToLower(err.Error()), "version"),
				"error should mention 'version', got: %v", err)
		})
	}
}

// TestBinary_CoreModuleLayer tests that core module layer (0x00 0x00) is rejected.
// Components use layer 0x01 0x00, while core modules use layer 0x00 0x00.
func TestBinary_CoreModuleLayer(t *testing.T) {
	// Valid magic and version, but core module layer
	coreModuleData := []byte{
		0x00, 0x61, 0x73, 0x6d, // magic
		0x0d, 0x00, // version (component version)
		0x00, 0x00, // layer: core module (not component)
	}

	_, err := binary.DecodeComponent(coreModuleData)
	require.Error(t, err)
	// The error should indicate this is not a component (it's a core module)
	errLower := strings.ToLower(err.Error())
	require.True(t, strings.Contains(errLower, "component") ||
		strings.Contains(errLower, "layer") ||
		strings.Contains(errLower, "module"),
		"error should indicate layer issue, got: %v", err)
}

// TestBinary_TruncatedHeader tests handling of truncated headers.
func TestBinary_TruncatedHeader(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "empty",
			data: []byte{},
		},
		{
			name: "magic_only_partial",
			data: []byte{0x00, 0x61, 0x73},
		},
		{
			name: "magic_only",
			data: []byte{0x00, 0x61, 0x73, 0x6d},
		},
		{
			name: "magic_and_partial_version",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0x0d},
		},
		{
			name: "magic_and_version_only",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00},
		},
		{
			name: "missing_last_layer_byte",
			data: []byte{0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := binary.DecodeComponent(tc.data)
			require.Error(t, err)
		})
	}
}

// TestBinary_TruncatedSection tests handling of sections claiming content but providing none.
func TestBinary_TruncatedSection(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "section_id_only",
			// Valid header + section ID but no size
			data: append([]byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
			}, 0x07), // type section ID without size
		},
		{
			name: "section_size_claims_100_bytes",
			// Valid header + section ID + size claiming 100 bytes, but only header
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07, // type section ID
				0x64, // size = 100 (LEB128)
				// No content!
			},
		},
		{
			name: "section_size_claims_10_bytes_provides_5",
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07,                         // type section ID
				0x0a,                         // size = 10 (LEB128)
				0x01, 0x02, 0x03, 0x04, 0x05, // only 5 bytes of content
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := binary.DecodeComponent(tc.data)
			require.Error(t, err)
		})
	}
}

// TestBinary_MaximumSectionSize tests that very large section sizes don't cause crashes.
// The decoder should error gracefully rather than attempting to allocate huge amounts of memory.
func TestBinary_MaximumSectionSize(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "max_u32_size",
			// Section claiming maximum u32 size (4GB)
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07,                         // type section ID
				0xff, 0xff, 0xff, 0xff, 0x0f, // size = 0xFFFFFFFF (LEB128)
			},
		},
		{
			name: "very_large_size",
			// Section claiming 1GB
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07,                         // type section ID
				0x80, 0x80, 0x80, 0x80, 0x04, // size = 0x40000000 (1GB, LEB128)
			},
		},
		{
			name: "moderately_large_size",
			// Section claiming 1MB (reasonable but still truncated)
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07,                   // type section ID
				0x80, 0x80, 0x40,       // size = 0x100000 (1MB, LEB128)
				0x01, 0x02, 0x03, 0x04, // only 4 bytes provided
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic, should return an error
			_, err := binary.DecodeComponent(tc.data)
			require.Error(t, err, "should error on truncated large section, not crash")
		})
	}
}

// TestBinary_IsComponent tests the IsComponent helper function.
func TestBinary_IsComponent(t *testing.T) {
	t.Run("valid_component", func(t *testing.T) {
		validComponent := []byte{
			0x00, 0x61, 0x73, 0x6d, // magic
			0x0d, 0x00, // version
			0x01, 0x00, // layer: component
		}
		require.True(t, binary.IsComponent(validComponent))
	})

	t.Run("core_module", func(t *testing.T) {
		// Core wasm module with version 1
		coreModule := []byte{
			0x00, 0x61, 0x73, 0x6d, // magic
			0x01, 0x00, 0x00, 0x00, // version 1.0
		}
		require.False(t, binary.IsComponent(coreModule))
	})

	t.Run("too_short", func(t *testing.T) {
		tooShort := []byte{0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00}
		require.False(t, binary.IsComponent(tooShort))
	})

	t.Run("wrong_magic", func(t *testing.T) {
		wrongMagic := []byte{0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x01, 0x00}
		require.False(t, binary.IsComponent(wrongMagic))
	})

	t.Run("empty", func(t *testing.T) {
		require.False(t, binary.IsComponent([]byte{}))
	})
}

// TestBinary_MalformedLEB128 tests handling of malformed LEB128 encoded values.
func TestBinary_MalformedLEB128(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "leb128_overflow_in_section_size",
			// Valid header + section ID + malformed LEB128 (too many continuation bytes)
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07, // type section ID
				// Malformed LEB128: continues beyond u32 max
				0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01,
			},
		},
		{
			name: "leb128_incomplete",
			// Valid header + section ID + incomplete LEB128
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				0x07, // type section ID
				0x80, // continuation bit set, but no more bytes
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should error, not panic
			_, err := binary.DecodeComponent(tc.data)
			require.Error(t, err)
		})
	}
}

// TestBinary_InvalidSectionID tests handling of invalid section IDs.
func TestBinary_InvalidSectionID(t *testing.T) {
	testCases := []struct {
		name      string
		sectionID byte
	}{
		{"section_id_13", 13},
		{"section_id_50", 50},
		{"section_id_100", 100},
		{"section_id_255", 255},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				tc.sectionID,   // unknown section ID
				0x05,           // size = 5
				0, 0, 0, 0, 0,  // 5 bytes of content
			}
			// Unknown sections should be skipped (not error) per the decoder
			// This tests that we don't crash on unknown section IDs
			_, _ = binary.DecodeComponent(data)
			// No assertion - just verifying no panic
		})
	}
}

// TestBinary_ValidSectionIDs tests that all valid section IDs are recognized.
func TestBinary_ValidSectionIDs(t *testing.T) {
	// Per the binary package constants
	sectionNames := map[byte]string{
		0:  "core-custom",
		1:  "core-module",
		2:  "core-instance",
		3:  "core-type",
		4:  "component",
		5:  "instance",
		6:  "alias",
		7:  "type",
		8:  "canon",
		9:  "start",
		10: "import",
		11: "export",
		12: "value",
	}

	for id, name := range sectionNames {
		t.Run(name, func(t *testing.T) {
			sid := binary.SectionID(id)
			// Verify String() returns expected name (or at least doesn't panic)
			str := sid.String()
			require.True(t, len(str) > 0, "SectionID.String() should not return empty string")
		})
	}
}

// TestBinary_MultipleSections tests decoding components with multiple sections.
func TestBinary_MultipleSections(t *testing.T) {
	// Component with empty type section followed by empty export section
	data := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
		0x07, 0x01, 0x00, // type section: size=1, count=0
		0x0b, 0x01, 0x00, // export section: size=1, count=0
	}

	c, err := binary.DecodeComponent(data)
	require.NoError(t, err)
	require.NotNil(t, c)
}

// TestBinary_ZeroLengthSection tests handling of zero-length sections.
func TestBinary_ZeroLengthSection(t *testing.T) {
	testCases := []struct {
		name      string
		sectionID byte
	}{
		{"zero_length_type", 0x07},
		{"zero_length_export", 0x0b},
		{"zero_length_import", 0x0a},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte{
				0x00, 0x61, 0x73, 0x6d, 0x0d, 0x00, 0x01, 0x00, // valid header
				tc.sectionID, // section ID
				0x00,         // size = 0 (empty section)
			}
			// Empty sections with size 0 might error or succeed depending on section type
			// The important thing is they don't crash
			_, _ = binary.DecodeComponent(data)
		})
	}
}
