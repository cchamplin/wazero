// imports/wasip2/filesystem/filesystem_test.go

package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
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
	selfHandle := types.ValBorrow(0)
	offset := types.ValU64(0)
	result, err := descriptorReadViaStream(context.Background(), nil, []types.Val{selfHandle, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorWriteViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>), offset (u64)
	// Returns: result<own<output-stream>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	offset := types.ValU64(0)
	result, err := descriptorWriteViaStream(context.Background(), nil, []types.Val{selfHandle, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorAppendViaStream(t *testing.T) {
	// Args: self (borrow<descriptor>)
	// Returns: result<own<output-stream>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorAppendViaStream(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorAdvise(t *testing.T) {
	// Args: self, offset, length, advice
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	offset := types.ValU64(0)
	length := types.ValU64(0)
	advice := types.ValEnum("normal")
	result, err := descriptorAdvise(context.Background(), nil, []types.Val{selfHandle, offset, length, advice})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSyncData(t *testing.T) {
	// Args: self
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorSyncData(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorGetFlags(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-flags, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorGetFlags(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorGetType(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-type, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorGetType(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetSize(t *testing.T) {
	// Args: self, size
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	size := types.ValU64(0)
	result, err := descriptorSetSize(context.Background(), nil, []types.Val{selfHandle, size})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetTimes(t *testing.T) {
	// Args: self, data-access-timestamp, data-modification-timestamp
	// Returns: result<_, error-code>
	selfHandle := types.ValBorrow(0)
	accessTime := types.ValVariant("no-change", nil)
	modTime := types.ValVariant("no-change", nil)
	result, err := descriptorSetTimes(context.Background(), nil, []types.Val{selfHandle, accessTime, modTime})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorRead(t *testing.T) {
	// Args: self, length, offset
	// Returns: result<tuple<list<u8>, bool>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	length := types.ValU64(100)
	offset := types.ValU64(0)
	result, err := descriptorRead(context.Background(), nil, []types.Val{selfHandle, length, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorWrite(t *testing.T) {
	// Args: self, buffer, offset
	// Returns: result<u64, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	buffer := types.ValList([]types.Val{types.ValU8(65), types.ValU8(66)})
	offset := types.ValU64(0)
	result, err := descriptorWrite(context.Background(), nil, []types.Val{selfHandle, buffer, offset})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorReadDirectory(t *testing.T) {
	// Args: self
	// Returns: result<own<directory-entry-stream>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorReadDirectory(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSync(t *testing.T) {
	// Args: self
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorSync(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorCreateDirectoryAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	path := types.ValString("newdir")
	result, err := descriptorCreateDirectoryAt(context.Background(), nil, []types.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorStat(t *testing.T) {
	// Args: self
	// Returns: result<descriptor-stat, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := descriptorStat(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorStatAt(t *testing.T) {
	// Args: self, path-flags, path
	// Returns: result<descriptor-stat, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	pathFlags := types.ValFlags(map[string]bool{"symlink-follow": true})
	path := types.ValString("file.txt")
	result, err := descriptorStatAt(context.Background(), nil, []types.Val{selfHandle, pathFlags, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSetTimesAt(t *testing.T) {
	// Args: self, path-flags, path, data-access-timestamp, data-modification-timestamp
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	pathFlags := types.ValFlags(map[string]bool{"symlink-follow": true})
	path := types.ValString("file.txt")
	accessTime := types.ValVariant("no-change", nil)
	modTime := types.ValVariant("no-change", nil)
	result, err := descriptorSetTimesAt(context.Background(), nil, []types.Val{selfHandle, pathFlags, path, accessTime, modTime})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
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

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead|DescriptorFlagWrite|DescriptorFlagMutateDirectory)
	handle, errH1 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH1 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH1)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	pathFlags := types.ValFlags(map[string]bool{"symlink-follow": true})
	pathVal := types.ValString("child.txt")
	accessTime := types.ValVariant("now", nil)
	modTime := types.ValVariant("now", nil)

	before := time.Now().Add(-time.Second)
	result, err := descriptorSetTimesAt(ctx, nil, []types.Val{selfHandle, pathFlags, pathVal, accessTime, modTime})
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
	selfHandle := types.ValBorrow(0)
	pathFlags := types.ValFlags(map[string]bool{"symlink-follow": false})
	oldPath := types.ValString("oldfile")
	newDescriptor := types.ValBorrow(1)
	newPath := types.ValString("newfile")
	result, err := descriptorLinkAt(context.Background(), nil, []types.Val{selfHandle, pathFlags, oldPath, newDescriptor, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	// Without a valid context/resource table, this should return bad-descriptor error
	isOk, _, _ := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
}

func TestDescriptorOpenAt(t *testing.T) {
	// Args: self, path-flags, path, open-flags, descriptor-flags
	// Returns: result<own<descriptor>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	pathFlags := types.ValFlags(map[string]bool{"symlink-follow": true})
	path := types.ValString("file.txt")
	openFlags := types.ValFlags(map[string]bool{"create": false})
	descFlags := types.ValFlags(map[string]bool{"read": true})
	result, err := descriptorOpenAt(context.Background(), nil, []types.Val{selfHandle, pathFlags, path, openFlags, descFlags})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorReadlinkAt(t *testing.T) {
	// Args: self, path
	// Returns: result<string, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	path := types.ValString("symlink")
	result, err := descriptorReadlinkAt(context.Background(), nil, []types.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorRemoveDirectoryAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	path := types.ValString("dir")
	result, err := descriptorRemoveDirectoryAt(context.Background(), nil, []types.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorRenameAt(t *testing.T) {
	// Args: self, old-path, new-descriptor, new-path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	oldPath := types.ValString("oldname")
	newDescriptor := types.ValBorrow(1)
	newPath := types.ValString("newname")
	result, err := descriptorRenameAt(context.Background(), nil, []types.Val{selfHandle, oldPath, newDescriptor, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorSymlinkAt(t *testing.T) {
	// Args: self, old-path, new-path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	oldPath := types.ValString("target")
	newPath := types.ValString("link")
	result, err := descriptorSymlinkAt(context.Background(), nil, []types.Val{selfHandle, oldPath, newPath})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorUnlinkFileAt(t *testing.T) {
	// Args: self, path
	// Returns: result<_, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	path := types.ValString("file")
	result, err := descriptorUnlinkFileAt(context.Background(), nil, []types.Val{selfHandle, path})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should return error without valid context")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorIsSameObject(t *testing.T) {
	// Args: self, other
	// Returns: bool
	selfHandle := types.ValBorrow(0)
	otherHandle := types.ValBorrow(1)
	result, err := descriptorIsSameObject(context.Background(), nil, []types.Val{selfHandle, otherHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindBool, result[0].Kind())
}

func TestDescriptorIsSameObject_SameFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	f1, err := os.Open(path)
	require.NoError(t, err)
	defer f1.Close()

	f2, err := os.Open(path)
	require.NoError(t, err)
	defer f2.Close()

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc1 := NewDescriptor(f1, false, path, DescriptorFlagRead)
	desc2 := NewDescriptor(f2, false, path, DescriptorFlagRead)
	h1, errH2 := table.NewResourceHandle(desc1, true, descriptorResourceType)
	if errH2 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH2)
	}
	h2, errH3 := table.NewResourceHandle(desc2, true, descriptorResourceType)
	if errH3 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH3)
	}

	selfHandle := types.ValBorrow(uint32(h1))
	otherHandle := types.ValBorrow(uint32(h2))
	result, err := descriptorIsSameObject(ctx, nil, []types.Val{selfHandle, otherHandle})
	require.NoError(t, err)
	require.True(t, result[0].Bool(), "same file opened twice should be same object")
}

func TestDescriptorIsSameObject_DifferentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "file1.txt")
	path2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(path1, []byte("hello"), 0644)
	os.WriteFile(path2, []byte("world"), 0644)

	f1, err := os.Open(path1)
	require.NoError(t, err)
	defer f1.Close()

	f2, err := os.Open(path2)
	require.NoError(t, err)
	defer f2.Close()

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc1 := NewDescriptor(f1, false, path1, DescriptorFlagRead)
	desc2 := NewDescriptor(f2, false, path2, DescriptorFlagRead)
	h1, errH4 := table.NewResourceHandle(desc1, true, descriptorResourceType)
	if errH4 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH4)
	}
	h2, errH5 := table.NewResourceHandle(desc2, true, descriptorResourceType)
	if errH5 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH5)
	}

	selfHandle := types.ValBorrow(uint32(h1))
	otherHandle := types.ValBorrow(uint32(h2))
	result, err := descriptorIsSameObject(ctx, nil, []types.Val{selfHandle, otherHandle})
	require.NoError(t, err)
	require.False(t, result[0].Bool(), "different files should not be same object")
}

func TestDescriptorMetadataHash(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead)
	handle, errH6 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH6 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH6)
	}

	selfHandle := types.ValBorrow(uint32(handle))

	// Call twice — should produce same hash
	result1, err := descriptorMetadataHash(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk1, hash1, _ := result1[0].Result()
	require.True(t, isOk1)

	result2, err := descriptorMetadataHash(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk2, hash2, _ := result2[0].Result()
	require.True(t, isOk2)

	rec1 := hash1.Record()
	rec2 := hash2.Record()
	require.Equal(t, rec1["lower"].U64(), rec2["lower"].U64(), "lower hash should be stable")
	require.Equal(t, rec1["upper"].U64(), rec2["upper"].U64(), "upper hash should be stable")

	// Hash should not be all zeros
	require.True(t, rec1["lower"].U64() != 0 || rec1["upper"].U64() != 0, "hash should not be zero")
}

func TestDescriptorMetadataHash_DifferentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "file1.txt")
	path2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(path1, []byte("hello"), 0644)
	os.WriteFile(path2, []byte("world"), 0644)

	f1, _ := os.Open(path1)
	defer f1.Close()
	f2, _ := os.Open(path2)
	defer f2.Close()

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	h1, errH7 := table.NewResourceHandle(NewDescriptor(f1, false, path1, DescriptorFlagRead), true, descriptorResourceType)
	if errH7 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH7)
	}
	h2, errH8 := table.NewResourceHandle(NewDescriptor(f2, false, path2, DescriptorFlagRead), true, descriptorResourceType)
	if errH8 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH8)
	}

	result1, _ := descriptorMetadataHash(ctx, nil, []types.Val{types.ValBorrow(uint32(h1))})
	result2, _ := descriptorMetadataHash(ctx, nil, []types.Val{types.ValBorrow(uint32(h2))})

	_, hash1, _ := result1[0].Result()
	_, hash2, _ := result2[0].Result()

	rec1 := hash1.Record()
	rec2 := hash2.Record()
	// Different files should produce different hashes
	require.True(t, rec1["lower"].U64() != rec2["lower"].U64() || rec1["upper"].U64() != rec2["upper"].U64(),
		"different files should produce different hashes")
}

func TestDescriptorMetadataHash_BadDescriptor(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	selfHandle := types.ValBorrow(0)
	result, err := descriptorMetadataHash(ctx, nil, []types.Val{selfHandle})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "should fail with bad descriptor")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}

func TestDescriptorMetadataHashAt(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "child.txt")
	os.WriteFile(filePath, []byte("hello"), 0644)

	dirFile, _ := os.Open(tmpDir)
	defer dirFile.Close()

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(dirFile, true, tmpDir, DescriptorFlagRead)
	handle, errH9 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH9 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH9)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	pathFlags := types.ValFlags(map[string]bool{"symlink-follow": true})
	pathVal := types.ValString("child.txt")

	result, err := descriptorMetadataHashAt(ctx, nil, []types.Val{selfHandle, pathFlags, pathVal})
	require.NoError(t, err)
	isOk, hash, _ := result[0].Result()
	require.True(t, isOk)

	rec := hash.Record()
	require.True(t, rec["lower"].U64() != 0 || rec["upper"].U64() != 0, "hash should not be zero")
}

func TestFilesystemErrorCode(t *testing.T) {
	// Args: err (borrow<error>)
	// Returns: option<error-code>
	errHandle := types.ValBorrow(0)
	result, err := filesystemErrorCode(context.Background(), nil, []types.Val{errHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindOption, result[0].Kind())
}

func TestFilesystemErrorCode_WithFSError(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an io.Error that wraps a FilesystemError
	fsErr := &FilesystemError{Code: ErrorCodeAccess}
	ioErr := wasipIO.NewError(fsErr)
	handle, errH10 := table.NewResourceHandle(ioErr, true, descriptorResourceType)
	if errH10 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH10)
	}

	errHandle := types.ValBorrow(uint32(handle))
	result, err := filesystemErrorCode(ctx, nil, []types.Val{errHandle})
	require.NoError(t, err)

	opt := result[0].Option()
	require.NotNil(t, opt, "should return Some for filesystem error")
	require.Equal(t, "access", opt.Enum())
}

func TestFilesystemErrorCode_NonFSError(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)

	// Create an io.Error that wraps a plain Go error (not a FilesystemError)
	ioErr := wasipIO.NewError(errors.New("some random error"))
	handle, errH11 := table.NewResourceHandle(ioErr, true, descriptorResourceType)
	if errH11 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH11)
	}

	errHandle := types.ValBorrow(uint32(handle))
	result, err := filesystemErrorCode(ctx, nil, []types.Val{errHandle})
	require.NoError(t, err)

	opt := result[0].Option()
	require.Nil(t, opt, "should return None for non-filesystem error")
}

func TestDirectoryEntryStreamReadEntry(t *testing.T) {
	// Args: self
	// Returns: result<option<directory-entry>, error-code>
	// Without a valid context/resource table, this should return bad-descriptor error
	selfHandle := types.ValBorrow(0)
	result, err := directoryEntryStreamReadEntry(context.Background(), nil, []types.Val{selfHandle})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())
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
	result, err := getDirectories(context.Background(), nil, []types.Val{})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindList, result[0].Kind())
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
	table := runtime.NewTable()
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
	handle, errH12 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH12 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH12)
	}
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
	handle, errH13 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH13 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH13)
	}
	return uint32(handle.Index()), path
}

