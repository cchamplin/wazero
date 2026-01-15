// imports/wasip2/filesystem/types.go

package filesystem

import "os"

// DescriptorType represents the type of a filesystem object.
// Matches wasi:filesystem/types descriptor-type enum.
type DescriptorType uint8

const (
	DescriptorTypeUnknown DescriptorType = iota
	DescriptorTypeBlockDevice
	DescriptorTypeCharacterDevice
	DescriptorTypeDirectory
	DescriptorTypeFifo
	DescriptorTypeSymbolicLink
	DescriptorTypeRegularFile
	DescriptorTypeSocket
)

// String returns the WASI name for this descriptor type.
func (dt DescriptorType) String() string {
	switch dt {
	case DescriptorTypeUnknown:
		return "unknown"
	case DescriptorTypeBlockDevice:
		return "block-device"
	case DescriptorTypeCharacterDevice:
		return "character-device"
	case DescriptorTypeDirectory:
		return "directory"
	case DescriptorTypeFifo:
		return "fifo"
	case DescriptorTypeSymbolicLink:
		return "symbolic-link"
	case DescriptorTypeRegularFile:
		return "regular-file"
	case DescriptorTypeSocket:
		return "socket"
	default:
		return "unknown"
	}
}

// DescriptorFlags represents flags for a descriptor.
// Matches wasi:filesystem/types descriptor-flags flags type.
type DescriptorFlags uint8

const (
	DescriptorFlagRead DescriptorFlags = 1 << iota
	DescriptorFlagWrite
	DescriptorFlagFileIntegritySync
	DescriptorFlagDataIntegritySync
	DescriptorFlagRequestedWriteSync
	DescriptorFlagMutateDirectory
)

// HasRead returns true if the read flag is set.
func (df DescriptorFlags) HasRead() bool {
	return df&DescriptorFlagRead != 0
}

// HasWrite returns true if the write flag is set.
func (df DescriptorFlags) HasWrite() bool {
	return df&DescriptorFlagWrite != 0
}

// ErrorCode represents filesystem error codes.
// Matches wasi:filesystem/types error-code enum.
type ErrorCode string

const (
	ErrorCodeAccess              ErrorCode = "access"
	ErrorCodeWouldBlock          ErrorCode = "would-block"
	ErrorCodeAlready             ErrorCode = "already"
	ErrorCodeBadDescriptor       ErrorCode = "bad-descriptor"
	ErrorCodeBusy                ErrorCode = "busy"
	ErrorCodeDeadlock            ErrorCode = "deadlock"
	ErrorCodeQuota               ErrorCode = "quota"
	ErrorCodeExist               ErrorCode = "exist"
	ErrorCodeFileTooLarge        ErrorCode = "file-too-large"
	ErrorCodeIllegalByteSequence ErrorCode = "illegal-byte-sequence"
	ErrorCodeInProgress          ErrorCode = "in-progress"
	ErrorCodeInterrupted         ErrorCode = "interrupted"
	ErrorCodeInvalid             ErrorCode = "invalid"
	ErrorCodeIO                  ErrorCode = "io"
	ErrorCodeIsDirectory         ErrorCode = "is-directory"
	ErrorCodeLoop                ErrorCode = "loop"
	ErrorCodeTooManyLinks        ErrorCode = "too-many-links"
	ErrorCodeMessageSize         ErrorCode = "message-size"
	ErrorCodeNameTooLong         ErrorCode = "name-too-long"
	ErrorCodeNoDevice            ErrorCode = "no-device"
	ErrorCodeNoEntry             ErrorCode = "no-entry"
	ErrorCodeNoLock              ErrorCode = "no-lock"
	ErrorCodeInsufficientMemory  ErrorCode = "insufficient-memory"
	ErrorCodeInsufficientSpace   ErrorCode = "insufficient-space"
	ErrorCodeNotDirectory        ErrorCode = "not-directory"
	ErrorCodeNotEmpty            ErrorCode = "not-empty"
	ErrorCodeNotRecoverable      ErrorCode = "not-recoverable"
	ErrorCodeUnsupported         ErrorCode = "unsupported"
	ErrorCodeNoTty               ErrorCode = "no-tty"
	ErrorCodeNoSuchDevice        ErrorCode = "no-such-device"
	ErrorCodeOverflow            ErrorCode = "overflow"
	ErrorCodeNotPermitted        ErrorCode = "not-permitted"
	ErrorCodePipe                ErrorCode = "pipe"
	ErrorCodeReadOnly            ErrorCode = "read-only"
	ErrorCodeInvalidSeek         ErrorCode = "invalid-seek"
	ErrorCodeTextFileBusy        ErrorCode = "text-file-busy"
	ErrorCodeCrossDevice         ErrorCode = "cross-device"
)

// Descriptor represents an open file or directory descriptor.
// Matches wasi:filesystem/types descriptor resource.
type Descriptor struct {
	file  *os.File
	isDir bool
	path  string
	flags DescriptorFlags
}

