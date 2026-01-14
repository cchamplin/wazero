// imports/wasip2/io/io.go

package io

import (
	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:io interfaces with the linker.
func Instantiate(linker *component.Linker) error {
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

// Placeholder implementations - will be replaced in subsequent tasks

func instantiatePoll(linker *component.Linker) error {
	return linker.DefineInstance("wasi:io/poll@0.2.0").Build()
}

func instantiateStreams(linker *component.Linker) error {
	return linker.DefineInstance("wasi:io/streams@0.2.0").Build()
}
