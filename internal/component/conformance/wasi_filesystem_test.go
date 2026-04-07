// Package conformance contains conformance tests for the Component Model implementation.
// This file implements Task 279: WASI Filesystem Conformance Tests.
package conformance

import (
	"context"
	"os"
	"testing"

	"github.com/tetratelabs/wazero/imports/wasip2"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// =============================================================================
// Task 279: WASI Filesystem Conformance Tests
// =============================================================================

// TestWASI_Filesystem_Preopens tests that get-directories returns preopens.
func TestWASI_Filesystem_Preopens(t *testing.T) {
	linker := component.NewLinker()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "wasi_fs_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Configure with a preopen
	config := wasip2.NewConfig().WithPreopen("/guest", tmpDir)

	err = wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the preopens interface
	preopensDef, ok := linker.Get("wasi:filesystem/preopens@0.2.0")
	require.True(t, ok, "preopens interface should be registered")

	instDef, ok := preopensDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	getDirsFunc, ok := instDef.Exports["get-directories"]
	require.True(t, ok, "get-directories function should be exported")

	funcDef, ok := getDirsFunc.(*component.FuncDef)
	require.True(t, ok, "should be a FuncDef")

	// Call get-directories
	result, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result), "get-directories should return exactly one value")

	// Result should be a list of tuples (descriptor_handle, path)
	dirList := result[0].List()
	require.Equal(t, 1, len(dirList), "should return 1 preopened directory")

	// Verify the preopen entry
	tuple := dirList[0].Tuple()
	require.Equal(t, 2, len(tuple), "each entry should be a tuple of (handle, path)")

	// First element is own<descriptor> handle
	handle := tuple[0].Own()
	require.True(t, handle >= 0, "should return a valid descriptor handle")

	// Second element is the guest path
	guestPath := tuple[1].StringVal()
	require.Equal(t, "/guest", guestPath, "guest path should match configuration")
}

// TestWASI_Filesystem_PreopensEmpty tests behavior with no preopens configured.
func TestWASI_Filesystem_PreopensEmpty(t *testing.T) {
	linker := component.NewLinker()

	// Config with no preopens
	config := wasip2.NewConfig()

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	preopensDef, _ := linker.Get("wasi:filesystem/preopens@0.2.0")
	instDef := preopensDef.(*component.InstanceDef)
	funcDef := instDef.Exports["get-directories"].(*component.FuncDef)

	result, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)

	dirList := result[0].List()
	require.Equal(t, 0, len(dirList), "should return empty list when no preopens configured")
}

// TestWASI_Filesystem_TypesInterfaceExists tests that the types interface exists.
func TestWASI_Filesystem_TypesInterfaceExists(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Get the filesystem types interface
	typesDef, ok := linker.Get("wasi:filesystem/types@0.2.0")
	require.True(t, ok, "filesystem/types interface should be registered")

	instDef, ok := typesDef.(*component.InstanceDef)
	require.True(t, ok, "should be an InstanceDef")

	// Verify descriptor resource exists
	descRes, ok := instDef.Exports["descriptor"]
	require.True(t, ok, "descriptor resource should be exported")
	require.NotNil(t, descRes, "descriptor resource should not be nil")

	// Verify directory-entry-stream resource exists
	dirEntryRes, ok := instDef.Exports["directory-entry-stream"]
	require.True(t, ok, "directory-entry-stream resource should be exported")
	require.NotNil(t, dirEntryRes, "directory-entry-stream resource should not be nil")
}

