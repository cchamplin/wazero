# Phase 4: Binary Parser Completeness

Add missing canonical operations and fix export kind mapping.

---

## Task 4.1: Fix Export Kind Mapping for Core Sorts

**Status:** PENDING

**Files:**
- Modify: `internal/component/binary/exports.go:54-65`
- Test: `internal/component/binary/exports_test.go`

**Step 1: Write the failing test**

```go
func TestDecodeExport_CoreSorts(t *testing.T) {
    cases := []struct {
        coreSort byte
        expected component.ExportKind
    }{
        {0x00, component.ExportKindFunc},
        {0x01, component.ExportKindTable},
        {0x02, component.ExportKindMemory},
        {0x03, component.ExportKindGlobal},
    }

    for _, tc := range cases {
        // Construct minimal export bytes with core sort
        // Decode and verify ExportKind matches expected
    }
}
```

**Step 2:** Run: `go test ./internal/component/binary -run "TestDecodeExport_CoreSorts" -v`

**Step 3: Implementation**

Add proper export kinds and mapping:

```go
// In component/component.go, add:
const (
    ExportKindFunc ExportKind = iota
    ExportKindTable
    ExportKindMemory
    ExportKindGlobal
    // ... existing kinds
)

// In binary/exports.go, fix mapping:
case 0x01:
    exp.Kind = component.ExportKindTable
case 0x02:
    exp.Kind = component.ExportKindMemory
case 0x03:
    exp.Kind = component.ExportKindGlobal
```

---

## Task 4.2: Implement Post-Return Function Calls

**Status:** PENDING

**Files:**
- Modify: `internal/component/component_linker.go`
- Test: `internal/component/component_linker_test.go`

**Step 1: Write the failing test**

```go
func TestPostReturnFunctionCalled(t *testing.T) {
    // Create component with post-return function
    // Call exported function
    // Verify post-return was invoked after results returned
}
```

**Step 2-6:** Implement, verify, and commit
