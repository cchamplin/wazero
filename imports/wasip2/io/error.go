// imports/wasip2/io/error.go

package io

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Error wraps a Go error for the wasi:io/error resource.
type Error struct {
	err error
}

// NewError creates a new Error resource wrapping a Go error.
func NewError(err error) *Error {
	return &Error{err: err}
}

// ToDebugString returns a human-readable debug string.
func (e *Error) ToDebugString() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap returns the underlying Go error.
func (e *Error) Unwrap() error {
	return e.err
}

func instantiateError(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/error@0.2.0")

	// Define the error resource type
	inst.Resource("error", func(rep uint32) {
		// Destructor - nothing to clean up for simple errors
	})

	// [method]error.to-debug-string: func(self: borrow<error>) -> string
	inst.FuncNoType("[method]error.to-debug-string", errorToDebugString)

	return inst.Build()
}

func errorToDebugString(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// args[0] is borrow<error> - the handle
	// For now, return empty string as we don't have ResourceTable integration yet
	// Full implementation will look up the handle in the table
	return []component.Val{component.ValString("")}, nil
}
