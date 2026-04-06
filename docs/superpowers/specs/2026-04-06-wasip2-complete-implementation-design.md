# WASI P2 Complete Implementation — Zero Stubs

**Date:** 2026-04-06  
**Scope:** Eliminate all stubs, TODOs, placeholder returns, and missing wiring across `imports/wasip2/` — filesystem, sockets, and HTTP server-side.  
**Approach:** Bottom-up (IO/Pollable → Filesystem → Sockets → HTTP), strict red/green TDD.  
**Reference:** WASI WIT specs in `debug-vendored/WASI/`, wasmtime implementation in `debug-vendored/wasmtime/crates/`.

---

## Layer 0: IO/Pollable Foundation

The existing `io.Pollable` with `NewPollable(readyFn, blockFn)` and `NewReadyPollable()` is already well-implemented. No changes needed — all consumers must create real Pollables instead of returning handle 0.

---

## Layer 1: Filesystem

**Module:** `imports/wasip2/filesystem/`  
**8 stubs to fix.** All are synchronous syscall wrappers. The existing `Descriptor` type provides `File()`, `Dir()`, `Flags().HasRead()`/`HasWrite()`, and `MapOSError()`.

### 1.1 `descriptor.set-size`

**File:** `filesystem.go:393` — currently returns success without doing anything.

**Implementation:**
- Permission check: require write flag, return `ErrorCodeAccess` if missing
- Directory check: return `ErrorCodeIsDirectory` if descriptor is a dir
- Call `desc.File().Truncate(int64(size))`
- Map OS errors via `MapOSError()`

**Test cases:**
- Truncate file to smaller size → file size reduced, data beyond new size gone
- Extend file to larger size → file size increased, new bytes are zeros
- Truncate to 0 → empty file
- Error on directory descriptor → `ErrorCodeIsDirectory`
- Error without write permission → `ErrorCodeAccess`

### 1.2 `descriptor.set-times`

**File:** `filesystem.go:400` — currently returns success without doing anything.

**Implementation:**
- Parse two `new-timestamp` variant args, each is `no-change` | `now` | `timestamp(datetime)`
- Permission check: write flag required, return `ErrorCodeAccess` if missing
- For `no-change`: stat the file, preserve existing value
- For `now`: use `time.Now()`
- For `timestamp(datetime)`: extract seconds + nanoseconds from the WASI datetime record
- Apply via `os.Chtimes()` or `syscall.UtimesNano` on the fd

**Test cases:**
- Set both to `now` → stat shows times updated to approximately current time
- Set to specific timestamp → stat shows exact timestamp
- `no-change` preserves existing value while other field updates
- Error without write permission → `ErrorCodeAccess`

### 1.3 `descriptor.set-times-at`

**File:** `filesystem.go:659` — currently returns success without doing anything.

**Implementation:**
- Same timestamp logic as set-times but operates on path relative to descriptor
- Respects `path-flags` for symlink following
- Uses the directory's base path + relative path
- Permission check: mutate flag on directory

**Test cases:**
- Set times on file via relative path → stat confirms
- Symlink follow flag respected
- Error without mutate permission

### 1.4 `descriptor.link-at`

**File:** `filesystem.go:666` — currently returns success without doing anything.

**Implementation:**
- Extract: old-path-flags, old-path, new-descriptor (borrow), new-path
- Per wasmtime spec: reject if `symlink-follow` flag is set → return `ErrorCodeInvalid`
- Permission check: require mutate on both source and target directories
- Call `os.Link(oldFullPath, newFullPath)`
- Map OS errors

**Test cases:**
- Create hard link → both paths stat to same inode
- Reject symlink-follow flag → `ErrorCodeInvalid`
- Error without mutate permission on source dir
- Error without mutate permission on target dir
- Link to non-existent source → error

### 1.5 `descriptor.advise`

**File:** `filesystem.go:324` — currently no-op stub.

**Implementation:**
- On Linux: call `syscall.Fadvise(fd, offset, length, advice)` mapping the advice enum to `FADV_NORMAL`, `FADV_SEQUENTIAL`, `FADV_RANDOM`, `FADV_WILLNEED`, `FADV_DONTNEED`, `FADV_NOREUSE`
- On other platforms: no-op returning success (optimization hint, spec allows this)
- Use build tags for platform-specific implementation

