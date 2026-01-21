# Phase 2: Canonical Options

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add parsing support for all canonical options defined in the spec.

**Architecture:** Extend `CanonicalOptions` struct and update `decodeCanonicalOptions` to handle opcodes 0x06-0x09.

**Tech Stack:** Go

**Gap Analysis Reference:** Section 6.3 - Canonical Options

---

## Context

Currently implemented options (0x00-0x05):
- 0x00: string-encoding=utf8
- 0x01: string-encoding=utf16
- 0x02: string-encoding=latin1+utf16
- 0x03: memory
- 0x04: realloc
- 0x05: post-return

Missing options:
- 0x06: async (🔀 gated)
- 0x07: callback (🔀 gated)
- 0x08: core-type
- 0x09: gc

---

## Reference Files

- **Spec:** `debug-vendored/component-model/design/mvp/Binary.md` (canonopt section)
- **wasmparser:** `debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/canonicals.rs:442-458`
- **Current impl:** `internal/component/binary/canonical.go:130-160`

---

## Task 2.1: Add Async Option (0x06) Parsing

**Files:**
- Modify: `internal/component/component.go` (CanonicalOptions struct)
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add Async field to CanonicalOptions**

In `internal/component/component.go`, find `CanonicalOptions` struct and add:

```go
// CanonicalOptions holds optional parameters for canonical operations.
type CanonicalOptions struct {
    StringEncoding StringEncoding
    MemoryIdx      *uint32 // nil if not specified
    ReallocIdx     *uint32 // nil if not specified
    PostReturnIdx  *uint32 // nil if not specified
    Async          bool    // NEW: true if async option specified
}
```

**Step 2: Handle 0x06 in decodeCanonicalOptions**

In `internal/component/binary/canonical.go`, find `decodeCanonicalOptions` and add case:

```go
case 0x06: // async
    opts.Async = true
```

**Step 3: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 2.2: Add Callback Option (0x07) Parsing

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CallbackIdx field to CanonicalOptions**

```go
type CanonicalOptions struct {
    StringEncoding StringEncoding
    MemoryIdx      *uint32 // nil if not specified
    ReallocIdx     *uint32 // nil if not specified
    PostReturnIdx  *uint32 // nil if not specified
    Async          bool    // true if async option specified
    CallbackIdx    *uint32 // NEW: callback function index
}
```

**Step 2: Handle 0x07 in decodeCanonicalOptions**

```go
case 0x07: // callback
    idx, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return opts, fmt.Errorf("read callback index: %w", err)
    }
    opts.CallbackIdx = &idx
```

**Step 3: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 2.3: Add Core-Type Option (0x08) Parsing

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add CoreTypeIdx field to CanonicalOptions**

```go
type CanonicalOptions struct {
    StringEncoding StringEncoding
    MemoryIdx      *uint32 // nil if not specified
    ReallocIdx     *uint32 // nil if not specified
    PostReturnIdx  *uint32 // nil if not specified
    Async          bool    // true if async option specified
    CallbackIdx    *uint32 // callback function index
    CoreTypeIdx    *uint32 // NEW: core type index for lowering
}
```

**Step 2: Handle 0x08 in decodeCanonicalOptions**

```go
case 0x08: // core-type
    idx, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return opts, fmt.Errorf("read core-type index: %w", err)
    }
    opts.CoreTypeIdx = &idx
```

**Step 3: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 2.4: Add GC Option (0x09) Parsing

**Files:**
- Modify: `internal/component/component.go`
- Modify: `internal/component/binary/canonical.go`

**Step 1: Add GC field to CanonicalOptions**

```go
type CanonicalOptions struct {
    StringEncoding StringEncoding
    MemoryIdx      *uint32 // nil if not specified
    ReallocIdx     *uint32 // nil if not specified
    PostReturnIdx  *uint32 // nil if not specified
    Async          bool    // true if async option specified
    CallbackIdx    *uint32 // callback function index
    CoreTypeIdx    *uint32 // core type index for lowering
    GC             bool    // NEW: use GC version of canonical ABI
}
```

**Step 2: Handle 0x09 in decodeCanonicalOptions**

```go
case 0x09: // gc
    opts.GC = true
```

**Step 3: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 2.5: Update CanonicalOptions Struct (Consolidation)

**Files:**
- Modify: `internal/component/component.go:271-278`

**Step 1: Verify final CanonicalOptions struct**

The complete struct should now be:

```go
// CanonicalOptions holds optional parameters for canonical operations.
type CanonicalOptions struct {
    StringEncoding StringEncoding
    MemoryIdx      *uint32 // nil if not specified
    ReallocIdx     *uint32 // nil if not specified
    PostReturnIdx  *uint32 // nil if not specified
    Async          bool    // true if async option specified (🔀)
    CallbackIdx    *uint32 // callback function index (🔀)
    CoreTypeIdx    *uint32 // core type index for lowering
    GC             bool    // use GC version of canonical ABI
}
```

**Step 2: Verify complete switch in decodeCanonicalOptions**

