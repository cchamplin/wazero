# Phase 6: Resource Lifecycle - Destructors & Cleanup

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add proper resource destructors and automatic cleanup to prevent resource leaks in long-running WASI components.

**Architecture:** Implement destructor callbacks for HTTP resources, add finalizers for critical resources, and create a type-safe resource table wrapper for better lifecycle management.

**Tech Stack:** Go interfaces, finalizers, type assertions

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 8 Resource Lifecycle Analysis

**Prerequisite:** Phases 1-5 should be complete to ensure all resources exist.

---

## Reference Materials

### Current Implementation
- **File:** `internal/component/resource_table.go`
- **Issues:**
  - No resource finalizers/destructors
  - No automatic cleanup on scope exit
  - Type safety relies on interface{} casts

### Resources Needing Destructors
| Interface | Resource | Current Cleanup | Needed |
|-----------|----------|-----------------|--------|
| wasi:http | fields | None | ✓ |
| wasi:http | incoming-body | None | Close reader |
| wasi:http | outgoing-body | None | ✓ |
| wasi:http | incoming-response | None | Close body |
| wasi:http | outgoing-request | None | ✓ |
| wasi:http | future-incoming-response | None | Cancel pending |
| wasi:io | input-stream | Manual | Close reader |
| wasi:io | output-stream | Manual | Flush/close writer |
| wasi:sockets | tcp-socket | Via destructor | ✓ existing |
| wasi:sockets | udp-socket | None | Close conn |
| wasi:filesystem | descriptor | Via Close() | ✓ existing |

---

## Task 6.1: Define Resource Destructor Interface

**Files:**
- Modify: `internal/component/resource_table.go`
- Test: `internal/component/resource_table_test.go`

**Step 1: Write failing test**

Add to `internal/component/resource_table_test.go`:

```go
package component

import (
	"testing"
)

// TestResource implements Destroyable
type TestResource struct {
	destroyed bool
}

func (r *TestResource) Destroy() {
	r.destroyed = true
}

func TestResourceTable_DestructorCalled(t *testing.T) {
	table := NewResourceTable()

	res := &TestResource{}
	handle := table.New(res, true)

	if res.destroyed {
		t.Error("resource should not be destroyed yet")
	}

	// Delete should call destructor
	table.Delete(handle)

	if !res.destroyed {
		t.Error("destructor should have been called")
	}
}

func TestResourceTable_DestructorNotCalledForBorrow(t *testing.T) {
	table := NewResourceTable()

	res := &TestResource{}
	handle := table.New(res, false) // borrow, not own

	// Delete borrow should not destroy
	table.Delete(handle)

	if res.destroyed {
		t.Error("destructor should not be called for borrows")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/component/... -run "TestResourceTable_Destructor"`
Expected: FAIL - Destroyable interface doesn't exist, Delete doesn't call destructor

**Step 3: Add Destroyable interface and update ResourceTable**

In `internal/component/resource_table.go`:

```go
package component

import (
	"sync"
)

// Destroyable is implemented by resources that need cleanup
type Destroyable interface {
	Destroy()
}

// resourceEntry holds a resource and its ownership status
type resourceEntry struct {
	value interface{}
	isOwn bool
}

// ResourceTable manages resource handles for the component model
type ResourceTable struct {
	mu        sync.Mutex
	resources map[int]*resourceEntry
	nextID    int
}

// NewResourceTable creates a new resource table
func NewResourceTable() *ResourceTable {
	return &ResourceTable{
		resources: make(map[int]*resourceEntry),
		nextID:    1,
	}
}

// New registers a new resource and returns its handle
func (t *ResourceTable) New(resource interface{}, isOwn bool) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	handle := t.nextID
	t.nextID++

	t.resources[handle] = &resourceEntry{
		value: resource,
		isOwn: isOwn,
	}

	return handle
}

// Get retrieves a resource by handle
func (t *ResourceTable) Get(handle int) interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry := t.resources[handle]
	if entry == nil {
		return nil
	}
	return entry.value
}

// Delete removes a resource and calls its destructor if owned
func (t *ResourceTable) Delete(handle int) {
	t.mu.Lock()
	entry := t.resources[handle]
	if entry == nil {
		t.mu.Unlock()
		return
	}
	delete(t.resources, handle)
	t.mu.Unlock()

	// Call destructor if owned and resource is Destroyable
	if entry.isOwn {
		if destroyable, ok := entry.value.(Destroyable); ok {
			destroyable.Destroy()
		}
	}
}

// Clear removes all resources, calling destructors for owned resources
func (t *ResourceTable) Clear() {
	t.mu.Lock()
	entries := make([]*resourceEntry, 0, len(t.resources))
	for _, entry := range t.resources {
		entries = append(entries, entry)
	}
	t.resources = make(map[int]*resourceEntry)
	t.mu.Unlock()

	// Call destructors outside lock
	for _, entry := range entries {
		if entry.isOwn {
			if destroyable, ok := entry.value.(Destroyable); ok {
				destroyable.Destroy()
			}
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/component/... -run "TestResourceTable_Destructor"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/component/resource_table.go internal/component/resource_table_test.go
git commit -m "feat(component): add Destroyable interface for resource cleanup

Resources implementing Destroy() will have it called when Delete() is
called on owned resources."
```

