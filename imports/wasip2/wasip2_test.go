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

func TestInstantiate_RandomInterfaces(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify random interfaces
	_, err = linker.MatchImport("wasi:random/random@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:random/insecure@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:random/insecure-seed@0.2.0")
	require.NoError(t, err)
}

func TestInstantiate_CLIInterfaces(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify cli interfaces
	_, err = linker.MatchImport("wasi:cli/environment@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:cli/exit@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:cli/stdin@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:cli/stdout@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:cli/stderr@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:cli/terminal-input@0.2.0")
	require.NoError(t, err)
	_, err = linker.MatchImport("wasi:cli/terminal-output@0.2.0")
	require.NoError(t, err)
}