// NewDescriptor creates a new Descriptor.
func NewDescriptor(file *os.File, isDir bool, path string, flags DescriptorFlags) *Descriptor {
	return &Descriptor{
		file:  file,
		isDir: isDir,
		path:  path,
		flags: flags,
	}
}

// File returns the underlying os.File.
func (d *Descriptor) File() *os.File {
	return d.file
}

// IsDir returns true if this is a directory descriptor.
func (d *Descriptor) IsDir() bool {
	return d.isDir
}

// Path returns the path associated with this descriptor.
func (d *Descriptor) Path() string {
	return d.path
}

// Flags returns the descriptor flags.
func (d *Descriptor) Flags() DescriptorFlags {
	return d.flags
}

// Close closes the descriptor.
func (d *Descriptor) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// DirectoryEntry represents an entry in a directory listing.
// Matches wasi:filesystem/types directory-entry record.
type DirectoryEntry struct {
	Type DescriptorType
	Name string
}

// DirectoryEntryStream provides iteration over directory entries.
// Matches wasi:filesystem/types directory-entry-stream resource.
type DirectoryEntryStream struct {
	entries []DirectoryEntry
	index   int
}

// NewDirectoryEntryStream creates a new directory entry stream.
func NewDirectoryEntryStream(entries []DirectoryEntry) *DirectoryEntryStream {
	return &DirectoryEntryStream{
		entries: entries,
		index:   0,
	}
}

// ReadEntry reads the next directory entry.
// Returns the entry and true if available, or nil and false if exhausted.
func (s *DirectoryEntryStream) ReadEntry() (*DirectoryEntry, bool) {
	if s.index >= len(s.entries) {
		return nil, false
	}
	entry := &s.entries[s.index]
	s.index++
	return entry, true
}

// DescriptorStat contains metadata about a file or directory.
// Matches wasi:filesystem/types descriptor-stat record.
type DescriptorStat struct {
	Type             DescriptorType
	LinkCount        uint64
	Size             uint64
	DataAccessTime   *Datetime
	DataModTime      *Datetime
	StatusChangeTime *Datetime
}

// Datetime represents a timestamp.
// Matches wasi:clocks/wall-clock datetime record.
type Datetime struct {
	Seconds     uint64
	Nanoseconds uint32
}

// MetadataHashValue represents a hash of file metadata.
// Matches wasi:filesystem/types metadata-hash-value record.
type MetadataHashValue struct {
	Lower uint64
	Upper uint64
}

// Advice represents file access advice for optimization hints.
// Matches wasi:filesystem/types advice enum.
type Advice uint8

const (
	AdviceNormal Advice = iota
	AdviceSequential
	AdviceRandom
	AdviceWillNeed
	AdviceDontNeed
	AdviceNoReuse
)

// String returns the WASI name for this advice.
func (a Advice) String() string {
	switch a {
	case AdviceNormal:
		return "normal"
	case AdviceSequential:
		return "sequential"
	case AdviceRandom:
		return "random"
	case AdviceWillNeed:
		return "will-need"
	case AdviceDontNeed:
		return "dont-need"
	case AdviceNoReuse:
		return "no-reuse"
	default:
		return "normal"
	}
}

// OpenFlags represents flags for opening files.
// Matches wasi:filesystem/types open-flags flags type.
type OpenFlags uint8

const (
	OpenFlagCreate OpenFlags = 1 << iota
	OpenFlagDirectory
	OpenFlagExclusive
	OpenFlagTruncate
)

// PathFlags represents flags for path resolution.
// Matches wasi:filesystem/types path-flags flags type.
type PathFlags uint8

const (
	PathFlagSymlinkFollow PathFlags = 1 << iota
)

// NewTimestamp represents a variant for setting timestamps.
// Matches wasi:filesystem/types new-timestamp variant.
type NewTimestamp struct {
	kind      newTimestampKind
	timestamp *Datetime
}

type newTimestampKind uint8

const (
	newTimestampKindNoChange newTimestampKind = iota
	newTimestampKindNow
	newTimestampKindTimestamp
)

// NewTimestampNoChange creates a "no change" timestamp variant.
func NewTimestampNoChange() NewTimestamp {
	return NewTimestamp{kind: newTimestampKindNoChange}
}

// NewTimestampNow creates a "set to current time" timestamp variant.
func NewTimestampNow() NewTimestamp {
	return NewTimestamp{kind: newTimestampKindNow}
}

// NewTimestampAt creates a specific timestamp variant.
func NewTimestampAt(dt *Datetime) NewTimestamp {
	return NewTimestamp{kind: newTimestampKindTimestamp, timestamp: dt}
}

// IsNoChange returns true if this is a "no change" timestamp.
func (t NewTimestamp) IsNoChange() bool {
	return t.kind == newTimestampKindNoChange
}

// IsNow returns true if this is a "set to current time" timestamp.
func (t NewTimestamp) IsNow() bool {
	return t.kind == newTimestampKindNow
}

// Timestamp returns the specific timestamp value, or nil if not set.
func (t NewTimestamp) Timestamp() *Datetime {
	return t.timestamp
}
