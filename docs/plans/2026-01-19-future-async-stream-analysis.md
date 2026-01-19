# Async and Streaming Functionality Analysis

**Date:** 2026-01-19
**Status:** Analysis Complete
**Goal:** Deep analysis of async/streaming functionality in wazero's WASI Preview 2 implementation

## Executive Summary

The wazero implementation has **partial stream support** that works for basic use cases but lacks the full async canonical operations defined in the Component Model spec. The current approach uses Go's synchronous blocking model with goroutines for concurrency, which is a reasonable design choice that avoids the complexity of full async support while still enabling practical stream-based I/O.

**Key Finding:** Full async canonical operations (task.*, stream.*, future.*, etc.) are **not required** for working WASI P2 streams. The current implementation can be made production-ready by fixing the identified gaps without implementing the async canonical ABI.

---

## 1. Current Async/Stream Implementation Status

### 1.1 Canonical Operations - Spec vs Implementation

#### Implemented (5 operations):
| Opcode | Operation | Status |
|--------|-----------|--------|
| 0x00 | `canon lift` | Implemented |
| 0x01 | `canon lower` | Implemented |
| 0x02 | `canon resource.new` | Implemented |
| 0x03 | `canon resource.drop` | Implemented |
| 0x04 | `canon resource.rep` | Implemented |

#### Not Implemented - Async Operations (20+ operations):
| Opcode Range | Category | Operations |
|--------------|----------|------------|
| 0x05-0x0d | Task operations | `task.backpressure`, `task.return`, `task.wait`, `task.poll`, `task.yield`, `task.cancel`, `subtask.cancel`, `subtask.drop` |
| 0x0e-0x14 | Stream operations | `stream.new`, `stream.read`, `stream.write`, `stream.cancel-read`, `stream.cancel-write`, `stream.close-readable`, `stream.close-writable` |
| 0x15-0x1b | Future operations | `future.new`, `future.read`, `future.write`, `future.cancel-read`, `future.cancel-write`, `future.close-readable`, `future.close-writable` |
| 0x1c-0x1e | Error-context | `error-context.new`, `error-context.debug-message`, `error-context.drop` |
| 0x1f-0x23 | Waitable-set | `waitable-set.new`, `waitable-set.wait`, `waitable-set.poll`, `waitable-set.drop` |
| 0x24-0x25 | Backpressure | `backpressure.set`, `yield` |
| 0x26-0x2b, 0x40-0x42 | Thread | Thread spawn and related operations |

**Source:** `internal/component/binary/canonical.go:14-20` only defines opcodes 0x00-0x04.

### 1.2 Type System Support

| Type | Parsing | Runtime | Notes |
|------|---------|---------|-------|
| `stream<T, E>` | Yes (0x66) | **No** | Parsed in `binary/types.go:615-648` but not lifted/lowered |
| `future<T>` | Yes (0x65) | **No** | Parsed in `binary/types.go:651-670` but not lifted/lowered |
| `error-context` | No (0x64) | No | Not implemented |
| Async function types | Yes (0x43) | **No** | Recognized but treated same as sync |

**Source:** `internal/component/component.go:622-631` defines `StreamTypeDef` and `FutureTypeDef`.

### 1.3 WASI Interface Implementation

The WASI io/streams and io/poll interfaces **ARE** implemented, but they use Go's io.Reader/io.Writer wrapped in resource handles, not Component Model `stream<T>` types:

| Interface | Status | Implementation |
|-----------|--------|----------------|
| `wasi:io/streams` | Functional | Go io.Reader/io.Writer wrapper (`imports/wasip2/io/streams.go`) |
| `wasi:io/poll` | Simplified | Sequential blocking, not true multiplexing (`imports/wasip2/io/poll.go`) |
| `wasi:io/error` | Partial | Basic error resource |

---

## 2. Stream Functionality End-to-End

### 2.1 How input-stream and output-stream Work Currently

