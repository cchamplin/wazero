# Phase 1: wasi:io/poll - Blocking I/O Foundation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement proper blocking behavior for pollables, enabling correct async I/O patterns throughout WASI.

**Architecture:** Add a ready channel to Pollable struct, implement actual blocking in `block` method using Go channels, and fix the `poll` function to correctly identify ready pollables.

**Tech Stack:** Go channels, select statements, context cancellation

**Gap Analysis Reference:** [wasip2-interface-gap-analysis.md](../wasip2-interface-gap-analysis.md) - Section 1.2 wasi:io/poll@0.2.0

---

## Reference Materials

### Specification
- **WIT File:** `debug-vendored/WASI/proposals/io/wit/poll.wit`
- **Key Functions:**
  - `[method]pollable.ready: func() -> bool`
  - `[method]pollable.block: func()` - Block until ready
  - `poll: func(in: list<borrow<pollable>>) -> list<u32>` - Return indices of ready pollables

### Current Implementation
- **File:** `imports/wasip2/io/poll.go`
- **Issues:**
  - `block` returns immediately without blocking
  - `poll` always returns `[0]` regardless of actual ready state

### Wasmtime Reference
- **File:** `debug-vendored/wasmtime/crates/wasi/src/host/io.rs`
- Look for `impl pollable` and `poll` function implementations

---

## Task 1.1: Add Ready State Infrastructure to Pollable

**Files:**
- Modify: `imports/wasip2/io/poll.go`
- Test: `imports/wasip2/io/poll_test.go` (create)

**Step 1: Write the failing test**

Create test file `imports/wasip2/io/poll_test.go`:

```go
package io

import (
	"testing"
	"time"
)

func TestPollable_Ready_InitiallyFalse(t *testing.T) {
	p := NewPollable()
	if p.IsReady() {
		t.Error("new pollable should not be ready initially")
	}
}

func TestPollable_Ready_AfterSignal(t *testing.T) {
	p := NewPollable()
	p.SetReady()
	if !p.IsReady() {
		t.Error("pollable should be ready after SetReady")
	}
}

func TestPollable_Block_ReturnsWhenReady(t *testing.T) {
	p := NewPollable()

	done := make(chan struct{})
	go func() {
		p.Block()
		close(done)
	}()

	// Should not return immediately
	select {
	case <-done:
		t.Error("Block should not return before SetReady")
	case <-time.After(50 * time.Millisecond):
		// Good, still blocking
	}

	// Signal ready
	p.SetReady()

	// Should return now
	select {
	case <-done:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Block should return after SetReady")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestPollable"`
Expected: FAIL - NewPollable, IsReady, SetReady, Block methods don't exist or behave correctly

**Step 3: Modify Pollable struct in poll.go**

In `imports/wasip2/io/poll.go`, update the Pollable struct:

```go
// Pollable represents a wasi:io/poll pollable resource.
// It can be used to wait for I/O readiness.
type Pollable struct {
	ready    chan struct{} // Closed when ready
	isReady  bool          // Cached ready state
	mu       sync.Mutex    // Protects isReady
	onReady  func()        // Optional callback when becoming ready
}

// NewPollable creates a new pollable that is not yet ready.
func NewPollable() *Pollable {
	return &Pollable{
		ready: make(chan struct{}),
	}
}

// NewReadyPollable creates a pollable that is already ready.
func NewReadyPollable() *Pollable {
	p := &Pollable{
		ready:   make(chan struct{}),
		isReady: true,
	}
	close(p.ready)
	return p
}

// IsReady returns true if the pollable is ready.
func (p *Pollable) IsReady() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isReady
}

// SetReady marks the pollable as ready, unblocking any waiters.
func (p *Pollable) SetReady() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isReady {
		p.isReady = true
		close(p.ready)
		if p.onReady != nil {
			p.onReady()
		}
	}
}

// Block blocks until the pollable is ready.
func (p *Pollable) Block() {
	<-p.ready
}

// ReadyChan returns the channel that is closed when ready.
// Use for select statements.
func (p *Pollable) ReadyChan() <-chan struct{} {
	return p.ready
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestPollable"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/poll.go imports/wasip2/io/poll_test.go
git commit -m "feat(wasip2): add ready state infrastructure to Pollable

- Add ready channel for blocking support
- Add IsReady, SetReady, Block methods
- Add NewPollable and NewReadyPollable constructors

Ref: docs/plans/wasip2-interface-gap-analysis.md Section 1.2"
```

