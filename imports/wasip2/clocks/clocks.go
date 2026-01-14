// Package clocks implements the wasi:clocks interfaces for WASI Preview 2.
// It provides wall clock and monotonic clock functionality.
package clocks

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:clocks interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateWallClock(linker); err != nil {
		return err
	}
	if err := instantiateMonotonicClock(linker); err != nil {
		return err
	}
	return nil
}
