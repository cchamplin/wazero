// Package filesystem implements the wasi:filesystem interfaces for WASI Preview 2.
// It provides filesystem access through descriptors and preopened directories.
package filesystem

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	goio "io"
	"os"
	"path/filepath"
	"time"

	wasipIO "github.com/tetratelabs/wazero/imports/wasip2/io"
	"github.com/tetratelabs/wazero/internal/component"
)

// getDescriptor retrieves a Descriptor from the ResourceTable using a handle.
func getDescriptor(ctx context.Context, handle uint32) (*Descriptor, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, errors.New("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, err
	}
	desc, ok := entry.Rep.(*Descriptor)
	if !ok {
		return nil, errors.New("handle is not a Descriptor")
	}
	return desc, nil
}

// getDirEntryStream retrieves a DirectoryEntryStream from the ResourceTable using a handle.
func getDirEntryStream(ctx context.Context, handle uint32) (*DirectoryEntryStream, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return nil, errors.New("no resource table in context")
	}
	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return nil, err
	}
	stream, ok := entry.Rep.(*DirectoryEntryStream)
	if !ok {
		return nil, errors.New("handle is not a DirectoryEntryStream")
	}
	return stream, nil
}

// FilesystemError wraps an ErrorCode so it can be stored in io.Error and extracted later.
type FilesystemError struct {
	Code ErrorCode
}

func (e *FilesystemError) Error() string {
	return string(e.Code)
}

// errorResult creates a result<_, error-code> error value.
func errorResult(code ErrorCode) []component.Val {
	errVal := component.ValEnum(string(code))
	return []component.Val{component.ValResultError(&errVal)}
}

// bytesToListU8 converts a byte slice to a component.Val list of u8.
func bytesToListU8(data []byte) component.Val {
	vals := make([]component.Val, len(data))
	for i, b := range data {
		vals[i] = component.ValU8(b)
	}
	return component.ValList(vals)
}

// listU8ToBytes converts a component.Val list of u8 to a byte slice.
func listU8ToBytes(list component.Val) []byte {
	vals := list.List()
	data := make([]byte, len(vals))
	for i, v := range vals {
		data[i] = v.U8()
	}
	return data
}

// fileInfoToDescriptorType converts os.FileInfo mode to DescriptorType.
func fileInfoToDescriptorType(info os.FileInfo) DescriptorType {
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return DescriptorTypeDirectory
	case mode.IsRegular():
		return DescriptorTypeRegularFile
	case mode&os.ModeSymlink != 0:
		return DescriptorTypeSymbolicLink
	case mode&os.ModeDevice != 0:
		if mode&os.ModeCharDevice != 0 {
			return DescriptorTypeCharacterDevice
		}
		return DescriptorTypeBlockDevice
	case mode&os.ModeNamedPipe != 0:
		return DescriptorTypeFifo
	case mode&os.ModeSocket != 0:
		return DescriptorTypeSocket
	default:
		return DescriptorTypeUnknown
	}
}

// fileInfoToStat converts os.FileInfo to a descriptor-stat record Val.
func fileInfoToStat(info os.FileInfo) component.Val {
	descType := fileInfoToDescriptorType(info)
	modTime := info.ModTime()
	modTimestamp := component.ValRecord(map[string]component.Val{
		"seconds":     component.ValU64(uint64(modTime.Unix())),
		"nanoseconds": component.ValU32(uint32(modTime.Nanosecond())),
	})
	modOpt := component.ValOption(&modTimestamp)

	stat := component.ValRecord(map[string]component.Val{
		"type":                        component.ValEnum(descType.String()),
		"link-count":                  component.ValU64(1), // Not easily available on all platforms
		"size":                        component.ValU64(uint64(info.Size())),
		"data-access-timestamp":       component.ValOption(nil), // Not easily available
		"data-modification-timestamp": modOpt,
		"status-change-timestamp":     component.ValOption(nil), // Not easily available
	})
	return stat
}

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

	return inst.SkipValidation().Build()
}

