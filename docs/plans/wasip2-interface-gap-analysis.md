# WASI Preview 2 Interface Gap Analysis

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement fixes based on this analysis.

**Goal:** Comprehensive defect and gap analysis for wazero's WASI Preview 2 interface implementations against official WASI specifications.

**Scope:** All wasi:* interfaces implemented in `imports/wasip2/`

**Reference Specs:** `debug-vendored/WASI/proposals/*/wit/*.wit`

**Regression Requirement:** All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests pass.

---

## Executive Summary

The wazero WASI Preview 2 implementation provides basic coverage of core interfaces but has significant gaps compared to the official WASI 0.2.9 specification. Key findings:

1. **Version Mismatch**: Implementation targets @0.2.0, specs are @0.2.9
2. **wasi:io**: Core streaming works, but blocking operations and splice are stubs
3. **wasi:clocks**: Functionally complete
4. **wasi:random**: Functionally complete
5. **wasi:filesystem**: Comprehensive but some methods are stubs
6. **wasi:sockets**: TCP is well-implemented; UDP and IP name lookup are stubs
7. **wasi:cli**: Functionally complete with terminal detection
8. **wasi:http**: Framework exists but handlers are not connected

---

## 1. wasi:io Interface Analysis

### 1.1 wasi:io/error@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/io/wit/error.wit`

**Implementation File:** `imports/wasip2/io/error.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `resource error` | ✓ | ✓ | **OK** |
| `[method]error.to-debug-string` | Returns debug string | Returns simple message | **PARTIAL** |

**Gaps:**
- `to-debug-string` returns generic message; should include error context/stack trace

---

### 1.2 wasi:io/poll@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/io/wit/poll.wit`

**Implementation File:** `imports/wasip2/io/poll.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `resource pollable` | ✓ | ✓ | **OK** |
| `[method]pollable.ready` | Returns bool | ✓ | **OK** |
| `[method]pollable.block` | Blocks until ready | Returns immediately | **STUB** |
| `poll` | Poll multiple pollables | Returns first ready | **PARTIAL** |

**Gaps:**
- `block` method does not actually block; returns immediately
- `poll` function returns `[0]` always (first index) rather than actual ready indices
- No actual async I/O integration

---

### 1.3 wasi:io/streams@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/io/wit/streams.wit`

**Implementation Files:** `imports/wasip2/io/streams.go`

#### input-stream Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `read` | Read up to N bytes | ✓ wraps io.Reader | **OK** |
| `blocking-read` | Block until data | Same as read (no blocking) | **PARTIAL** |
| `skip` | Skip N bytes | ✓ seeks or reads/discards | **OK** |
| `blocking-skip` | Block until skip done | Same as skip (no blocking) | **PARTIAL** |
| `subscribe` | Get pollable | ✓ returns pollable | **OK** |

#### output-stream Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `check-write` | Check write capacity | Returns 4096 always | **PARTIAL** |
| `write` | Write bytes | ✓ wraps io.Writer | **OK** |
| `blocking-write-and-flush` | Block until written | Same as write | **PARTIAL** |
| `flush` | Flush buffer | No-op (no buffering) | **OK** |
| `blocking-flush` | Block until flushed | No-op | **OK** |
| `subscribe` | Get pollable | ✓ returns pollable | **OK** |
| `write-zeroes` | Write N zeroes | ✓ | **OK** |
| `blocking-write-zeroes-and-flush` | Block write zeroes | Same as write-zeroes | **PARTIAL** |
| `splice` | Transfer from input | Returns 0, not implemented | **STUB** |
| `blocking-splice` | Block splice | Returns 0, not implemented | **STUB** |

**Gaps:**
- All `blocking-*` methods don't actually block
- `splice` operations not implemented
- `check-write` returns hardcoded value instead of actual buffer capacity

---

## 2. wasi:clocks Interface Analysis

### 2.1 wasi:clocks/monotonic-clock@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/clocks/wit/monotonic-clock.wit`