The current implementation uses Go's standard I/O types:

```go
// imports/wasip2/io/streams.go:54-58
type InputStream struct {
    reader goio.Reader
    closed bool
}

// imports/wasip2/io/streams.go:132-136
type OutputStream struct {
    writer goio.Writer
    closed bool
}
```

**Key methods implemented:**
- `read`, `blocking-read`, `skip`, `blocking-skip`, `subscribe` (input)
- `check-write`, `write`, `blocking-write-and-flush`, `flush`, `subscribe` (output)
- `splice`, `blocking-splice` for stream-to-stream transfer

### 2.2 Integration Status by WASI Interface

#### Filesystem (`imports/wasip2/filesystem/filesystem.go`)
| Function | Line | Status | Issue |
|----------|------|--------|-------|
| `descriptor.read-via-stream` | 229-232 | **STUB** | Returns placeholder handle 0 |
| `descriptor.write-via-stream` | 237-240 | **STUB** | Returns placeholder handle 0 |
| `descriptor.append-via-stream` | 245-248 | **STUB** | Returns placeholder handle 0 |
| `descriptor.read` | 336-378 | Working | Direct read (not stream-based) |
| `descriptor.write` | 382-414 | Working | Direct write (not stream-based) |

**Impact:** Components using `read-via-stream` or `write-via-stream` will receive a non-functional handle (0).

#### Sockets (`imports/wasip2/sockets/tcp.go`)
| Function | Line | Status | Issue |
|----------|------|--------|-------|
| `tcp-socket.finish-connect` | 194-267 | Working | Returns TcpInputStream/TcpOutputStream |
| `tcp-socket.accept` | 318-374 | Working | Returns streams for accepted connection |
| `tcp-socket.subscribe` | 683-686 | **STUB** | Returns placeholder pollable handle 0 |

The TCP implementation creates proper `TcpInputStream`/`TcpOutputStream` types (lines 730-789), but these are **not** registered in the resource table with proper type association for `wasi:io/streams`.

#### HTTP (`imports/wasip2/http/`)
| Function | Status | Notes |
|----------|--------|-------|
| Outgoing request body | **Working** | Uses `io.OutputStream` via `OutgoingBody.Write()` |
| Incoming response body | **Working** | Uses `io.InputStream` via `IncomingBody.Stream()` |
| Future response | **Working** | Uses goroutine + channel for async |

**The HTTP implementation is the most complete streaming integration.**

### 2.3 Trace: `descriptor.read-via-stream` Call Flow

1. Component calls `[method]descriptor.read-via-stream(descriptor, offset)`
2. `descriptorReadViaStream()` in `filesystem.go:229-232` is invoked
3. **Current behavior:** Returns `ValResultOk(&handle)` where `handle = ValOwn(0)`
4. **Expected behavior:** Should create an `io.InputStream` wrapping the file at offset, register it in the resource table, and return the handle

**Gap:** No actual stream is created. The component receives handle 0 which doesn't reference a valid stream resource.

---

## 3. What's Needed for Working Async/Streams

### 3.1 Full Async vs Synchronous-with-Streams

| Approach | Complexity | Use Cases | Recommendation |
|----------|-----------|-----------|----------------|
| **Full Async** | High | Multiple concurrent components, server workloads, component composition | Not needed now |
| **Synchronous-with-Streams** | Medium | Single component, client workloads, basic I/O | **Recommended** |

**Analysis:**

The Component Model async support (task.*, stream.*, future.*) is primarily designed for:
1. **Component composition** - allowing multiple components to share a thread
2. **Non-blocking I/O** - enabling true async multiplexing
3. **Backpressure** - flow control between producer/consumer

For wazero's current use cases (single component execution, Go host integration), **synchronous blocking with goroutine-based concurrency is sufficient**.

### 3.2 Minimum Viable Implementation

To make streams work with the current blocking model:

#### P0 - Critical (Streams Don't Work)