---

## Task 6.2: Add Destructor to HTTP Fields

**Files:**
- Modify: `imports/wasip2/http/http.go`
- Test: `imports/wasip2/http/http_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/http/http_test.go` (create if needed):

```go
package http

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
)

func TestFields_Destroy(t *testing.T) {
	fields := NewFields()
	fields.Set("Content-Type", []byte("text/plain"))

	// Verify fields exist
	if len(fields.entries) == 0 {
		t.Error("expected entries")
	}

	// Destroy should clear
	fields.Destroy()

	if len(fields.entries) != 0 {
		t.Error("entries should be cleared after Destroy")
	}
}

func TestFields_ImplementsDestroyable(t *testing.T) {
	fields := NewFields()

	// Verify implements interface
	var _ component.Destroyable = fields
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/http/... -run "TestFields_Destroy|TestFields_Implements"`
Expected: FAIL - Fields doesn't implement Destroyable

**Step 3: Add Destroy method to Fields**

In `imports/wasip2/http/http.go`:

```go
// Destroy cleans up the fields resource
func (f *Fields) Destroy() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/http/... -run "TestFields_Destroy|TestFields_Implements"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/http_test.go
git commit -m "feat(wasip2): add Destroy method to HTTP Fields

Fields now implements Destroyable for automatic cleanup."
```

---

## Task 6.3: Add Destructor to HTTP Bodies

**Files:**
- Modify: `imports/wasip2/http/http.go`
- Test: `imports/wasip2/http/http_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/http/http_test.go`:

```go
func TestIncomingBody_Destroy(t *testing.T) {
	// Create a mock reader that tracks close
	closed := false
	reader := &mockReadCloser{
		onClose: func() { closed = true },
	}

	body := &IncomingBody{reader: reader}

	body.Destroy()

	if !closed {
		t.Error("reader should be closed on Destroy")
	}
}

func TestOutgoingBody_Destroy(t *testing.T) {
	body := NewOutgoingBody()
	body.outputStream.Write([]byte("test"))

	body.Destroy()

	// Buffer should be reset
	if body.outputStream.Len() != 0 {
		t.Error("buffer should be cleared on Destroy")
	}
}

type mockReadCloser struct {
	onClose func()
}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (m *mockReadCloser) Close() error {
	if m.onClose != nil {
		m.onClose()
	}
	return nil
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/http/... -run "TestIncomingBody_Destroy|TestOutgoingBody_Destroy"`
Expected: FAIL - Destroy methods don't exist

**Step 3: Add Destroy methods to body types**

In `imports/wasip2/http/http.go`:

```go
// Destroy cleans up the incoming body
func (b *IncomingBody) Destroy() {
	if b.reader != nil {
		b.reader.Close()
		b.reader = nil
	}
	b.inputStream = nil
	b.consumed = true
}

// Destroy cleans up the outgoing body
func (b *OutgoingBody) Destroy() {
	if b.outputStream != nil {
		b.outputStream.Reset()
	}
	b.wasiStream = nil
	b.finished = true
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/http/... -run "TestIncomingBody_Destroy|TestOutgoingBody_Destroy"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/http_test.go
git commit -m "feat(wasip2): add Destroy methods to HTTP body types

IncomingBody closes reader, OutgoingBody resets buffer."
```

---

## Task 6.4: Add Destructor to HTTP Request/Response

**Files:**
- Modify: `imports/wasip2/http/http.go`
- Test: `imports/wasip2/http/http_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/http/http_test.go`:

```go
func TestOutgoingRequest_Destroy(t *testing.T) {
	req := &OutgoingRequest{
		headers: NewFields(),
		body:    NewOutgoingBody(),
	}
	req.headers.Set("Test", []byte("value"))

	req.Destroy()

	// Headers and body should be destroyed
	if req.headers != nil && len(req.headers.entries) > 0 {
		t.Error("headers should be cleared")
	}
}

func TestIncomingResponse_Destroy(t *testing.T) {
	closed := false
	reader := &mockReadCloser{onClose: func() { closed = true }}

	resp := &IncomingResponse{
		headers: NewFields(),
		body:    &IncomingBody{reader: reader},
	}

	resp.Destroy()

	if !closed {
		t.Error("body reader should be closed")
	}
}

func TestFutureIncomingResponse_Destroy(t *testing.T) {
	future := &FutureIncomingResponse{
		ready: make(chan struct{}),
	}

	// Start a fake pending request
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(future.ready)
	}()

	// Destroy should handle pending state
	future.Destroy()

	// Should be marked consumed
	if !future.consumed {
		t.Error("future should be marked consumed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/http/... -run "TestOutgoingRequest_Destroy|TestIncomingResponse_Destroy|TestFutureIncomingResponse_Destroy"`
Expected: FAIL - Destroy methods don't exist

**Step 3: Add Destroy methods**

In `imports/wasip2/http/http.go`:

```go
// Destroy cleans up the outgoing request
func (r *OutgoingRequest) Destroy() {
	if r.headers != nil {
		r.headers.Destroy()
	}
	if r.body != nil {
		r.body.Destroy()
	}
	r.headers = nil
	r.body = nil
}

// Destroy cleans up the incoming response
func (r *IncomingResponse) Destroy() {
	if r.headers != nil {
		r.headers.Destroy()
	}
	if r.body != nil {
		r.body.Destroy()
	}
	r.headers = nil
	r.body = nil
}

// Destroy cleans up the future response
func (f *FutureIncomingResponse) Destroy() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Mark as consumed to prevent further use
	f.consumed = true

	// Clean up response if present
	if f.response != nil {
		f.response.Destroy()
		f.response = nil
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/http/... -run "TestOutgoingRequest_Destroy|TestIncomingResponse_Destroy|TestFutureIncomingResponse_Destroy"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/http/http.go imports/wasip2/http/http_test.go
git commit -m "feat(wasip2): add Destroy methods to HTTP request/response types

Full cleanup chain for HTTP resources."
```

---

## Task 6.5: Add Destructor to UDP Socket

**Files:**
- Modify: `imports/wasip2/sockets/udp.go`
- Test: `imports/wasip2/sockets/udp_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/sockets/udp_test.go`:

```go
func TestUdpSocket_Destroy(t *testing.T) {
	sock, _ := NewUdpSocket(AddressFamilyIPv4)
	sock.StartBind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	sock.FinishBind()

	// Verify socket is bound
	if sock.conn == nil {
		t.Fatal("expected bound socket")
	}

	sock.Destroy()

	// Connection should be closed
	if sock.conn != nil {
		t.Error("connection should be nil after Destroy")
	}

	// State should indicate closed
	if sock.state != UdpStateClosed {
		t.Error("state should be Closed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Destroy"`
Expected: FAIL - Destroy doesn't exist, UdpStateClosed not defined

**Step 3: Add Destroy method and Closed state**

In `imports/wasip2/sockets/udp.go`:

```go
const (
	UdpStateUnbound UdpState = iota
	UdpStateBinding
	UdpStateBound
	UdpStateConnected
	UdpStateClosed
)

// Destroy closes the UDP socket and releases resources
func (s *UdpSocket) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	s.state = UdpStateClosed
	s.pendingLocalAddr = nil
	s.remoteAddr = nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/sockets/... -run "TestUdpSocket_Destroy"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/sockets/udp.go imports/wasip2/sockets/udp_test.go
git commit -m "feat(wasip2): add Destroy method to UdpSocket

Closes connection and releases all resources."
```

---

## Task 6.6: Add Destructor to Streams

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go`

**Step 1: Write failing test**

Add to `imports/wasip2/io/streams_test.go`:

```go
func TestInputStream_Destroy(t *testing.T) {
	closed := false
	reader := &mockReadCloser{onClose: func() { closed = true }}

	stream := NewInputStream(reader)
	stream.Destroy()

	if !closed {
		t.Error("reader should be closed on Destroy")
	}
}

func TestOutputStream_Destroy(t *testing.T) {
	flushed := false
	closed := false

	writer := &mockWriteCloser{
		onFlush: func() { flushed = true },
		onClose: func() { closed = true },
	}

	stream := NewOutputStream(writer)
	stream.Destroy()

	if !flushed {
		t.Error("writer should be flushed on Destroy")
	}
	if !closed {
		t.Error("writer should be closed on Destroy")
	}
}

type mockReadCloser struct {
	onClose func()
}

func (m *mockReadCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (m *mockReadCloser) Close() error {
	if m.onClose != nil {
		m.onClose()
	}
	return nil
}

type mockWriteCloser struct {
	bytes.Buffer
	onFlush func()
	onClose func()
}

func (m *mockWriteCloser) Flush() error {
	if m.onFlush != nil {
		m.onFlush()
	}
	return nil
}

func (m *mockWriteCloser) Close() error {
	if m.onClose != nil {
		m.onClose()
	}
	return nil
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_Destroy|TestOutputStream_Destroy"`
Expected: FAIL - Destroy methods don't exist

**Step 3: Add Destroy methods to streams**

In `imports/wasip2/io/streams.go`:

```go
// Closer is implemented by readers/writers that can be closed
type Closer interface {
	Close() error
}

// Destroy closes the input stream if it supports closing
func (s *InputStream) Destroy() {
	if closer, ok := s.reader.(Closer); ok {
		closer.Close()
	}
	s.reader = nil
}

// Destroy flushes and closes the output stream
func (s *OutputStream) Destroy() {
	// Flush if supported
	if flusher, ok := s.writer.(Flusher); ok {
		flusher.Flush()
	}

	// Close if supported
	if closer, ok := s.writer.(Closer); ok {
		closer.Close()
	}

	s.writer = nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestInputStream_Destroy|TestOutputStream_Destroy"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): add Destroy methods to streams

InputStream closes reader, OutputStream flushes and closes writer."
```

---

## Phase 6 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check that Destroyable interface is in the right package
2. Verify Delete() is called at appropriate times
3. Check existing destructors still work
4. Debug and fix before proceeding to Phase 7

**Mark Phase 6 complete in README.md**