**Implementation File:** `imports/wasip2/clocks/monotonic.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `now` | Returns instant (u64 ns) | ✓ time.Now().UnixNano() | **OK** |
| `resolution` | Clock resolution (u64 ns) | Returns 1 (nanosecond) | **OK** |
| `subscribe-instant` | Pollable for instant | ✓ creates pollable | **OK** |
| `subscribe-duration` | Pollable for duration | ✓ creates pollable | **OK** |

**Status:** Functionally complete ✓

---

### 2.2 wasi:clocks/wall-clock@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/clocks/wit/wall-clock.wit`

**Implementation File:** `imports/wasip2/clocks/wall.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `now` | Returns datetime record | ✓ time.Now() converted | **OK** |
| `resolution` | Clock resolution | Returns 1ns | **OK** |

**Types:**
- `datetime` record: `{seconds: u64, nanoseconds: u32}` - ✓ implemented

**Status:** Functionally complete ✓

---

## 3. wasi:random Interface Analysis

### 3.1 wasi:random/random@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/random/wit/random.wit`

**Implementation File:** `imports/wasip2/random/random.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-random-bytes` | Return N random bytes | ✓ crypto/rand, capped 64KB | **OK** |
| `get-random-u64` | Return random u64 | ✓ crypto/rand | **OK** |

**Note:** 64KB cap on `get-random-bytes` is reasonable security measure.

**Status:** Functionally complete ✓

---

### 3.2 wasi:random/insecure@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/random/wit/insecure.wit`

**Implementation File:** `imports/wasip2/random/insecure.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-insecure-random-bytes` | Return N bytes | ✓ math/rand | **OK** |
| `get-insecure-random-u64` | Return u64 | ✓ math/rand | **OK** |

**Status:** Functionally complete ✓

---

### 3.3 wasi:random/insecure-seed@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/random/wit/insecure-seed.wit`

**Implementation File:** `imports/wasip2/random/insecure.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `insecure-seed` | Return (u64, u64) tuple | ✓ returns random seed pair | **OK** |

**Status:** Functionally complete ✓

---

## 4. wasi:filesystem Interface Analysis

### 4.1 wasi:filesystem/types@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/filesystem/wit/types.wit`

**Implementation Files:** `imports/wasip2/filesystem/filesystem.go`, `imports/wasip2/filesystem/types.go`

#### descriptor Resource Methods

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `read-via-stream` | Get input-stream at offset | ✓ | **OK** |
| `write-via-stream` | Get output-stream at offset | ✓ | **OK** |
| `append-via-stream` | Get append output-stream | ✓ | **OK** |
| `advise` | Provide access hints | Returns ok (no-op) | **STUB** |
| `sync-data` | Sync data to disk | ✓ os.File.Sync() | **OK** |
| `get-flags` | Get descriptor flags | ✓ | **OK** |
| `get-type` | Get descriptor type | ✓ | **OK** |
| `set-size` | Truncate file | ✓ os.Truncate | **OK** |
| `set-times` | Set timestamps | ✓ os.Chtimes | **OK** |
| `read` | Read at offset | ✓ | **OK** |
| `write` | Write at offset | ✓ | **OK** |
| `read-directory` | Read dir entries | ✓ | **OK** |
| `sync` | Full sync | ✓ | **OK** |
| `create-directory-at` | Create subdir | ✓ | **OK** |
| `stat` | Get metadata | ✓ | **OK** |
| `stat-at` | Get metadata at path | ✓ | **OK** |
| `set-times-at` | Set times at path | ✓ | **OK** |
| `link-at` | Create hard link | ✓ | **OK** |
| `open-at` | Open file at path | ✓ | **OK** |
| `readlink-at` | Read symlink | ✓ | **OK** |
| `remove-directory-at` | Remove subdir | ✓ | **OK** |
| `rename-at` | Rename entry | ✓ | **OK** |
| `symlink-at` | Create symlink | ✓ | **OK** |
| `unlink-file-at` | Remove file | ✓ | **OK** |
| `is-same-object` | Compare descriptors | ✓ compares inodes | **OK** |
| `metadata-hash` | Get metadata hash | ✓ | **OK** |
| `metadata-hash-at` | Get hash at path | ✓ | **OK** |

