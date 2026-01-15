// Package filesystem implements the wasi:filesystem interfaces for WASI Preview 2.
// It provides filesystem access through descriptors and preopened directories.
package filesystem

import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// Instantiate registers all wasi:filesystem interfaces with the linker.
func Instantiate(linker *component.Linker) error {
	if err := instantiateTypes(linker); err != nil {
		return err
	}
	if err := instantiatePreopens(linker); err != nil {
		return err
	}
	return nil
}

// instantiateTypes registers wasi:filesystem/types@0.2.0
func instantiateTypes(linker *component.Linker) error {
	inst := linker.DefineInstance("wasi:filesystem/types@0.2.0")

	// descriptor resource
	inst.Resource("descriptor", func(rep uint32) {
		// Destructor - close descriptor
		// Full implementation will look up descriptor in ResourceTable and close it
	})

	// descriptor methods
	inst.FuncNoType("[method]descriptor.read-via-stream", descriptorReadViaStream)
	inst.FuncNoType("[method]descriptor.write-via-stream", descriptorWriteViaStream)
	inst.FuncNoType("[method]descriptor.append-via-stream", descriptorAppendViaStream)
	inst.FuncNoType("[method]descriptor.advise", descriptorAdvise)
	inst.FuncNoType("[method]descriptor.sync-data", descriptorSyncData)
	inst.FuncNoType("[method]descriptor.get-flags", descriptorGetFlags)
	inst.FuncNoType("[method]descriptor.get-type", descriptorGetType)
	inst.FuncNoType("[method]descriptor.set-size", descriptorSetSize)
	inst.FuncNoType("[method]descriptor.set-times", descriptorSetTimes)
	inst.FuncNoType("[method]descriptor.read", descriptorRead)
	inst.FuncNoType("[method]descriptor.write", descriptorWrite)
	inst.FuncNoType("[method]descriptor.read-directory", descriptorReadDirectory)
	inst.FuncNoType("[method]descriptor.sync", descriptorSync)
	inst.FuncNoType("[method]descriptor.create-directory-at", descriptorCreateDirectoryAt)
	inst.FuncNoType("[method]descriptor.stat", descriptorStat)
	inst.FuncNoType("[method]descriptor.stat-at", descriptorStatAt)
	inst.FuncNoType("[method]descriptor.set-times-at", descriptorSetTimesAt)
	inst.FuncNoType("[method]descriptor.link-at", descriptorLinkAt)
	inst.FuncNoType("[method]descriptor.open-at", descriptorOpenAt)
	inst.FuncNoType("[method]descriptor.readlink-at", descriptorReadlinkAt)
	inst.FuncNoType("[method]descriptor.remove-directory-at", descriptorRemoveDirectoryAt)
	inst.FuncNoType("[method]descriptor.rename-at", descriptorRenameAt)
	inst.FuncNoType("[method]descriptor.symlink-at", descriptorSymlinkAt)
	inst.FuncNoType("[method]descriptor.unlink-file-at", descriptorUnlinkFileAt)
	inst.FuncNoType("[method]descriptor.is-same-object", descriptorIsSameObject)
	inst.FuncNoType("[method]descriptor.metadata-hash", descriptorMetadataHash)
	inst.FuncNoType("[method]descriptor.metadata-hash-at", descriptorMetadataHashAt)

	// filesystem-error-code function
	inst.FuncNoType("filesystem-error-code", filesystemErrorCode)

	// directory-entry-stream resource
	inst.Resource("directory-entry-stream", func(rep uint32) {
		// Destructor - release directory entry stream
	})
	inst.FuncNoType("[method]directory-entry-stream.read-directory-entry", directoryEntryStreamReadEntry)

	return inst.Build()
}

