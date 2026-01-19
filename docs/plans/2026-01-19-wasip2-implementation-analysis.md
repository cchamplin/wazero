# WASI Preview 2 Implementation Analysis

**Date:** 2026-01-19
**Status:** Analysis Complete
**Goal:** Thorough analysis of wazero's WASI Preview 2 implementation against specs and wasmtime

## Executive Summary

The wazero WASI Preview 2 implementation has made significant progress:
- **Calculator tests pass**: Both Rust (add.wasm) and C (subtract.wasm) real-world components work
- **All tests pass**: ~720 tests across component model and wasip2 packages with no failures
- **Good coverage**: 43.8%-96% coverage across packages

However, the analysis identifies **40+ implementation gaps** that need attention before the implementation can be considered production-ready.

## Test Results Summary

| Package | Tests | Status | Coverage |
|---------|-------|--------|----------|
| internal/component | 217 | PASS | 43.8% |
| internal/component/abi | 24 | PASS | 76.0% |
| internal/component/binary | 19 | PASS | 70.2% |
| internal/component/types | 56 | PASS | 96.0% |
| imports/wasip2 | 28 | PASS | 89.1% |
| imports/wasip2/cli | 24 | PASS | 92.3% |
| imports/wasip2/clocks | 18 | PASS | 80.4% |
| imports/wasip2/filesystem | 61 | PASS | 69.7% |
| imports/wasip2/http | 90 | PASS | 51.0% |
| imports/wasip2/io | 102 | PASS | 73.7% |
| imports/wasip2/random | 15 | PASS | 61.7% |
| imports/wasip2/sockets | 42 | PASS | 61.8% |

**Critical Constraint:** Calculator tests MUST continue passing throughout any remediation work.

---

## Critical Issues (P0 - Blockers)

### 1. Memory Allocation for Lists Uses Fixed Offset

**File:** `internal/component/instance.go:192`

```go
// TODO: In a full implementation, use realloc for proper allocation
ptr := uint32(0)
```

**Impact:** List data written to memory offset 0 will overwrite existing data. Any component using lists as parameters risks data corruption.

**Recommendation:** Implement proper realloc-based allocation with memory tracking.

### 2. String Lowering Not Implemented

**File:** `internal/component/canon_lower.go:377-378`

```go
case 0x73: // string
    return nil, fmt.Errorf("string lowering requires memory context")
```

**Impact:** Components that accept string parameters will fail at runtime.

**Recommendation:** Implement string lowering with realloc memory allocation and UTF-8 encoding.

### 3. Float Bit Interpretation Incorrect

**File:** `internal/component/canon_lower.go:75-76`

```go
case ValKindF32:
    return ValF32(float32(coreVal))  // Wrong - should use Float32frombits
```

**Impact:** Float values are cast directly rather than using `math.Float32frombits`/`Float64frombits`, corrupting bit patterns.

**Recommendation:** Use proper bit conversion functions for float types.

### 4. Limited List Element Type Support

**File:** `internal/component/instance.go:194-208`

Only `s32` and `u32` element types are supported. Lists of strings, records, variants, and other complex types fail.

**Recommendation:** Implement complete list element type support based on Canonical ABI spec.

---

## High Priority Issues (P1)

### 5. Filesystem Stream Methods Stubbed

**File:** `imports/wasip2/filesystem/filesystem.go`

| Function | Line | Status |
|----------|------|--------|
| `descriptorReadViaStream` | 230 | Returns placeholder handle (0) |
| `descriptorWriteViaStream` | 238 | Returns placeholder handle (0) |
| `descriptorAppendViaStream` | 246 | Returns placeholder handle (0) |
| `descriptorAdvise` | 254 | No-op stub |
| `descriptorSetSize` | 323 | Stub - returns success |
| `descriptorSetTimes` | 330 | Stub - returns success |

**Impact:** Stream-based file I/O doesn't work. Components using `read-via-stream` or `write-via-stream` will get non-functional handles.

### 6. HTTP Incoming Request Methods Are Placeholders

**File:** `imports/wasip2/http/http.go`

| Function | Line | Returns |
|----------|------|---------|
| `incomingRequestMethod` | 725 | Hardcoded GET |
| `incomingRequestPathWithQuery` | 732 | None |
| `incomingRequestScheme` | 738 | None |
| `incomingRequestAuthority` | 745 | None |

**Impact:** Server-side HTTP (incoming requests) is completely non-functional.

### 7. Socket Subscribe Returns Placeholder

**File:** `imports/wasip2/sockets/tcp.go:681-686`

```go
func tcpSocketSubscribe(ctx context.Context, args []component.Val) ([]component.Val, error) {
    // Return placeholder pollable handle
    return []component.Val{component.ValOwn(0)}, nil
}
```

**Impact:** Async socket operations that use polling won't work correctly.

### 8. Poll Implementation Is Simplified