#### directory-entry-stream Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `read-directory-entry` | Read next entry | ✓ | **OK** |

**Types Implemented:**
- `descriptor-type` enum ✓
- `descriptor-flags` flags ✓
- `path-flags` flags ✓
- `open-flags` flags ✓
- `metadata-hash-value` record ✓
- `descriptor-stat` record ✓
- `directory-entry` record ✓
- `error-code` enum ✓

**Gaps:**
- `advise` is a no-op (acceptable for portability)
- No `@unstable` features implemented

**Status:** Comprehensive implementation ✓

---

### 4.2 wasi:filesystem/preopens@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/filesystem/wit/preopens.wit`

**Implementation File:** `imports/wasip2/filesystem/preopens.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-directories` | List preopened dirs | ✓ returns (descriptor, path) list | **OK** |

**Status:** Functionally complete ✓

---

## 5. wasi:sockets Interface Analysis

### 5.1 wasi:sockets/network@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/sockets/wit/network.wit`

**Implementation File:** `imports/wasip2/sockets/sockets.go`

| Item | Spec | Implementation | Status |
|------|------|----------------|--------|
| `resource network` | Network capability | ✓ defined | **OK** |
| `error-code` enum | 36 error codes | ✓ all defined | **OK** |
| `ip-address-family` enum | ipv4, ipv6 | ✓ | **OK** |
| `ip-address` variant | IPv4/IPv6 address | ✓ | **OK** |
| `ip-socket-address` variant | Socket address | ✓ | **OK** |

**Status:** Functionally complete ✓

---

### 5.2 wasi:sockets/tcp@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/sockets/wit/tcp.wit`

**Implementation File:** `imports/wasip2/sockets/tcp.go`

#### tcp-socket Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `start-bind` | Begin async bind | ✓ validates + stores | **OK** |
| `finish-bind` | Complete bind | ✓ performs bind | **OK** |
| `start-connect` | Begin async connect | ✓ validates + stores | **OK** |
| `finish-connect` | Complete connect | ✓ performs connect | **OK** |
| `start-listen` | Begin async listen | ✓ | **OK** |
| `finish-listen` | Complete listen | ✓ | **OK** |
| `accept` | Accept connection | ✓ returns socket + streams | **OK** |
| `local-address` | Get bound address | ✓ | **OK** |
| `remote-address` | Get peer address | ✓ | **OK** |
| `is-listening` | Check listen state | ✓ | **OK** |
| `address-family` | Get address family | ✓ | **OK** |
| `set-listen-backlog-size` | Set backlog | ✓ | **OK** |
| `keep-alive-enabled` | Get keepalive | ✓ | **OK** |
| `set-keep-alive-enabled` | Set keepalive | ✓ | **OK** |
| `keep-alive-idle-time` | Get idle time | ✓ | **OK** |
| `set-keep-alive-idle-time` | Set idle time | ✓ | **OK** |
| `keep-alive-interval` | Get interval | ✓ | **OK** |
| `set-keep-alive-interval` | Set interval | ✓ | **OK** |
| `keep-alive-count` | Get count | ✓ | **OK** |
| `set-keep-alive-count` | Set count | ✓ | **OK** |
| `hop-limit` | Get TTL | ✓ | **OK** |
| `set-hop-limit` | Set TTL | ✓ | **OK** |
| `receive-buffer-size` | Get recv buffer | ✓ | **OK** |
| `set-receive-buffer-size` | Set recv buffer | ✓ | **OK** |
| `send-buffer-size` | Get send buffer | ✓ | **OK** |
| `set-send-buffer-size` | Set send buffer | ✓ | **OK** |
| `subscribe` | Get pollable | ✓ | **OK** |
| `shutdown` | Shutdown socket | ✓ | **OK** |

**Status:** Comprehensive implementation ✓

---

### 5.3 wasi:sockets/udp@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/sockets/wit/udp.wit`