// TestWASI_Filesystem_DescriptorMethods tests that descriptor methods exist.
func TestWASI_Filesystem_DescriptorMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, ok := linker.Get("wasi:filesystem/types@0.2.0")
	require.True(t, ok, "filesystem/types interface should be registered")

	instDef := typesDef.(*component.InstanceDef)

	// Expected descriptor methods
	expectedMethods := []string{
		"[method]descriptor.read-via-stream",
		"[method]descriptor.write-via-stream",
		"[method]descriptor.append-via-stream",
		"[method]descriptor.advise",
		"[method]descriptor.sync-data",
		"[method]descriptor.get-flags",
		"[method]descriptor.get-type",
		"[method]descriptor.set-size",
		"[method]descriptor.set-times",
		"[method]descriptor.read",
		"[method]descriptor.write",
		"[method]descriptor.read-directory",
		"[method]descriptor.sync",
		"[method]descriptor.create-directory-at",
		"[method]descriptor.stat",
		"[method]descriptor.stat-at",
		"[method]descriptor.set-times-at",
		"[method]descriptor.link-at",
		"[method]descriptor.open-at",
		"[method]descriptor.readlink-at",
		"[method]descriptor.remove-directory-at",
		"[method]descriptor.rename-at",
		"[method]descriptor.symlink-at",
		"[method]descriptor.unlink-file-at",
		"[method]descriptor.is-same-object",
		"[method]descriptor.metadata-hash",
		"[method]descriptor.metadata-hash-at",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_Filesystem_DirectoryEntryStreamMethods tests that directory entry stream methods exist.
func TestWASI_Filesystem_DirectoryEntryStreamMethods(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	typesDef, ok := linker.Get("wasi:filesystem/types@0.2.0")
	require.True(t, ok, "filesystem/types interface should be registered")

	instDef := typesDef.(*component.InstanceDef)

	// Expected directory-entry-stream methods
	expectedMethods := []string{
		"[method]directory-entry-stream.read-directory-entry",
	}

	for _, method := range expectedMethods {
		methodDef, ok := instDef.Exports[method]
		require.True(t, ok, "method %s should be exported", method)
		require.NotNil(t, methodDef, "method %s should not be nil", method)
	}
}

// TestWASI_Filesystem_StatOnPreopen tests stat on a preopened directory.
func TestWASI_Filesystem_StatOnPreopen(t *testing.T) {
	linker := component.NewLinker()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "wasi_fs_stat_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Configure with a preopen
	config := wasip2.NewConfig().WithPreopen("/guest", tmpDir)

	err = wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	// Get the preopen descriptor
	preopensDef, _ := linker.Get("wasi:filesystem/preopens@0.2.0")
	preInst := preopensDef.(*component.InstanceDef)
	getDirsFunc := preInst.Exports["get-directories"].(*component.FuncDef)

	result, err := getDirsFunc.Callback(ctx, []types.Val{})
	require.NoError(t, err)

	dirList := result[0].List()
	require.Equal(t, 1, len(dirList), "should have one preopen")

	// Get the descriptor handle from the preopen
	tuple := dirList[0].Tuple()
	descHandle := tuple[0].Own()

	// Now call stat on the descriptor
	typesDef, _ := linker.Get("wasi:filesystem/types@0.2.0")
	typesInst := typesDef.(*component.InstanceDef)
	statFunc := typesInst.Exports["[method]descriptor.stat"].(*component.FuncDef)

	// Call stat with borrow<descriptor>
	statResult, err := statFunc.Callback(ctx, []types.Val{types.ValBorrow(descHandle)})
	require.NoError(t, err)
	require.Equal(t, 1, len(statResult), "stat should return exactly one value")

	// Result should be result<descriptor-stat, error-code>
	isOk, okVal, _ := statResult[0].Result()
	require.True(t, isOk, "stat should succeed on a valid preopen")

	// okVal should be a descriptor-stat record
	stat := okVal.Record()
	require.NotNil(t, stat, "stat result should be a record")

	// Verify the type field indicates a directory
	typeVal, ok := stat["type"]
	require.True(t, ok, "stat should have a 'type' field")

	typeName := typeVal.Enum()
	require.Equal(t, "directory", typeName, "preopen should be a directory")
}

// TestWASI_Filesystem_InterfaceRegistration tests that all filesystem interfaces are properly registered.
func TestWASI_Filesystem_InterfaceRegistration(t *testing.T) {
	linker := component.NewLinker()

	err := wasip2.Instantiate(linker)
	require.NoError(t, err)

	// Verify all filesystem interfaces are registered
	interfaces := []string{
		"wasi:filesystem/types@0.2.0",
		"wasi:filesystem/preopens@0.2.0",
	}

	for _, iface := range interfaces {
		def, ok := linker.Get(iface)
		require.True(t, ok, "interface %s should be registered", iface)
		require.NotNil(t, def, "interface %s definition should not be nil", iface)
	}
}

// TestWASI_Filesystem_MultiplePreopens tests multiple preopened directories.
func TestWASI_Filesystem_MultiplePreopens(t *testing.T) {
	linker := component.NewLinker()

	// Create temporary directories for testing
	tmpDir1, err := os.MkdirTemp("", "wasi_fs_test1_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "wasi_fs_test2_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir2)

	// Configure with multiple preopens
	config := wasip2.NewConfig().
		WithPreopen("/dir1", tmpDir1).
		WithPreopen("/dir2", tmpDir2)

	err = wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	preopensDef, _ := linker.Get("wasi:filesystem/preopens@0.2.0")
	instDef := preopensDef.(*component.InstanceDef)
	funcDef := instDef.Exports["get-directories"].(*component.FuncDef)

	result, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)

	dirList := result[0].List()
	require.Equal(t, 2, len(dirList), "should return 2 preopened directories")

	// Collect guest paths
	guestPaths := make(map[string]bool)
	for _, entry := range dirList {
		tuple := entry.Tuple()
		guestPath := tuple[1].StringVal()
		guestPaths[guestPath] = true
	}

	require.True(t, guestPaths["/dir1"], "should have /dir1 preopen")
	require.True(t, guestPaths["/dir2"], "should have /dir2 preopen")
}

// TestWASI_Filesystem_InvalidPreopenPath tests behavior with an invalid host path.
func TestWASI_Filesystem_InvalidPreopenPath(t *testing.T) {
	linker := component.NewLinker()

	// Configure with an invalid host path
	config := wasip2.NewConfig().WithPreopen("/guest", "/nonexistent/path/that/should/not/exist")

	err := wasip2.InstantiateWithConfig(linker, config)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = wasip2.WithConfig(ctx, config)
	table := runtime.NewResourceTable()
	ctx = component.WithResourceTable(ctx, table)

	preopensDef, _ := linker.Get("wasi:filesystem/preopens@0.2.0")
	instDef := preopensDef.(*component.InstanceDef)
	funcDef := instDef.Exports["get-directories"].(*component.FuncDef)

	result, err := funcDef.Callback(ctx, []types.Val{})
	require.NoError(t, err)

	// Invalid paths should be skipped, so should return empty list
	dirList := result[0].List()
	require.Equal(t, 0, len(dirList), "invalid preopen paths should be skipped")
}