func TestDescriptorRead_HostFunction(t *testing.T) {
	ctx := createTestContext()
	testContent := []byte("Hello, World!")
	handle, path := createTestFileDescriptor(t, ctx, testContent)
	defer os.Remove(path)

	// Test reading from file
	result, err := descriptorRead(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(uint64(len(testContent))),
		types.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindTuple, ok.Kind())

	tuple := ok.Tuple()
	require.Equal(t, 2, len(tuple))
	require.Equal(t, types.ValKindList, tuple[0].Kind())
	require.Equal(t, types.ValKindBool, tuple[1].Kind())

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
	dataVals := make([]types.Val, len(data))
	for i, b := range data {
		dataVals[i] = types.ValU8(b)
	}

	result, err := descriptorWrite(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValList(dataVals),
		types.ValU64(0),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindU64, ok.Kind())
	require.Equal(t, uint64(len(data)), ok.U64())

	// Verify content was written
	written, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, written)
}

func TestDescriptorStat_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	result, err := descriptorStat(ctx, nil, []types.Val{
		types.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindRecord, ok.Kind())

	// Check type field
	typeField, hasType := ok.RecordField("type")
	require.True(t, hasType)
	require.Equal(t, types.ValKindEnum, typeField.Kind())
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

	result, err := descriptorStatAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValFlags(map[string]bool{"symlink-follow": true}),
		types.ValString("testfile.txt"),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindRecord, ok.Kind())

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

	result, err := descriptorGetType(ctx, nil, []types.Val{
		types.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindEnum, ok.Kind())
	require.Equal(t, "directory", ok.Enum())
}

func TestDescriptorOpenAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	// Create a test file to open
	testFile := filepath.Join(tmpDir, "opentest.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	result, err := descriptorOpenAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValFlags(map[string]bool{"symlink-follow": true}),
		types.ValString("opentest.txt"),
		types.ValFlags(map[string]bool{}),
		types.ValFlags(map[string]bool{"read": true}),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())
}

func TestDescriptorOpenAt_CreateFile(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	result, err := descriptorOpenAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValFlags(map[string]bool{"symlink-follow": true}),
		types.ValString("newfile.txt"),
		types.ValFlags(map[string]bool{"create": true}),
		types.ValFlags(map[string]bool{"read": true, "write": true}),
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

	result, err := descriptorReadDirectory(ctx, nil, []types.Val{
		types.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())

	// Now read entries from the stream
	streamHandle := ok.Own()

	// Read first entry
	entryResult, err := directoryEntryStreamReadEntry(ctx, nil, []types.Val{
		types.ValBorrow(streamHandle),
	})
	require.NoError(t, err)
	isOk, ok, _ = entryResult[0].Result()
	require.True(t, isOk)
	require.NotNil(t, ok.Option()) // Should have an entry
}

func TestDescriptorCreateDirectoryAt_HostFunction(t *testing.T) {
	ctx := createTestContext()
	handle, tmpDir := createTestDirDescriptor(t, ctx)

	result, err := descriptorCreateDirectoryAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValString("subdir"),
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

	result, err := descriptorUnlinkFileAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValString("todelete.txt"),
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

	result, err := descriptorRemoveDirectoryAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValString("toremove"),
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

	result, err := descriptorRenameAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValString("oldname.txt"),
		types.ValBorrow(handle), // same directory
		types.ValString("newname.txt"),
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

	result, err := descriptorSync(ctx, nil, []types.Val{
		types.ValBorrow(handle),
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

	result, err := descriptorGetFlags(ctx, nil, []types.Val{
		types.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindFlags, ok.Kind())

	flags := ok.Flags()
	require.True(t, flags["read"])
	require.True(t, flags["write"])
}

// Test error cases

func TestDescriptorRead_BadDescriptor(t *testing.T) {
	ctx := createTestContext()

	// Try to read with non-existent handle
	result, err := descriptorRead(ctx, nil, []types.Val{
		types.ValBorrow(999), // invalid handle
		types.ValU64(100),
		types.ValU64(0),
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
	result, err := descriptorOpenAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValFlags(map[string]bool{"symlink-follow": true}),
		types.ValString("nonexistent.txt"),
		types.ValFlags(map[string]bool{}), // no create flag
		types.ValFlags(map[string]bool{"read": true}),
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

	result, err := descriptorStatAt(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValFlags(map[string]bool{"symlink-follow": true}),
		types.ValString("nonexistent.txt"),
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
	result, err := descriptorReadViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(0), // offset
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())

	// Get the input stream handle
	streamHandle := ok.Own()

	// Now read from the stream using the io package functions
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	// Lookup the stream and read from it
	rawEntry1, err := table.Get(runtime.Handle(streamHandle))
	entry, _ := rawEntry1.(*runtime.ResourceHandleEntry)
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
	result, err := descriptorReadViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(7), // offset
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
	rawEntry2, err := table.Get(runtime.Handle(streamHandle))
	entry, _ := rawEntry2.(*runtime.ResourceHandleEntry)
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
	result, err := descriptorReadViaStream(ctx, nil, []types.Val{
		types.ValBorrow(999),
		types.ValU64(0),
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
	result, err := descriptorReadViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(0),
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
	result, err := descriptorWriteViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(0), // offset
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())

	// Get the output stream handle
	streamHandle := ok.Own()

	// Verify the stream can be used to write
	table := component.ResourceTableFromContext(ctx)
	require.NotNil(t, table)

	rawEntry3, err := table.Get(runtime.Handle(streamHandle))
	entry, _ := rawEntry3.(*runtime.ResourceHandleEntry)
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
	result, err := descriptorWriteViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(6), // offset
	})
	require.NoError(t, err)

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")

	// Get the output stream handle
	streamHandle := ok.Own()

	// Get the stream and write
	table := component.ResourceTableFromContext(ctx)
	rawEntry4, err := table.Get(runtime.Handle(streamHandle))
	entry, _ := rawEntry4.(*runtime.ResourceHandleEntry)
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
	result, err := descriptorWriteViaStream(ctx, nil, []types.Val{
		types.ValBorrow(999),
		types.ValU64(0),
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
	result, err := descriptorWriteViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(0),
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
	result, err := descriptorAppendViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, types.ValKindResult, result[0].Kind())

	isOk, ok, _ := result[0].Result()
	require.True(t, isOk, "should return ok result")
	require.NotNil(t, ok)
	require.Equal(t, types.ValKindOwn, ok.Kind())

	// Get the output stream handle
	streamHandle := ok.Own()

	// Verify the stream can be used to append
	table := component.ResourceTableFromContext(ctx)
	rawEntry5, err := table.Get(runtime.Handle(streamHandle))
	entry, _ := rawEntry5.(*runtime.ResourceHandleEntry)
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
	result, err := descriptorAppendViaStream(ctx, nil, []types.Val{
		types.ValBorrow(999),
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
	result, err := descriptorAppendViaStream(ctx, nil, []types.Val{
		types.ValBorrow(handle),
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
	result, err := descriptorSetSize(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(5),
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
	result, err := descriptorSetSize(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(10),
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
	handle, errH14 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH14 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH14)
	}

	result, err := descriptorSetSize(ctx, nil, []types.Val{
		types.ValBorrow(uint32(handle.Index())),
		types.ValU64(0),
	})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-size should fail without write permission")
	require.Equal(t, "access", errVal.Enum())
}

func TestDescriptorSetSize_IsDirectory(t *testing.T) {
	ctx := createTestContext()
	handle, _ := createTestDirDescriptor(t, ctx)

	result, err := descriptorSetSize(ctx, nil, []types.Val{
		types.ValBorrow(handle),
		types.ValU64(0),
	})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "set-size should fail on directory")
	require.Equal(t, "is-directory", errVal.Enum())
}

func TestDescriptorSetSize_BadDescriptor(t *testing.T) {
	result, err := descriptorSetSize(context.Background(), nil, []types.Val{
		types.ValBorrow(0),
		types.ValU64(0),
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

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle, errH15 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH15 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH15)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	accessTime := types.ValVariant("now", nil)
	modTime := types.ValVariant("now", nil)

	before := time.Now().Add(-time.Second)
	result, err := descriptorSetTimes(ctx, nil, []types.Val{selfHandle, accessTime, modTime})
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

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead|DescriptorFlagWrite)
	handle, errH16 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH16 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH16)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	// timestamp(datetime) where datetime is record { seconds: u64, nanoseconds: u32 }
	targetTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	dt := types.ValRecord(map[string]types.Val{
		"seconds":     types.ValU64(uint64(targetTime.Unix())),
		"nanoseconds": types.ValU32(0),
	})
	accessTime := types.ValVariant("timestamp", &dt)
	modTime := types.ValVariant("timestamp", &dt)

	result, err := descriptorSetTimes(ctx, nil, []types.Val{selfHandle, accessTime, modTime})
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

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead) // read-only
	handle, errH17 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH17 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH17)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	accessTime := types.ValVariant("now", nil)
	modTime := types.ValVariant("now", nil)
	result, err := descriptorSetTimes(ctx, nil, []types.Val{selfHandle, accessTime, modTime})
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

	selfHandle := types.ValBorrow(handle)
	oldPathFlags := types.ValFlags(map[string]bool{}) // no symlink-follow
	oldPath := types.ValString("source.txt")
	newDesc := types.ValBorrow(handle) // same directory
	newPath := types.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, nil, []types.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
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

	selfHandle := types.ValBorrow(handle)
	oldPathFlags := types.ValFlags(map[string]bool{"symlink-follow": true}) // should be rejected
	oldPath := types.ValString("source.txt")
	newDesc := types.ValBorrow(handle)
	newPath := types.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, nil, []types.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
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
	h, errH18 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH18 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH18)
	}
	handle := uint32(h.Index())

	selfHandle := types.ValBorrow(handle)
	oldPathFlags := types.ValFlags(map[string]bool{})
	oldPath := types.ValString("source.txt")
	newDesc := types.ValBorrow(handle)
	newPath := types.ValString("link.txt")

	result, err := descriptorLinkAt(ctx, nil, []types.Val{selfHandle, oldPathFlags, oldPath, newDesc, newPath})
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

	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	desc := NewDescriptor(f, false, path, DescriptorFlagRead)
	handle, errH19 := table.NewResourceHandle(desc, true, descriptorResourceType)
	if errH19 != nil {
		t.Fatalf("NewResourceHandle failed: %v", errH19)
	}

	selfHandle := types.ValBorrow(uint32(handle))
	offset := types.ValU64(0)
	length := types.ValU64(100)

	// Test each advice variant
	adviceVariants := []string{"normal", "sequential", "random", "will-need", "dont-need", "no-reuse"}
	for _, advice := range adviceVariants {
		adviceVal := types.ValEnum(advice)
		result, err := descriptorAdvise(ctx, nil, []types.Val{selfHandle, offset, length, adviceVal})
		require.NoError(t, err)
		isOk, _, _ := result[0].Result()
		require.True(t, isOk, "advise with %s should succeed", advice)
	}
}

func TestDescriptorAdvise_BadDescriptor(t *testing.T) {
	table := runtime.NewTable()
	ctx := component.WithResourceTable(context.Background(), table)
	selfHandle := types.ValBorrow(0)
	offset := types.ValU64(0)
	length := types.ValU64(100)
	advice := types.ValEnum("normal")
	result, err := descriptorAdvise(ctx, nil, []types.Val{selfHandle, offset, length, advice})
	require.NoError(t, err)
	isOk, _, errVal := result[0].Result()
	require.False(t, isOk, "advise should fail without valid descriptor")
	require.Equal(t, "bad-descriptor", errVal.Enum())
}