**Implementation File:** `imports/wasip2/sockets/sockets.go`

| Item | Spec | Implementation | Status |
|------|------|----------------|--------|
| `resource udp-socket` | UDP socket | Defined but empty | **STUB** |
| `resource incoming-datagram-stream` | Receive stream | Not implemented | **MISSING** |
| `resource outgoing-datagram-stream` | Send stream | Not implemented | **MISSING** |
| `incoming-datagram` record | Received datagram | Not implemented | **MISSING** |
| `outgoing-datagram` record | Sent datagram | Not implemented | **MISSING** |

**Critical Gaps:**
- UDP socket has no methods implemented
- Datagram streams not implemented
- No send/receive functionality

---

### 5.4 wasi:sockets/ip-name-lookup@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/sockets/wit/ip-name-lookup.wit`

**Implementation:** `imports/wasip2/sockets/sockets.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `resolve-addresses` | DNS resolution | Not implemented | **MISSING** |
| `resource resolve-address-stream` | Async resolution | Not implemented | **MISSING** |

**Critical Gaps:**
- No DNS resolution support
- Required for hostname-based connections

---

### 5.5 wasi:sockets/tcp-create-socket@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `create-tcp-socket` | Create socket | ✓ | **OK** |

---

### 5.6 wasi:sockets/udp-create-socket@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `create-udp-socket` | Create socket | Returns error | **STUB** |

---

## 6. wasi:cli Interface Analysis

### 6.1 wasi:cli/environment@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/cli/wit/environment.wit`

**Implementation File:** `imports/wasip2/cli/cli.go`

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-environment` | Get env vars | ✓ from WASIConfig | **OK** |
| `get-arguments` | Get CLI args | ✓ from WASIConfig | **OK** |
| `initial-cwd` | Get working dir | ✓ os.Getwd() | **OK** |

**Status:** Functionally complete ✓

---

### 6.2 wasi:cli/exit@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `exit` | Exit with result | ✓ returns ExitError | **OK** |

**Status:** Functionally complete ✓

---

### 6.3 wasi:cli/stdin@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-stdin` | Get input stream | ✓ from WASIConfig | **OK** |

**Status:** Functionally complete ✓

---

### 6.4 wasi:cli/stdout@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-stdout` | Get output stream | ✓ from WASIConfig | **OK** |

**Status:** Functionally complete ✓

---

### 6.5 wasi:cli/stderr@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-stderr` | Get output stream | ✓ from WASIConfig | **OK** |

**Status:** Functionally complete ✓

---

### 6.6 wasi:cli/terminal-input@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/cli/wit/terminal.wit`

| Item | Spec | Implementation | Status |
|------|------|----------------|--------|
| `resource terminal-input` | Terminal input | ✓ defined | **OK** |

---

### 6.7 wasi:cli/terminal-output@0.2.0

| Item | Spec | Implementation | Status |
|------|------|----------------|--------|
| `resource terminal-output` | Terminal output | ✓ defined | **OK** |

---

### 6.8 wasi:cli/terminal-stdin@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-terminal-stdin` | Get terminal input | ✓ with TTY detection | **OK** |

---

### 6.9 wasi:cli/terminal-stdout@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-terminal-stdout` | Get terminal output | ✓ with TTY detection | **OK** |

---

### 6.10 wasi:cli/terminal-stderr@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `get-terminal-stderr` | Get terminal output | ✓ with TTY detection | **OK** |

**Status:** wasi:cli is functionally complete ✓

---

## 7. wasi:http Interface Analysis

### 7.1 wasi:http/types@0.2.0

**Spec File:** `debug-vendored/WASI/proposals/http/wit/types.wit`

**Implementation File:** `imports/wasip2/http/http.go`

#### fields Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `[constructor]fields` | Create fields | ✓ | **OK** |
| `from-list` | Create from list | ✓ | **OK** |
| `get` | Get header values | ✓ | **OK** |
| `has` | Check header exists | ✓ | **OK** |
| `set` | Set header | ✓ | **OK** |
| `delete` | Delete header | ✓ | **OK** |
| `append` | Append header | ✓ | **OK** |
| `entries` | Get all entries | ✓ | **OK** |
| `clone` | Clone fields | ✓ | **OK** |

