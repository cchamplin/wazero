// imports/wasip2/filesystem/filesystem_test.go

package filesystem

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstantiate(t *testing.T) {
	linker := component.NewLinker()
	err := Instantiate(linker)
	require.NoError(t, err)

	// Verify all interfaces are registered
	interfaces := []string{
		"wasi:filesystem/types@0.2.0",
		"wasi:filesystem/preopens@0.2.0",
	}

	for _, iface := range interfaces {
		def, err := linker.MatchImport(iface)
		require.NoError(t, err, "interface %s should be registered", iface)
		_, ok := def.(*component.InstanceDef)
		require.True(t, ok, "expected InstanceDef for %s", iface)
	}
}

func TestInstantiate_Duplicate(t *testing.T) {
	linker := component.NewLinker()

	// First registration should succeed
	err := Instantiate(linker)
	require.NoError(t, err)

	// Second registration should fail
	err = Instantiate(linker)
	require.Error(t, err)
}

// ====================
// Types Interface Tests
// ====================

func TestInstantiateTypes(t *testing.T) {
	linker := component.NewLinker()
	err := instantiateTypes(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:filesystem/types@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify descriptor resource is defined
	_, hasDescriptor := instDef.Exports["descriptor"]
	require.True(t, hasDescriptor, "descriptor resource should be defined")

	// Verify directory-entry-stream resource is defined
	_, hasDirEntryStream := instDef.Exports["directory-entry-stream"]
	require.True(t, hasDirEntryStream, "directory-entry-stream resource should be defined")

	// Verify descriptor methods
	descriptorMethods := []string{
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

	for _, method := range descriptorMethods {
		_, hasMethod := instDef.Exports[method]
		require.True(t, hasMethod, "%s should be defined", method)
	}

	// Verify filesystem-error-code function
	_, hasErrorCode := instDef.Exports["filesystem-error-code"]
	require.True(t, hasErrorCode, "filesystem-error-code function should be defined")

	// Verify directory-entry-stream method
	_, hasReadEntry := instDef.Exports["[method]directory-entry-stream.read-directory-entry"]
	require.True(t, hasReadEntry, "[method]directory-entry-stream.read-directory-entry should be defined")
}

// Descriptor method tests

func TestDescriptorReadViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>), offset (u64)
	// Returns: result<own<input-stream>, error-code>
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	result, err := descriptorReadViaStream(context.Background(), []component.Val{selfHandle, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestDescriptorWriteViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>), offset (u64)
	// Returns: result<own<output-stream>, error-code>
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	result, err := descriptorWriteViaStream(context.Background(), []component.Val{selfHandle, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestDescriptorAppendViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>)
	// Returns: result<own<output-stream>, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorAppendViaStream(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestDescriptorAdvise(t *testing.T) {
	// Args: self, offset, length, advice
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	length := component.ValU64(0)
	advice := component.ValEnum("normal")
	result, err := descriptorAdvise(context.Background(), []component.Val{selfHandle, offset, length, advice})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result (no-op)")
}

func TestDescriptorSyncData(t *testing.T) {
	// Args: self
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorSyncData(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorGetFlags(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-flags, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorGetFlags(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindFlags, ok.Kind())
}

func TestDescriptorGetType(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-type, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorGetType(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindEnum, ok.Kind())
}

func TestDescriptorSetSize(t *testing.T) {
	// Args: self, size
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(0)
	result, err := descriptorSetSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorSetTimes(t *testing.T) {
	// Args: self, data-access-timestamp, data-modification-timestamp
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	accessTime := component.ValVariant("no-change", nil)
	modTime := component.ValVariant("no-change", nil)
	result, err := descriptorSetTimes(context.Background(), []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorRead(t *testing.T) {
	// Args: self, length, offset
	// Returns: result<tuple<list<u8>, bool>, error-code>
	selfHandle := component.ValBorrow(0)
	length := component.ValU64(100)
	offset := component.ValU64(0)
	result, err := descriptorRead(context.Background(), []component.Val{selfHandle, length, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindTuple, ok.Kind())
	tuple := ok.Tuple()
	require.Equal(t, 2, len(tuple))
	require.Equal(t, component.ValKindList, tuple[0].Kind()) // list<u8>
	require.Equal(t, component.ValKindBool, tuple[1].Kind()) // bool (EOF)
}

func TestDescriptorWrite(t *testing.T) {
	// Args: self, buffer, offset
	// Returns: result<u64, error-code>
	selfHandle := component.ValBorrow(0)
	buffer := component.ValList([]component.Val{component.ValU8(65), component.ValU8(66)})
	offset := component.ValU64(0)
	result, err := descriptorWrite(context.Background(), []component.Val{selfHandle, buffer, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
}

func TestDescriptorReadDirectory(t *testing.T) {
	// Args: self
	// Returns: result<own<directory-entry-stream>, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorReadDirectory(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestDescriptorSync(t *testing.T) {
	// Args: self
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorSync(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorCreateDirectoryAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	path := component.ValString("newdir")
	result, err := descriptorCreateDirectoryAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorStat(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-stat, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorStat(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindRecord, ok.Kind())
}

func TestDescriptorStatAt(t *testing.T) {
	// Args: self, path-flags, path
	// Returns: result<descriptor-stat, error-code>
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	result, err := descriptorStatAt(context.Background(), []component.Val{selfHandle, pathFlags, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindRecord, ok.Kind())
}

func TestDescriptorSetTimesAt(t *testing.T) {
	// Args: self, path-flags, path, data-access-timestamp, data-modification-timestamp
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	accessTime := component.ValVariant("no-change", nil)
	modTime := component.ValVariant("no-change", nil)
	result, err := descriptorSetTimesAt(context.Background(), []component.Val{selfHandle, pathFlags, path, accessTime, modTime})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorLinkAt(t *testing.T) {
	// Args: self, old-path-flags, old-path, new-descriptor, new-path
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": false})
	oldPath := component.ValString("oldfile")
	newDescriptor := component.ValBorrow(1)
	newPath := component.ValString("newfile")
	result, err := descriptorLinkAt(context.Background(), []component.Val{selfHandle, pathFlags, oldPath, newDescriptor, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorOpenAt(t *testing.T) {
	// Args: self, path-flags, path, open-flags, descriptor-flags
	// Returns: result<own<descriptor>, error-code>
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	openFlags := component.ValFlags(map[string]bool{"create": false})
	descFlags := component.ValFlags(map[string]bool{"read": true})
	result, err := descriptorOpenAt(context.Background(), []component.Val{selfHandle, pathFlags, path, openFlags, descFlags})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestDescriptorReadlinkAt(t *testing.T) {
	// Args: self, path
	// Returns: result<string, error-code>
	selfHandle := component.ValBorrow(0)
	path := component.ValString("symlink")
	result, err := descriptorReadlinkAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindString, ok.Kind())
}

func TestDescriptorRemoveDirectoryAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	path := component.ValString("dir")
	result, err := descriptorRemoveDirectoryAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorRenameAt(t *testing.T) {
	// Args: self, old-path, new-descriptor, new-path
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	oldPath := component.ValString("oldname")
	newDescriptor := component.ValBorrow(1)
	newPath := component.ValString("newname")
	result, err := descriptorRenameAt(context.Background(), []component.Val{selfHandle, oldPath, newDescriptor, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorSymlinkAt(t *testing.T) {
	// Args: self, old-path, new-path
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	oldPath := component.ValString("target")
	newPath := component.ValString("link")
	result, err := descriptorSymlinkAt(context.Background(), []component.Val{selfHandle, oldPath, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorUnlinkFileAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	selfHandle := component.ValBorrow(0)
	path := component.ValString("file")
	result, err := descriptorUnlinkFileAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorIsSameObject(t *testing.T) {
	// Args: self, other
	// Returns: bool
	selfHandle := component.ValBorrow(0)
	otherHandle := component.ValBorrow(1)
	result, err := descriptorIsSameObject(context.Background(), []component.Val{selfHandle, otherHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindBool, result[0].Kind())
}

func TestDescriptorMetadataHash(t *testing.T) {
	// Args: self
	// Returns: result<metadata-hash-value, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := descriptorMetadataHash(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindRecord, ok.Kind())
}

func TestDescriptorMetadataHashAt(t *testing.T) {
	// Args: self, path-flags, path
	// Returns: result<metadata-hash-value, error-code>
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	result, err := descriptorMetadataHashAt(context.Background(), []component.Val{selfHandle, pathFlags, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindRecord, ok.Kind())
}

func TestFilesystemErrorCode(t *testing.T) {
	// Args: err (borrow<error>)
	// Returns: option<error-code>
	errHandle := component.ValBorrow(0)
	result, err := filesystemErrorCode(context.Background(), []component.Val{errHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindOption, result[0].Kind())
}

func TestDirectoryEntryStreamReadEntry(t *testing.T) {
	// Args: self
	// Returns: result<option<directory-entry>, error-code>
	selfHandle := component.ValBorrow(0)
	result, err := directoryEntryStreamReadEntry(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOption, ok.Kind())
}

// ====================
// Preopens Interface Tests
// ====================

func TestInstantiatePreopens(t *testing.T) {
	linker := component.NewLinker()
	err := instantiatePreopens(linker)
	require.NoError(t, err)

	def, err := linker.MatchImport("wasi:filesystem/preopens@0.2.0")
	require.NoError(t, err)

	instDef, ok := def.(*component.InstanceDef)
	require.True(t, ok, "expected InstanceDef")

	// Verify get-directories function is defined
	_, hasGetDirs := instDef.Exports["get-directories"]
	require.True(t, hasGetDirs, "get-directories function should be defined")
}

func TestGetDirectories(t *testing.T) {
	// Returns: list<tuple<own<descriptor>, string>>
	result, err := getDirectories(context.Background(), []component.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindList, result[0].Kind())
	// Returns empty list as placeholder
	list := result[0].List()
	require.NotNil(t, list)
}

// ====================
// Type Tests
// ====================

func TestDescriptorType(t *testing.T) {
	// Verify descriptor type enum values
	require.Equal(t, DescriptorType(0), DescriptorTypeUnknown)
	require.Equal(t, DescriptorType(1), DescriptorTypeBlockDevice)
	require.Equal(t, DescriptorType(2), DescriptorTypeCharacterDevice)
	require.Equal(t, DescriptorType(3), DescriptorTypeDirectory)
	require.Equal(t, DescriptorType(4), DescriptorTypeFifo)
	require.Equal(t, DescriptorType(5), DescriptorTypeSymbolicLink)
	require.Equal(t, DescriptorType(6), DescriptorTypeRegularFile)
	require.Equal(t, DescriptorType(7), DescriptorTypeSocket)
}

func TestDescriptorTypeString(t *testing.T) {
	tests := []struct {
		dt       DescriptorType
		expected string
	}{
		{DescriptorTypeUnknown, "unknown"},
		{DescriptorTypeBlockDevice, "block-device"},
		{DescriptorTypeCharacterDevice, "character-device"},
		{DescriptorTypeDirectory, "directory"},
		{DescriptorTypeFifo, "fifo"},
		{DescriptorTypeSymbolicLink, "symbolic-link"},
		{DescriptorTypeRegularFile, "regular-file"},
		{DescriptorTypeSocket, "socket"},
	}

	for _, tc := range tests {
		require.Equal(t, tc.expected, tc.dt.String())
	}
}

func TestDescriptorFlags(t *testing.T) {
	// Verify descriptor flag bit values
	require.Equal(t, DescriptorFlags(1), DescriptorFlagRead)
	require.Equal(t, DescriptorFlags(2), DescriptorFlagWrite)
	require.Equal(t, DescriptorFlags(4), DescriptorFlagFileIntegritySync)
	require.Equal(t, DescriptorFlags(8), DescriptorFlagDataIntegritySync)
	require.Equal(t, DescriptorFlags(16), DescriptorFlagRequestedWriteSync)
	require.Equal(t, DescriptorFlags(32), DescriptorFlagMutateDirectory)
}

func TestErrorCodeValues(t *testing.T) {
	// Verify some key error code values
	require.Equal(t, ErrorCode("access"), ErrorCodeAccess)
	require.Equal(t, ErrorCode("bad-descriptor"), ErrorCodeBadDescriptor)
	require.Equal(t, ErrorCode("busy"), ErrorCodeBusy)
	require.Equal(t, ErrorCode("exist"), ErrorCodeExist)
	require.Equal(t, ErrorCode("no-entry"), ErrorCodeNoEntry)
	require.Equal(t, ErrorCode("not-directory"), ErrorCodeNotDirectory)
	require.Equal(t, ErrorCode("not-permitted"), ErrorCodeNotPermitted)
}