```go
func decodeCanonicalOptions(r *bytes.Reader, count uint32) (component.CanonicalOptions, error) {
    var opts component.CanonicalOptions
    opts.StringEncoding = component.StringEncodingUTF8 // default

    for i := uint32(0); i < count; i++ {
        optByte, err := r.ReadByte()
        if err != nil {
            return opts, fmt.Errorf("read option %d: %w", i, err)
        }

        switch optByte {
        case 0x00:
            opts.StringEncoding = component.StringEncodingUTF8
        case 0x01:
            opts.StringEncoding = component.StringEncodingUTF16
        case 0x02:
            opts.StringEncoding = component.StringEncodingLatin1UTF16
        case 0x03:
            idx, _, err := leb128.DecodeUint32(r)
            if err != nil {
                return opts, fmt.Errorf("read memory index: %w", err)
            }
            opts.MemoryIdx = &idx
        case 0x04:
            idx, _, err := leb128.DecodeUint32(r)
            if err != nil {
                return opts, fmt.Errorf("read realloc index: %w", err)
            }
            opts.ReallocIdx = &idx
        case 0x05:
            idx, _, err := leb128.DecodeUint32(r)
            if err != nil {
                return opts, fmt.Errorf("read post-return index: %w", err)
            }
            opts.PostReturnIdx = &idx
        case 0x06:
            opts.Async = true
        case 0x07:
            idx, _, err := leb128.DecodeUint32(r)
            if err != nil {
                return opts, fmt.Errorf("read callback index: %w", err)
            }
            opts.CallbackIdx = &idx
        case 0x08:
            idx, _, err := leb128.DecodeUint32(r)
            if err != nil {
                return opts, fmt.Errorf("read core-type index: %w", err)
            }
            opts.CoreTypeIdx = &idx
        case 0x09:
            opts.GC = true
        default:
            return opts, fmt.Errorf("unknown canonical option: 0x%02x", optByte)
        }
    }

    return opts, nil
}
```

---

## Task 2.6: Add Canonical Options Tests

**Files:**
- Create: `internal/component/binary/canonical_options_test.go`

**Step 1: Create test file**

```go
package binary

import (
    "bytes"
    "testing"

    "github.com/tetratelabs/wazero/internal/component"
)

func TestDecodeCanonicalOptions(t *testing.T) {
    tests := []struct {
        name     string
        input    []byte
        count    uint32
        expected component.CanonicalOptions
    }{
        {
            name:  "empty options defaults to utf8",
            input: []byte{},
            count: 0,
            expected: component.CanonicalOptions{
                StringEncoding: component.StringEncodingUTF8,
            },
        },
        {
            name:  "async option",
            input: []byte{0x06},
            count: 1,
            expected: component.CanonicalOptions{
                StringEncoding: component.StringEncodingUTF8,
                Async:          true,
            },
        },
        {
            name:  "callback option with index 5",
            input: []byte{0x07, 0x05},
            count: 1,
            expected: component.CanonicalOptions{
                StringEncoding: component.StringEncodingUTF8,
                CallbackIdx:    ptrUint32(5),
            },
        },
        {
            name:  "gc option",
            input: []byte{0x09},
            count: 1,
            expected: component.CanonicalOptions{
                StringEncoding: component.StringEncodingUTF8,
                GC:             true,
            },
        },
        {
            name:  "multiple options",
            input: []byte{0x06, 0x03, 0x00, 0x04, 0x01},
            count: 3,
            expected: component.CanonicalOptions{
                StringEncoding: component.StringEncodingUTF8,
                Async:          true,
                MemoryIdx:      ptrUint32(0),
                ReallocIdx:     ptrUint32(1),
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r := bytes.NewReader(tt.input)
            got, err := decodeCanonicalOptions(r, tt.count)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            // Compare fields
            if got.Async != tt.expected.Async {
                t.Errorf("Async: got %v, want %v", got.Async, tt.expected.Async)
            }
            if got.GC != tt.expected.GC {
                t.Errorf("GC: got %v, want %v", got.GC, tt.expected.GC)
            }
            // Add more field comparisons as needed
        })
    }
}

func ptrUint32(v uint32) *uint32 {
    return &v
}
```

**Step 2: Run tests**

```bash
CGO_ENABLED=0 go test ./internal/component/binary/... -run TestDecodeCanonicalOptions -v
```
Expected: All tests pass

---

## Task 2.7: Run Regression Tests and Commit

**Step 1: Run calculator regression tests**

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```
Expected: Both add and subtract pass

**Step 2: Commit changes**

```bash
git add internal/component/component.go internal/component/binary/canonical.go internal/component/binary/canonical_options_test.go
git commit -m "feat(component): add canonical options 0x06-0x09 per spec

Add parsing for:
- 0x06: async option (sets Async flag)
- 0x07: callback option (stores CallbackIdx)
- 0x08: core-type option (stores CoreTypeIdx)
- 0x09: gc option (sets GC flag)

Ref: docs/plans/component-model-binary-parser-gap-analysis.md Section 6.3
Ref: debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/canonicals.rs:442-458

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Verification Checklist

- [ ] CanonicalOptions struct has Async, CallbackIdx, CoreTypeIdx, GC fields
- [ ] decodeCanonicalOptions handles 0x06, 0x07, 0x08, 0x09
- [ ] Tests cover all new options
- [ ] Calculator add/subtract tests pass
- [ ] Changes committed