---

## Task 1.2: Update Pollable WASI Methods to Use New Infrastructure

**Files:**
- Modify: `imports/wasip2/io/poll.go`
- Test: `imports/wasip2/io/poll_test.go`

**Step 1: Write failing test for WASI ready method**

Add to `imports/wasip2/io/poll_test.go`:

```go
func TestPollableReady_WASIMethod(t *testing.T) {
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	// Create pollable and register
	p := NewPollable()
	handle := table.New(p, true)

	// Test ready returns false initially
	args := []component.Val{component.ValBorrow(uint32(handle))}
	result, err := pollableReady(ctx, args)
	if err != nil {
		t.Fatalf("ready failed: %v", err)
	}
	if result[0].Bool() {
		t.Error("ready should return false for new pollable")
	}

	// Mark ready and test again
	p.SetReady()
	result, err = pollableReady(ctx, args)
	if err != nil {
		t.Fatalf("ready failed: %v", err)
	}
	if !result[0].Bool() {
		t.Error("ready should return true after SetReady")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestPollableReady_WASIMethod"`
Expected: FAIL - pollableReady doesn't use IsReady()

**Step 3: Update pollableReady function**

In `imports/wasip2/io/poll.go`, update the `pollableReady` function:

```go
// pollableReady implements [method]pollable.ready
func pollableReady(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	handle := args[0].Borrow()
	resource := table.Get(int(handle))
	if resource == nil {
		return []component.Val{component.ValBool(false)}, nil
	}

	pollable, ok := resource.(*Pollable)
	if !ok {
		return []component.Val{component.ValBool(false)}, nil
	}

	return []component.Val{component.ValBool(pollable.IsReady())}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestPollableReady_WASIMethod"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/poll.go imports/wasip2/io/poll_test.go
git commit -m "feat(wasip2): update pollable.ready to use IsReady method

WASI [method]pollable.ready now correctly returns the pollable's
ready state instead of always returning true."
```

---

## Task 1.3: Implement WASI Block Method with Actual Blocking

**Files:**
- Modify: `imports/wasip2/io/poll.go`
- Test: `imports/wasip2/io/poll_test.go`

**Step 1: Write failing test for WASI block method**

Add to `imports/wasip2/io/poll_test.go`:

```go
func TestPollableBlock_WASIMethod(t *testing.T) {
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	p := NewPollable()
	handle := table.New(p, true)

	args := []component.Val{component.ValBorrow(uint32(handle))}

	done := make(chan error)
	go func() {
		_, err := pollableBlock(ctx, args)
		done <- err
	}()

	// Should not return immediately
	select {
	case err := <-done:
		t.Errorf("block returned too early: %v", err)
	case <-time.After(50 * time.Millisecond):
		// Good, still blocking
	}

	// Signal ready
	p.SetReady()

	// Should return now
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("block returned error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("block should return after SetReady")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestPollableBlock_WASIMethod"`
Expected: FAIL - block returns immediately

**Step 3: Update pollableBlock function**

In `imports/wasip2/io/poll.go`, update the `pollableBlock` function:

```go
// pollableBlock implements [method]pollable.block
func pollableBlock(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{}, nil
	}

	handle := args[0].Borrow()
	resource := table.Get(int(handle))
	if resource == nil {
		return []component.Val{}, nil
	}

	pollable, ok := resource.(*Pollable)
	if !ok {
		return []component.Val{}, nil
	}

	// Block until ready, respecting context cancellation
	select {
	case <-pollable.ReadyChan():
		// Ready
	case <-ctx.Done():
		// Context cancelled
	}

	return []component.Val{}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestPollableBlock_WASIMethod"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/poll.go imports/wasip2/io/poll_test.go
git commit -m "feat(wasip2): implement actual blocking in pollable.block

WASI [method]pollable.block now blocks until the pollable becomes
ready or the context is cancelled."
```

---

## Task 1.4: Implement Correct Poll Function for Multiple Pollables

**Files:**
- Modify: `imports/wasip2/io/poll.go`
- Test: `imports/wasip2/io/poll_test.go`

**Step 1: Write failing test for poll function**

Add to `imports/wasip2/io/poll_test.go`:

```go
func TestPoll_ReturnsReadyIndices(t *testing.T) {
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	// Create 3 pollables
	p0 := NewPollable()
	p1 := NewPollable()
	p2 := NewPollable()

	h0 := table.New(p0, true)
	h1 := table.New(p1, true)
	h2 := table.New(p2, true)

	// Mark p1 as ready
	p1.SetReady()

	// Build args: list<borrow<pollable>>
	handles := []component.Val{
		component.ValBorrow(uint32(h0)),
		component.ValBorrow(uint32(h1)),
		component.ValBorrow(uint32(h2)),
	}
	args := []component.Val{component.ValList(handles)}

	result, err := pollFunc(ctx, args)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}

	// Should return [1] since p1 is ready
	readyList := result[0].List()
	if len(readyList) != 1 {
		t.Fatalf("expected 1 ready pollable, got %d", len(readyList))
	}
	if readyList[0].U32() != 1 {
		t.Errorf("expected index 1, got %d", readyList[0].U32())
	}
}

func TestPoll_ReturnsMultipleReadyIndices(t *testing.T) {
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	// Create 3 pollables
	p0 := NewPollable()
	p1 := NewPollable()
	p2 := NewPollable()

	h0 := table.New(p0, true)
	h1 := table.New(p1, true)
	h2 := table.New(p2, true)

	// Mark p0 and p2 as ready
	p0.SetReady()
	p2.SetReady()

	handles := []component.Val{
		component.ValBorrow(uint32(h0)),
		component.ValBorrow(uint32(h1)),
		component.ValBorrow(uint32(h2)),
	}
	args := []component.Val{component.ValList(handles)}

	result, err := pollFunc(ctx, args)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}

	// Should return [0, 2] since p0 and p2 are ready
	readyList := result[0].List()
	if len(readyList) != 2 {
		t.Fatalf("expected 2 ready pollables, got %d", len(readyList))
	}

	// Check indices (order may vary)
	indices := make(map[uint32]bool)
	for _, v := range readyList {
		indices[v.U32()] = true
	}
	if !indices[0] || !indices[2] {
		t.Errorf("expected indices 0 and 2, got %v", indices)
	}
}

func TestPoll_BlocksUntilOneReady(t *testing.T) {
	ctx := context.Background()
	table := component.NewResourceTable()
	ctx = component.ContextWithResourceTable(ctx, table)

	p0 := NewPollable()
	p1 := NewPollable()

	h0 := table.New(p0, true)
	h1 := table.New(p1, true)

	handles := []component.Val{
		component.ValBorrow(uint32(h0)),
		component.ValBorrow(uint32(h1)),
	}
	args := []component.Val{component.ValList(handles)}

	done := make(chan []component.Val)
	go func() {
		result, _ := pollFunc(ctx, args)
		done <- result
	}()

	// Should block since none ready
	select {
	case <-done:
		t.Error("poll should block when no pollables ready")
	case <-time.After(50 * time.Millisecond):
		// Good
	}

	// Make p1 ready
	p1.SetReady()

	select {
	case result := <-done:
		readyList := result[0].List()
		if len(readyList) != 1 || readyList[0].U32() != 1 {
			t.Errorf("expected [1], got %v", readyList)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("poll should return after pollable becomes ready")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestPoll_"`
Expected: FAIL - poll always returns [0]

**Step 3: Implement correct poll function**

In `imports/wasip2/io/poll.go`, rewrite the `pollFunc` function:

```go
// pollFunc implements poll: func(in: list<borrow<pollable>>) -> list<u32>
func pollFunc(ctx context.Context, args []component.Val) ([]component.Val, error) {
	table := component.ResourceTableFromContext(ctx)
	if table == nil {
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	handleList := args[0].List()
	if len(handleList) == 0 {
		return []component.Val{component.ValList([]component.Val{})}, nil
	}

	// Collect pollables
	pollables := make([]*Pollable, len(handleList))
	for i, h := range handleList {
		handle := h.Borrow()
		resource := table.Get(int(handle))
		if resource == nil {
			continue
		}
		p, ok := resource.(*Pollable)
		if ok {
			pollables[i] = p
		}
	}

	// First check if any are already ready
	readyIndices := collectReadyIndices(pollables)
	if len(readyIndices) > 0 {
		return []component.Val{component.ValList(readyIndices)}, nil
	}

	// None ready, need to block until at least one is ready
	// Build a select across all pollable channels
	cases := make([]reflect.SelectCase, len(pollables)+1)
	for i, p := range pollables {
		if p != nil {
			cases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(p.ReadyChan()),
			}
		} else {
			// Nil pollable - use nil channel (never selected)
			cases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf((<-chan struct{})(nil)),
			}
		}
	}
	// Add context done case
	cases[len(pollables)] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}

	// Wait for at least one to be ready
	chosen, _, _ := reflect.Select(cases)

	// Context cancelled
	if chosen == len(pollables) {
		return []component.Val{component.ValList([]component.Val{})}, ctx.Err()
	}

	// Re-collect ready indices (more may have become ready)
	readyIndices = collectReadyIndices(pollables)
	return []component.Val{component.ValList(readyIndices)}, nil
}

// collectReadyIndices returns the indices of ready pollables
func collectReadyIndices(pollables []*Pollable) []component.Val {
	var ready []component.Val
	for i, p := range pollables {
		if p != nil && p.IsReady() {
			ready = append(ready, component.ValU32(uint32(i)))
		}
	}
	return ready
}
```