#### incoming-request Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `method` | Get HTTP method | ✓ | **OK** |
| `path-with-query` | Get path | ✓ | **OK** |
| `scheme` | Get scheme | ✓ | **OK** |
| `authority` | Get authority | ✓ | **OK** |
| `headers` | Get headers | ✓ | **OK** |
| `consume` | Get body | ✓ | **OK** |

#### outgoing-request Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `[constructor]outgoing-request` | Create request | ✓ | **OK** |
| `body` | Get body stream | ✓ | **OK** |
| `method` | Get method | ✓ | **OK** |
| `set-method` | Set method | ✓ | **OK** |
| `path-with-query` | Get path | ✓ | **OK** |
| `set-path-with-query` | Set path | ✓ | **OK** |
| `scheme` | Get scheme | ✓ | **OK** |
| `set-scheme` | Set scheme | ✓ | **OK** |
| `authority` | Get authority | ✓ | **OK** |
| `set-authority` | Set authority | ✓ | **OK** |
| `headers` | Get headers | ✓ | **OK** |

#### incoming-response Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `status` | Get status code | ✓ | **OK** |
| `headers` | Get headers | ✓ | **OK** |
| `consume` | Get body | ✓ | **OK** |

#### outgoing-response Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `[constructor]outgoing-response` | Create response | ✓ | **OK** |
| `status-code` | Get status | ✓ | **OK** |
| `set-status-code` | Set status | ✓ | **OK** |
| `headers` | Get headers | ✓ | **OK** |
| `body` | Get body stream | ✓ | **OK** |

#### incoming-body Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `%stream` | Get input stream | ✓ | **OK** |
| `finish` | Signal completion | ✓ | **OK** |

#### outgoing-body Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `write` | Get output stream | ✓ | **OK** |
| `finish` | Finish with trailers | ✓ | **OK** |

#### future-incoming-response Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `subscribe` | Get pollable | ✓ | **OK** |
| `get` | Get response | ✓ | **OK** |

#### future-trailers Resource

| Method | Spec | Implementation | Status |
|--------|------|----------------|--------|
| `subscribe` | Get pollable | ✓ | **OK** |
| `get` | Get trailers | ✓ | **OK** |

---

### 7.2 wasi:http/outgoing-handler@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `handle` | Send request | Stub, returns error | **STUB** |

**Critical Gap:** Cannot make outgoing HTTP requests

---

### 7.3 wasi:http/incoming-handler@0.2.0

| Function | Spec | Implementation | Status |
|----------|------|----------------|--------|
| `handle` | Handle request | Not implemented | **MISSING** |

**Critical Gap:** Cannot handle incoming HTTP requests

---

## 8. Resource Lifecycle Analysis

### 8.1 Resource Table Implementation

**File:** `internal/component/resource_table.go`

| Feature | Implementation | Status |
|---------|----------------|--------|
| Resource creation | ✓ `New()` | **OK** |
| Resource retrieval | ✓ `Get()` | **OK** |
| Resource deletion | ✓ `Delete()` | **OK** |
| Handle generation | ✓ sequential IDs | **OK** |
| Type safety | Partial (interface{}) | **PARTIAL** |
| Ownership tracking | ✓ isOwn flag | **OK** |
| Borrow validation | ✓ | **OK** |

**Gaps:**
- No resource finalizers/destructors
- No automatic cleanup on scope exit
- Type safety relies on interface{} casts

### 8.2 Resource Lifecycle by Interface

| Interface | Resource | Cleanup | Status |
|-----------|----------|---------|--------|
| wasi:io | input-stream | Manual | **OK** |
| wasi:io | output-stream | Manual | **OK** |
| wasi:io | pollable | Manual | **OK** |
| wasi:filesystem | descriptor | Via Close() | **OK** |
| wasi:sockets | tcp-socket | Via destructor | **OK** |
| wasi:cli | terminal-input | No-op destructor | **OK** |
| wasi:cli | terminal-output | No-op destructor | **OK** |
| wasi:http | fields | No destructor | **MISSING** |
| wasi:http | bodies | No destructor | **MISSING** |

