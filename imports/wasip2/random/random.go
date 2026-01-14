// Package random implements the wasi:random interfaces for WASI Preview 2.
// It provides cryptographically secure and insecure random number generation.
package random

import (
	"context"
	"crypto/rand"
	"encoding/binary"

	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:random interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateRandom(linker); err != nil {
		return err
	}
	if err := instantiateInsecure(linker); err != nil {
		return err
	}
	if err := instantiateInsecureSeed(linker); err != nil {
		return err
	}
	return nil
}

// GetRandomBytes returns cryptographically secure random bytes.
// The length is capped at 64KB to prevent excessive allocations.
func GetRandomBytes(length uint64) []byte {
	// Cap at reasonable size to prevent DoS
	if length > 64*1024 {
		length = 64 * 1024
	}
	buf := make([]byte, length)
	rand.Read(buf)
	return buf
}

// GetRandomU64 returns a cryptographically secure random uint64.
func GetRandomU64() uint64 {
	var buf [8]byte
	rand.Read(buf[:])
	return binary.LittleEndian.Uint64(buf[:])
}

func instantiateRandom(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:random/random@0.2.0")

	inst.FuncNoType("get-random-bytes", getRandomBytes)
	inst.FuncNoType("get-random-u64", getRandomU64)

	return inst.Build()
}

func getRandomBytes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	length := args[0].U64()
	bytes := GetRandomBytes(length)

	// Convert to list of u8
	vals := make([]component.Val, len(bytes))
	for i, b := range bytes {
		vals[i] = component.ValU8(b)
	}
	return []component.Val{component.ValList(vals)}, nil
}

func getRandomU64(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValU64(GetRandomU64())}, nil
}
