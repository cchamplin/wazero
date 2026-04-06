// imports/wasip2/filesystem/filesystem_test.go

package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
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
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	result, err := descriptorReadViaStream(context.Background(), []component.Val{selfHandle, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorWriteViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>), offset (u64)
	// Returns: result<own<output-stream>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	result, err := descriptorWriteViaStream(context.Background(), []component.Val{selfHandle, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorAppendViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>)
	// Returns: result<own<output-stream>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorAppendViaStream(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorAdvise(t *testing.T) {
	// Args: self, offset, length, advice
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	length := component.ValU64(0)
	advice := component.ValEnum("normal")
	result, err := descriptorAdvise(context.Background(), []component.Val{selfHandle, offset, length, advice})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSyncData(t *testing.T) {
	// Args: self
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorSyncData(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorGetFlags(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-flags, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorGetFlags(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorGetType(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-type, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorGetType(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetSize(t *testing.T) {
	// Args: self, size
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	size := component.ValU64(0)
	result, err := descriptorSetSize(context.Background(), []component.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
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
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorRead(t *testing.T) {
	// Args: self, length, offset
	// Returns: result<tuple<list<u8>, bool>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	length := component.ValU64(100)
	offset := component.ValU64(0)
	result, err := descriptorRead(context.Background(), []component.Val{selfHandle, length, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorWrite(t *testing.T) {
	// Args: self, buffer, offset
	// Returns: result<u64, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	buffer := component.ValList([]component.Val{component.ValU8(65), component.ValU8(66)})
	offset := component.ValU64(0)
	result, err := descriptorWrite(context.Background(), []component.Val{selfHandle, buffer, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorReadDirectory(t *testing.T) {
	// Args: self
	// Returns: result<own<directory-entry-stream>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorReadDirectory(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSync(t *testing.T) {
	// Args: self
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorSync(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorCreateDirectoryAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	path := component.ValString("newdir")
	result, err := descriptorCreateDirectoryAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorStat(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-stat, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := descriptorStat(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorStatAt(t *testing.T) {
	// Args: self, path-flags, path
	// Returns: result<descriptor-stat, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	result, err := descriptorStatAt(context.Background(), []component.Val{selfHandle, pathFlags, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetTimesAt(t *testing.T) {
	// Args: self, path-flags, path, data-access-timestamp, data-modification-timestamp
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	accessTime := component.ValVariant("no-change", nil)
	modTime := component.ValVariant("no-change", nil)
	result, err := descriptorSetTimesAt(context.Background(), []component.Val{selfHandle, pathFlags, path, accessTime, modTime})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetTimesAt_SetToNow(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "child.txt")
	err := os.WriteFile(filePath, []byte("hello"), 0644)
	require.NoError(t, err)

	// Set old times
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filePath, oldTime, oldTime)

	dirFile, err := os.Open(tmpDir)
	require.NoError(t, err)
	defer dirFile.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	pathVal := component.ValString("child.txt")
	accessTime := component.ValVariant("now", nil)
	modTime := component.ValVariant("now", nil)

	before := time.Now().Add(-time.Second)
	result, err := descriptorSetTimesAt(ctx, []component.Val{selfHandle, pathFlags, pathVal, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-times-at should succeed")
	after := time.Now().Add(time.Second)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.True(t, info.ModTime().After(before))
	require.True(t, info.ModTime().Before(after))
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
	// Without a valid context/resource table, this should return bad-descriptor error
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
}

func TestDescriptorOpenAt(t *testing.T) {
	// Args: self, path-flags, path, open-flags, descriptor-flags
	// Returns: result<own<descriptor>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	pathFlags := component.ValFlags(map[string]bool{"symlink-follow": true})
	path := component.ValString("file.txt")
	openFlags := component.ValFlags(map[string]bool{"create": false})
	descFlags := component.ValFlags(map[string]bool{"read": true})
	result, err := descriptorOpenAt(context.Background(), []component.Val{selfHandle, pathFlags, path, openFlags, descFlags})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorReadlinkAt(t *testing.T) {
	// Args: self, path
	// Returns: result<string, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	path := component.ValString("symlink")
	result, err := descriptorReadlinkAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorRemoveDirectoryAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	path := component.ValString("dir")
	result, err := descriptorRemoveDirectoryAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorRenameAt(t *testing.T) {
	// Args: self, old-path, new-descriptor, new-path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	oldPath := component.ValString("oldname")
	newDescriptor := component.ValBorrow(1)
	newPath := component.ValString("newname")
	result, err := descriptorRenameAt(context.Background(), []component.Val{selfHandle, oldPath, newDescriptor, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSymlinkAt(t *testing.T) {
	// Args: self, old-path, new-path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	oldPath := component.ValString("target")
	newPath := component.ValString("link")
	result, err := descriptorSymlinkAt(context.Background(), []component.Val{selfHandle, oldPath, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorUnlinkFileAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	path := component.ValString("file")
	result, err := descriptorUnlinkFileAt(context.Background(), []component.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
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
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := component.ValBorrow(0)
	result, err := directoryEntryStreamReadEntry(context.Background(), []component.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
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

// ====================
// Host Function Tests with Real File Operations
// ====================

// createTestContext creates a context with a ResourceTable for testing.
func createTestContext() context.Context {
	table := component.NewResourceTable()
	return component.WithResourceTable(context.Background(), table)
}

// createTestDirDescriptor creates a test directory and adds it to the resource table.
func createTestDirDescriptor(t *testing.T, ctx context.Context) (uint32, string) {
	t.Helper()
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	tmpDir := t.TempDir()
	file, err := os.Open(tmpDir)
	require.NoError(t, err)

	desc := NewDescriptor(file, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)
	handle := table.New(desc, true)
	return uint32(handle.Index()), tmpDir
}

// createTestFileDescriptor creates a test file and adds it to the resource table.
func createTestFileDescriptor(t *testing.T, ctx context.Context, content []byte) (uint32, string) {
	t.Helper()
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)

	if len(content) > 0 {
		_, err = tmpFile.Write(content)
		require.NoError(t, err)
	}

	path := tmpFile.Name()
	desc := NewDescriptor(tmpFile, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle := table.New(desc, true)
	return uint32(handle.Index()), path
}

func TestDescriptorRead_HostFunction(t *testing.T) {
	ctx := createTestContext()
	testContent := []byte("Hello, World!")
	handle, path := createTestFileDescriptor(t, ctx, testContent)
	defer os.Remove(path)

	// Test reading from file
	result, err := descriptorRead(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(uint64(len(testContent))),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindTuple, ok.Kind())

	tuple := ok.Tuple()
	require.Equal(t, 2, len(tuple))
	require.Equal(t, component.ValKindList, tuple[0].Kind())
	require.Equal(t, component.ValKindBool, tuple[1].Kind())

	// Verify content
	list := tuple[0].List()
	require.Equal(t, len(testContent), len(list))
	for i, v := range list {
		require.Equal(t, testContent[i], v.U8())
	}
}

func TestDescriptorWrite_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, path := createTestFileDescriptor(t, ctx, nil)
	defer os.Remove(path)

	// Write data
	data := []byte("Test Data")
	dataVals := make([]component.Val, len(data))
	for i, b := range data {
		dataVals[i] = component.ValU8(b)
	}

	result, err := descriptorWrite(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValList(dataVals),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindU64, ok.Kind())
	require.Equal(t, uint64(len(data)), ok.U64())

	// Verify content was written
	written, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, written)
}

func TestDescriptorStat_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	result, err := descriptorStat(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindRecord, ok.Kind())

	// Check type field
	typeField, hasType := ok.RecordField("type")
	require.True(t, hasType)
	require.Equal(t, component.ValKindEnum, typeField.Kind())
	require.Equal(t, "directory", typeField.Enum())

	_ = tmpDir // tmpDir is cleaned up by t.TempDir()
}

func TestDescriptorStatAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a test file in the temp directory
	testFile := filepath.Join(tmpDir, "testfile.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	result, err := descriptorStatAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValFlags(map[string]bool{"symlink-follow": true}),
		component.ValString("testfile.txt"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindRecord, ok.Kind())

	// Check type field
	typeField, hasType := ok.RecordField("type")
	require.True(t, hasType)
	require.Equal(t, "regular-file", typeField.Enum())

	// Check size field
	sizeField, hasSize := ok.RecordField("size")
	require.True(t, hasSize)
	require.Equal(t, uint64(4), sizeField.U64())
}

func TestDescriptorGetType_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	result, err := descriptorGetType(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindEnum, ok.Kind())
	require.Equal(t, "directory", ok.Enum())
}

func TestDescriptorOpenAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a test file to open
	testFile := filepath.Join(tmpDir, "opentest.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	result, err := descriptorOpenAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValFlags(map[string]bool{"symlink-follow": true}),
		component.ValString("opentest.txt"),
		component.ValFlags(map[string]bool{}),
		component.ValFlags(map[string]bool{"read": true}),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())
}

func TestDescriptorOpenAt_CreateFile(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	result, err := descriptorOpenAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValFlags(map[string]bool{"symlink-follow": true}),
		component.ValString("newfile.txt"),
		component.ValFlags(map[string]bool{"create": true}),
		component.ValFlags(map[string]bool{"read": true, "write": true}),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)

	// Verify file was created
	_, statErr := os.Stat(filepath.Join(tmpDir, "newfile.txt"))
	require.NoError(t, statErr)
}

func TestDescriptorReadDirectory_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create some test files
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("2"), 0644)
	require.NoError(t, err)

	result, err := descriptorReadDirectory(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())

	// Now read entries from the stream
	streamHandle := ok.Own()

	// Read first entry
	entryResult, err := directoryEntryStreamReadEntry(ctx, []component.Val{
		component.ValBorrow(streamHandle),
	})
	require.NoError(t, err)
	isOk, ok, _ = entryResult[0].Result()
	require.True(t, isOk)
	require.NotNil(t, ok.Option()) // Should have an entry
}

func TestDescriptorCreateDirectoryAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	result, err := descriptorCreateDirectoryAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValString("subdir"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")

	// Verify directory was created
	info, statErr := os.Stat(filepath.Join(tmpDir, "subdir"))
	require.NoError(t, statErr)
	require.True(t, info.IsDir())
}

func TestDescriptorUnlinkFileAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a test file
	testFile := filepath.Join(tmpDir, "todelete.txt")
	err := os.WriteFile(testFile, []byte("delete me"), 0644)
	require.NoError(t, err)

	result, err := descriptorUnlinkFileAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValString("todelete.txt"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")

	// Verify file was deleted
	_, statErr := os.Stat(testFile)
	require.True(t, os.IsNotExist(statErr))
}

func TestDescriptorRemoveDirectoryAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a test directory
	testDir := filepath.Join(tmpDir, "toremove")
	err := os.Mkdir(testDir, 0755)
	require.NoError(t, err)

	result, err := descriptorRemoveDirectoryAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValString("toremove"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")

	// Verify directory was removed
	_, statErr := os.Stat(testDir)
	require.True(t, os.IsNotExist(statErr))
}

func TestDescriptorRenameAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a test file
	oldPath := filepath.Join(tmpDir, "oldname.txt")
	err := os.WriteFile(oldPath, []byte("rename me"), 0644)
	require.NoError(t, err)

	result, err := descriptorRenameAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValString("oldname.txt"),
		component.ValBorrow(handle), // same directory
		component.ValString("newname.txt"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")

	// Verify rename happened
	_, oldErr := os.Stat(oldPath)
	require.True(t, os.IsNotExist(oldErr))

	newPath := filepath.Join(tmpDir, "newname.txt")
	_, newErr := os.Stat(newPath)
	require.NoError(t, newErr)
}

func TestDescriptorSync_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, path := createTestFileDescriptor(t, ctx, []byte("sync me"))
	defer os.Remove(path)

	result, err := descriptorSync(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
}

func TestDescriptorGetFlags_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, path := createTestFileDescriptor(t, ctx, nil)
	defer os.Remove(path)

	result, err := descriptorGetFlags(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindFlags, ok.Kind())

	flags := ok.Flags()
	require.True(t, flags["read"])
	require.True(t, flags["write"])
}

// Test error cases

func TestDescriptorRead_BadDescriptor(t *testing.T) {
	ctx := createTestContext()

	// Try to read with non-existent handle
	result, err := descriptorRead(ctx, []component.Val{
		component.ValBorrow(999), // invalid handle
		component.ValU64(100),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.NotNil(t, errVal)
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorOpenAt_NoEntry(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	// Try to open non-existent file without create flag
	result, err := descriptorOpenAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValFlags(map[string]bool{"symlink-follow": true}),
		component.ValString("nonexistent.txt"),
		component.ValFlags(map[string]bool{}), // no create flag
		component.ValFlags(map[string]bool{"read": true}),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.NotNil(t, errVal)
	require.Equal(t, "no-entry", errVal.Enum())
}

func TestDescriptorStatAt_NoEntry(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	result, err := descriptorStatAt(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValFlags(map[string]bool{"symlink-follow": true}),
		component.ValString("nonexistent.txt"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.NotNil(t, errVal)
	require.Equal(t, "no-entry", errVal.Enum())
}

// ====================
// Stream Method Tests
// ====================

func TestDescriptorReadViaStream_HostFunction(t *testing.T) {
	ctx := createTestContext()
	content := []byte("Hello, WASI!")
	handle, path := createTestFileDescriptor(t, ctx, content)
	defer os.Remove(path)

	// Call read-via-stream with offset 0
	result, err := descriptorReadViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(0), // offset
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())

	// Get the input stream handle
	streamHandle := ok.Own()

	// Now read from the stream using the io package functions
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	// Lookup the stream and read from it
	entry, err := table.Get(component.Handle(streamHandle))
	require.NoError(t, err)
	require.NotNil(t, entry)

	// The entry should contain an InputStream
	_, isInputStream := entry.Rep.(*wasipIO.InputStream)
	require.True(t, isInputStream, "should return an InputStream")
}

func TestDescriptorReadViaStream_WithOffset(t *testing.T) {
	ctx := createTestContext()
	content := []byte("Hello, WASI!")
	handle, path := createTestFileDescriptor(t, ctx, content)
	defer os.Remove(path)

	// Call read-via-stream with offset 7 (should read "WASI!")
	result, err := descriptorReadViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(7), // offset
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)

	// Get the input stream handle
	streamHandle := ok.Own()

	// Verify the stream can read from offset
	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(component.Handle(streamHandle))
	require.NoError(t, err)

	inputStream, ok2 := entry.Rep.(*wasipIO.InputStream)
	require.True(t, ok2)

	// Read from the stream
	data, streamErr := inputStream.Read(100)
	require.Nil(t, streamErr)
	require.Equal(t, "WASI!", string(data))
}

func TestDescriptorReadViaStream_BadDescriptor(t *testing.T) {
	ctx := createTestContext()

	// Try with invalid handle
	result, err := descriptorReadViaStream(ctx, []component.Val{
		component.ValBorrow(999),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorReadViaStream_IsDirectory(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	// Try to read via stream from a directory
	result, err := descriptorReadViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.Equal(t, "is-directory", errVal.Enum())
}

func TestDescriptorWriteViaStream_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, path := createTestFileDescriptor(t, ctx, nil)
	defer os.Remove(path)

	// Call write-via-stream with offset 0
	result, err := descriptorWriteViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(0), // offset
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())

	// Get the output stream handle
	streamHandle := ok.Own()

	// Verify the stream can be used to write
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	entry, err := table.Get(component.Handle(streamHandle))
	require.NoError(t, err)
	require.NotNil(t, entry)

	outputStream, isOutputStream := entry.Rep.(*wasipIO.OutputStream)
	require.True(t, isOutputStream, "should return an OutputStream")

	// Write to the stream
	streamErr := outputStream.Write([]byte("Test Write"))
	require.Nil(t, streamErr)

	// Close the stream to flush
	outputStream.Close()

	// Verify the file content
	written, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, "Test Write", string(written))
}

func TestDescriptorWriteViaStream_WithOffset(t *testing.T) {
	ctx := createTestContext()
	content := []byte("Hello World") // 11 chars, no trailing !
	handle, path := createTestFileDescriptor(t, ctx, content)
	defer os.Remove(path)

	// Call write-via-stream with offset 6
	result, err := descriptorWriteViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(6), // offset
	})
	require.NoError(t, err)

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")

	// Get the output stream handle
	streamHandle := ok.Own()

	// Get the stream and write
	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(component.Handle(streamHandle))
	require.NoError(t, err)

	outputStream := entry.Rep.(*wasipIO.OutputStream)

	// Write "WASI!" at offset 6, replacing "World"
	streamErr := outputStream.Write([]byte("WASI!"))
	require.Nil(t, streamErr)
	outputStream.Close()

	// Verify the file content - "Hello " + "WASI!" = "Hello WASI!"
	written, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, "Hello WASI!", string(written))
}

func TestDescriptorWriteViaStream_BadDescriptor(t *testing.T) {
	ctx := createTestContext()

	// Try with invalid handle
	result, err := descriptorWriteViaStream(ctx, []component.Val{
		component.ValBorrow(999),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorWriteViaStream_IsDirectory(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	// Try to write via stream to a directory
	result, err := descriptorWriteViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.Equal(t, "is-directory", errVal.Enum())
}

func TestDescriptorAppendViaStream_HostFunction(t *testing.T) {
	ctx := createTestContext()
	content := []byte("Hello")
	handle, path := createTestFileDescriptor(t, ctx, content)
	defer os.Remove(path)

	// Call append-via-stream
	result, err := descriptorAppendViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, component.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, component.ValKindOwn, ok.Kind())

	// Get the output stream handle
	streamHandle := ok.Own()

	// Verify the stream can be used to append
	table := component.ResourceTableFromContext(ctx)
	entry, err := table.Get(component.Handle(streamHandle))
	require.NoError(t, err)

	outputStream, isOutputStream := entry.Rep.(*wasipIO.OutputStream)
	require.True(t, isOutputStream, "should return an OutputStream")

	// Write to the stream (should append)
	streamErr := outputStream.Write([]byte(", World!"))
	require.Nil(t, streamErr)
	outputStream.Close()

	// Verify the file content has been appended
	written, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, "Hello, World!", string(written))
}

func TestDescriptorAppendViaStream_BadDescriptor(t *testing.T) {
	ctx := createTestContext()

	// Try with invalid handle
	result, err := descriptorAppendViaStream(ctx, []component.Val{
		component.ValBorrow(999),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorAppendViaStream_IsDirectory(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	// Try to append via stream to a directory
	result, err := descriptorAppendViaStream(ctx, []component.Val{
		component.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error result")
	require.Equal(t, "is-directory", errVal.Enum())
}

// ====================
// descriptor.set-size tests
// ====================

func TestDescriptorSetSize_Truncate(t *testing.T) {
	ctx := createTestContext()
	testContent := []byte("hello world")
	handle, path := createTestFileDescriptor(t, ctx, testContent)
	defer os.Remove(path)

	// Truncate to 5 bytes
	result, err := descriptorSetSize(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(5),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-size should succeed")

	// Verify file is now 5 bytes
	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	require.Equal(t, int64(5), info.Size())
}

func TestDescriptorSetSize_Extend(t *testing.T) {
	ctx := createTestContext()
	testContent := []byte("hello")
	handle, path := createTestFileDescriptor(t, ctx, testContent)
	defer os.Remove(path)

	// Extend to 10 bytes
	result, err := descriptorSetSize(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(10),
	})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-size should succeed")

	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	require.Equal(t, int64(10), info.Size())
}

func TestDescriptorSetSize_NoWritePermission(t *testing.T) {
	ctx := createTestContext()
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	tmpFile, err := os.CreateTemp("", "test-readonly-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte("hello"))
	require.NoError(t, err)

	// Create descriptor with read-only flags
	desc := NewDescriptor(tmpFile, false, tmpFile.Name(), DescriptorFlagRead)
	handle := table.New(desc, true)

	result, err := descriptorSetSize(ctx, []component.Val{
		component.ValBorrow(uint32(handle.Index())),
		component.ValU64(0),
	})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-size should fail without write permission")
	require.Equal(t, "access", errVal.Enum())
}

func TestDescriptorSetSize_IsDirectory(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	result, err := descriptorSetSize(ctx, []component.Val{
		component.ValBorrow(handle),
		component.ValU64(0),
	})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-size should fail on directory")
	require.Equal(t, "is-directory", errVal.Enum())
}

func TestDescriptorSetSize_BadDescriptor(t *testing.T) {
	result, err := descriptorSetSize(context.Background(), []component.Val{
		component.ValBorrow(0),
		component.ValU64(0),
	})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-size should fail with bad descriptor")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetTimes_SetToNow(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	// Set old times first
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(path, oldTime, oldTime)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	accessTime := component.ValVariant("now", nil)
	modTime := component.ValVariant("now", nil)

	before := time.Now().Add(-time.Second)
	result, err := descriptorSetTimes(ctx, []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-times should succeed")
	after := time.Now().Add(time.Second)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, info.ModTime().After(before), "mod time should be updated")
	require.True(t, info.ModTime().Before(after), "mod time should be recent")
}

func TestDescriptorSetTimes_SetTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	// timestamp(datetime) where datetime is record { seconds: u64, nanoseconds: u32 }
	targetTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	dt := component.ValRecord(map[string]component.Val{
		"seconds":     component.ValU64(uint64(targetTime.Unix())),
		"nanoseconds": component.ValU32(0),
	})
	accessTime := component.ValVariant("timestamp", &dt)
	modTime := component.ValVariant("timestamp", &dt)

	result, err := descriptorSetTimes(ctx, []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "set-times should succeed")

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, targetTime.Unix(), info.ModTime().Unix())
}

func TestDescriptorSetTimes_NoWritePermission(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello"), 0644)
	require.NoError(t, err)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead) // read-only
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	accessTime := component.ValVariant("now", nil)
	modTime := component.ValVariant("now", nil)
	result, err := descriptorSetTimes(ctx, []component.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-times should fail without write permission")
	require.Equal(t, "access", errVal.Enum())
}

func TestDescriptorLinkAt_CreateHardLink(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(srcPath, []byte("hello"), 0644)
	require.NoError(t, err)

	selfHandle := component.ValBorrow(handle)
	oldPathFlags := component.ValFlags(map[string]bool{}) // no symlink-follow
	oldPath := component.ValString("source.txt")
	newDesc := component.ValBorrow(handle) // same directory
	newPath := component.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, []component.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
	require.NoError(t, err)
	isOk, _, _ := result[0].Result()
	require.True(t, isOk, "link-at should succeed")

	// Verify hard link exists and has same content
	linkContent, err := os.ReadFile(filepath.Join(tmpDir, "link.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(linkContent))
}

func TestDescriptorLinkAt_RejectSymlinkFollow(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(srcPath, []byte("hello"), 0644)

	selfHandle := component.ValBorrow(handle)
	oldPathFlags := component.ValFlags(map[string]bool{"symlink-follow": true}) // should be rejected
	oldPath := component.ValString("source.txt")
	newDesc := component.ValBorrow(handle)
	newPath := component.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, []component.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "link-at should reject symlink-follow")
	require.Equal(t, "invalid", errVal.Enum())
}

func TestDescriptorLinkAt_NoMutateDirectory(t *testing.T) {
	ctx := createTestContext()
	table := component.ResourceTableFromContext(ctx)

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(srcPath, []byte("hello"), 0644)

	// Create descriptor without MutateDirectory flag
	file, err := os.Open(tmpDir)
	require.NoError(t, err)
	desc := NewDescriptor(file, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite)
	h := table.New(desc, true)
	handle := uint32(h.Index())

	selfHandle := component.ValBorrow(handle)
	oldPathFlags := component.ValFlags(map[string]bool{})
	oldPath := component.ValString("source.txt")
	newDesc := component.ValBorrow(handle)
	newPath := component.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, []component.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "link-at should fail without mutate-directory")
	require.Equal(t, "access", errVal.Enum())
}

// ====================
// Descriptor Advise Tests
// ====================

func TestDescriptorAdvise_WithRealFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(path, []byte("hello world test data"), 0644)
	require.NoError(t, err)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead)
	handle := table.New(desc, true)

	selfHandle := component.ValBorrow(uint32(handle))
	offset := component.ValU64(0)
	length := component.ValU64(100)

	// Test each advice variant
	adviceVariants := []string{"normal", "sequential", "random", "will-need", "dont-need", "no-reuse"}
	for _, advice := range adviceVariants {
		adviceVal := component.ValEnum(advice)
		result, err := descriptorAdvise(ctx, []component.Val{selfHandle, offset, length, adviceVal})
		require.NoError(t, err)
		isOk, _, _ := result[0].Result()
		require.True(t, isOk, "advise with %s should succeed", advice)
	}
}

func TestDescriptorAdvise_BadDescriptor(t *testing.T) {
	table := component.NewResourceTable()
	ctx := component.WithResourceTable(context.Background(), table)
	selfHandle := component.ValBorrow(0)
	offset := component.ValU64(0)
	length := component.ValU64(100)
	advice := component.ValEnum("normal")
	result, err := descriptorAdvise(ctx, []component.Val{selfHandle, offset, length, advice})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "advise should fail without valid descriptor")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}