1. **Fix filesystem stream methods** (`filesystem.go`)
   - Implement `descriptorReadViaStream`: Create file-backed InputStream at offset
   - Implement `descriptorWriteViaStream`: Create file-backed OutputStream at offset
   - Implement `descriptorAppendViaStream`: Create append-mode OutputStream

2. **Fix socket subscribe** (`tcp.go:683-686`)
   - Return actual Pollable with proper ready/block functions tied to socket state

#### P1 - Important (Polling is Inefficient)

3. **Improve poll implementation** (`poll.go:127-162`)
   - Current: Sequential blocking on each pollable
   - Needed: Use Go channels and `select` for concurrent waiting
   - Alternative: Use `context.Context` cancellation

4. **Ensure stream handles are properly typed**
   - TCP streams (`TcpInputStream`, `TcpOutputStream`) should be retrievable as `wasi:io/streams` resources
   - May need type aliases or interface implementations

#### P2 - Nice to Have (Advanced Use Cases)

5. **Stream type support** (if needed for specific components)
   - Parse and handle `stream<T>` canonical lowering/lifting
   - Would require significant work in `canon_lift.go` and `canon_lower.go`

### 3.3 Architectural Changes Needed

**Minimal changes for P0/P1:**
- No architectural changes required
- Fix existing stub implementations
- Improve poll to use Go's concurrency primitives

**Full async support would require:**
- New Task abstraction in `internal/component/`
- Per-task context and state management
- Integration of Component Model stream/future types into lift/lower
- Rewrite of poll to support waitable-set operations

---

## 4. Recommendations

### 4.1 Prioritized Implementation Plan

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| **P0** | Fix `descriptorReadViaStream` | 2 hours | Filesystem streaming works |
| **P0** | Fix `descriptorWriteViaStream` | 2 hours | File write streaming works |
| **P0** | Fix `descriptorAppendViaStream` | 1 hour | Append streaming works |
| **P1** | Fix `tcpSocketSubscribe` | 2 hours | Socket polling works |
| **P1** | Improve poll implementation | 4 hours | Efficient multiplexing |
| **P2** | Add stream type runtime support | 2-3 days | Component Model streams |
| **P2** | Add future type runtime support | 1-2 days | Component Model futures |
| **Long-term** | Full async canonical ops | 2-4 weeks | Full spec compliance |

### 4.2 Recommended Approach: Synchronous-with-Streams

**Rationale:**
1. **Maintains wazero's simplicity** - No complex async runtime needed
2. **Go-native concurrency** - Goroutines provide natural parallelism
3. **Sufficient for WASI P2** - Current WASI interfaces use resource-based streams, not CM stream types
4. **Pragmatic** - Calculator tests already work; focus on completing existing patterns

**Not pursuing full async because:**
1. Component Model async (WASI 0.3) is still in preview
2. True async is complex and benefits mainly component composition scenarios
3. wazero's target use case (Go host embedding) already has goroutines
4. Blocking I/O with goroutines is functionally equivalent for single-component use

### 4.3 Impact on Existing Working Tests

**Calculator tests (`internal/component/wasip2test/calculator_test.go`):**
- **No impact** - Calculator tests don't use filesystem streaming, socket polling, or HTTP
- Safe to implement all P0/P1 items without breaking these tests

**Recommended validation after each change:**
```bash
CGO_ENABLED=0 go test -v ./internal/component/wasip2test/... -run Calculator
```

---

## 5. Detailed Implementation Notes

### 5.1 Implementing `descriptorReadViaStream`

