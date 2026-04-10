// imports/wasip2/random/random_test.go

package random

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestGetRandomBytes(t *testing.T) {
	bytes := GetRandomBytes(100)
	require.Equal(t, 100, len(bytes))

	// Should have some non-zero bytes (extremely unlikely all zeros from crypto/rand)
	hasNonZero := false
	for _, b := range bytes {
		if b != 0 {
			hasNonZero = true
			break
		}
	}
	require.True(t, hasNonZero, "random bytes should contain at least one non-zero byte")
}

func TestGetRandomBytes_Cap(t *testing.T) {
	// Request more than cap
	bytes := GetRandomBytes(1024 * 1024)
	require.Equal(t, 64*1024, len(bytes))
}

func TestGetRandomBytes_Zero(t *testing.T) {
	bytes := GetRandomBytes(0)
	require.Equal(t, 0, len(bytes))
}

func TestGetRandomU64(t *testing.T) {
	// Generate several values, should be different (extremely unlikely to be the same)
	values := make(map[uint64]bool)
	for i := 0; i < 10; i++ {
		v := GetRandomU64()
		values[v] = true
	}
	require.True(t, len(values) > 1, "multiple random u64 values should be different")
}

func TestInstantiateRandom(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	err := instantiateRandom(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:random/random@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasGetRandomBytes := instDef.Exports["get-random-bytes"]
	require.True(t, hasGetRandomBytes, "get-random-bytes function should be defined")

	_, hasGetRandomU64 := instDef.Exports["get-random-u64"]
	require.True(t, hasGetRandomU64, "get-random-u64 function should be defined")
}

func TestInstantiateRandom_Duplicate(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	// First registration should succeed
	err := instantiateRandom(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateRandom(linker)
	require.Error(t, err)
}
