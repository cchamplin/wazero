# Phase 1: Index Space Management

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track all 12 Component Model index spaces correctly during binary parsing.

**Architecture:** Add missing index counters to `Component` struct, update alias/decoder to increment appropriate spaces based on operation type.

**Tech Stack:** Go

**Gap Analysis Reference:** Section 2 - Index Space Management

---

## Context

The Component Model defines 12 index spaces:

**Component-level (5):**
- functions, values, types, component instances, components

**Core WebAssembly 1.0 (5):**
- functions, tables, memories, globals, types

**Core Extended (2):**
- module instances, modules

Currently only 3 are tracked: `NextFuncIdx`, `NextCoreFuncIdx`, `NextCoreMemoryIdx`

---

## Reference Files

- **Spec:** `debug-vendored/component-model/design/mvp/Explainer.md` (Index Spaces section)
- **wasmparser:** `debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/aliases.rs`
- **Current impl:** `internal/component/component.go:64-76`

---

## Task 1.1: Add Missing Index Counters to Component Struct

**Files:**
- Modify: `internal/component/component.go:64-76`

**Step 1: Read the current Component struct**

```bash
# Understand the current structure
```

**Step 2: Add the missing index counters**

Add these fields after the existing `Next*Idx` fields in the `Component` struct:

```go
// Component-level index spaces
NextFuncIdx             uint32 // Already exists
NextValueIdx            uint32 // NEW: values index space
NextTypeIdx             uint32 // NEW: types index space
NextComponentInstanceIdx uint32 // NEW: component instances index space
NextComponentIdx        uint32 // NEW: components index space

// Core WebAssembly 1.0 index spaces
NextCoreFuncIdx    uint32 // Already exists
NextCoreTableIdx   uint32 // NEW: core tables index space
NextCoreMemoryIdx  uint32 // Already exists
NextCoreGlobalIdx  uint32 // NEW: core globals index space
NextCoreTypeIdx    uint32 // NEW: core types index space

// Core Extended index spaces
NextModuleInstanceIdx uint32 // NEW: module instances index space
NextModuleIdx         uint32 // NEW: modules index space
```

**Step 3: Run tests to verify no compile errors**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 1.2: Update alias.go to Increment All Index Spaces

**Files:**
- Modify: `internal/component/binary/alias.go:87-126`

**Step 1: Read current alias decoding logic**

Look at `decodeAliasSection` and understand how `Idx` is assigned.

**Step 2: Update core export alias index assignment**

Currently only handles `CoreSortFunc` (0x00) and `CoreSortMemory` (0x02). Add handling for:

```go
case component.AliasKindCoreExport:
    switch alias.CoreSort {
    case component.CoreSortFunc:
        alias.Idx = c.NextCoreFuncIdx
        c.NextCoreFuncIdx++
    case component.CoreSortTable:
        alias.Idx = c.NextCoreTableIdx
        c.NextCoreTableIdx++
    case component.CoreSortMemory:
        alias.Idx = c.NextCoreMemoryIdx
        c.NextCoreMemoryIdx++
    case component.CoreSortGlobal:
        alias.Idx = c.NextCoreGlobalIdx
        c.NextCoreGlobalIdx++
    case component.CoreSortType:
        alias.Idx = c.NextCoreTypeIdx
        c.NextCoreTypeIdx++
    case component.CoreSortModule:
        alias.Idx = c.NextModuleIdx
        c.NextModuleIdx++
    case component.CoreSortInstance:
        alias.Idx = c.NextModuleInstanceIdx
        c.NextModuleInstanceIdx++
    }
```

**Step 3: Update component export alias index assignment**

Add handling for component-level sorts:

```go
case component.AliasKindExport:
    switch alias.Sort {
    case component.SortFunc:
        alias.Idx = c.NextFuncIdx
        c.NextFuncIdx++
    case component.SortValue:
        alias.Idx = c.NextValueIdx
        c.NextValueIdx++
    case component.SortType:
        alias.Idx = c.NextTypeIdx
        c.NextTypeIdx++
    case component.SortComponent:
        alias.Idx = c.NextComponentIdx
        c.NextComponentIdx++
    case component.SortInstance:
        alias.Idx = c.NextComponentInstanceIdx
        c.NextComponentInstanceIdx++
    }
```

**Step 4: Update outer alias index assignment**

```go
case component.AliasKindOuter:
    switch alias.Sort {
    case component.SortType:
        alias.Idx = c.NextTypeIdx
        c.NextTypeIdx++
    case component.SortComponent:
        alias.Idx = c.NextComponentIdx
        c.NextComponentIdx++
    // Note: SortCoreSort with CoreSortModule also valid for outer
    case component.SortCoreSort:
        if alias.CoreSort == component.CoreSortModule {
            alias.Idx = c.NextModuleIdx
            c.NextModuleIdx++
        } else if alias.CoreSort == component.CoreSortType {
            alias.Idx = c.NextCoreTypeIdx
            c.NextCoreTypeIdx++
        }
    }
```

**Step 5: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 1.3: Update decoder.go for Type/Module/Instance Index Effects

**Files:**
- Modify: `internal/component/binary/decoder.go`

**Step 1: Track type indices when decoding type section**

In `decodeTypeSection`, increment `NextTypeIdx` for each type:

