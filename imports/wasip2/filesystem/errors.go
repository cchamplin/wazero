// imports/wasip2/filesystem/errors.go

package filesystem

import (
	"errors"
	"os"
	"syscall"
)

// MapOSError maps os package errors to WASI error codes.
// It handles common os.Err* sentinel errors as well as wrapped
// syscall errors in os.PathError and os.LinkError.
func MapOSError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeInvalid // Should not happen
	}

	switch {
	case errors.Is(err, os.ErrNotExist):
		return ErrorCodeNoEntry
	case errors.Is(err, os.ErrExist):
		return ErrorCodeExist
	case errors.Is(err, os.ErrPermission):
		return ErrorCodeAccess
	case errors.Is(err, os.ErrInvalid):
		return ErrorCodeInvalid
	case errors.Is(err, os.ErrClosed):
		return ErrorCodeBadDescriptor
	}

	// Try to extract syscall errno from PathError
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			return MapSyscallErrno(errno)
		}
	}

	// Try to extract syscall errno from LinkError
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		if errno, ok := linkErr.Err.(syscall.Errno); ok {
			return MapSyscallErrno(errno)
		}
	}

	// Try to extract syscall errno from SyscallError
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		if errno, ok := syscallErr.Err.(syscall.Errno); ok {
			return MapSyscallErrno(errno)
		}
	}

	// Default to IO error for unknown errors
	return ErrorCodeIO
}

// MapSyscallErrno maps syscall errno values to WASI error codes.
// This provides comprehensive mapping for all WASI filesystem error codes.
func MapSyscallErrno(errno syscall.Errno) ErrorCode {
	switch errno {
	case syscall.EACCES:
		return ErrorCodeAccess
	case syscall.EAGAIN:
		return ErrorCodeWouldBlock
	case syscall.EALREADY:
		return ErrorCodeAlready
	case syscall.EBADF:
		return ErrorCodeBadDescriptor
	case syscall.EBUSY:
		return ErrorCodeBusy
	case syscall.EDEADLK:
		return ErrorCodeDeadlock
	case syscall.EDQUOT:
		return ErrorCodeQuota
	case syscall.EEXIST:
		return ErrorCodeExist
	case syscall.EFBIG:
		return ErrorCodeFileTooLarge
	case syscall.EILSEQ:
		return ErrorCodeIllegalByteSequence
	case syscall.EINPROGRESS:
		return ErrorCodeInProgress
	case syscall.EINTR:
		return ErrorCodeInterrupted
	case syscall.EINVAL:
		return ErrorCodeInvalid
	case syscall.EIO:
		return ErrorCodeIO
	case syscall.EISDIR:
		return ErrorCodeIsDirectory
	case syscall.ELOOP:
		return ErrorCodeLoop
	case syscall.EMLINK:
		return ErrorCodeTooManyLinks
	case syscall.EMSGSIZE:
		return ErrorCodeMessageSize
	case syscall.ENAMETOOLONG:
		return ErrorCodeNameTooLong
	case syscall.ENODEV:
		return ErrorCodeNoDevice
	case syscall.ENOENT:
		return ErrorCodeNoEntry
	case syscall.ENOLCK:
		return ErrorCodeNoLock
	case syscall.ENOMEM:
		return ErrorCodeInsufficientMemory
	case syscall.ENOSPC:
		return ErrorCodeInsufficientSpace
	case syscall.ENOTDIR:
		return ErrorCodeNotDirectory
	case syscall.ENOTEMPTY:
		return ErrorCodeNotEmpty
	case syscall.ENOTRECOVERABLE:
		return ErrorCodeNotRecoverable
	case syscall.ENOTSUP:
		return ErrorCodeUnsupported
	case syscall.ENOTTY:
		return ErrorCodeNoTty
	case syscall.ENXIO:
		return ErrorCodeNoSuchDevice
	case syscall.EOVERFLOW:
		return ErrorCodeOverflow
	case syscall.EPERM:
		return ErrorCodeNotPermitted
	case syscall.EPIPE:
		return ErrorCodePipe
	case syscall.EROFS:
		return ErrorCodeReadOnly
	case syscall.ESPIPE:
		return ErrorCodeInvalidSeek
	case syscall.ETXTBSY:
		return ErrorCodeTextFileBusy
	case syscall.EXDEV:
		return ErrorCodeCrossDevice
	default:
		return ErrorCodeIO
	}
}