// descriptorReadViaStream returns an input-stream for reading from a descriptor at an offset.
// Signature: func(self: borrow<descriptor>, offset: u64) -> result<own<input-stream>, error-code>
func descriptorReadViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder input-stream handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// descriptorWriteViaStream returns an output-stream for writing to a descriptor at an offset.
// Signature: func(self: borrow<descriptor>, offset: u64) -> result<own<output-stream>, error-code>
func descriptorWriteViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder output-stream handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// descriptorAppendViaStream returns an output-stream for appending to a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<own<output-stream>, error-code>
func descriptorAppendViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder output-stream handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// descriptorAdvise provides advice about expected access patterns.
// Signature: func(self: borrow<descriptor>, offset: u64, length: u64, advice: advice) -> result<_, error-code>
func descriptorAdvise(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// No-op stub - advice is an optimization hint
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorSyncData synchronizes file data to storage.
// Signature: func(self: borrow<descriptor>) -> result<_, error-code>
func descriptorSyncData(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorGetFlags returns the flags of a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<descriptor-flags, error-code>
func descriptorGetFlags(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder flags (read + write)
	flags := component.ValFlags(map[string]bool{
		"read":  true,
		"write": true,
	})
	return []component.Val{component.ValResultOk(&flags)}, nil
}

// descriptorGetType returns the type of a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<descriptor-type, error-code>
func descriptorGetType(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return "directory" as default placeholder
	dt := component.ValEnum("directory")
	return []component.Val{component.ValResultOk(&dt)}, nil
}

// descriptorSetSize sets the size of a file.
// Signature: func(self: borrow<descriptor>, size: u64) -> result<_, error-code>
func descriptorSetSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorSetTimes sets the access and modification times of a file.
// Signature: func(self: borrow<descriptor>, access: new-timestamp, modification: new-timestamp) -> result<_, error-code>
func descriptorSetTimes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorRead reads bytes from a file at an offset.
// Signature: func(self: borrow<descriptor>, length: u64, offset: u64) -> result<tuple<list<u8>, bool>, error-code>
func descriptorRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty bytes + EOF as placeholder
	emptyList := component.ValList([]component.Val{})
	eof := component.ValBool(true)
	tuple := component.ValTuple([]component.Val{emptyList, eof})
	return []component.Val{component.ValResultOk(&tuple)}, nil
}

// descriptorWrite writes bytes to a file at an offset.
// Signature: func(self: borrow<descriptor>, buffer: list<u8>, offset: u64) -> result<u64, error-code>
func descriptorWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return the length written (pretend success)
	// args[1] is the buffer list
	var length uint64 = 0
	if len(args) > 1 && args[1].Kind() == component.ValKindList {
		length = uint64(len(args[1].List()))
	}
	written := component.ValU64(length)
	return []component.Val{component.ValResultOk(&written)}, nil
}

// descriptorReadDirectory returns a directory entry stream for reading directory contents.
// Signature: func(self: borrow<descriptor>) -> result<own<directory-entry-stream>, error-code>
func descriptorReadDirectory(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder directory-entry-stream handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// descriptorSync synchronizes file data and metadata to storage.
// Signature: func(self: borrow<descriptor>) -> result<_, error-code>
func descriptorSync(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorCreateDirectoryAt creates a directory relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path: string) -> result<_, error-code>
func descriptorCreateDirectoryAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorStat returns metadata about a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<descriptor-stat, error-code>
func descriptorStat(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return zeroed metadata as placeholder
	stat := component.ValRecord(map[string]component.Val{
		"type":                        component.ValEnum("directory"),
		"link-count":                  component.ValU64(1),
		"size":                        component.ValU64(0),
		"data-access-timestamp":       component.ValOption(nil),
		"data-modification-timestamp": component.ValOption(nil),
		"status-change-timestamp":     component.ValOption(nil),
	})
	return []component.Val{component.ValResultOk(&stat)}, nil
}

// descriptorStatAt returns metadata about a file relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string) -> result<descriptor-stat, error-code>
func descriptorStatAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return zeroed metadata as placeholder
	stat := component.ValRecord(map[string]component.Val{
		"type":                        component.ValEnum("regular-file"),
		"link-count":                  component.ValU64(1),
		"size":                        component.ValU64(0),
		"data-access-timestamp":       component.ValOption(nil),
		"data-modification-timestamp": component.ValOption(nil),
		"status-change-timestamp":     component.ValOption(nil),
	})
	return []component.Val{component.ValResultOk(&stat)}, nil
}

