// imports/wasip2/random/insecure_test.go

package random

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestGetInsecureRandomBytes(t *testing.T) {
	bytes := GetInsecureRandomBytes(100)
	require.Equal(t, 100, len(bytes))
}

func TestGetInsecureRandomBytes_Cap(t *testing.T) {
	// Request more than cap
	bytes := GetInsecureRandomBytes(1024 * 1024)
	require.Equal(t, 64*1024, len(bytes))
}

func TestGetInsecureRandomBytes_Zero(t *testing.T) {
	bytes := GetInsecureRandomBytes(0)
	require.Equal(t, 0, len(bytes))
}

func TestGetInsecureRandomU64(t *testing.T) {
	values := make(map[uint64]bool)
	for i := 0; i < 10; i++ {
		v := GetInsecureRandomU64()
		values[v] = true
	}
	require.True(t, len(values) > 1, "multiple random u64 values should be different")
}

func TestInsecureSeed_Deterministic(t *testing.T) {
	// Per spec: insecure_seed returns same value within instance
	s1a, s1b := InsecureSeed()
	s2a, s2b := InsecureSeed()
	require.Equal(t, s1a, s2a, "seed first value should be consistent")
	require.Equal(t, s1b, s2b, "seed second value should be consistent")
}

func TestInstantiateInsecure(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateInsecure(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:random/insecure@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasGetInsecureRandomBytes := instDef.Exports["get-insecure-random-bytes"]
	require.True(t, hasGetInsecureRandomBytes, "get-insecure-random-bytes function should be defined")

	_, hasGetInsecureRandomU64 := instDef.Exports["get-insecure-random-u64"]
	require.True(t, hasGetInsecureRandomU64, "get-insecure-random-u64 function should be defined")
}

func TestInstantiateInsecure_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := instantiateInsecure(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateInsecure(linker)
	require.Error(t, err)
}

func TestInstantiateInsecureSeed(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateInsecureSeed(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:random/insecure-seed@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasInsecureSeed := instDef.Exports["insecure-seed"]
	require.True(t, hasInsecureSeed, "insecure-seed function should be defined")
}

func TestInstantiateInsecureSeed_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := instantiateInsecureSeed(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateInsecureSeed(linker)
	require.Error(t, err)
}
