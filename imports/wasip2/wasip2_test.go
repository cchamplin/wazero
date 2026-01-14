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
