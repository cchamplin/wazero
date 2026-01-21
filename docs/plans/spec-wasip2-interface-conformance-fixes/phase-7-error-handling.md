# Phase 7: Error Handling - Debug Strings & Mapping

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve error handling with better debug strings, context preservation, and accurate WASI error code mapping.

**Architecture:** Enhance the wasi:io/error resource to include error context and stack traces, map internal Go errors to specific WASI error codes rather than generic ones.

**Tech Stack:** Go errors package, runtime for stack traces, error wrapping

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 9 Error Handling Analysis

**Prerequisite:** All prior phases should be complete.

---

## Reference Materials

### Specification
- **WIT File:** `debug-vendored/WASI/proposals/io/wit/error.wit`
- **Key Methods:**
  - `[method]error.to-debug-string` - Should return useful debug information

### Current Implementation
- **File:** `imports/wasip2/io/error.go`
- **Issues:**
  - `to-debug-string` returns simple message without context
  - No stack trace preservation
  - Generic error mapping

### Error Code Definitions
- `wasi:io/streams` - stream-error (closed, last-operation-failed)
- `wasi:filesystem/types` - error-code (36+ codes)
- `wasi:sockets/network` - error-code (36 codes)
- `wasi:http/types` - error-code (40+ codes)

---

## Task 7.1: Enhance Error Resource with Context

**Files:**
- Modify: `imports/wasip2/io/error.go`
- Test: `imports/wasip2/io/error_test.go`

**Step 1: Write failing test**

Create `imports/wasip2/io/error_test.go`:

```go
package io

import (
	"errors"
	"strings"
	"testing"
)

func TestError_ToDebugString_IncludesMessage(t *testing.T) {
	err := NewError(errors.New("file not found"))

	debugStr := err.ToDebugString()

	if !strings.Contains(debugStr, "file not found") {
		t.Errorf("debug string should contain error message, got: %s", debugStr)
	}
}

func TestError_ToDebugString_IncludesSource(t *testing.T) {
	err := NewErrorWithSource(errors.New("connection failed"), "tcp-connect")

	debugStr := err.ToDebugString()

	if !strings.Contains(debugStr, "tcp-connect") {
		t.Errorf("debug string should contain source, got: %s", debugStr)
	}
	if !strings.Contains(debugStr, "connection failed") {
		t.Errorf("debug string should contain message, got: %s", debugStr)
	}
}

func TestError_ToDebugString_IncludesStackTrace(t *testing.T) {
	err := NewErrorWithStack(errors.New("panic recovered"))

	debugStr := err.ToDebugString()

	// Should contain file:line from stack
	if !strings.Contains(debugStr, ".go:") {
		t.Errorf("debug string should contain stack trace, got: %s", debugStr)
	}
}

func TestError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewError(originalErr)

	unwrapped := errors.Unwrap(err)
	if unwrapped != originalErr {
		t.Error("Unwrap should return original error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestError_"`
Expected: FAIL - Enhanced Error type doesn't exist

**Step 3: Implement enhanced Error type**

Update `imports/wasip2/io/error.go`:

```go
package io

import (
	"fmt"
	"runtime"
	"strings"
)

// Error represents a wasi:io/error resource with enhanced debugging
type Error struct {
	err    error
	source string
	stack  []uintptr
}

// NewError creates a new error from a Go error
func NewError(err error) *Error {
	return &Error{
		err: err,
	}
}

// NewErrorWithSource creates an error with source context
func NewErrorWithSource(err error, source string) *Error {
	return &Error{
		err:    err,
		source: source,
	}
}

// NewErrorWithStack creates an error with stack trace
func NewErrorWithStack(err error) *Error {
	// Capture stack trace
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs) // Skip Callers and NewErrorWithStack

	return &Error{
		err:   err,
		stack: pcs[:n],
	}
}

// NewErrorFull creates an error with all context
func NewErrorFull(err error, source string) *Error {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)

	return &Error{
		err:    err,
		source: source,
		stack:  pcs[:n],
	}
}

// ToDebugString returns a detailed debug string for the error
func (e *Error) ToDebugString() string {
	if e == nil || e.err == nil {
		return "no error"
	}

	var sb strings.Builder

	// Add source if present
	if e.source != "" {
		sb.WriteString("[")
		sb.WriteString(e.source)
		sb.WriteString("] ")
	}

	// Add error message
	sb.WriteString(e.err.Error())

	// Add stack trace if present
	if len(e.stack) > 0 {
		sb.WriteString("\n\nStack trace:\n")
		frames := runtime.CallersFrames(e.stack)
		for {
			frame, more := frames.Next()
			if frame.Function == "" {
				break
			}
			sb.WriteString(fmt.Sprintf("  %s\n    %s:%d\n",
				frame.Function, frame.File, frame.Line))
			if !more {
				break
			}
		}
	}

	return sb.String()
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.err == nil {
		return "no error"
	}
	return e.err.Error()
}

// Unwrap returns the underlying error for errors.Unwrap
func (e *Error) Unwrap() error {
	return e.err
}

// Destroy implements Destroyable (no-op for errors)
func (e *Error) Destroy() {
	// Nothing to clean up
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestError_"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/error.go imports/wasip2/io/error_test.go
git commit -m "feat(wasip2): enhance Error with source context and stack traces

ToDebugString now includes source location and optional stack trace."
```

