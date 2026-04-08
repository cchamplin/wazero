// imports/wasip2/io/error.go

package io

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/tetratelabs/wazero/internal/component"
	cmpruntime "github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// Error represents a wasi:io/error resource with enhanced debugging.
// It wraps a Go error and optionally includes source context and stack trace.
type Error struct {
	err    error
	source string
	stack  []uintptr
}

// NewError creates a new Error resource wrapping a Go error.
func NewError(err error) *Error {
	return &Error{
		err: err,
	}
}

// NewErrorWithSource creates an error with source context.
// The source indicates where the error originated (e.g., "tcp-connect", "file-read").
func NewErrorWithSource(err error, source string) *Error {
	return &Error{
		err:    err,
		source: source,
	}
}

// NewErrorWithStack creates an error with stack trace for debugging.
func NewErrorWithStack(err error) *Error {
	// Capture stack trace
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs) // Skip Callers and NewErrorWithStack

	return &Error{
		err:   err,
		stack: pcs[:n],
	}
}

// NewErrorFull creates an error with both source context and stack trace.
func NewErrorFull(err error, source string) *Error {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs) // Skip Callers and NewErrorFull

	return &Error{
		err:    err,
		source: source,
		stack:  pcs[:n],
	}
}

// ToDebugString returns a detailed debug string for the error.
// It includes the source context and stack trace if available.
func (e *Error) ToDebugString() string {
	if e == nil || e.err == nil {
		return ""
	}

	var sb strings.Builder

	// Add source if present
	if e.source != "" {
		sb.WriteString("[")
		sb.WriteString(e.source)
		sb.WriteString("] ")
	}

	// Add error message
	sb.WriteString(e.err.Error())

	// Add stack trace if present
	if len(e.stack) > 0 {
		sb.WriteString("\n\nStack trace:\n")
		frames := runtime.CallersFrames(e.stack)
		for {
			frame, more := frames.Next()
			if frame.Function == "" {
				break
			}
			sb.WriteString(fmt.Sprintf("  %s\n    %s:%d\n",
				frame.Function, frame.File, frame.Line))
			if !more {
				break
			}
		}
	}

	return sb.String()
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.err == nil {
		return "no error"
	}
	return e.err.Error()
}

// Unwrap returns the underlying Go error for use with errors.Unwrap.
func (e *Error) Unwrap() error {
	return e.err
}

// Destroy implements Destroyable (no-op for errors).
func (e *Error) Destroy() {
	// Nothing to clean up
}

func instantiateError(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:io/error@0.2.0")

	// Define the error resource type
	inst.Resource("error", func(rep uint32) {
		// Destructor - nothing to clean up for simple errors
	})

	// [method]error.to-debug-string: func(self: borrow<error>) -> string
	inst.FuncNoType("[method]error.to-debug-string", errorToDebugString)

	return inst.SkipValidation().Build()
}

// errorToDebugString implements [method]error.to-debug-string
func errorToDebugString(ctx context.Context, args []types.Val) ([]types.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []types.Val{types.ValString("no resource table")}, nil
	}

	handle := args[0].Borrow()
	entry, err := table.Get(cmpruntime.Handle(handle))
	if err != nil {
		return []types.Val{types.ValString("invalid error handle")}, nil
	}

	resEntry, ok := entry.(*cmpruntime.ResourceHandleEntry)
	if !ok {
		return []types.Val{types.ValString("not a resource handle")}, nil
	}
	wasiErr, ok := resEntry.Rep.(*Error)
	if !ok {
		// Try to handle as generic error
		if genericErr, ok := resEntry.Rep.(error); ok {
			return []types.Val{types.ValString(genericErr.Error())}, nil
		}
		return []types.Val{types.ValString("not an error resource")}, nil
	}

	return []types.Val{types.ValString(wasiErr.ToDebugString())}, nil
}
