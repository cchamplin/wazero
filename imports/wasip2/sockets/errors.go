package sockets

import (
	"errors"
	"net"
	"syscall"
)

// MapNetError maps net package errors to WASI error codes.
// It handles various net error types including OpError, DNSError, and AddrError,
// as well as errors implementing the net.Error interface for timeout detection.
func MapNetError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeUnknown
	}

	// Check for timeout via net.Error interface
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return ErrorCodeTimeout
	}

	// Check for OpError
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Check if underlying error is syscall.Errno
		if errno, ok := opErr.Err.(syscall.Errno); ok {
			return MapSocketErrno(errno)
		}

		// Check for wrapped syscall.Errno
		var errno syscall.Errno
		if errors.As(opErr.Err, &errno) {
			return MapSocketErrno(errno)
		}

		// Check for DNS error
		var dnsErr *net.DNSError
		if errors.As(opErr.Err, &dnsErr) {
			if dnsErr.IsTimeout {
				return ErrorCodeTimeout
			}
			if dnsErr.IsNotFound {
				return ErrorCodeNameUnresolvable
			}
			if dnsErr.IsTemporary {
				return ErrorCodeTemporaryResolverFailure
			}
			return ErrorCodeNameUnresolvable
		}
	}

	// Check for standalone DNSError
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsTimeout {
			return ErrorCodeTimeout
		}
		if dnsErr.IsNotFound {
			return ErrorCodeNameUnresolvable
		}
		if dnsErr.IsTemporary {
			return ErrorCodeTemporaryResolverFailure
		}
		return ErrorCodeNameUnresolvable
	}

	// Check for AddrError
	var addrErr *net.AddrError
	if errors.As(err, &addrErr) {
		return ErrorCodeInvalidArgument
	}

	// Check for syscall.Errno directly
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return MapSocketErrno(errno)
	}

	return ErrorCodeUnknown
}

// MapSocketErrno maps syscall errno to socket error codes per WASI sockets/network specification.
func MapSocketErrno(errno syscall.Errno) ErrorCode {
	switch errno {
	case syscall.EACCES:
		return ErrorCodeAccessDenied
	case syscall.EADDRINUSE:
		return ErrorCodeAddressInUse
	case syscall.EADDRNOTAVAIL:
		return ErrorCodeAddressNotBindable
	case syscall.EAFNOSUPPORT:
		return ErrorCodeNotSupported
	case syscall.EAGAIN: // EWOULDBLOCK is same as EAGAIN on most systems
		return ErrorCodeWouldBlock
	case syscall.EALREADY:
		return ErrorCodeConcurrencyConflict
	case syscall.ECONNABORTED:
		return ErrorCodeConnectionAborted
	case syscall.ECONNREFUSED:
		return ErrorCodeConnectionRefused
	case syscall.ECONNRESET:
		return ErrorCodeConnectionReset
	case syscall.EHOSTUNREACH:
		return ErrorCodeRemoteUnreachable
	case syscall.EINPROGRESS:
		return ErrorCodeWouldBlock
	case syscall.EINVAL:
		return ErrorCodeInvalidArgument
	case syscall.EISCONN:
		return ErrorCodeInvalidState // Already connected
	case syscall.EMSGSIZE:
		return ErrorCodeDatagramTooLarge
	case syscall.ENETDOWN:
		return ErrorCodeNetworkDown
	case syscall.ENETRESET:
		return ErrorCodeConnectionReset
	case syscall.ENETUNREACH:
		return ErrorCodeNetworkUnreachable
	case syscall.ENOBUFS:
		return ErrorCodeOutOfMemory
	case syscall.ENOPROTOOPT:
		return ErrorCodeNotSupported
	case syscall.ENOTCONN:
		return ErrorCodeInvalidState // Not connected
	case syscall.ENOTSOCK:
		return ErrorCodeInvalidArgument
	case syscall.EPERM:
		return ErrorCodeAccessDenied
	case syscall.EPIPE:
		return ErrorCodeConnectionReset
	case syscall.ETIMEDOUT:
		return ErrorCodeTimeout
	case syscall.EMFILE, syscall.ENFILE:
		return ErrorCodeNewSocketLimit
	default:
		return ErrorCodeUnknown
	}
}