Also add the import for `reflect` at the top of the file.

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestPoll_"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/poll.go imports/wasip2/io/poll_test.go
git commit -m "feat(wasip2): implement correct poll function

- Poll now returns indices of all ready pollables
- Blocks until at least one pollable is ready
- Uses reflect.Select for dynamic channel selection
- Respects context cancellation"
```

---

## Task 1.5: Update Stream Pollables to Signal Ready Correctly

**Files:**
- Modify: `imports/wasip2/io/streams.go`
- Test: `imports/wasip2/io/streams_test.go` (create)

**Step 1: Write failing test**

Create `imports/wasip2/io/streams_test.go`:

```go
package io

import (
	"bytes"
	"testing"
)

func TestInputStream_Subscribe_ReturnsReadyPollable(t *testing.T) {
	// InputStream with data available should have ready pollable
	data := []byte("hello")
	stream := NewInputStream(bytes.NewReader(data))

	pollable := stream.Subscribe()
	if !pollable.IsReady() {
		t.Error("pollable should be ready when data is available")
	}
}

func TestOutputStream_Subscribe_ReturnsReadyPollable(t *testing.T) {
	// OutputStream writing to buffer should be ready
	var buf bytes.Buffer
	stream := NewOutputStream(&buf)

	pollable := stream.Subscribe()
	if !pollable.IsReady() {
		t.Error("pollable should be ready when writing to buffer")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./imports/wasip2/io/... -run "TestInput|TestOutput"`
Expected: FAIL - Subscribe may not exist or not return ready pollable

**Step 3: Update streams.go to return ready pollables**

In `imports/wasip2/io/streams.go`, update the Subscribe methods:

```go
// Subscribe returns a pollable for this input stream.
// For simplicity, we return a ready pollable since Go's io.Reader
// interface doesn't provide async readiness notification.
func (s *InputStream) Subscribe() *Pollable {
	// For Go readers, we assume they're always "ready" in the sense
	// that a read operation can be attempted. The read itself may block,
	// but that's handled by the blocking-read method.
	return NewReadyPollable()
}

// Subscribe returns a pollable for this output stream.
func (s *OutputStream) Subscribe() *Pollable {
	// For Go writers, we assume they're always ready to accept writes.
	return NewReadyPollable()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./imports/wasip2/io/... -run "TestInput|TestOutput"`
Expected: PASS

**Step 5: Commit**

```bash
git add imports/wasip2/io/streams.go imports/wasip2/io/streams_test.go
git commit -m "feat(wasip2): streams return ready pollables from Subscribe

Go's io.Reader/io.Writer don't have async readiness, so we return
ready pollables and let the actual read/write operations block."
```

---

## Phase 1 Checkpoint

**Run calculator tests to verify no regressions:**

```bash
go test -v ./internal/component/wasip2test/... -run "TestCalculator"
```

**Expected:** All tests pass for `add` and `subtract` plugins.

**If tests fail:**
1. Check error messages
2. Verify resource table operations still work
3. Check that poll changes don't affect stdin/stdout streams
4. Debug and fix before proceeding to Phase 2

**Mark Phase 1 complete in README.md** by changing:
```
| 1 | wasi:io/poll - Blocking I/O Foundation | HIGH | [ ] Not Started |
```
to:
```
| 1 | wasi:io/poll - Blocking I/O Foundation | HIGH | [x] Complete |
```