---

## 9. Error Handling Analysis

### 9.1 Error Code Mapping

| Interface | Error Enum | Mapping Quality | Status |
|-----------|------------|-----------------|--------|
| wasi:io | stream-error | ✓ Proper mapping | **OK** |
| wasi:filesystem | error-code | ✓ OS errors mapped | **OK** |
| wasi:sockets | error-code | ✓ Comprehensive | **OK** |
| wasi:http | error-code | ✓ HTTP-specific | **OK** |

### 9.2 Error Propagation

**Gaps:**
- Some internal errors silently converted to generic errors
- Stack traces not preserved in `error.to-debug-string`
- Some functions return placeholder errors instead of specific codes

---

## 10. Platform Compatibility Analysis

### 10.1 OS-Specific Features

| Feature | Linux | macOS | Windows | Status |
|---------|-------|-------|---------|--------|
| File operations | ✓ | ✓ | ✓ | **OK** |
| TCP sockets | ✓ | ✓ | ✓ | **OK** |
| UDP sockets | - | - | - | **NOT IMPL** |
| TTY detection | ✓ | ✓ | ? | **PARTIAL** |
| Symlinks | ✓ | ✓ | Limited | **PARTIAL** |
| Hard links | ✓ | ✓ | Limited | **PARTIAL** |

### 10.2 Known Platform Issues

1. **Windows symlinks** require elevated privileges
2. **Windows hard links** have filesystem restrictions
3. **Windows TTY** detection may not work for all terminals
4. **Socket options** may have different defaults per platform

---

## 11. Priority Recommendations

### High Priority (Critical for Production)

1. **Implement `poll.block` properly** - Required for any blocking I/O
2. **Implement `poll` function correctly** - Required for multiplexed I/O
3. **Implement UDP sockets** - Required for DNS, gaming, streaming
4. **Implement IP name lookup** - Required for hostname resolution
5. **Connect HTTP outgoing-handler** - Required for HTTP client functionality

### Medium Priority (Feature Completeness)

6. **Implement stream `splice` operations** - Efficient data transfer
7. **Make blocking stream operations actually block** - Spec compliance
8. **Add resource destructors for HTTP types** - Memory management
9. **Improve error.to-debug-string** - Better debugging

### Low Priority (Polish)

10. **Update version strings to 0.2.9** - Spec alignment
11. **Implement `advise` hints** - Performance optimization
12. **Improve check-write accuracy** - Buffer management
13. **Add comprehensive platform tests** - Portability assurance

---

## 12. Test Regression Requirement

**Critical:** Any fixes must not break:
- `internal/component/wasip2test/calculator_test.go`
- Tests for `add` and `subtract` plugins

**Current test dependencies:**
- wasi:cli/environment (for args/env)
- wasi:io/streams (for stdin/stdout)
- Basic resource table operations

---

## Appendix: Version Comparison

| Interface | Implementation | Spec | Difference |
|-----------|---------------|------|------------|
| wasi:io/error | 0.2.0 | 0.2.9 | Minor |
| wasi:io/poll | 0.2.0 | 0.2.9 | Minor |
| wasi:io/streams | 0.2.0 | 0.2.9 | Minor |
| wasi:clocks/* | 0.2.0 | 0.2.9 | Minor |
| wasi:random/* | 0.2.0 | 0.2.9 | Minor |
| wasi:filesystem/* | 0.2.0 | 0.2.9 | Minor |
| wasi:sockets/* | 0.2.0 | 0.2.9 | Minor |
| wasi:cli/* | 0.2.0 | 0.2.9 | Minor |
| wasi:http/* | 0.2.0 | 0.2.9 | Minor |

The version differences are primarily documentation and annotation improvements; the core API surface remains compatible.
