// imports/wasip2/clocks/wall.go

package clocks

import (
	"context"
	"time"

	"github.com/tetratelabs/wazero/internal/component"
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

	inst.FuncNoType("now", wallClockNow)
	inst.FuncNoType("resolution", wallClockResolution)

	return inst.SkipValidation().Build()
}

func wallClockNow(ctx context.Context, args []component.Val) ([]component.Val, error) {
	dt := WallClockNow()
	return []component.Val{
		component.ValRecord(map[string]component.Val{
			"seconds":     component.ValU64(dt.Seconds),
			"nanoseconds": component.ValU32(dt.Nanoseconds),
		}),
	}, nil
}

func wallClockResolution(ctx context.Context, args []component.Val) ([]component.Val, error) {
	dt := WallClockResolution()
	return []component.Val{
		component.ValRecord(map[string]component.Val{
			"seconds":     component.ValU64(dt.Seconds),
			"nanoseconds": component.ValU32(dt.Nanoseconds),
		}),
	}, nil
}
