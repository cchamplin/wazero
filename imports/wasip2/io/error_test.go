// imports/wasip2/io/error_test.go

package io

import (
	"errors"
	"testing"

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

func TestInstantiateError(t *testing.T) {
	linker := component.NewLinker()
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
	linker := component.NewLinker()

	// First registration should succeed
	err := instantiateError(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = instantiateError(linker)
	require.Error(t, err)
}
