package sockets

import (
	"net"
	"syscall"
	"testing"
)

func TestMapNetError_ConnectionRefused(t *testing.T) {
	// Create a connection refused error
	err := &net.OpError{
		Op:  "dial",
		Err: syscall.ECONNREFUSED,
	}

	code := MapNetError(err)

	if code != ErrorCodeConnectionRefused {
		t.Errorf("expected ConnectionRefused, got %v", code)
	}
}

func TestMapNetError_Timeout(t *testing.T) {
	err := &net.OpError{
		Op:  "dial",
		Err: &net.DNSError{IsTimeout: true},
	}

	code := MapNetError(err)

	if code != ErrorCodeTimeout {
		t.Errorf("expected Timeout, got %v", code)
	}
}

func TestMapNetError_DNSNotFound(t *testing.T) {
	err := &net.OpError{
		Op:  "dial",
		Err: &net.DNSError{IsNotFound: true},
	}

	code := MapNetError(err)

	if code != ErrorCodeNameUnresolvable {
		t.Errorf("expected NameUnresolvable, got %v", code)
	}
}

func TestMapNetError_AddrError(t *testing.T) {
	err := &net.AddrError{
		Err:  "invalid address",
		Addr: "bad",
	}

	code := MapNetError(err)

	if code != ErrorCodeInvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", code)
	}
}

func TestMapNetError_TimeoutInterface(t *testing.T) {
	// net.DNSError implements net.Error with Timeout() method
	err := &net.DNSError{IsTimeout: true}

	code := MapNetError(err)

	if code != ErrorCodeTimeout {
		t.Errorf("expected Timeout, got %v", code)
	}
}

func TestMapNetError_Nil(t *testing.T) {
	code := MapNetError(nil)

	if code != ErrorCodeUnknown {
		t.Errorf("expected Unknown, got %v", code)
	}
}

func TestMapSocketErrno(t *testing.T) {
	tests := []struct {
		name     string
		errno    syscall.Errno
		expected ErrorCode
	}{
		{"ECONNREFUSED", syscall.ECONNREFUSED, ErrorCodeConnectionRefused},
		{"ECONNRESET", syscall.ECONNRESET, ErrorCodeConnectionReset},
		{"ECONNABORTED", syscall.ECONNABORTED, ErrorCodeConnectionAborted},
		{"ETIMEDOUT", syscall.ETIMEDOUT, ErrorCodeTimeout},
		{"EHOSTUNREACH", syscall.EHOSTUNREACH, ErrorCodeRemoteUnreachable},
		{"ENETUNREACH", syscall.ENETUNREACH, ErrorCodeNetworkUnreachable},
		{"EADDRINUSE", syscall.EADDRINUSE, ErrorCodeAddressInUse},
		{"EADDRNOTAVAIL", syscall.EADDRNOTAVAIL, ErrorCodeAddressNotBindable},
		{"EACCES", syscall.EACCES, ErrorCodeAccessDenied},
		{"EPERM", syscall.EPERM, ErrorCodeAccessDenied},
		{"EAGAIN", syscall.EAGAIN, ErrorCodeWouldBlock},
		{"EALREADY", syscall.EALREADY, ErrorCodeConcurrencyConflict},
		{"EINPROGRESS", syscall.EINPROGRESS, ErrorCodeWouldBlock},
		{"EINVAL", syscall.EINVAL, ErrorCodeInvalidArgument},
		{"EISCONN", syscall.EISCONN, ErrorCodeInvalidState},
		{"EMSGSIZE", syscall.EMSGSIZE, ErrorCodeDatagramTooLarge},
		{"ENETDOWN", syscall.ENETDOWN, ErrorCodeNetworkDown},
		{"ENETRESET", syscall.ENETRESET, ErrorCodeConnectionReset},
		{"ENOBUFS", syscall.ENOBUFS, ErrorCodeOutOfMemory},
		{"ENOPROTOOPT", syscall.ENOPROTOOPT, ErrorCodeNotSupported},
		{"ENOTCONN", syscall.ENOTCONN, ErrorCodeInvalidState},
		{"ENOTSOCK", syscall.ENOTSOCK, ErrorCodeInvalidArgument},
		{"EPIPE", syscall.EPIPE, ErrorCodeConnectionReset},
		{"EMFILE", syscall.EMFILE, ErrorCodeNewSocketLimit},
		{"ENFILE", syscall.ENFILE, ErrorCodeNewSocketLimit},
		{"EAFNOSUPPORT", syscall.EAFNOSUPPORT, ErrorCodeNotSupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := MapSocketErrno(tt.errno)
			if code != tt.expected {
				t.Errorf("errno %v: expected %v, got %v", tt.errno, tt.expected, code)
			}
		})
	}
}

func TestMapSocketErrno_Unknown(t *testing.T) {
	// Use an errno that we don't specifically handle
	code := MapSocketErrno(syscall.ENOENT)

	if code != ErrorCodeUnknown {
		t.Errorf("expected Unknown for unhandled errno, got %v", code)
	}
}