```go
func decodeTypeSection(c *component.Component, r *bytes.Reader) error {
    count, _, err := leb128.DecodeUint32(r)
    if err != nil {
        return fmt.Errorf("read type count: %w", err)
    }

    for i := uint32(0); i < count; i++ {
        typeDef, err := decodeTypeDef(r)
        if err != nil {
            return fmt.Errorf("decode type %d: %w", i, err)
        }
        c.Types = append(c.Types, typeDef)
        c.NextTypeIdx++ // ADD THIS
    }
    return nil
}
```

**Step 2: Track module indices when decoding module section**

In the section switch case for module (section 1):

```go
case 1: // core:module
    mod, modData, err := decodeModule(sectionReader)
    if err != nil {
        return nil, fmt.Errorf("decoding module: %w", err)
    }
    c.CoreModules = append(c.CoreModules, mod)
    c.CoreModuleData = append(c.CoreModuleData, modData)
    c.NextModuleIdx++ // ADD THIS
```

**Step 3: Track instance indices when decoding instance sections**

For core instance section (section 2):

```go
case 2: // vec(core:instance)
    if err := decodeCoreInstanceSection(&c, sectionReader); err != nil {
        return nil, fmt.Errorf("decoding core instance section: %w", err)
    }
    // Note: decodeCoreInstanceSection should increment NextModuleInstanceIdx
```

Update `decodeCoreInstanceSection` in `core_instance.go`:

```go
func decodeCoreInstanceSection(c *component.Component, r *bytes.Reader) error {
    // ... existing code ...
    for i := uint32(0); i < count; i++ {
        ci, err := decodeCoreInstance(r)
        // ...
        c.CoreInstances = append(c.CoreInstances, ci)
        c.NextModuleInstanceIdx++ // ADD THIS
    }
    return nil
}
```

For component instance section (section 5):

```go
func decodeInstanceSection(c *component.Component, r *bytes.Reader) error {
    // ... existing code ...
    for i := uint32(0); i < count; i++ {
        ci, err := decodeComponentInstance(r)
        // ...
        c.ComponentInstances[i] = ci
        c.NextComponentInstanceIdx++ // ADD THIS
    }
    return nil
}
```

**Step 4: Track component indices for nested components**

For component section (section 4):

```go
case 4: // component
    nested, err := decodeComponent(sectionReader)
    if err != nil {
        return nil, fmt.Errorf("decoding nested component: %w", err)
    }
    c.Components = append(c.Components, nested)
    c.NextComponentIdx++ // ADD THIS
```

**Step 5: Run build to verify**

```bash
CGO_ENABLED=0 go build ./internal/component/...
```
Expected: Build succeeds

---

## Task 1.4: Add Index Space Tracking Tests

**Files:**
- Create: `internal/component/binary/index_space_test.go`

**Step 1: Create test file with basic test structure**

```go
package binary

import (
    "bytes"
    "testing"

    "github.com/tetratelabs/wazero/internal/component"
)

func TestIndexSpaceTracking(t *testing.T) {
    t.Run("core export alias increments correct space", func(t *testing.T) {
        // Create minimal component with alias that exports a table
        // Verify NextCoreTableIdx increments
    })

    t.Run("type section increments type index", func(t *testing.T) {
        // Create minimal component with type definitions
        // Verify NextTypeIdx matches type count
    })

    t.Run("module section increments module index", func(t *testing.T) {
        // Create minimal component with embedded module
        // Verify NextModuleIdx increments
    })
}
```

**Step 2: Run the new tests**

```bash
CGO_ENABLED=0 go test ./internal/component/binary/... -run TestIndexSpaceTracking -v
```
Expected: Tests pass (or skip if no test binaries available)

---

## Task 1.5: Run Regression Tests and Commit

**Step 1: Run calculator regression tests**

```bash
CGO_ENABLED=0 go test ./internal/component/wasip2test/... -run "TestCalculatorPlugins/(add|subtract)" -v
```
Expected: Both add and subtract pass

**Step 2: Run full component binary tests**

```bash
CGO_ENABLED=0 go test ./internal/component/binary/... -v
```
Expected: All tests pass

**Step 3: Commit changes**

```bash
git add internal/component/component.go internal/component/binary/alias.go internal/component/binary/decoder.go internal/component/binary/core_instance.go internal/component/binary/instance.go
git commit -m "feat(component): track all 12 index spaces per spec

Add missing index counters to Component struct:
- Component-level: NextValueIdx, NextTypeIdx, NextComponentInstanceIdx, NextComponentIdx
- Core: NextCoreTableIdx, NextCoreGlobalIdx, NextCoreTypeIdx
- Extended: NextModuleInstanceIdx, NextModuleIdx

Update alias.go to increment appropriate index space for all sort types.
Update decoder.go to track indices for types, modules, instances.

Ref: docs/plans/component-model-binary-parser-gap-analysis.md Section 2

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Verification Checklist

- [ ] All 12 index spaces have counters in Component struct
- [ ] Alias operations increment correct index space
- [ ] Type section increments NextTypeIdx
- [ ] Module section increments NextModuleIdx
- [ ] Core instance section increments NextModuleInstanceIdx
- [ ] Component instance section increments NextComponentInstanceIdx
- [ ] Nested component section increments NextComponentIdx
- [ ] Calculator add/subtract tests pass
- [ ] Changes committed