**File:** `imports/wasip2/io/poll.go:127-162`

The poll implementation blocks on each pollable in turn rather than proper async multiplexing. Comment notes: "In a real implementation, we'd use a more sophisticated approach (select on channels, etc.)"

**Impact:** Polling multiple sources concurrently is inefficient and potentially incorrect.

---

## Medium Priority Issues (P2)

### 9. String Encoding Limited to UTF-8

**File:** `internal/component/instance.go:562`

```go
// TODO: Support other string encodings based on canonical options
```

The Canonical ABI supports UTF-8, UTF-16, and Latin1+UTF-16, but only UTF-8 is implemented.

### 10. Post-Return Functions Not Called

**File:** `internal/component/component_linker.go:1702-1758`

The `PostReturnIdx` canonical option is parsed but never executed. This means cleanup functions specified by the component are not invoked.

### 11. MAX_FLAT_PARAMS Not Enforced

Only `MaxFlatResults = 1` is defined. The spec also defines `MAX_FLAT_PARAMS = 16`. Beyond this limit, parameters should spill to memory.

### 12. Export Kind Mapping Incorrect for Core Sorts

**File:** `internal/component/binary/exports.go:54-65`

```go
case 0x01:
    exp.Kind = component.ExportKindFunc // table - WRONG
case 0x02:
    exp.Kind = component.ExportKindFunc // memory - WRONG
case 0x03:
    exp.Kind = component.ExportKindFunc // global - WRONG
```

All core sorts are mapped to `ExportKindFunc` instead of proper export kinds.

### 13. Resource Generation Scanning is O(n)

**File:** `internal/component/instance.go:791-799`

```go
for gen := uint32(1); gen < 1000; gen++ {
    h = MakeHandle(handleIdx, gen)
    // ...
}
```

Brute-force generation scanning is both inefficient and potentially incorrect.

### 14. Val Accessors Can Panic

**File:** `internal/component/val.go`

All accessor methods use unchecked type assertions:
```go
func (v Val) Bool() bool { return v.v.(bool) }
```

Calling the wrong accessor causes a panic instead of returning an error.

---

## Missing Features

### Binary Parser Missing Async Operations

**File:** `internal/component/binary/canonical.go`

Only these canonical operations are implemented:
- `CanonOpLift` (0x00)
- `CanonOpLower` (0x01)
- `CanonOpResourceNew` (0x02)
- `CanonOpResourceDrop` (0x03)
- `CanonOpResourceRep` (0x04)

**Missing from spec:**
- `canon task.return`, `task.cancel`, `subtask.cancel`, `subtask.drop`
- Stream operations (0x0e-0x14)
- Future operations (0x15-0x1b)
- Error-context operations (0x1c-0x1e)
- Waitable-set operations (0x1f-0x23)
- Backpressure operations (0x24-0x25)
- Thread operations (0x26-0x2b, 0x40-0x42)

### Type System Gaps

| Type | Opcode | Parsing | Runtime |
|------|--------|---------|---------|
| error-context | 0x64 | No | No |
| stream | 0x66 | Yes | No |
| future | 0x65 | Yes | No |
| variant | 0x71 | Yes | No |
| flags | 0x6e | Yes | No |
| enum | 0x6d | Yes | No |

---

## Comparison with Wasmtime

### Architectural Differences

| Aspect | Wasmtime | Wazero |
|--------|----------|--------|
| Type safety | Compile-time via generated traits | Runtime via dynamic Val types |
| Bindings | Auto-generated from WIT | Manual registration |
| Function signatures | Strongly typed | `[]component.Val` arrays |
| Async support | First-class with Tokio | Synchronous/blocking |
| Resource parent-child | Yes, via `push_child` | No |
| Poll implementation | Proper async multiplexing | Sequential blocking |

### Key Wasmtime Features Not in Wazero

1. **True async runtime** - Wasmtime uses Tokio for non-blocking I/O
2. **Resource parent-child tracking** - Automatic cleanup when parent dropped
3. **IP Name Lookup** - DNS resolution interface
4. **HTTP/2 support** - Via hyper integration
5. **TLS configuration** - Via rustls integration
6. **Platform-specific errno mapping** - Comprehensive Unix/Windows support

### Wazero Advantages

1. **Simpler synchronous model** - Easier to understand and debug
2. **Zero dependencies** - Maintains wazero's zero-dep philosophy
3. **Go-native** - Natural integration with Go applications
4. **Smaller footprint** - Less complexity for basic use cases

---

## WASI Interface Implementation Status

