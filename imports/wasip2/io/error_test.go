// imports/wasip2/io/error_test.go

package io

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestError_ToDebugString(t *testing.T) {
	goErr := errors.New("test error message")
	errResource := NewError(goErr)
	require.Equal(t, "test error message", errResource.ToDebugString())
}

func TestError_ToDebugString_NilError(t *testing.T) {
	errResource := NewError(nil)
	require.Equal(t, "", errResource.ToDebugString())
}

func TestError_Unwrap(t *testing.T) {
	goErr := errors.New("test error")
	errResource := NewError(goErr)
	require.Equal(t, goErr, errResource.Unwrap())
}

func TestError_Unwrap_NilError(t *testing.T) {
	errResource := NewError(nil)
	require.Nil(t, errResource.Unwrap())
}

func TestError_ToDebugString_IncludesMessage(t *testing.T) {
	err := NewError(errors.New("file not found"))

	debugStr := err.ToDebugString()

	if !strings.Contains(debugStr, "file not found") {
		t.Errorf("debug string should contain error message, got: %s", debugStr)
	}
}

func TestError_ToDebugString_IncludesSource(t *testing.T) {
	err := NewErrorWithSource(errors.New("connection failed"), "tcp-connect")

	debugStr := err.ToDebugString()

	if !strings.Contains(debugStr, "tcp-connect") {
		t.Errorf("debug string should contain source, got: %s", debugStr)
	}
	if !strings.Contains(debugStr, "connection failed") {
		t.Errorf("debug string should contain message, got: %s", debugStr)
	}
}

func TestError_ToDebugString_IncludesStackTrace(t *testing.T) {
	err := NewErrorWithStack(errors.New("panic recovered"))

	debugStr := err.ToDebugString()

	// Should contain file:line from stack
	if !strings.Contains(debugStr, ".go:") {
		t.Errorf("debug string should contain stack trace, got: %s", debugStr)
	}
}

func TestError_NewErrorFull(t *testing.T) {
	err := NewErrorFull(errors.New("full error"), "socket-write")

	debugStr := err.ToDebugString()

	// Should contain source
	if !strings.Contains(debugStr, "socket-write") {
		t.Errorf("debug string should contain source, got: %s", debugStr)
	}
	// Should contain message
	if !strings.Contains(debugStr, "full error") {
		t.Errorf("debug string should contain message, got: %s", debugStr)
	}
	// Should contain stack trace
	if !strings.Contains(debugStr, ".go:") {
		t.Errorf("debug string should contain stack trace, got: %s", debugStr)
	}
}

func TestError_ErrorInterface(t *testing.T) {
	err := NewError(errors.New("interface test"))

	// Error type should implement error interface
	var e error = err
	if e.Error() != "interface test" {
		t.Errorf("Error() should return error message, got: %s", e.Error())
	}
}

func TestError_ErrorInterface_NilError(t *testing.T) {
	err := NewError(nil)

	// Should handle nil gracefully
	if err.Error() != "no error" {
		t.Errorf("Error() with nil should return 'no error', got: %s", err.Error())
	}
}

func TestError_Destroy(t *testing.T) {
	err := NewError(errors.New("destroy test"))

	// Destroy should not panic
	err.Destroy()
}

func TestError_UnwrapWithErrorsPackage(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewError(originalErr)

	unwrapped := errors.Unwrap(err)
	if unwrapped != originalErr {
		t.Error("errors.Unwrap should return original error")
	}
}

func TestInstantiateError(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)
	err := instantiateError(linker)
	require.NoError(t, err)

	// Verify interface is registered
	def, err := linker.MatchImport("wasi:io/error@0.2.0")
	require.NoError(t, err)

	// Verify it's an instance definition
	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify exports exist
	_, hasResource := instDef.Exports["error"]
	require.True(t, hasResource, "error resource should be defined")

	_, hasMethod := instDef.Exports["[method]error.to-debug-string"]
	require.True(t, hasMethod, "to-debug-string method should be defined")
}

func TestInstantiateError_Duplicate(t *testing.T) {
	rt := wazero.NewRuntime(context.TODO())
	defer rt.Close(context.TODO())
	linker := component.NewComponentLinker(rt)

	// First registration should succeed
	err := instantiateError(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateError(linker)
	require.Error(t, err)
}
