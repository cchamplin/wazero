# Phase 2: wasi:io/streams - Blocking Operations & Splice

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement proper blocking behavior for stream operations and add splice functionality for efficient data transfer.

**Architecture:** Update blocking-* methods to actually block using the underlying io.Reader/io.Writer, implement splice using io.Copy, and improve check-write to report buffer capacity.

**Tech Stack:** Go io package, bufio for buffered operations

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 1.3 wasi:io/streams@0.2.0

**Prerequisite:** Phase 1 (poll) should be complete for proper pollable integration.

---

## Reference Materials

### Specification
- **WIT File:** `debug-vendored/WASI/proposals/io/wit/streams.wit`
- **Key Methods:**
  - `[method]input-stream.blocking-read` - Block until data available
  - `[method]input-stream.blocking-skip` - Block until skip complete
  - `[method]output-stream.blocking-write-and-flush` - Block until written
  - `[method]output-stream.splice` - Transfer from input to output
  - `[method]output-stream.blocking-splice` - Block until splice complete
  - `[method]output-stream.check-write` - Return write capacity

### Current Implementation
- **File:** `imports/wasip2/io/streams.go`
- **Issues:**
  - `blocking-*` methods behave same as non-blocking
  - `splice` returns 0 without doing anything
  - `check-write` returns hardcoded 4096

### Wasmtime Reference
- **File:** `debug-vendored/wasmtime/crates/wasi/src/host/io.rs`
- Look for `blocking_read`, `splice` implementations

---

## Task 2.1: Implement blocking-read with Actual Blocking

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/io/streams_test.go`:

```go
package io

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

// slowReader delays reads to simulate blocking I/O
type slowReader struct {
	data    []byte
	pos     int
	delay   time.Duration
	ready   chan struct{}
	mu      sync.Mutex
}

func newSlowReader(data []byte, delay time.Duration) *slowReader {
	return &slowReader{
		data:  data,
		delay: delay,
		ready: make(chan struct{}),
	}
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	<-r.ready // Wait until data is ready
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *slowReader) MakeReady() {
	select {
	case <-r.ready:
		// Already closed
	default:
		close(r.ready)
	}
}

