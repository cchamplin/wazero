// imports/wasip2/random/insecure.go

package random

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/tetratelabs/wazero/internal/component"
)

var (
	insecureRNG  *rand.Rand
	insecureOnce sync.Once
	insecureMu   sync.Mutex
)

func getInsecureRNG() *rand.Rand {
	insecureOnce.Do(func() {
		insecureRNG = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
	return insecureRNG
}

// GetInsecureRandomBytes returns non-cryptographic random bytes.
// The length is capped at 64KB to prevent excessive allocations.
func GetInsecureRandomBytes(length uint64) []byte {
	if length > 64*1024 {
		length = 64 * 1024
	}
	buf := make([]byte, length)
	insecureMu.Lock()
	getInsecureRNG().Read(buf)
	insecureMu.Unlock()
	return buf
}

// GetInsecureRandomU64 returns a non-cryptographic random uint64.
func GetInsecureRandomU64() uint64 {
	insecureMu.Lock()
	v := getInsecureRNG().Uint64()
	insecureMu.Unlock()
	return v
}

// InsecureSeed returns the seed for insecure random.
// Per spec, must return same value within a component instance.
var (
	seedOnce     sync.Once
	seed1, seed2 uint64
)

// InsecureSeed returns a pair of u64 values that can be used to seed
// a pseudo-random number generator. Per the WASI spec, the same values
// must be returned within a component instance.
func InsecureSeed() (uint64, uint64) {
	seedOnce.Do(func() {
		seed1 = GetRandomU64()
		seed2 = GetRandomU64()
	})
	return seed1, seed2
}

func instantiateInsecure(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:random/insecure@0.2.0")

	inst.FuncNoType("get-insecure-random-bytes", getInsecureRandomBytes)
	inst.FuncNoType("get-insecure-random-u64", getInsecureRandomU64)

	return inst.SkipValidation().Build()
}

func instantiateInsecureSeed(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:random/insecure-seed@0.2.0")

	inst.FuncNoType("insecure-seed", insecureSeedFunc)

	return inst.SkipValidation().Build()
}

func getInsecureRandomBytes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	length := args[0].U64()
	bytes := GetInsecureRandomBytes(length)

	vals := make([]component.Val, len(bytes))
	for i, b := range bytes {
		vals[i] = component.ValU8(b)
	}
	return []component.Val{component.ValList(vals)}, nil
}

func getInsecureRandomU64(ctx context.Context, args []component.Val) ([]component.Val, error) {
	return []component.Val{component.ValU64(GetInsecureRandomU64())}, nil
}

func insecureSeedFunc(ctx context.Context, args []component.Val) ([]component.Val, error) {
	s1, s2 := InsecureSeed()
	return []component.Val{component.ValTuple([]component.Val{
		component.ValU64(s1),
		component.ValU64(s2),
	})}, nil
}
