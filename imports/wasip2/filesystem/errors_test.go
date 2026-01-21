// imports/wasip2/filesystem/errors_test.go

package filesystem

import (
	"os"
	"syscall"
	"testing"
)

func TestMapOSError_ENOENT(t *testing.T) {
	err := os.ErrNotExist

	code := MapOSError(err)

	if code != ErrorCodeNoEntry {
		t.Errorf("expected NoEntry, got %v", code)
	}
}

func TestMapOSError_EACCES(t *testing.T) {
	err := os.ErrPermission

	code := MapOSError(err)

	if code != ErrorCodeAccess {
		t.Errorf("expected Access, got %v", code)
	}
}

func TestMapOSError_EEXIST(t *testing.T) {
	err := os.ErrExist

	code := MapOSError(err)

	if code != ErrorCodeExist {
		t.Errorf("expected Exist, got %v", code)
	}
}

func TestMapOSError_Nil(t *testing.T) {
	code := MapOSError(nil)

	if code != ErrorCodeInvalid {
		t.Errorf("expected Invalid for nil error, got %v", code)
	}
}

func TestMapOSError_ErrInvalid(t *testing.T) {
	err := os.ErrInvalid

	code := MapOSError(err)

	if code != ErrorCodeInvalid {
		t.Errorf("expected Invalid, got %v", code)
	}
}

func TestMapOSError_ErrClosed(t *testing.T) {
	err := os.ErrClosed

	code := MapOSError(err)

	if code != ErrorCodeBadDescriptor {
		t.Errorf("expected BadDescriptor, got %v", code)
	}
}

func TestMapOSError_PathError(t *testing.T) {
	// Simulate a PathError wrapping syscall.ENOENT
	err := &os.PathError{
		Op:   "open",
		Path: "/nonexistent",
		Err:  syscall.ENOENT,
	}

	code := MapOSError(err)

	if code != ErrorCodeNoEntry {
		t.Errorf("expected NoEntry, got %v", code)
	}
}

func TestMapOSError_LinkError(t *testing.T) {
	// Simulate a LinkError wrapping syscall.EEXIST
	err := &os.LinkError{
		Op:  "link",
		Old: "/old",
		New: "/new",
		Err: syscall.EEXIST,
	}

	code := MapOSError(err)

	if code != ErrorCodeExist {
		t.Errorf("expected Exist, got %v", code)
	}
}

func TestMapSyscallErrno(t *testing.T) {
	tests := []struct {
		errno    syscall.Errno
		expected ErrorCode
	}{
		{syscall.ENOENT, ErrorCodeNoEntry},
		{syscall.EACCES, ErrorCodeAccess},
		{syscall.EPERM, ErrorCodeNotPermitted},
		{syscall.EEXIST, ErrorCodeExist},
		{syscall.ENOTDIR, ErrorCodeNotDirectory},
		{syscall.EISDIR, ErrorCodeIsDirectory},
		{syscall.ENOTEMPTY, ErrorCodeNotEmpty},
		{syscall.ENOSPC, ErrorCodeInsufficientSpace},
		{syscall.EROFS, ErrorCodeReadOnly},
		{syscall.EINVAL, ErrorCodeInvalid},
		{syscall.EBADF, ErrorCodeBadDescriptor},
		{syscall.EBUSY, ErrorCodeBusy},
		{syscall.EDEADLK, ErrorCodeDeadlock},
		{syscall.EFBIG, ErrorCodeFileTooLarge},
		{syscall.EINTR, ErrorCodeInterrupted},
		{syscall.EIO, ErrorCodeIO},
		{syscall.ELOOP, ErrorCodeLoop},
		{syscall.EMLINK, ErrorCodeTooManyLinks},
		{syscall.ENAMETOOLONG, ErrorCodeNameTooLong},
		{syscall.ENODEV, ErrorCodeNoDevice},
		{syscall.ENOMEM, ErrorCodeInsufficientMemory},
		{syscall.ENOTTY, ErrorCodeNoTty},
		{syscall.EOVERFLOW, ErrorCodeOverflow},
		{syscall.EPIPE, ErrorCodePipe},
		{syscall.ESPIPE, ErrorCodeInvalidSeek},
		{syscall.EXDEV, ErrorCodeCrossDevice},
	}

	for _, tt := range tests {
		code := MapSyscallErrno(tt.errno)
		if code != tt.expected {
			t.Errorf("errno %v: expected %v, got %v", tt.errno, tt.expected, code)
		}
	}
}

func TestMapSyscallErrno_UnknownErrno(t *testing.T) {
	// Use a high errno value that's unlikely to be mapped
	unknownErrno := syscall.Errno(9999)

	code := MapSyscallErrno(unknownErrno)

	if code != ErrorCodeIO {
		t.Errorf("expected IO for unknown errno, got %v", code)
	}
}