**Test cases:**
- Call with each advice variant → no error returned
- Valid on file descriptors (not directories, per some platforms)

### 1.6 `descriptor.is-same-object`

**File:** `filesystem.go:1007` — currently compares handle values instead of dev+ino.

**Implementation:**
- Stat both descriptors via `os.File.Stat()` → extract `syscall.Stat_t` via `sys.Sys()`
- Compare `Dev` and `Ino` fields
- Return bool (infallible per spec)

**Test cases:**
- Same file → true
- Different files → false
- Same file opened twice via different descriptors → true
- File and its hard link → true

### 1.7 `descriptor.metadata-hash` / `descriptor.metadata-hash-at`

**File:** `filesystem.go:1019-1037` — currently returns zeroed hashes.

**Implementation:**
- Stat the file to get dev + ino
- Hash using wasmtime's algorithm:
  ```
  hasher := fnv or default hasher
  hasher.Write(uint64 dev)
  hasher.Write(uint64 ino)
  lower = hasher.Sum64()
  upper = lower ^ 4614256656552045848  // wasmtime's pi constant
  ```
- `metadata-hash-at` additionally takes path-flags and path, resolves relative to descriptor

**Test cases:**
- Hash is stable across multiple calls on same file
- Different files produce different hashes
- Hash matches dev+ino-based algorithm (verify lower ^ upper == pi constant)
- `metadata-hash-at` resolves path correctly
- Hard-linked files produce same hash

### 1.8 `filesystem-error-code`

**File:** `filesystem.go:1041` — currently returns None.

**Implementation:**
- Takes `borrow<io-error>` — look up the `io.Error` resource from the table
- Attempt to extract/downcast to a filesystem `ErrorCode`
- Return `option<error-code>` — `Some` if it's a filesystem error, `None` otherwise

**Test cases:**
- Filesystem error wrapped in io-error → returns Some(error-code)
- Non-filesystem io-error → returns None
- Various error codes round-trip correctly

---

## Layer 2: Sockets

**Module:** `imports/wasip2/sockets/`  
**10 stubs to fix.** DNS resolution is fully stubbed. All subscribe methods return placeholder handle 0. `network-error-code` is missing entirely.

### 2.1 `instance-network`

**File:** `network.go:35` — currently returns handle 0.

**Implementation:**
- Create a real `Network` resource struct with permission flags
- Store in resource table, return owned handle
- Wire existing socket functions that take `borrow<network>` to validate the handle

**Test cases:**
- Returns valid handle (non-zero)
- Handle is retrievable from resource table
- Multiple calls return distinct handles

### 2.2 `resolve-addresses`

**File:** `network.go:63` — currently returns handle 0.

**Implementation:**
- Validate input: reject empty string, whitespace, URLs, strings with ports → `ErrorCodeInvalidArgument`
- For IP literals (IPv4/IPv6, including bracketed `[::1]`): return directly without DNS lookup
- For hostnames: call `net.DefaultResolver.LookupHost()` in a goroutine
- Create `ResolveAddressStream` resource with: completion channel, result slice, cursor index
- Store in resource table, return result with owned handle

**Test cases:**
- Resolve IP literal "127.0.0.1" → returns exact IPv4 address
- Resolve IPv6 literal "::1" → returns exact IPv6 address
- Resolve bracketed "[::1]" → returns exact IPv6 address
- Resolve "localhost" → returns at least one address
- Empty string → `ErrorCodeInvalidArgument`
- Whitespace " " → `ErrorCodeInvalidArgument`
- Invalid chars "a.b<&>" → `ErrorCodeInvalidArgument`
- Port in address "127.0.0.1:80" → `ErrorCodeInvalidArgument`
- URI format "http://example.com/" → `ErrorCodeInvalidArgument`
- IPv6 with port "[::]:80" → `ErrorCodeInvalidArgument`

### 2.3 `resolve-next-address`

**File:** `network.go:71` — currently returns None.

**Implementation:**
- If resolution still pending and no results: return error `would-block`
- If results available: return next address from slice, advance cursor
- If all addresses exhausted: return `Ok(None)`
- If resolution failed: return appropriate error code

**Test cases:**
- After resolving IP literal: first call returns address, second returns None
- After resolving hostname: returns addresses one by one, then None
- Before resolution completes: returns `would-block`

### 2.4 `resolve-address-stream.subscribe`

