// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 277: WASI Random Conformance Tests.
package conformance

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 277: WASI Random Conformance Tests
// =============================================================================

// TestWASI_Random_GetRandomBytes tests that get-random-bytes returns the
// requested number of bytes.
func TestWASI_Random_GetRandomBytes(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the random interface
	randomDef, ok := linker.Get("wasi:random/random@0.2.0")
	require.True(t, ok, "random interface should be registered")

	instDef, ok := randomDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	getRandomFunc, ok := instDef.Exports["get-random-bytes"]
	require.True(t, ok, "get-random-bytes function should be exported")

	funcDef, ok := getRandomFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Test various sizes
	testSizes := []uint64{0, 1, 8, 16, 32, 64, 128, 256, 1024}

	for _, size := range testSizes {
		t.Run("Size_"+string(rune('0'+size%10)), func(t *testing.T) {
			result, err := funcDef.Callback(ctx, []types.Val{types.ValU64(size)})
			require.NoError(t, err)
			require.Equal(t, 1, len(result), "get-random-bytes should return exactly one value")

			byteList := result[0].List()
			require.Equal(t, int(size), len(byteList), "should return exactly %d bytes, got %d", size, len(byteList))

			// Verify all values are valid u8s (implicitly true since they're in the list)
			for i, v := range byteList {
				b := v.U8()
				_ = b // Just verify it can be read as u8
				if i > 100 {
					break // Don't need to check all bytes
				}
			}
		})
	}
}

// TestWASI_Random_GetRandomBytesNonDeterministic tests that random bytes
// are actually random (different between calls).
func TestWASI_Random_GetRandomBytesNonDeterministic(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	randomDef, _ := linker.Get("wasi:random/random@0.2.0")
	instDef := randomDef.(*component.InstanceDef)
	funcDef := instDef.Exports["get-random-bytes"].(*component.FuncDef)

	// Get two sets of random bytes
	result1, err := funcDef.Callback(ctx, []types.Val{types.ValU64(32)})
	require.NoError(t, err)

	result2, err := funcDef.Callback(ctx, []types.Val{types.ValU64(32)})
	require.NoError(t, err)

	list1 := result1[0].List()
	list2 := result2[0].List()

	// Convert to byte slices for comparison
	bytes1 := make([]byte, len(list1))
	bytes2 := make([]byte, len(list2))
	for i := range list1 {
		bytes1[i] = list1[i].U8()
		bytes2[i] = list2[i].U8()
	}

	// They should be different (statistically near-impossible to be the same)
	different := false
	for i := range bytes1 {
		if bytes1[i] != bytes2[i] {
			different = true
			break
		}
	}
	require.True(t, different, "two random byte sequences should be different")
}

// TestWASI_Random_GetRandomU64 tests that get-random-u64 returns a u64 value.
func TestWASI_Random_GetRandomU64(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the random interface
	randomDef, ok := linker.Get("wasi:random/random@0.2.0")
	require.True(t, ok, "random interface should be registered")

	instDef, ok := randomDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	getRandomU64Func, ok := instDef.Exports["get-random-u64"]
	require.True(t, ok, "get-random-u64 function should be exported")

	funcDef, ok := getRandomU64Func.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call the function
	result, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-random-u64 should return exactly one value")

	// Get the u64 value
	val := result[0].U64()
	_ = val // Value is valid, just verify it can be read
}

// TestWASI_Random_GetRandomU64NonDeterministic tests that random u64 values
// are actually random (different between calls).
func TestWASI_Random_GetRandomU64NonDeterministic(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	randomDef, _ := linker.Get("wasi:random/random@0.2.0")
	instDef := randomDef.(*component.InstanceDef)
	funcDef := instDef.Exports["get-random-u64"].(*component.FuncDef)

	// Get multiple random values
	values := make([]uint64, 10)
	for i := 0; i < 10; i++ {
		result, err := funcDef.Callback(ctx, []types.Val{})
		require.NoError(t, err)
		values[i] = result[0].U64()
	}

	// Check that at least some values are different
	// (statistically impossible for all 10 to be the same)
	allSame := true
	for i := 1; i < len(values); i++ {
		if values[i] != values[0] {
			allSame = false
			break
		}
	}
	require.False(t, allSame, "random u64 values should not all be the same")
}