```go
// filesystem.go - Replace stub at line 229
func descriptorReadViaStream(ctx context.Context, args []component.Val) ([]component.Val, error) {
    handle := args[0].Borrow()
    offset := args[1].U64()

    desc, err := getDescriptor(ctx, handle)
    if err != nil {
        return errorResult(ErrorCodeBadDescriptor), nil
    }

    if desc.IsDir() {
        return errorResult(ErrorCodeIsDirectory), nil
    }

    // Create a reader at the specified offset
    file := desc.File()
    reader := io.NewSectionReader(file, int64(offset), 1<<62) // Large limit

    // Create InputStream
    inputStream := io.NewInputStream(reader)

    // Register in resource table
    table := component.ResourceTableFromContext(ctx)
    if table == nil {
        return errorResult(ErrorCodeIO), nil
    }

    streamHandle := table.New(inputStream, true)
    handleVal := component.ValOwn(uint32(streamHandle.Index()))
    return []component.Val{component.ValResultOk(&handleVal)}, nil
}
```

### 5.2 Improving Poll Implementation

```go
// poll.go - Replace loop at line 127
func pollPoll(ctx context.Context, args []component.Val) ([]component.Val, error) {
    handles := args[0].List()
    if len(handles) == 0 {
        return []component.Val{component.ValList(nil)}, nil
    }

    // Create done channel
    done := make(chan int, len(handles))
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Start watchers for each pollable
    for i, h := range handles {
        idx := i
        pollable, err := getPollable(ctx, h.Borrow())
        if err != nil {
            continue
        }
        go func() {
            if pollable.Ready() {
                select {
                case done <- idx:
                case <-ctx.Done():
                }
                return
            }
            pollable.Block() // Block until ready
            select {
            case done <- idx:
            case <-ctx.Done():
            }
        }()
    }

    // Wait for first ready
    select {
    case idx := <-done:
        cancel() // Cancel other watchers
        // Collect all ready indices
        readyIndices := []component.Val{component.ValU32(uint32(idx))}
        // Drain any others that are also ready
        for {
            select {
            case idx := <-done:
                readyIndices = append(readyIndices, component.ValU32(uint32(idx)))
            default:
                return []component.Val{component.ValList(readyIndices)}, nil
            }
        }
    case <-ctx.Done():
        return []component.Val{component.ValList(nil)}, nil
    }
}
```

---

## 6. Comparison with Wasmtime

| Feature | Wasmtime | Wazero (current) | Wazero (recommended) |
|---------|----------|------------------|---------------------|
| Async runtime | Tokio-based | None | Goroutine-based |
| Stream types | Full support | Parsing only | Not needed for P2 |
| Future types | Full support | Parsing only | Not needed for P2 |
| poll implementation | Proper multiplexing | Sequential | Channel-based select |
| Filesystem streams | Full support | Stubbed | Implement P0 |
| HTTP streams | Full support | **Working** | Maintain |
| TCP streams | Full support | Partial | Fix subscribe |

---

## 7. Conclusion

The wazero WASI P2 implementation has a solid foundation for streaming I/O. The gaps are primarily in:

1. **Filesystem stream methods** - Stubbed, need implementation
2. **Socket subscribe** - Returns placeholder, need real pollable
3. **Poll efficiency** - Sequential blocking, need concurrent select

**Recommendation:** Implement P0 and P1 fixes (~10 hours of work) to achieve working stream support without pursuing full Component Model async operations. This pragmatic approach maintains wazero's simplicity while enabling practical streaming use cases.

Full async support (task.*, stream.*, future.* canonical operations) should be deferred until:
- WASI 0.3 is stable
- Component composition use cases become a priority
- The Component Model async spec stabilizes

---

## Sources

- [WebAssembly Component Model Canonical ABI](https://github.com/WebAssembly/component-model/blob/main/design/mvp/CanonicalABI.md)
- [Component Model Explainer](https://github.com/WebAssembly/component-model/blob/main/design/mvp/Explainer.md)
- [WASI Roadmap](https://wasi.dev/roadmap)
- [Looking Ahead to WASIp3](https://www.fermyon.com/blog/looking-ahead-to-wasip3)
- [WASI 0.3 preview announcement](https://progosling.com/en/dev-digest/2025-08/wasi-0-3-native-async-aug-2025)