**File:** `network.go:79` — currently returns handle 0.

**Implementation:**
- Create real `io.Pollable` using `io.NewPollable(readyFn, blockFn)`
- `readyFn`: non-blocking check if DNS resolution goroutine completed
- `blockFn`: wait on completion channel
- Store in resource table, return owned handle

**Test cases:**
- Subscribe returns valid pollable handle
- Pollable becomes ready after resolution completes
- Block on pollable → unblocks when resolution finishes
- For IP literals: pollable is immediately ready

### 2.5 `tcp-socket.subscribe`

**File:** `tcp.go:684` — currently returns handle 0.

**Implementation:**
- Create Pollable reflecting socket's current async operation state
- `readyFn`: check if pending operation (bind/connect/listen/accept) can make progress
- `blockFn`: block until the underlying Go net socket is ready
- Store in resource table, return owned handle

**Test cases:**
- TCP bind: start-bind, subscribe, block, finish-bind succeeds
- TCP connect: start-connect, subscribe, block, finish-connect succeeds (against local listener)
- TCP listen: start-listen, subscribe, block, finish-listen succeeds
- TCP accept: subscribe on listening socket, connect from client, pollable becomes ready

### 2.6 `udp-socket.subscribe`

**File:** `udp.go:377` — currently returns handle 0.

**Implementation:**
- Per wasmtime: UDP socket subscribe `ready()` is a no-op — UDP operations don't block at socket level
- Create immediately-ready Pollable via `io.NewReadyPollable()`
- Store in resource table, return owned handle

**Test cases:**
- Subscribe returns valid pollable handle
- Pollable is immediately ready (no blocking)

### 2.7 `incoming-datagram-stream.subscribe`

**File:** `udp.go:444` — currently returns handle 0.

**Implementation:**
- Create Pollable that becomes ready when data is available to receive
- `readyFn`: non-blocking check if UDP socket has data (via `SetReadDeadline(past)` + read attempt, or platform-specific readability check)
- `blockFn`: block waiting for readable state on the underlying `net.UDPConn`
- Store in resource table, return owned handle

**Test cases:**
- Subscribe returns valid handle
- No data available → pollable not ready
- Send data from another socket → pollable becomes ready
- Block on pollable → unblocks when data arrives, receive succeeds

### 2.8 `outgoing-datagram-stream.subscribe`

**File:** `udp.go:553` — currently returns handle 0.

**Implementation:**
- State-aware Pollable:
  - `Idle` or `Permitted(n)`: immediately ready
  - `Waiting`: block for writability, then reset to `Idle`
- Store in resource table, return owned handle

**Test cases:**
- In Idle state → pollable immediately ready
- In Permitted state → pollable immediately ready
- In Waiting state → pollable blocks until writable, then ready
- After send exhausts permits → subscribe, block, check-send returns new permits

### 2.9 `outgoing-datagram-stream.check-send`

**File:** Currently missing or placeholder.

**Implementation:**
- Add `SendState` enum to outgoing datagram stream: `Idle`, `Permitted(n int)`, `Waiting`
- State machine:
  - `Idle` → transition to `Permitted(16)`, return 16
  - `Permitted(n)` → return n
  - `Waiting` → return 0
- After `send()` call: decrement permitted count; when 0, transition to `Waiting`

**Test cases:**
- Initial state: check-send returns 16 (transitions Idle→Permitted)
- Second call without send: still returns 16
- After sending datagrams: returns remaining permits
- Exhaust all permits: returns 0 (Waiting state)
- Subscribe + block after exhaustion → resets to Idle, check-send returns 16 again

### 2.10 `network-error-code`

**File:** Not implemented — missing from `network.go` entirely.

**Implementation:**
- Register `network-error-code` function in the `wasi:sockets/network@0.2.0` instance
- Takes `borrow<io-error>` — look up `io.Error` resource from table
- Attempt to downcast to a socket `ErrorCode`
- Return `option<error-code>` — `Some` if it's a socket error, `None` otherwise
- Follows same pattern as `http-error-code` and `filesystem-error-code`

**Test cases:**
- Socket error (e.g., connection refused) wrapped in io-error → returns Some(connection-refused)
- Non-socket io-error → returns None
- Various socket error codes round-trip correctly

---

## Layer 3: HTTP Server-Side

