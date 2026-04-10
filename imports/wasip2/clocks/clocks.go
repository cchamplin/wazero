// Package clocks implements the wasi:clocks interfaces for WASI Preview 2.
// It provides wall clock and monotonic clock functionality.
package clocks

import (
	"github.com/tetratelabs/wazero/api"
)

// Instantiate registers all wasi:clocks interfaces with the linker.
func Instantiate(linker api.ComponentLinker) error {
	if err := instantiateWallClock(linker); err != nil {
		return err
	}
	if err := instantiateMonotonicClock(linker); err != nil {
		return err
	}
	return nil
}