// descriptorSetTimesAt sets the times of a file relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string, access: new-timestamp, modification: new-timestamp) -> result<_, error-code>
func descriptorSetTimesAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorLinkAt creates a hard link relative to descriptors.
// Signature: func(self: borrow<descriptor>, old-path-flags: path-flags, old-path: string, new-descriptor: borrow<descriptor>, new-path: string) -> result<_, error-code>
func descriptorLinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorOpenAt opens a file relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string, open-flags: open-flags, descriptor-flags: descriptor-flags) -> result<own<descriptor>, error-code>
func descriptorOpenAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return placeholder descriptor handle
	handle := component.ValOwn(0)
	return []component.Val{component.ValResultOk(&handle)}, nil
}

// descriptorReadlinkAt reads the target of a symbolic link.
// Signature: func(self: borrow<descriptor>, path: string) -> result<string, error-code>
func descriptorReadlinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return empty string as placeholder
	target := component.ValString("")
	return []component.Val{component.ValResultOk(&target)}, nil
}

// descriptorRemoveDirectoryAt removes a directory relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path: string) -> result<_, error-code>
func descriptorRemoveDirectoryAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorRenameAt renames a file or directory.
// Signature: func(self: borrow<descriptor>, old-path: string, new-descriptor: borrow<descriptor>, new-path: string) -> result<_, error-code>
func descriptorRenameAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorSymlinkAt creates a symbolic link.
// Signature: func(self: borrow<descriptor>, old-path: string, new-path: string) -> result<_, error-code>
func descriptorSymlinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorUnlinkFileAt removes a file.
// Signature: func(self: borrow<descriptor>, path: string) -> result<_, error-code>
func descriptorUnlinkFileAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Stub - return success
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorIsSameObject compares two descriptors for identity.
// Signature: func(self: borrow<descriptor>, other: borrow<descriptor>) -> bool
func descriptorIsSameObject(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Compare handle values - for placeholder, compare the borrow values
	if len(args) >= 2 {
		selfHandle := args[0].Borrow()
		otherHandle := args[1].Borrow()
		return []component.Val{component.ValBool(selfHandle == otherHandle)}, nil
	}
	return []component.Val{component.ValBool(false)}, nil
}

// descriptorMetadataHash returns a hash of file metadata.
// Signature: func(self: borrow<descriptor>) -> result<metadata-hash-value, error-code>
func descriptorMetadataHash(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return zeroed hash as placeholder
	hash := component.ValRecord(map[string]component.Val{
		"lower": component.ValU64(0),
		"upper": component.ValU64(0),
	})
	return []component.Val{component.ValResultOk(&hash)}, nil
}

// descriptorMetadataHashAt returns a hash of file metadata for a path.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string) -> result<metadata-hash-value, error-code>
func descriptorMetadataHashAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return zeroed hash as placeholder
	hash := component.ValRecord(map[string]component.Val{
		"lower": component.ValU64(0),
		"upper": component.ValU64(0),
	})
	return []component.Val{component.ValResultOk(&hash)}, nil
}

// filesystemErrorCode converts an error to a filesystem error code.
// Signature: func(err: borrow<error>) -> option<error-code>
func filesystemErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None - no error code available for this error
	return []component.Val{component.ValOption(nil)}, nil
}

// directoryEntryStreamReadEntry reads the next entry from a directory stream.
// Signature: func(self: borrow<directory-entry-stream>) -> result<option<directory-entry>, error-code>
func directoryEntryStreamReadEntry(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// Return None - no more entries (empty directory placeholder)
	none := component.ValOption(nil)
	return []component.Val{component.ValResultOk(&none)}, nil
}