// descriptorReadViaStream returns an input-stream for reading from a descriptor at an offset.
// Signature: func(self: borrow<descriptor>, offset: u64) -> result<own<input-stream>, error-code>
func descriptorReadViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	descHandle := args[0].Borrow()
	offset := args[1].U64()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	desc, err := getDescriptor(ctx, descHandle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Cannot read from a directory
	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	// Check read permission
	if !desc.Flags().HasRead() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Open a new file handle for reading at the specified offset
	file, err := os.Open(desc.Path())
	if err != nil {
		return errorResult(MapOSError(err)), nil
	}

	// Seek to the specified offset
	if offset > 0 {
		_, err = file.Seek(int64(offset), goio.SeekStart)
		if err != nil {
			file.Close()
			return errorResult(MapOSError(err)), nil
		}
	}

	// Create an input stream from the file
	inputStream := wasipIO.NewInputStream(file)

	// Add the stream to the resource table
	streamHandle := table.New(inputStream, true)
	handleVal := component.ValOwn(uint32(streamHandle.Index()))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// descriptorWriteViaStream returns an output-stream for writing to a descriptor at an offset.
// Signature: func(self: borrow<descriptor>, offset: u64) -> result<own<output-stream>, error-code>
func descriptorWriteViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	descHandle := args[0].Borrow()
	offset := args[1].U64()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	desc, err := getDescriptor(ctx, descHandle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Cannot write to a directory
	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	// Check write permission
	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Open a new file handle for writing at the specified offset
	file, err := os.OpenFile(desc.Path(), os.O_WRONLY, 0)
	if err != nil {
		return errorResult(MapOSError(err)), nil
	}

	// Seek to the specified offset
	if offset > 0 {
		_, err = file.Seek(int64(offset), goio.SeekStart)
		if err != nil {
			file.Close()
			return errorResult(MapOSError(err)), nil
		}
	}

	// Create an output stream from the file
	outputStream := wasipIO.NewOutputStream(file)

	// Add the stream to the resource table
	streamHandle := table.New(outputStream, true)
	handleVal := component.ValOwn(uint32(streamHandle.Index()))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// descriptorAppendViaStream returns an output-stream for appending to a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<own<output-stream>, error-code>
func descriptorAppendViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
	descHandle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	desc, err := getDescriptor(ctx, descHandle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Cannot append to a directory
	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	// Check write permission
	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Open a new file handle in append mode
	file, err := os.OpenFile(desc.Path(), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return errorResult(MapOSError(err)), nil
	}

	// Create an output stream from the file
	outputStream := wasipIO.NewOutputStream(file)

	// Add the stream to the resource table
	streamHandle := table.New(outputStream, true)
	handleVal := component.ValOwn(uint32(streamHandle.Index()))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// descriptorAdvise provides advice about expected access patterns.
// Signature: func(self: borrow<descriptor>, offset: u64, length: u64, advice: advice) -> result<_, error-code>
func descriptorAdvise(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	offset := args[1].U64()
	length := args[2].U64()
	advice := args[3].Enum()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.File() == nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if advErr := fadvise(desc.File(), offset, length, advice); advErr != nil {
		return errorResult(MapOSError(advErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorSyncData synchronizes file data to storage.
// Signature: func(self: borrow<descriptor>) -> result<_, error-code>
func descriptorSyncData(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.File() != nil {
		if syncErr := desc.File().Sync(); syncErr != nil {
			return errorResult(MapOSError(syncErr)), nil
		}
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorGetFlags returns the flags of a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<descriptor-flags, error-code>
func descriptorGetFlags(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Convert descriptor flags to component flags
	descFlags := desc.Flags()
	flags := component.ValFlags(map[string]bool{
		"read":                 descFlags.HasRead(),
		"write":                descFlags.HasWrite(),
		"file-integrity-sync":  descFlags&DescriptorFlagFileIntegritySync != 0,
		"data-integrity-sync":  descFlags&DescriptorFlagDataIntegritySync != 0,
		"requested-write-sync": descFlags&DescriptorFlagRequestedWriteSync != 0,
		"mutate-directory":     descFlags&DescriptorFlagMutateDirectory != 0,
	})
	return []component.Val{component.ValResultOk(&flags)}, nil
}

// descriptorGetType returns the type of a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<descriptor-type, error-code>
func descriptorGetType(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Get file info
	info, statErr := desc.File().Stat()
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	descType := fileInfoToDescriptorType(info)
	dt := component.ValEnum(descType.String())
	return []component.Val{component.ValResultOk(&dt)}, nil
}

// descriptorSetSize sets the size of a file.
// Signature: func(self: borrow<descriptor>, size: u64) -> result<_, error-code>
func descriptorSetSize(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	size := args[1].U64()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	if truncErr := desc.File().Truncate(int64(size)); truncErr != nil {
		return errorResult(MapOSError(truncErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// parseNewTimestamp extracts a time.Time from a new-timestamp variant.
// The variant is: no-change | now | timestamp(datetime)
// For no-change, returns the provided fallback time.
func parseNewTimestamp(v component.Val, fallback time.Time) time.Time {
	caseName, payload := v.Variant()
	switch caseName {
	case "no-change":
		return fallback
	case "now":
		return time.Now()
	case "timestamp":
		rec := payload.Record()
		seconds := rec["seconds"].U64()
		nanoseconds := rec["nanoseconds"].U32()
		return time.Unix(int64(seconds), int64(nanoseconds))
	default:
		return fallback
	}
}

// descriptorSetTimes sets the access and modification times of a file.
// Signature: func(self: borrow<descriptor>, access: new-timestamp, modification: new-timestamp) -> result<_, error-code>
func descriptorSetTimes(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	accessTimeArg := args[1]
	modTimeArg := args[2]

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Get current times as fallback for no-change
	info, statErr := desc.File().Stat()
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}
	currentModTime := info.ModTime()
	// Use mod time as fallback for access time since Go doesn't expose atime easily
	currentAtime := currentModTime

	atime := parseNewTimestamp(accessTimeArg, currentAtime)
	mtime := parseNewTimestamp(modTimeArg, currentModTime)

	if chtErr := os.Chtimes(desc.Path(), atime, mtime); chtErr != nil {
		return errorResult(MapOSError(chtErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorRead reads bytes from a file at an offset.
// Signature: func(self: borrow<descriptor>, length: u64, offset: u64) -> result<tuple<list<u8>, bool>, error-code>
func descriptorRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	length := args[1].U64()
	offset := args[2].U64()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Check if this is a directory
	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	// Check read permission
	if !desc.Flags().HasRead() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Read from file at offset
	buf := make([]byte, length)
	n, readErr := desc.File().ReadAt(buf, int64(offset))

	// Determine EOF status
	eof := false
	if readErr != nil {
		if readErr == goio.EOF {
			eof = true
		} else {
			return errorResult(MapOSError(readErr)), nil
		}
	}
	// Also EOF if we read less than requested
	if uint64(n) < length {
		eof = true
	}

	// Return result<tuple<list<u8>, bool>, error-code>
	bytesVal := bytesToListU8(buf[:n])
	eofVal := component.ValBool(eof)
	tuple := component.ValTuple([]component.Val{bytesVal, eofVal})
	return []component.Val{component.ValResultOk(&tuple)}, nil
}

// descriptorWrite writes bytes to a file at an offset.
// Signature: func(self: borrow<descriptor>, buffer: list<u8>, offset: u64) -> result<u64, error-code>
func descriptorWrite(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	buffer := args[1]
	offset := args[2].U64()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Check if this is a directory
	if desc.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	// Check write permission
	if !desc.Flags().HasWrite() {
		return errorResult(ErrorCodeAccess), nil
	}

	// Convert buffer to bytes
	data := listU8ToBytes(buffer)

	// Write to file at offset
	n, writeErr := desc.File().WriteAt(data, int64(offset))
	if writeErr != nil {
		return errorResult(MapOSError(writeErr)), nil
	}

	// Return result<u64, error-code>
	written := component.ValU64(uint64(n))
	return []component.Val{component.ValResultOk(&written)}, nil
}

// descriptorReadDirectory returns a directory entry stream for reading directory contents.
// Signature: func(self: borrow<descriptor>) -> result<own<directory-entry-stream>, error-code>
func descriptorReadDirectory(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Read directory entries
	entries, readErr := os.ReadDir(desc.Path())
	if readErr != nil {
		return errorResult(MapOSError(readErr)), nil
	}

	// Convert to DirectoryEntry slice
	dirEntries := make([]DirectoryEntry, len(entries))
	for i, entry := range entries {
		info, err := entry.Info()
		var entryType DescriptorType
		if err != nil {
			entryType = DescriptorTypeUnknown
		} else {
			entryType = fileInfoToDescriptorType(info)
		}
		dirEntries[i] = DirectoryEntry{
			Type: entryType,
			Name: entry.Name(),
		}
	}

	// Create directory entry stream
	stream := NewDirectoryEntryStream(dirEntries)

	// Add to resource table
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return errorResult(ErrorCodeIO), nil
	}

	newHandle := table.New(stream, true)
	handleVal := component.ValOwn(uint32(newHandle.Index()))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// descriptorSync synchronizes file data and metadata to storage.
// Signature: func(self: borrow<descriptor>) -> result<_, error-code>
func descriptorSync(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.File() != nil {
		if syncErr := desc.File().Sync(); syncErr != nil {
			return errorResult(MapOSError(syncErr)), nil
		}
	}
	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorCreateDirectoryAt creates a directory relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path: string) -> result<_, error-code>
func descriptorCreateDirectoryAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	path := args[1].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path
	fullPath := filepath.Join(desc.Path(), path)

	// Security check
	cleanPath := filepath.Clean(fullPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Create the directory
	if mkdirErr := os.Mkdir(cleanPath, 0755); mkdirErr != nil {
		return errorResult(MapOSError(mkdirErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorStat returns metadata about a descriptor.
// Signature: func(self: borrow<descriptor>) -> result<descriptor-stat, error-code>
func descriptorStat(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Get file info
	info, statErr := desc.File().Stat()
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	stat := fileInfoToStat(info)
	return []component.Val{component.ValResultOk(&stat)}, nil
}

// descriptorStatAt returns metadata about a file relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string) -> result<descriptor-stat, error-code>
func descriptorStatAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	pathFlags := args[1].Flags()
	path := args[2].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path relative to the descriptor's path
	fullPath := filepath.Join(desc.Path(), path)

	// Security check: ensure the path doesn't escape
	cleanPath := filepath.Clean(fullPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Determine symlink handling
	followSymlinks := pathFlags["symlink-follow"]

	// Get file info
	var info os.FileInfo
	var statErr error
	if followSymlinks {
		info, statErr = os.Stat(cleanPath)
	} else {
		info, statErr = os.Lstat(cleanPath)
	}
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	stat := fileInfoToStat(info)
	return []component.Val{component.ValResultOk(&stat)}, nil
}

// descriptorSetTimesAt sets the times of a file relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string, access: new-timestamp, modification: new-timestamp) -> result<_, error-code>
func descriptorSetTimesAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	pathFlags := args[1].Flags()
	pathStr := args[2].StringVal()
	accessTimeArg := args[3]
	modTimeArg := args[4]

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if !desc.Flags().HasWrite() && desc.Flags()&DescriptorFlagMutateDirectory == 0 {
		return errorResult(ErrorCodeAccess), nil
	}

	fullPath := filepath.Join(desc.Path(), pathStr)

	// Respect symlink-follow flag: use Lstat for no-follow, Stat for follow
	var info os.FileInfo
	var statErr error
	if pathFlags["symlink-follow"] {
		info, statErr = os.Stat(fullPath)
	} else {
		info, statErr = os.Lstat(fullPath)
	}
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}
	currentModTime := info.ModTime()
	currentAtime := currentModTime

	atime := parseNewTimestamp(accessTimeArg, currentAtime)
	mtime := parseNewTimestamp(modTimeArg, currentModTime)

	if chtErr := os.Chtimes(fullPath, atime, mtime); chtErr != nil {
		return errorResult(MapOSError(chtErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorLinkAt creates a hard link relative to descriptors.
// Signature: func(self: borrow<descriptor>, old-path-flags: path-flags, old-path: string, new-descriptor: borrow<descriptor>, new-path: string) -> result<_, error-code>
func descriptorLinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	oldPathFlags := args[1].Flags()
	oldPath := args[2].StringVal()
	newDescHandle := args[3].Borrow()
	newPath := args[4].StringVal()

	// Per wasmtime: reject if symlink-follow is set (hard links shouldn't follow symlinks)
	if oldPathFlags["symlink-follow"] {
		return errorResult(ErrorCodeInvalid), nil
	}

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if desc.Flags()&DescriptorFlagMutateDirectory == 0 {
		return errorResult(ErrorCodeAccess), nil
	}

	newDesc, err := getDescriptor(ctx, newDescHandle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	if newDesc.Flags()&DescriptorFlagMutateDirectory == 0 {
		return errorResult(ErrorCodeAccess), nil
	}

	oldFullPath := filepath.Join(desc.Path(), oldPath)
	newFullPath := filepath.Join(newDesc.Path(), newPath)

	if linkErr := os.Link(oldFullPath, newFullPath); linkErr != nil {
		return errorResult(MapOSError(linkErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorOpenAt opens a file relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string, open-flags: open-flags, descriptor-flags: descriptor-flags) -> result<own<descriptor>, error-code>
func descriptorOpenAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	pathFlags := args[1].Flags()
	path := args[2].StringVal()
	openFlags := args[3].Flags()
	descFlags := args[4].Flags()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path relative to the descriptor's path
	fullPath := filepath.Join(desc.Path(), path)

	// Security check: ensure the path doesn't escape the preopened directory
	// by checking that the cleaned path is still under the descriptor's path
	cleanPath := filepath.Clean(fullPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Determine symlink handling
	followSymlinks := pathFlags["symlink-follow"]

	// Check if the path exists and get info
	var info os.FileInfo
	var statErr error
	if followSymlinks {
		info, statErr = os.Stat(cleanPath)
	} else {
		info, statErr = os.Lstat(cleanPath)
	}

	// Determine open mode flags
	create := openFlags["create"]
	exclusive := openFlags["exclusive"]
	truncate := openFlags["truncate"]
	directory := openFlags["directory"]

	// Convert descriptor flags
	read := descFlags["read"]
	write := descFlags["write"]

	var osFlags int
	if read && write {
		osFlags = os.O_RDWR
	} else if write {
		osFlags = os.O_WRONLY
	} else {
		osFlags = os.O_RDONLY
	}

	if create {
		osFlags |= os.O_CREATE
	}
	if exclusive {
		osFlags |= os.O_EXCL
	}
	if truncate {
		osFlags |= os.O_TRUNC
	}

	// Handle directory open flag
	if directory {
		// Must open a directory
		if statErr == nil && !info.IsDir() {
			return errorResult(ErrorCodeNotDirectory), nil
		}
	}

	// If file doesn't exist and create flag isn't set, return error
	if os.IsNotExist(statErr) && !create {
		return errorResult(ErrorCodeNoEntry), nil
	}

	// Open the file
	var file *os.File
	var openErr error
	if directory || (statErr == nil && info.IsDir()) {
		// Open directory
		file, openErr = os.Open(cleanPath)
	} else {
		// Open regular file
		file, openErr = os.OpenFile(cleanPath, osFlags, 0644)
	}
	if openErr != nil {
		return errorResult(MapOSError(openErr)), nil
	}

	// Get file info after open
	fileInfo, statErr := file.Stat()
	if statErr != nil {
		file.Close()
		return errorResult(MapOSError(statErr)), nil
	}

	// Build descriptor flags
	var newDescFlags DescriptorFlags
	if read {
		newDescFlags |= DescriptorFlagRead
	}
	if write {
		newDescFlags |= DescriptorFlagWrite
	}

	// Create new descriptor
	newDesc := NewDescriptor(file, fileInfo.IsDir(), cleanPath, newDescFlags)

	// Add to resource table
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		file.Close()
		return errorResult(ErrorCodeIO), nil
	}

	newHandle := table.New(newDesc, true)
	handleVal := component.ValOwn(uint32(newHandle.Index()))
	return []component.Val{component.ValResultOk(&handleVal)}, nil
}

// descriptorReadlinkAt reads the target of a symbolic link.
// Signature: func(self: borrow<descriptor>, path: string) -> result<string, error-code>
func descriptorReadlinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	path := args[1].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path
	fullPath := filepath.Join(desc.Path(), path)

	// Security check
	cleanPath := filepath.Clean(fullPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Read the symlink target
	targetPath, readErr := os.Readlink(cleanPath)
	if readErr != nil {
		return errorResult(MapOSError(readErr)), nil
	}

	target := component.ValString(targetPath)
	return []component.Val{component.ValResultOk(&target)}, nil
}

// descriptorRemoveDirectoryAt removes a directory relative to a descriptor.
// Signature: func(self: borrow<descriptor>, path: string) -> result<_, error-code>
func descriptorRemoveDirectoryAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	path := args[1].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path
	fullPath := filepath.Join(desc.Path(), path)

	// Security check
	cleanPath := filepath.Clean(fullPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Check that it's a directory
	info, statErr := os.Stat(cleanPath)
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}
	if !info.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Remove the directory (must be empty)
	if rmErr := os.Remove(cleanPath); rmErr != nil {
		return errorResult(MapOSError(rmErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorRenameAt renames a file or directory.
// Signature: func(self: borrow<descriptor>, old-path: string, new-descriptor: borrow<descriptor>, new-path: string) -> result<_, error-code>
func descriptorRenameAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	oldPath := args[1].StringVal()
	newHandle := args[2].Borrow()
	newPath := args[3].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	newDesc, err := getDescriptor(ctx, newHandle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Both must be directories
	if !desc.IsDir() || !newDesc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build full paths
	oldFullPath := filepath.Join(desc.Path(), oldPath)
	newFullPath := filepath.Join(newDesc.Path(), newPath)

	// Security checks
	oldCleanPath := filepath.Clean(oldFullPath)
	oldBasePath := filepath.Clean(desc.Path())
	if len(oldCleanPath) < len(oldBasePath) || oldCleanPath[:len(oldBasePath)] != oldBasePath {
		return errorResult(ErrorCodeAccess), nil
	}

	newCleanPath := filepath.Clean(newFullPath)
	newBasePath := filepath.Clean(newDesc.Path())
	if len(newCleanPath) < len(newBasePath) || newCleanPath[:len(newBasePath)] != newBasePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Rename
	if renameErr := os.Rename(oldCleanPath, newCleanPath); renameErr != nil {
		return errorResult(MapOSError(renameErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorSymlinkAt creates a symbolic link.
// Signature: func(self: borrow<descriptor>, old-path: string, new-path: string) -> result<_, error-code>
func descriptorSymlinkAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	oldPath := args[1].StringVal() // target
	newPath := args[2].StringVal() // link name

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path for the link (newPath is relative to descriptor)
	linkPath := filepath.Join(desc.Path(), newPath)

	// Security check
	cleanPath := filepath.Clean(linkPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Create symlink (oldPath is the target, which can be relative or absolute)
	if symlinkErr := os.Symlink(oldPath, cleanPath); symlinkErr != nil {
		return errorResult(MapOSError(symlinkErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorUnlinkFileAt removes a file.
// Signature: func(self: borrow<descriptor>, path: string) -> result<_, error-code>
func descriptorUnlinkFileAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	path := args[1].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Parent must be a directory
	if !desc.IsDir() {
		return errorResult(ErrorCodeNotDirectory), nil
	}

	// Build the full path
	fullPath := filepath.Join(desc.Path(), path)

	// Security check
	cleanPath := filepath.Clean(fullPath)
	basePath := filepath.Clean(desc.Path())
	if len(cleanPath) < len(basePath) || cleanPath[:len(basePath)] != basePath {
		return errorResult(ErrorCodeAccess), nil
	}

	// Check that it's not a directory
	info, statErr := os.Lstat(cleanPath)
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}
	if info.IsDir() {
		return errorResult(ErrorCodeIsDirectory), nil
	}

	// Remove the file
	if rmErr := os.Remove(cleanPath); rmErr != nil {
		return errorResult(MapOSError(rmErr)), nil
	}

	return []component.Val{component.ValResultOk(nil)}, nil
}

// descriptorIsSameObject compares two descriptors for identity using dev+ino via os.SameFile.
// Signature: func(self: borrow<descriptor>, other: borrow<descriptor>) -> bool
func descriptorIsSameObject(ctx context.Context, args []component.Val) ([]component.Val, error) {
	selfHandle := args[0].Borrow()
	otherHandle := args[1].Borrow()

	selfDesc, err := getDescriptor(ctx, selfHandle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	otherDesc, err := getDescriptor(ctx, otherHandle)
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	selfInfo, err := selfDesc.File().Stat()
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	otherInfo, err := otherDesc.File().Stat()
	if err != nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	return []component.Val{component.ValBool(os.SameFile(selfInfo, otherInfo))}, nil
}

// descriptorMetadataHash returns a hash of file metadata.
// Signature: func(self: borrow<descriptor>) -> result<metadata-hash-value, error-code>
func descriptorMetadataHash(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	info, statErr := desc.File().Stat()
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	lower, upper := computeMetadataHash(info)
	hash := component.ValRecord(map[string]component.Val{
		"lower": component.ValU64(lower),
		"upper": component.ValU64(upper),
	})
	return []component.Val{component.ValResultOk(&hash)}, nil
}

// descriptorMetadataHashAt returns a hash of file metadata for a path.
// Signature: func(self: borrow<descriptor>, path-flags: path-flags, path: string) -> result<metadata-hash-value, error-code>
func descriptorMetadataHashAt(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()
	// args[1] = path-flags (symlink-follow handled by os.Stat vs os.Lstat)
	pathStr := args[2].StringVal()

	desc, err := getDescriptor(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	fullPath := filepath.Join(desc.Path(), pathStr)
	info, statErr := os.Stat(fullPath)
	if statErr != nil {
		return errorResult(MapOSError(statErr)), nil
	}

	lower, upper := computeMetadataHash(info)
	hash := component.ValRecord(map[string]component.Val{
		"lower": component.ValU64(lower),
		"upper": component.ValU64(upper),
	})
	return []component.Val{component.ValResultOk(&hash)}, nil
}

// computeMetadataHashFallback hashes name + size when dev/ino unavailable.
func computeMetadataHashFallback(info os.FileInfo) (uint64, uint64) {
	h := fnv.New64a()
	goio.WriteString(h, info.Name())
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(info.Size()))
	h.Write(buf[:])
	lower := h.Sum64()
	upper := lower ^ 4614256656552045848
	return lower, upper
}

// filesystemErrorCode converts an error to a filesystem error code.
// Signature: func(err: borrow<error>) -> option<error-code>
func filesystemErrorCode(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	entry, err := table.Get(component.Handle(handle))
	if err != nil {
		return []component.Val{component.ValOption(nil)}, nil
	}

	ioErr, ok := entry.Rep.(*wasipIO.Error)
	if !ok {
		return []component.Val{component.ValOption(nil)}, nil
	}

	// Unwrap the Go error and check if it's a FilesystemError
	var fsErr *FilesystemError
	if errors.As(ioErr.Unwrap(), &fsErr) {
		codeVal := component.ValEnum(string(fsErr.Code))
		return []component.Val{component.ValOption(&codeVal)}, nil
	}

	return []component.Val{component.ValOption(nil)}, nil
}

// directoryEntryStreamReadEntry reads the next entry from a directory stream.
// Signature: func(self: borrow<directory-entry-stream>) -> result<option<directory-entry>, error-code>
func directoryEntryStreamReadEntry(ctx context.Context, args []component.Val) ([]component.Val, error) {
	handle := args[0].Borrow()

	stream, err := getDirEntryStream(ctx, handle)
	if err != nil {
		return errorResult(ErrorCodeBadDescriptor), nil
	}

	// Read next entry
	entry, ok := stream.ReadEntry()
	if !ok {
		// No more entries
		none := component.ValOption(nil)
		return []component.Val{component.ValResultOk(&none)}, nil
	}

	// Build directory-entry record
	dirEntry := component.ValRecord(map[string]component.Val{
		"type": component.ValEnum(entry.Type.String()),
		"name": component.ValString(entry.Name),
	})
	opt := component.ValOption(&dirEntry)
	return []component.Val{component.ValResultOk(&opt)}, nil
}