**Module:** `imports/wasip2/http/`  
**8 items to fix.** This is the architecturally significant piece — channel-based response pattern matching wasmtime.

### 3.1 `outgoing-response` — Full Resource Table Wiring

**File:** `http.go:842-869` — all 5 functions return hardcoded values.

The `OutgoingResponse` type already exists in `types.go`. Wire the stubs to the resource table:

- **Constructor** (`http.go:842`): take `own<fields>` arg, look up Fields from table, create `NewOutgoingResponse(headers)` with default status 200, store in table, return owned handle
- **status-code** (`http.go:848`): borrow handle → table lookup → return `resp.StatusCode()`
- **set-status-code** (`http.go:854`): borrow handle → table lookup → `resp.SetStatusCode(code)` → return result ok
- **headers** (`http.go:860`): borrow handle → table lookup → get headers, store as new resource in table, return owned handle
- **body** (`http.go:866`): borrow handle → table lookup → `resp.Body()` (one-time, track bodyConsumed), create OutgoingBody, store in table, return result with owned handle

**Test cases:**
- Constructor with headers → resource stored, handle valid, status defaults to 200
- Get/set status code → round-trips correctly (200, 404, 500, etc.)
- Headers returns child resource with correct field values
- Body returned once → success; second call → error
- Destroy cleans up headers and body

### 3.2 `response-outparam` — Channel-Based Pattern

**File:** `http.go:1170` — currently a no-op.

Following wasmtime's `oneshot::channel` pattern, translated to Go:

**Type changes to `ResponseOutparam`:**
- Add `result chan ResponseResult` (buffered size 1)
- `ResponseResult` is a struct: `{ Response *OutgoingResponse; Err *ErrorCode }`

**`NewResponseOutparam()`**: creates the struct with the buffered channel.

**`response-outparam.set` (static)**:
- Takes `own<response-outparam>` and `result<own<outgoing-response>, error-code>`
- Remove outparam from table (consumes it)
- If result is Ok: remove OutgoingResponse from table, send it through channel
- If result is Err: extract error-code, send through channel
- Ignore channel send errors (host may have timed out, per wasmtime)

**Public Go API on ResponseOutparam:**
- `WaitForResponse(ctx context.Context) (*OutgoingResponse, *ErrorCode, error)` — blocks on channel with context cancellation

**Test cases:**
- Set with Ok(response) → response available on channel
- Set with Err(error-code) → error available on channel
- Outparam consumed after set (handle invalid in table)
- Channel send after receiver dropped → no panic (silent ignore)
- WaitForResponse with cancelled context → returns context error

### 3.3 `incoming-handler.handle` — Go HTTP Bridge

**File:** `incoming.go:23` — currently a no-op.

The `incoming-handler` is an *export* from the component. The host infrastructure needs to support calling it.

**Public Go API:**
- `NewHTTPHandler(callHandle func(ctx context.Context, request, outparam component.Handle) error) http.Handler` — wraps a component's handle export as a Go `http.Handler`
- The returned handler:
  1. Creates `IncomingRequest` from `*http.Request` (method, scheme, authority, path, headers, body via `NewIncomingBodyFromReader(req.Body)`)
  2. Creates `ResponseOutparam` with channel
  3. Stores both in resource table
  4. Calls `callHandle(ctx, requestHandle, outparamHandle)`
  5. Waits for response on outparam channel
  6. Writes OutgoingResponse to `http.ResponseWriter` (status, headers, body)

**The registered stub** in `incoming.go` stays as-is — it's the linker registration for when a component exports this interface. The host-side bridge is the new public API.

**Test cases:**
- Simple GET → handler receives correct method/path/headers, sets 200 response, host receives it
- POST with body → handler can read request body via consume()
- Handler sets error via outparam → host receives error
- Response with headers and body → host reads correct status, headers, body content
- Handler timeout → context cancellation propagates

### 3.4 `incoming-request.consume` — Wire to Actual Body

**File:** `http.go:829` — creates empty IncomingBody, has TODO about accessing actual request body.

**Implementation:**
- `IncomingRequest` already stores a body field in `types.go`
- When created from `*http.Request` (in the HTTP bridge), wrap `req.Body` via `NewIncomingBodyFromReader(req.Body)`
- `consume()`: return the stored body (one-time, tracked via `bodyConsumed` flag)
- If body is nil (e.g., GET request): create empty IncomingBody