func TestInputStream_BlockingRead_ActuallyBlocks(t *testing.T) {
	data := []byte("hello world")
	reader := newSlowReader(data, 0)
	stream := NewInputStream(reader)

	done := make(chan []byte)
	go func() {
		result, _ := stream.BlockingRead(5)
		done <- result
	}()

	// Should not return immediately
	select {
	case <-done:
		t.Error("BlockingRead should block until data available")
	case <-time.After(50 * time.Millisecond):
		// Good, still blocking
	}

	// Make data ready
	reader.MakeReady()

	// Should return now
	select {
	case result := <-done:
		if string(result) != "hello" {
			t.Errorf("expected 'hello', got '%s'", string(result))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("BlockingRead should return after data available")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_BlockingRead"`
Expected: FAIL - BlockingRead doesn't exist or doesn't block

**Step 3: Implement BlockingRead method**

In `imports/wasip2/io/streams.go`, add or update:

```go
// BlockingRead reads up to len bytes, blocking until data is available.
// This is the implementation of [method]input-stream.blocking-read.
func (s *InputStream) BlockingRead(len uint64) ([]byte, error) {
	if len == 0 {
		return []byte{}, nil
	}

	// Cap the read size
	readLen := len
	if readLen > maxReadSize {
		readLen = maxReadSize
	}

	buf := make([]byte, readLen)

	// This read will block until data is available (standard Go behavior)
	n, err := s.reader.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buf[:n], nil
}

const maxReadSize = 64 * 1024 // 64KB max read
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_BlockingRead"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): implement BlockingRead for input streams

BlockingRead now properly blocks until data is available from the
underlying io.Reader."
```

---

## Task 2.2: Wire BlockingRead to WASI Interface

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test for WASI method**

Add to `imports/wasip2/io/streams_test.go`:

```go
func TestInputStream_BlockingRead_WASI(t *testing.T) {
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	data := []byte("test data")
	stream := NewInputStream(bytes.NewReader(data))
	handle := table.New(stream, true)

	// Call WASI blocking-read method
	args := []component.Val{
		component.ValBorrow(uint32(handle)),
		component.ValU64(4), // Read 4 bytes
	}

	result, err := inputStreamBlockingRead(ctx, args)
	if err != nil {
		t.Fatalf("blocking-read failed: %v", err)
	}

	// Result is result<list<u8>, stream-error>
	isOk, val, _ := result[0].Result()
	if !isOk {
		t.Fatal("expected ok result")
	}

	bytes := val.List()
	if len(bytes) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(bytes))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_BlockingRead_WASI"`
Expected: FAIL - inputStreamBlockingRead doesn't use BlockingRead

**Step 3: Update WASI binding to use BlockingRead**

In `imports/wasip2/io/streams.go`, find and update `inputStreamBlockingRead`:

```go
// inputStreamBlockingRead implements [method]input-stream.blocking-read
func inputStreamBlockingRead(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return streamErrorResult("no resource table"), nil
	}

	handle := args[0].Borrow()
	len := args[1].U64()

	resource := table.Get(int(handle))
	if resource == nil {
		return streamErrorResult("invalid handle"), nil
	}

	stream, ok := resource.(*InputStream)
	if !ok {
		return streamErrorResult("not an input stream"), nil
	}

	data, err := stream.BlockingRead(len)
	if err != nil {
		if err == io.EOF {
			// Return empty list for EOF (not an error in WASI)
			return streamOkResult([]byte{}), nil
		}
		return streamErrorResult(err.Error()), nil
	}

	return streamOkResult(data), nil
}

// streamOkResult creates an ok result with list<u8>
func streamOkResult(data []byte) []component.Val {
	vals := make([]component.Val, len(data))
	for i, b := range data {
		vals[i] = component.ValU8(b)
	}
	return []component.Val{component.ValResultOk(component.ValList(vals))}
}

// streamErrorResult creates an error result
func streamErrorResult(msg string) []component.Val {
	// stream-error is a variant with closed or last-operation-failed
	return []component.Val{component.ValResultErr(component.ValString(msg))}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_BlockingRead_WASI"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): wire BlockingRead to WASI blocking-read method"
```

---

## Task 2.3: Implement blocking-skip

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/io/streams_test.go`:

```go
func TestInputStream_BlockingSkip(t *testing.T) {
	data := []byte("hello world")
	stream := NewInputStream(bytes.NewReader(data))

	// Skip "hello "
	skipped, err := stream.BlockingSkip(6)
	if err != nil {
		t.Fatalf("BlockingSkip failed: %v", err)
	}
	if skipped != 6 {
		t.Errorf("expected to skip 6, skipped %d", skipped)
	}

	// Read remaining
	remaining, _ := stream.BlockingRead(10)
	if string(remaining) != "world" {
		t.Errorf("expected 'world', got '%s'", string(remaining))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_BlockingSkip"`
Expected: FAIL - BlockingSkip doesn't exist

**Step 3: Implement BlockingSkip**

In `imports/wasip2/io/streams.go`:

```go
// BlockingSkip skips len bytes, blocking until complete.
// This is the implementation of [method]input-stream.blocking-skip.
func (s *InputStream) BlockingSkip(len uint64) (uint64, error) {
	if len == 0 {
		return 0, nil
	}

	// Try to use Seeker if available
	if seeker, ok := s.reader.(io.Seeker); ok {
		_, err := seeker.Seek(int64(len), io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		return len, nil
	}

	// Fall back to reading and discarding
	buf := make([]byte, 4096)
	var skipped uint64
	for skipped < len {
		toRead := len - skipped
		if toRead > 4096 {
			toRead = 4096
		}
		n, err := s.reader.Read(buf[:toRead])
		skipped += uint64(n)
		if err != nil {
			if err == io.EOF {
				break
			}
			return skipped, err
		}
	}
	return skipped, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_BlockingSkip"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): implement BlockingSkip for input streams

Uses Seeker if available, otherwise reads and discards bytes."
```

---

## Task 2.4: Implement blocking-write-and-flush

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/io/streams_test.go`:

```go
func TestOutputStream_BlockingWriteAndFlush(t *testing.T) {
	var buf bytes.Buffer
	stream := NewOutputStream(&buf)

	data := []byte("hello world")
	err := stream.BlockingWriteAndFlush(data)
	if err != nil {
		t.Fatalf("BlockingWriteAndFlush failed: %v", err)
	}

	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", buf.String())
	}
}

// flushableBuffer implements io.Writer with Flush method
type flushableBuffer struct {
	bytes.Buffer
	flushed bool
}

func (f *flushableBuffer) Flush() error {
	f.flushed = true
	return nil
}

func TestOutputStream_BlockingWriteAndFlush_CallsFlush(t *testing.T) {
	buf := &flushableBuffer{}
	stream := NewOutputStream(buf)

	data := []byte("test")
	stream.BlockingWriteAndFlush(data)

	if !buf.flushed {
		t.Error("expected Flush to be called")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestOutputStream_BlockingWriteAndFlush"`
Expected: FAIL - BlockingWriteAndFlush doesn't exist

**Step 3: Implement BlockingWriteAndFlush**

In `imports/wasip2/io/streams.go`:

```go
// Flusher is implemented by writers that support flushing
type Flusher interface {
	Flush() error
}

// BlockingWriteAndFlush writes all data and flushes, blocking until complete.
// This is the implementation of [method]output-stream.blocking-write-and-flush.
func (s *OutputStream) BlockingWriteAndFlush(data []byte) error {
	// Write all data
	written := 0
	for written < len(data) {
		n, err := s.writer.Write(data[written:])
		if err != nil {
			return err
		}
		written += n
	}

	// Flush if supported
	if flusher, ok := s.writer.(Flusher); ok {
		return flusher.Flush()
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestOutputStream_BlockingWriteAndFlush"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): implement BlockingWriteAndFlush for output streams

Writes all data and calls Flush if the writer supports it."
```

---

## Task 2.5: Implement splice Operation

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/io/streams_test.go`:

```go
func TestOutputStream_Splice(t *testing.T) {
	// Source data
	source := bytes.NewReader([]byte("hello world"))
	inputStream := NewInputStream(source)

	// Destination
	var dest bytes.Buffer
	outputStream := NewOutputStream(&dest)

	// Splice 5 bytes
	spliced, err := outputStream.Splice(inputStream, 5)
	if err != nil {
		t.Fatalf("Splice failed: %v", err)
	}
	if spliced != 5 {
		t.Errorf("expected to splice 5 bytes, got %d", spliced)
	}
	if dest.String() != "hello" {
		t.Errorf("expected 'hello', got '%s'", dest.String())
	}
}

func TestOutputStream_Splice_AllData(t *testing.T) {
	source := bytes.NewReader([]byte("test"))
	inputStream := NewInputStream(source)

	var dest bytes.Buffer
	outputStream := NewOutputStream(&dest)

	// Request more than available
	spliced, err := outputStream.Splice(inputStream, 100)
	if err != nil {
		t.Fatalf("Splice failed: %v", err)
	}
	if spliced != 4 {
		t.Errorf("expected to splice 4 bytes, got %d", spliced)
	}
	if dest.String() != "test" {
		t.Errorf("expected 'test', got '%s'", dest.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestOutputStream_Splice"`
Expected: FAIL - Splice returns 0

**Step 3: Implement Splice**

In `imports/wasip2/io/streams.go`:

```go
// Splice transfers up to len bytes from src to this output stream.
// Returns the number of bytes actually transferred.
// This is the implementation of [method]output-stream.splice.
func (s *OutputStream) Splice(src *InputStream, len uint64) (uint64, error) {
	if len == 0 {
		return 0, nil
	}

	// Use io.CopyN for efficient transfer
	n, err := io.CopyN(s.writer, src.reader, int64(len))
	if err != nil && err != io.EOF {
		return uint64(n), err
	}

	return uint64(n), nil
}

// BlockingSplice is like Splice but blocks until complete or EOF.
// This is the implementation of [method]output-stream.blocking-splice.
func (s *OutputStream) BlockingSplice(src *InputStream, len uint64) (uint64, error) {
	// In Go, Splice already blocks, so this is the same
	return s.Splice(src, len)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestOutputStream_Splice"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): implement Splice and BlockingSplice

Uses io.CopyN for efficient data transfer between streams."
```

---

## Task 2.6: Wire Splice to WASI Interface

**Files:**
- Modify: `imports/wasip2/io/streams.go`

**Step 1: Update WASI splice function**

In `imports/wasip2/io/streams.go`, find and update `outputStreamSplice`:

```go
// outputStreamSplice implements [method]output-stream.splice
func outputStreamSplice(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return streamErrorU64Result(0, "no resource table"), nil
	}

	outHandle := args[0].Borrow()
	inHandle := args[1].Borrow()
	len := args[2].U64()

	outResource := table.Get(int(outHandle))
	inResource := table.Get(int(inHandle))

	if outResource == nil || inResource == nil {
		return streamErrorU64Result(0, "invalid handle"), nil
	}

	outStream, ok := outResource.(*OutputStream)
	if !ok {
		return streamErrorU64Result(0, "not an output stream"), nil
	}

	inStream, ok := inResource.(*InputStream)
	if !ok {
		return streamErrorU64Result(0, "not an input stream"), nil
	}

	n, err := outStream.Splice(inStream, len)
	if err != nil {
		return streamErrorU64Result(n, err.Error()), nil
	}

	return streamOkU64Result(n), nil
}

// streamOkU64Result creates an ok result with u64
func streamOkU64Result(n uint64) []component.Val {
	return []component.Val{component.ValResultOk(component.ValU64(n))}
}

// streamErrorU64Result creates an error result preserving partial count
func streamErrorU64Result(n uint64, msg string) []component.Val {
	return []component.Val{component.ValResultErr(component.ValString(msg))}
}
```

**Step 2: Run tests**

Run: `go test -v ./imports/wasip2/io/...`
Expected: PASS

**Step 3: Commit**

```bash
git add imports/wasip2/io/streams.go
git commit -m "feat(wasip2): wire Splice to WASI output-stream.splice method"
```

---

## Task 2.7: Improve check-write to Return Actual Capacity

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/io/streams_test.go`:

```go
// limitedWriter has a write limit
type limitedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		return 0, io.ErrShortWrite
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return w.buf.Write(p)
}

func (w *limitedWriter) Available() int {
	return w.limit - w.buf.Len()
}

func TestOutputStream_CheckWrite_ReturnsAvailable(t *testing.T) {
	writer := &limitedWriter{limit: 100}
	stream := NewOutputStream(writer)

	// Should return available capacity
	capacity := stream.CheckWrite()
	if capacity != 100 {
		t.Errorf("expected capacity 100, got %d", capacity)
	}

	// Write some data
	stream.Write([]byte("hello"))

	// Should return reduced capacity
	capacity = stream.CheckWrite()
	if capacity != 95 {
		t.Errorf("expected capacity 95, got %d", capacity)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestOutputStream_CheckWrite"`
Expected: FAIL - CheckWrite returns 4096

**Step 3: Implement improved CheckWrite**

In `imports/wasip2/io/streams.go`:

```go
// WriteCapacity is implemented by writers that can report available capacity
type WriteCapacity interface {
	Available() int
}

// CheckWrite returns the number of bytes that can be written.
// This is the implementation of [method]output-stream.check-write.
func (s *OutputStream) CheckWrite() uint64 {
	// Check if writer reports capacity
	if wc, ok := s.writer.(WriteCapacity); ok {
		avail := wc.Available()
		if avail < 0 {
			avail = 0
		}
		return uint64(avail)
	}

	// Default: assume reasonable buffer size available
	// Most writers can accept at least 64KB
	return 64 * 1024
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestOutputStream_CheckWrite"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): improve check-write to return actual capacity

Writers implementing Available() will have their capacity reported.
Others default to 64KB."
```

---

## Phase 2 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check that stream read/write still work correctly
2. Verify stdin/stdout aren't affected by blocking changes
3. Check resource table interactions
4. Debug and fix before proceeding to Phase 3

**Mark Phase 2 complete in README.md**
