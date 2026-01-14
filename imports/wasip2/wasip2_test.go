package wasip2

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify io interfaces are registered
	_, err = linker.MatchImport("wasi:io/error@0.2.0")
	require.NoError(t, err)
}

func TestInstantiate_IOInterfaces(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify io interfaces
	_, err = linker.MatchImport("wasi:io/error@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:io/poll@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:io/streams@0.2.0")
	require.NoError(t, err)
}

func TestInstantiate_ClocksInterfaces(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify clocks interfaces
	_, err = linker.MatchImport("wasi:clocks/wall-clock@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:clocks/monotonic-clock@0.2.0")
	require.NoError(t, err)
}