---

## Task 7.2: Update WASI Error Binding to Use Enhanced Error

**Files:**
- Modify: `imports/wasip2/io/error.go`

**Step 1: Update WASI binding**

In `imports/wasip2/io/error.go`, update the WASI method:

```go
import (
	"context"

	"github.com/tetratelabs/wazero/internal/component"
)

// errorToDebugString implements [method]error.to-debug-string
func errorToDebugString(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValString("no resource table")}, nil
	}

	handle := args[0].Borrow()
	resource := table.Get(int(handle))
	if resource == nil {
		return []component.Val{component.ValString("invalid error handle")}, nil
	}

	err, ok := resource.(*Error)
	if !ok {
		// Try to handle as generic error
		if genericErr, ok := resource.(error); ok {
			return []component.Val{component.ValString(genericErr.Error())}, nil
		}
		return []component.Val{component.ValString("not an error resource")}, nil
	}

	return []component.Val{component.ValString(err.ToDebugString())}, nil
}
```

**Step 2: Run tests**

Run: `go test -v ./imports/wasip2/io/...`
Expected: PASS

**Step 3: Commit**

```bash
git add imports/wasip2/io/error.go
git commit -m "feat(wasip2): wire enhanced Error to WASI to-debug-string"
```

---

## Task 7.3: Create Error Code Mapper for Filesystem

**Files:**
- Create: `imports/wasip2/filesystem/errors.go`
- Test: `imports/wasip2/filesystem/errors_test.go`

**Step 1: Write failing test**

Create `imports/wasip2/filesystem/errors_test.go`:

```go
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

func TestMapSyscallError(t *testing.T) {
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
		{syscall.ENOSPC, ErrorCodeNoSpace},
		{syscall.EROFS, ErrorCodeReadOnly},
		{syscall.EINVAL, ErrorCodeInvalid},
		{syscall.EBADF, ErrorCodeBadDescriptor},
	}

	for _, tt := range tests {
		code := MapSyscallErrno(tt.errno)
		if code != tt.expected {
			t.Errorf("errno %v: expected %v, got %v", tt.errno, tt.expected, code)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/filesystem/... -run "TestMap"`
Expected: FAIL - MapOSError, ErrorCode types don't exist

**Step 3: Create errors.go**

Create `imports/wasip2/filesystem/errors.go`:

```go
package filesystem

import (
	"errors"
	"os"
	"syscall"
)

// ErrorCode represents wasi:filesystem/types error-code
type ErrorCode uint8

const (
	ErrorCodeAccess ErrorCode = iota
	ErrorCodeWouldBlock
	ErrorCodeAlready
	ErrorCodeBadDescriptor
	ErrorCodeBusy
	ErrorCodeDeadlock
	ErrorCodeQuota
	ErrorCodeExist
	ErrorCodeFileTooLarge
	ErrorCodeIllegalByteSequence
	ErrorCodeInProgress
	ErrorCodeInterrupted
	ErrorCodeInvalid
	ErrorCodeIO
	ErrorCodeIsDirectory
	ErrorCodeLoop
	ErrorCodeTooManyLinks
	ErrorCodeMessageSize
	ErrorCodeNameTooLong
	ErrorCodeNoDevice
	ErrorCodeNoEntry
	ErrorCodeNoLock
	ErrorCodeInsufficientMemory
	ErrorCodeNoSpace
	ErrorCodeNotDirectory
	ErrorCodeNotEmpty
	ErrorCodeNotRecoverable
	ErrorCodeUnsupported
	ErrorCodeNoTTY
	ErrorCodeNoSuchDevice
	ErrorCodeOverflow
	ErrorCodeNotPermitted
	ErrorCodePipe
	ErrorCodeReadOnly
	ErrorCodeInvalidSeek
	ErrorCodeTextFileBusy
	ErrorCodeCrossDevice
)

// MapOSError maps os package errors to WASI error codes
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

	// Try to extract syscall errno
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			return MapSyscallErrno(errno)
		}
	}

	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		if errno, ok := linkErr.Err.(syscall.Errno); ok {
			return MapSyscallErrno(errno)
		}
	}

	// Default to IO error
	return ErrorCodeIO
}

// MapSyscallErrno maps syscall errno to WASI error codes
func MapSyscallErrno(errno syscall.Errno) ErrorCode {
	switch errno {
	case syscall.EACCES:
		return ErrorCodeAccess
	case syscall.EAGAIN, syscall.EWOULDBLOCK:
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
		return ErrorCodeNoSpace
	case syscall.ENOTDIR:
		return ErrorCodeNotDirectory
	case syscall.ENOTEMPTY:
		return ErrorCodeNotEmpty
	case syscall.ENOTRECOVERABLE:
		return ErrorCodeNotRecoverable
	case syscall.ENOTSUP:
		return ErrorCodeUnsupported
	case syscall.ENOTTY:
		return ErrorCodeNoTTY
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

// String returns the error code name
func (c ErrorCode) String() string {
	names := []string{
		"access", "would-block", "already", "bad-descriptor", "busy",
		"deadlock", "quota", "exist", "file-too-large", "illegal-byte-sequence",
		"in-progress", "interrupted", "invalid", "io", "is-directory",
		"loop", "too-many-links", "message-size", "name-too-long", "no-device",
		"no-entry", "no-lock", "insufficient-memory", "no-space", "not-directory",
		"not-empty", "not-recoverable", "unsupported", "no-tty", "no-such-device",
		"overflow", "not-permitted", "pipe", "read-only", "invalid-seek",
		"text-file-busy", "cross-device",
	}
	if int(c) < len(names) {
		return names[c]
	}
	return "unknown"
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/filesystem/... -run "TestMap"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/filesystem/errors.go imports/wasip2/filesystem/errors_test.go
git commit -m "feat(wasip2): add comprehensive filesystem error code mapping

Maps os and syscall errors to WASI error codes."
```

---

## Task 7.4: Create Error Code Mapper for Sockets

**Files:**
- Modify: `imports/wasip2/sockets/sockets.go`
- Test: `imports/wasip2/sockets/errors_test.go`

**Step 1: Write failing test**

Create `imports/wasip2/sockets/errors_test.go`:

```go
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

func TestMapSocketErrno(t *testing.T) {
	tests := []struct {
		errno    syscall.Errno
		expected ErrorCode
	}{
		{syscall.ECONNREFUSED, ErrorCodeConnectionRefused},
		{syscall.ECONNRESET, ErrorCodeConnectionReset},
		{syscall.ECONNABORTED, ErrorCodeConnectionAborted},
		{syscall.ETIMEDOUT, ErrorCodeTimeout},
		{syscall.EHOSTUNREACH, ErrorCodeHostUnreachable},
		{syscall.ENETUNREACH, ErrorCodeNetworkUnreachable},
		{syscall.EADDRINUSE, ErrorCodeAddressInUse},
		{syscall.EADDRNOTAVAIL, ErrorCodeAddressNotAvailable},
	}

	for _, tt := range tests {
		code := MapSocketErrno(tt.errno)
		if code != tt.expected {
			t.Errorf("errno %v: expected %v, got %v", tt.errno, tt.expected, code)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestMapNet|TestMapSocket"`
Expected: FAIL - MapNetError doesn't exist

**Step 3: Add error mapping functions**

Add to `imports/wasip2/sockets/sockets.go` or create a new `errors.go`:

```go
// MapNetError maps net package errors to WASI error codes
func MapNetError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeUnknown
	}

	// Check for timeout
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return ErrorCodeTimeout
	}

	// Check for OpError
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Check underlying error
		if errno, ok := opErr.Err.(syscall.Errno); ok {
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
			return ErrorCodeNameUnresolvable
		}
	}

	// Check for AddrError
	var addrErr *net.AddrError
	if errors.As(err, &addrErr) {
		return ErrorCodeInvalidArgument
	}

	return ErrorCodeUnknown
}

// MapSocketErrno maps syscall errno to socket error codes
func MapSocketErrno(errno syscall.Errno) ErrorCode {
	switch errno {
	case syscall.EACCES:
		return ErrorCodeAccessDenied
	case syscall.EADDRINUSE:
		return ErrorCodeAddressInUse
	case syscall.EADDRNOTAVAIL:
		return ErrorCodeAddressNotAvailable
	case syscall.EAFNOSUPPORT:
		return ErrorCodeAddressFamilyNotSupported
	case syscall.EAGAIN, syscall.EWOULDBLOCK:
		return ErrorCodeWouldBlock
	case syscall.EALREADY:
		return ErrorCodeAlreadyConnected
	case syscall.ECONNABORTED:
		return ErrorCodeConnectionAborted
	case syscall.ECONNREFUSED:
		return ErrorCodeConnectionRefused
	case syscall.ECONNRESET:
		return ErrorCodeConnectionReset
	case syscall.EHOSTUNREACH:
		return ErrorCodeHostUnreachable
	case syscall.EINPROGRESS:
		return ErrorCodeInProgress
	case syscall.EINVAL:
		return ErrorCodeInvalidArgument
	case syscall.EISCONN:
		return ErrorCodeAlreadyConnected
	case syscall.EMSGSIZE:
		return ErrorCodeDatagramTooLarge
	case syscall.ENETDOWN:
		return ErrorCodeNetworkDown
	case syscall.ENETRESET:
		return ErrorCodeNetworkReset
	case syscall.ENETUNREACH:
		return ErrorCodeNetworkUnreachable
	case syscall.ENOBUFS:
		return ErrorCodeOutOfMemory
	case syscall.ENOPROTOOPT:
		return ErrorCodeNotSupported
	case syscall.ENOTCONN:
		return ErrorCodeNotConnected
	case syscall.ENOTSOCK:
		return ErrorCodeInvalidState
	case syscall.EPERM:
		return ErrorCodeAccessDenied
	case syscall.EPIPE:
		return ErrorCodeConnectionReset
	case syscall.ETIMEDOUT:
		return ErrorCodeTimeout
	default:
		return ErrorCodeUnknown
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestMapNet|TestMapSocket"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/sockets.go imports/wasip2/sockets/errors_test.go
git commit -m "feat(wasip2): add comprehensive socket error code mapping

Maps net and syscall errors to WASI socket error codes."
```

---

## Task 7.5: Integrate Error Mapping into Filesystem Operations

**Files:**
- Modify: `imports/wasip2/filesystem/filesystem.go`

**Step 1: Update filesystem functions to use error mapper**

In `imports/wasip2/filesystem/filesystem.go`, update error returns:

```go
// Example: update descriptorStat to use error mapping
func descriptorStat(ctx context.Context, args []component.Val) ([]component.Val, error) {
	// ... existing code to get descriptor ...

	info, err := desc.file.Stat()
	if err != nil {
		// Use error mapper instead of generic error
		return filesystemErrorResult(MapOSError(err)), nil
	}

	// ... rest of function ...
}

// filesystemErrorResult creates an error result with the error code
func filesystemErrorResult(code ErrorCode) []component.Val {
	return []component.Val{component.ValResultErr(component.ValU8(uint8(code)))}
}
```

Apply similar changes to all filesystem operations that return errors.

**Step 2: Run tests**

Run: `go test -v ./imports/wasip2/filesystem/...`
Expected: PASS

**Step 3: Commit**

```bash
git add imports/wasip2/filesystem/filesystem.go
git commit -m "feat(wasip2): use error code mapper in filesystem operations

All filesystem errors now return specific WASI error codes."
```

---

## Phase 7 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check that error code constants match expected values
2. Verify error mapping doesn't change success cases
3. Check WASI error result format
4. Debug and fix

**Run all WASI tests:**

```bash
go test -v ./imports/wasip2/...
```

**Mark Phase 7 complete in README.md**

---

## Final Verification

After completing all 7 phases:

1. **Run full test suite:**
   ```bash
   go test -v ./...
   ```

2. **Run calculator integration tests:**
   ```bash
   go test -v ./internal/component/wasip2test/... -run "TestCalculator"
   ```

3. **Review gap analysis coverage:**
   - Verify each HIGH priority item is addressed
   - Confirm MEDIUM priority items are complete
   - Document any remaining LOW priority items for future work

4. **Update README.md:**
   - Mark all phases as complete
   - Add notes about any deferred work
   - Link to test results