**Test cases:**
- Request with body → consume returns body, read returns content
- Request without body → consume returns empty body
- Second consume call → error
- Body content matches original request body

### 3.5 `incoming-body.finish` — Real FutureTrailers

**File:** `http.go` — currently returns `ValOwn(0)`.

**Implementation:**
- Static method: remove IncomingBody from table (consumes it)
- Create `FutureTrailers` in `Waiting` state
- If the body's underlying reader has trailer support (e.g., from `http.Response.Trailer`): set up async trailer reading
- For simple cases without trailers: resolve immediately to `Done(Ok(None))`
- Store FutureTrailers in table, return owned handle

**Test cases:**
- Finish body → returns valid future-trailers handle
- Body without trailers → future resolves to Ok(None)
- Body consumed before finish (input-stream dropped) → still works
- Future-trailers handle is valid in resource table

### 3.6 `future-trailers` — State Machine

**File:** `http.go` — get and subscribe currently return nil/placeholder.

**Three states:**
- `Waiting` — body still being read or trailers not yet available
- `Done(result<option<trailers>, error-code>)` — trailers resolved
- `Consumed` — get() already called

**`get()`:**
- `Waiting`: check if trailers ready (non-blocking). Not ready → return outer `None`. Ready → transition to `Done`, fall through
- `Done`: transition to `Consumed`, return `Some(result)`
- `Consumed`: return outer `None` (one-time retrieval per spec)

**`subscribe()`:**
- Real Pollable: ready when state is `Done` or `Consumed`
- `readyFn`: check state != Waiting
- `blockFn`: wait for transition out of Waiting

**Test cases:**
- Get on pending future → returns None
- Subscribe + block → get returns Some(Ok(None)) for body without trailers
- Get called twice → second returns None (consumed)
- Subscribe on already-done future → immediately ready
- Body with trailers → get returns Some(Ok(Some(trailers))) with correct field values

### 3.7 `http-error-code`

**File:** `http.go:1299` — currently returns empty option.

**Implementation:**
- Takes `borrow<io-error>` — look up `io.Error` resource from table
- Attempt to downcast underlying error to HTTP `ErrorCode`
- The existing `ErrorCodeFromError()` function already maps Go errors → ErrorCode
- Return `Some(error-code)` if it's an HTTP error, `None` otherwise

**Test cases:**
- HTTP error (e.g., DNS timeout wrapped in io-error) → returns Some(DNS-timeout)
- Connection refused error → returns Some(connection-refused)
- Non-HTTP io-error → returns None
- Various error codes map correctly

### 3.8 `fields.has` — Missing Method

**File:** `http.go:40-45` — not registered; Fields type in `types.go` lacks Has method.

**Implementation:**
- Add `Has(name string) bool` to Fields type: check if key exists in the map
- Register `[method]fields.has` in `instantiateTypes` between `get` and `set`
- Implementation function: borrow handle → table lookup → `fields.Has(name)` → return bool

**Test cases:**
- Has existing key → true
- Has missing key → false
- Has after delete → false
- Has after append on new key → true
- Case-sensitive: "Content-Type" vs "content-type" are different keys

---

## Cross-Cutting Concerns

### Error Bridge Pattern

Three error-code extraction functions share the same pattern:
- `http-error-code`: io-error → option<http-error-code>
- `filesystem-error-code`: io-error → option<fs-error-code>
- `network-error-code`: io-error → option<socket-error-code> (check if this exists/is stubbed)

All downcast an `io.Error` resource to a module-specific error code. The io module needs a way to store typed errors that can be extracted by module-specific functions. `network-error-code` is `@unstable` in the WIT spec but implemented by wasmtime — include it for conformance.

### Resource Table Discipline

Every function that currently returns `ValOwn(0)` or a hardcoded value must be converted to proper resource table operations:
- Constructors: `table.New(resource, true)` → return `ValOwn(handle)`
- Borrowing methods: `table.Get(handle)` → type-assert → operate
- Consuming statics: `table.Remove(handle)` → type-assert → consume
- Error on invalid handle: return appropriate error result

### Platform-Specific Code

- `descriptor.advise`: Linux uses `syscall.Fadvise`, other platforms no-op
- `descriptor.set-times`: may need `syscall.UtimesNano` on Linux vs `os.Chtimes` cross-platform
- `descriptor.is-same-object` / `metadata-hash`: need `syscall.Stat_t` access for dev+ino — Linux/macOS direct, Windows may need alternative

