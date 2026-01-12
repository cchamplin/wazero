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
