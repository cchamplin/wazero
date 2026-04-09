// imports/wasip2/clocks/wall.go

// WIT source of truth: debug-vendored/WASI/proposals/clocks/wit/wall-clock.wit
// Package version: wasi:clocks@0.2.9 (wazero targets wasi:clocks@0.2.0)
//
package clocks

import (
	"context"
	"time"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// Datetime represents the wall-clock datetime record.
// Per wasi:clocks/wall-clock@0.2.0 spec.
type Datetime struct {
	Seconds     uint64
	Nanoseconds uint32
}

// WallClockNow returns the current wall clock time.
func WallClockNow() Datetime {
	now := time.Now()
	return Datetime{
		Seconds:     uint64(now.Unix()),
		Nanoseconds: uint32(now.Nanosecond()),
	}
}

// WallClockResolution returns the resolution of the wall clock.
// Go's time.Time has nanosecond precision.
func WallClockResolution() Datetime {
	return Datetime{
		Seconds:     0,
		Nanoseconds: 1,
	}
}

func instantiateWallClock(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:clocks/wall-clock@0.2.0")

	inst.Func("now", wallClockNow)
	inst.Func("resolution", wallClockResolution)

	return inst.SkipValidation().Build()
}

func wallClockNow(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	dt := WallClockNow()
	return []types.Val{
		types.ValRecord(map[string]types.Val{
			"seconds":     types.ValU64(dt.Seconds),
			"nanoseconds": types.ValU32(dt.Nanoseconds),
		}),
	}, nil
}

func wallClockResolution(ctx context.Context, _ *types.TypeFunc, args []types.Val) ([]types.Val, error) {
	dt := WallClockResolution()
	return []types.Val{
		types.ValRecord(map[string]types.Val{
			"seconds":     types.ValU64(dt.Seconds),
			"nanoseconds": types.ValU32(dt.Nanoseconds),
		}),
	}, nil
}