### TDD Order

Within each layer, for every function:
1. Write failing test(s) first (red) — test the expected behavior per spec
2. Implement the minimum code to pass (green)
3. Refactor if needed

Tests are Go unit tests calling host functions directly with `component.Val` arguments, following the existing patterns in `http_test.go`, `tcp_test.go`, etc.

---

## Complete Inventory of Changes

| # | Module | Function | File | Current State | Target State |
|---|--------|----------|------|---------------|--------------|
| 1 | filesystem | `descriptor.set-size` | filesystem.go:393 | No-op stub | `File().Truncate()` |
| 2 | filesystem | `descriptor.set-times` | filesystem.go:400 | No-op stub | `os.Chtimes` / syscall |
| 3 | filesystem | `descriptor.set-times-at` | filesystem.go:659 | No-op stub | Path-relative time setting |
| 4 | filesystem | `descriptor.link-at` | filesystem.go:666 | No-op stub | `os.Link()` |
| 5 | filesystem | `descriptor.advise` | filesystem.go:324 | No-op stub | `syscall.Fadvise` / no-op |
| 6 | filesystem | `descriptor.is-same-object` | filesystem.go:1007 | Compares handles | Compare dev+ino via stat |
| 7 | filesystem | `descriptor.metadata-hash` | filesystem.go:1019 | Returns zeros | Hash dev+ino |
| 8 | filesystem | `descriptor.metadata-hash-at` | filesystem.go:1030 | Returns zeros | Hash dev+ino at path |
| 9 | filesystem | `filesystem-error-code` | filesystem.go:1041 | Returns None | Downcast io-error |
| 10 | sockets | `instance-network` | network.go:35 | Returns handle 0 | Real Network resource |
| 11 | sockets | `resolve-addresses` | network.go:63 | Returns handle 0 | `net.LookupHost` async |
| 12 | sockets | `resolve-next-address` | network.go:71 | Returns None | Iterator over results |
| 13 | sockets | `resolve-address-stream.subscribe` | network.go:79 | Returns handle 0 | Real Pollable |
| 14 | sockets | `tcp-socket.subscribe` | tcp.go:684 | Returns handle 0 | Real Pollable |
| 15 | sockets | `udp-socket.subscribe` | udp.go:377 | Returns handle 0 | Immediately-ready Pollable |
| 16 | sockets | `incoming-datagram-stream.subscribe` | udp.go:444 | Returns handle 0 | Real Pollable (readable) |
| 17 | sockets | `outgoing-datagram-stream.subscribe` | udp.go:553 | Returns handle 0 | State-aware Pollable |
| 18 | sockets | `outgoing-datagram-stream.check-send` | udp.go | Placeholder | SendState machine |
| 19 | sockets | `network-error-code` | network.go | Missing entirely | Downcast io-error |
| 20 | http | `outgoing-response` constructor | http.go:842 | Returns handle 0 | Resource table wiring |
| 21 | http | `outgoing-response.status-code` | http.go:848 | Returns 200 | Table lookup |
| 22 | http | `outgoing-response.set-status-code` | http.go:854 | No-op | Table lookup + set |
| 23 | http | `outgoing-response.headers` | http.go:860 | Returns handle 0 | Table lookup + child |
| 24 | http | `outgoing-response.body` | http.go:866 | Returns handle 0 | Table lookup + OutgoingBody |
| 25 | http | `response-outparam.set` | http.go:1170 | No-op | Channel send |
| 26 | http | `incoming-handler` bridge | incoming.go:23 | No-op | Go HTTP handler bridge |
| 27 | http | `incoming-request.consume` | http.go:829 | Empty body | Wire to actual body |
| 28 | http | `incoming-body.finish` | http.go | Returns handle 0 | Real FutureTrailers |
| 29 | http | `future-trailers.get` | http.go | Returns nil | State machine |
| 30 | http | `future-trailers.subscribe` | http.go | Returns nil | Real Pollable |
| 31 | http | `http-error-code` | http.go:1299 | Returns None | Downcast io-error |
| 32 | http | `fields.has` | http.go (missing) | Not registered | New method |

**Total: 32 items across 3 modules. Zero stubs remaining after implementation.**