| Interface | Registration | Implementation | Notes |
|-----------|--------------|----------------|-------|
| wasi:cli/environment | Yes | Partial | Missing initial-cwd on some platforms |
| wasi:cli/exit | Yes | Complete | |
| wasi:cli/stdin | Yes | Complete | |
| wasi:cli/stdout | Yes | Complete | |
| wasi:cli/stderr | Yes | Complete | |
| wasi:io/error | Yes | Partial | Debug strings not implemented |
| wasi:io/streams | Yes | Partial | Error resource handling incomplete |
| wasi:io/poll | Yes | Partial | Simplified polling |
| wasi:clocks/monotonic | Yes | Complete | |
| wasi:clocks/wall | Yes | Complete | |
| wasi:random/random | Yes | Complete | |
| wasi:random/insecure | Yes | Complete | |
| wasi:random/insecure-seed | Yes | Complete | |
| wasi:filesystem/types | Yes | Partial | Stream methods stubbed |
| wasi:filesystem/preopens | Yes | Complete | |
| wasi:sockets/network | Yes | Partial | Placeholder implementation |
| wasi:sockets/tcp | Yes | Partial | Subscribe returns placeholder |
| wasi:sockets/udp | Yes | Partial | Similar issues to TCP |
| wasi:http/types | Yes | Partial | Incoming request stubbed |
| wasi:http/outgoing-handler | Yes | Functional | Basic HTTP client works |
| wasi:http/incoming-handler | Yes | Stub | Server-side not functional |

---

## Recommended Test Components to Port

To improve test coverage and validate the implementation, we recommend porting these components from wasmtime:

### Phase 1: Non-WASI Tests (Immediate)

1. **Temperature Converter** (`convert.wasm`)
   - Tests: f32 types, host function imports
   - Source: `wasmtime/examples/component/`

2. **Variant Type Test** (`variant_area.wasm`)
   - Tests: Variant types with different payloads
   - Source: `wasmtime/tests/misc_testsuite/component-model/types.wast`

3. **Flags Type Test** (`flags_permissions.wasm`)
   - Tests: Flags bitmask handling
   - Source: `wasmtime/tests/misc_testsuite/component-model/types.wast`

4. **String Echo Test** (`string_echo.wasm`)
   - Tests: String types with memory
   - Source: `component-model/test/values/strings.wast`

### Phase 2: Resource Tests

5. **Simple Resource Test** (`resource_counter.wasm`)
   - Tests: Basic resource create/drop/methods
   - Source: `component-model/test/resources/multiple-resources.wast`

6. **Key-Value Store** (`resource_kv.wasm`)
   - Tests: Multiple resource types, borrowing
   - Source: `wasmtime/examples/resource-component/`

### Phase 3: WASI P2 Integration

7. **Filesystem Test** (`wasi_file_ops.wasm`)
   - Tests: File read/write with preopens
   - Source: `wasmtime/crates/test-programs/src/bin/p2_file_read_write.rs`

8. **HTTP Test** (`wasi_http_client.wasm`)
   - Tests: Outbound HTTP requests
   - Source: `wasmtime/crates/test-programs/src/bin/p2_http_outbound_request_get.rs`

---

## Remediation Priority

### Immediate (Calculator Tests Must Keep Passing)

1. Fix float bit conversion in canon_lower.go
2. Implement string lowering with realloc
3. Fix list memory allocation
4. Add support for complex list element types

### Short-term

5. Implement filesystem stream methods
6. Fix socket subscribe to return proper pollable
7. Implement proper poll multiplexing
8. Add missing export kinds for core sorts

### Medium-term

9. Implement variant/flags/enum runtime support
10. Add post-return function calls
11. Implement MAX_FLAT_PARAMS limit
12. Add string encoding options

### Long-term

13. Parse and implement async canonical operations
14. Implement stream/future types
15. Add error-context support
16. Improve resource table efficiency

---

## Files Summary

| Directory | Files with Issues | Primary Concerns |
|-----------|------------------|------------------|
| internal/component/ | 8+ | Memory allocation, type lifting/lowering |
| internal/component/binary/ | 5+ | Missing async ops, incorrect export kinds |
| imports/wasip2/filesystem/ | 3+ | Stream methods stubbed |
| imports/wasip2/http/ | 2+ | Incoming handler stubbed |
| imports/wasip2/sockets/ | 3+ | Subscribe placeholders |
| imports/wasip2/io/ | 2+ | Poll simplification |

---

## Conclusion

The wazero WASI Preview 2 implementation provides a solid foundation with working real-world examples (calculator plugins). However, significant work remains for production readiness:

1. **Type system gaps**: Variants, flags, enums not fully implemented at runtime
2. **Memory management**: List/string allocation needs proper realloc integration
3. **Async model**: Current synchronous/blocking approach limits concurrency
4. **Interface coverage**: Several WASI interfaces have stub implementations

The implementation is suitable for:
- Simple components with primitive types
- Basic resource handling
- Outbound HTTP requests
- File operations (non-stream)

Not yet suitable for:
- Complex component types (nested variants, flags, enums)
- Server-side HTTP handling
- High-concurrency async workloads
- Full filesystem streaming
