// imports/wasip2/io/io.go

package io

import (
	"github.com/tetratelabs/wazero/api"
)

// Instantiate registers all wasi:io interfaces with the linker.
func Instantiate(linker api.ComponentLinker) error {
	if err := instantiateError(linker); err != nil {
		return err
	}
	if err := instantiatePoll(linker); err != nil {
		return err
	}
	if err := instantiateStreams(linker); err != nil {
		return err
	}
	return nil
}