// TestWASI_Random_Insecure tests the insecure random functions.
func TestWASI_Random_Insecure(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the insecure random interface
	insecureDef, ok := linker.Get("wasi:random/insecure@0.2.0")
	require.True(t, ok, "insecure random interface should be registered")

	instDef, ok := insecureDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Test get-insecure-random-bytes
	t.Run("GetInsecureRandomBytes", func(t *testing.T) {
		getInsecureFunc, ok := instDef.Exports["get-insecure-random-bytes"]
		require.True(t, ok, "get-insecure-random-bytes function should be exported")

		funcDef, ok := getInsecureFunc.(*component.FuncDef)
		require.True(t, ok, "should be a FuncDef")

		result, err := funcDef.Callback(ctx, []types.Val{types.ValU64(16)})
		require.NoError(t, err)
		require.Equal(t, 1, len(result), "get-insecure-random-bytes should return exactly one value")

		byteList := result[0].List()
		require.Equal(t, 16, len(byteList), "should return exactly 16 bytes")
	})

	// Test get-insecure-random-u64
	t.Run("GetInsecureRandomU64", func(t *testing.T) {
		getInsecureU64Func, ok := instDef.Exports["get-insecure-random-u64"]
		require.True(t, ok, "get-insecure-random-u64 function should be exported")

		funcDef, ok := getInsecureU64Func.(*component.FuncDef)
		require.True(t, ok, "should be a FuncDef")

		result, err := funcDef.Callback(ctx, []types.Val{})
		require.NoError(t, err)
		require.Equal(t, 1, len(result), "get-insecure-random-u64 should return exactly one value")

		val := result[0].U64()
		_ = val // Value is valid
	})
}

// TestWASI_Random_InsecureSeed tests the insecure-seed function.
func TestWASI_Random_InsecureSeed(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	ctx := context.Background()

	// Get the insecure-seed interface
	insecureSeedDef, ok := linker.Get("wasi:random/insecure-seed@0.2.0")
	require.True(t, ok, "insecure-seed interface should be registered")

	instDef, ok := insecureSeedDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	insecureSeedFunc, ok := instDef.Exports["insecure-seed"]
	require.True(t, ok, "insecure-seed function should be exported")

	funcDef, ok := insecureSeedFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call insecure-seed
	result, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "insecure-seed should return exactly one value")

	// Result should be a tuple of (u64, u64)
	tuple := result[0].Tuple()
	require.Equal(t, 2, len(tuple), "seed should be a tuple of 2 u64s")

	seed1 := tuple[0].U64()
	seed2 := tuple[1].U64()

	// Per WASI spec, the same seed should be returned within a component instance
	result2, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)

	tuple2 := result2[0].Tuple()
	seed1_2 := tuple2[0].U64()
	seed2_2 := tuple2[1].U64()

	require.Equal(t, seed1, seed1_2, "insecure-seed should return consistent values within an instance")
	require.Equal(t, seed2, seed2_2, "insecure-seed should return consistent values within an instance")
}

// TestWASI_Random_InterfaceRegistration tests that all random interfaces are properly registered.
func TestWASI_Random_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify all random interfaces are registered
	interfaces := []string{
		"wasi:random/random@0.2.0",
		"wasi:random/insecure@0.2.0",
		"wasi:random/insecure-seed@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_Random_AllFunctionsExist verifies all expected functions exist.
func TestWASI_Random_AllFunctionsExist(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Test wasi:random/random@0.2.0
	t.Run("Random", func(t *testing.T) {
		randomDef, ok := linker.Get("wasi:random/random@0.2.0")
		require.True(t, ok, "random interface should be registered")

		instDef, ok := randomDef.(*component.InstanceDef)
		require.True(t, ok, "should be an InstanceDef")

		expectedFunctions := []string{
			"get-random-bytes",
			"get-random-u64",
		}

		for _, fn := range expectedFunctions {
			funcDef, ok := instDef.Exports[fn]
			require.True(t, ok, "function %s should be exported", fn)
			require.NotNil(t, funcDef, "function %s should not be nil", fn)
		}
	})

	// Test wasi:random/insecure@0.2.0
	t.Run("Insecure", func(t *testing.T) {
		insecureDef, ok := linker.Get("wasi:random/insecure@0.2.0")
		require.True(t, ok, "insecure interface should be registered")

		instDef, ok := insecureDef.(*component.InstanceDef)
		require.True(t, ok, "should be an InstanceDef")

		expectedFunctions := []string{
			"get-insecure-random-bytes",
			"get-insecure-random-u64",
		}

		for _, fn := range expectedFunctions {
			funcDef, ok := instDef.Exports[fn]
			require.True(t, ok, "function %s should be exported", fn)
			require.NotNil(t, funcDef, "function %s should not be nil", fn)
		}
	})

	// Test wasi:random/insecure-seed@0.2.0
	t.Run("InsecureSeed", func(t *testing.T) {
		insecureSeedDef, ok := linker.Get("wasi:random/insecure-seed@0.2.0")
		require.True(t, ok, "insecure-seed interface should be registered")

		instDef, ok := insecureSeedDef.(*component.InstanceDef)
		require.True(t, ok, "should be an InstanceDef")

		expectedFunctions := []string{
			"insecure-seed",
		}

		for _, fn := range expectedFunctions {
			funcDef, ok := instDef.Exports[fn]
			require.True(t, ok, "function %s should be exported", fn)
			require.NotNil(t, funcDef, "function %s should not be nil", fn)
		}
	})
}
