# Component Model Binary Format Parser Gap Analysis

**Date:** 2026-01-20
**Analyst:** Claude Opus 4.5
**Spec Reference:** debug-vendored/component-model/design/mvp/Binary.md
**Implementation:** internal/component/binary/

---

## Executive Summary

This document analyzes gaps between the wazero Component Model binary parser and the official specification. The parser implements core functionality needed for basic components but lacks many advanced features, particularly around async operations, comprehensive index space tracking, and full validation.

**Current Status:**
- ✅ **Working:** Basic component parsing, type definitions, canon lift/lower, aliases, imports/exports
- ⚠️ **Partial:** Index space management, value definitions, canonical options
- ❌ **Missing:** 30+ canonical builtins, async features, comprehensive validation

---

## 1. Preamble & Section Parsing

### Status: ✅ COMPLETE

| Feature | Status | Location |
|---------|--------|----------|
| Magic bytes (0x00 0x61 0x73 0x6D) | ✅ | binary.go:11 |
| Version (0x0d 0x00) | ✅ | binary.go:15 |
| Layer (0x01 0x00) | ✅ | binary.go:18 |
| Section 0: core:custom | ✅ Skipped | decoder.go:124 |
| Section 1: core:module | ✅ | decoder.go:74 |
| Section 2: vec(core:instance) | ✅ | decoder.go:77 |
| Section 3: vec(core:type) | ✅ | decoder.go:81 |
| Section 4: component | ✅ | decoder.go:109 |
| Section 5: vec(instance) | ✅ | decoder.go:105 |
| Section 6: vec(alias) | ✅ | decoder.go:97 |
| Section 7: vec(type) | ✅ | decoder.go:85 |
| Section 8: vec(canon) | ✅ | decoder.go:89 |
| Section 9: start | ✅ | decoder.go:116 |
| Section 10: vec(import) | ✅ | decoder.go:101 |
| Section 11: vec(export) | ✅ | decoder.go:93 |
| Section 12: vec(value) 🪙 | ✅ Partial | decoder.go:120 |

---

## 2. Index Space Management

### Status: ⚠️ PARTIAL

The Component Model defines 12 index spaces that must be tracked. The current implementation only tracks 3.

| Index Space | Tracked | Field |
|-------------|---------|-------|
| **Component-level** |||
| (component) functions | ✅ | `NextFuncIdx` |
| (component) values | ❌ | - |
| (component) types | ❌ | - |
| component instances | ❌ | - |
| components | ❌ | - |
| **Core (WebAssembly 1.0)** |||
| (core) functions | ✅ | `NextCoreFuncIdx` |
| (core) tables | ❌ | - |
| (core) memories | ✅ | `NextCoreMemoryIdx` |
| (core) globals | ❌ | - |
| (core) types | ❌ | - |
| **Core (Extended)** |||
| module instances | ❌ | - |
| modules | ❌ | - |

### Gaps:
1. **Missing index counters:** Need `NextCoreTableIdx`, `NextCoreGlobalIdx`, `NextCoreTypeIdx`, `NextValueIdx`, `NextTypeIdx`, `NextComponentInstanceIdx`, `NextComponentIdx`, `NextModuleInstanceIdx`, `NextModuleIdx`
2. **Missing index updates:** Alias operations for non-func/memory sorts don't update indices
3. **Import index effects:** Imports should increment appropriate index spaces

### Location: component.go:64-76

---

## 3. Instance Definitions

### Status: ✅ COMPLETE

| Feature | Status | Location |
|---------|--------|----------|
| core:instance | ✅ | core_instance.go |
| core:instanceexpr (instantiate 0x00) | ✅ | core_instance.go:24 |
| core:instanceexpr (inline 0x01) | ✅ | core_instance.go:62 |
| core:instantiatearg (with instance 0x12) | ✅ | core_instance.go:42-48 |
| core:sortidx | ✅ | core_instance.go:75-88 |
| core:sort (func/table/memory/global/type/module/instance) | ✅ | component.go:344-352 |
| component instance | ✅ | instance.go |
| instanceexpr (instantiate 0x00) | ✅ | instance.go:24 |
| instanceexpr (inline 0x01) | ✅ | instance.go:59 |
| instantiatearg | ✅ | instance.go:36-56 |
| sort (core/func/value/type/component/instance) | ✅ | component.go:311-319 |

---

## 4. Alias Definitions

### Status: ⚠️ PARTIAL

| Feature | Status | Location |
|---------|--------|----------|
| Export alias (0x00) | ✅ | alias.go:47 |
| Core export alias (0x01) | ✅ | alias.go:58 |
| Outer alias (0x02) | ✅ | alias.go:69 |
| Sort parsing (core prefix 0x00) | ✅ | alias.go:29-38 |

### Gaps:
1. **Outer alias validation:** Spec requires sort to be `type`, `module`, or `component` only
2. **Resource restriction:** Cannot outer-alias resource types (generative)
3. **Index space updates:** Only func and memory sorts update index spaces
4. **Missing sorts:** Outer aliases for table, global, value don't update spaces

### Location: alias.go:87-126

---

## 5. Type Definitions

### Status: ⚠️ MOSTLY COMPLETE

#### 5.1 Core Types

| Feature | Status | Location |
|---------|--------|----------|
| core:type 0x60 (func) | ✅ | core_type.go:25 |
| core:type 0x50 (module) | ✅ | core_type.go:34 |
| core:moduledecl 0x00 (import) | ✅ | core_type.go:108 |
| core:moduledecl 0x01 (type) | ✅ | core_type.go:131 |
| core:moduledecl 0x02 (outer alias) | ✅ | core_type.go:146 |
| core:moduledecl 0x03 (export) | ✅ | core_type.go:155 |
| **0x00 0x50 prefix for non-final sub** | ❌ | Not handled |

#### 5.2 Primitive Value Types

| Opcode | Type | Status | Location |
|--------|------|--------|----------|
| 0x7f | bool | ✅ | valtype.go:13 |
| 0x7e | s8 | ✅ | valtype.go:14 |
| 0x7d | u8 | ✅ | valtype.go:15 |
| 0x7c | s16 | ✅ | valtype.go:16 |
| 0x7b | u16 | ✅ | valtype.go:17 |
| 0x7a | s32 | ✅ | valtype.go:18 |
| 0x79 | u32 | ✅ | valtype.go:19 |
| 0x78 | s64 | ✅ | valtype.go:20 |
| 0x77 | u64 | ✅ | valtype.go:21 |
| 0x76 | f32 | ✅ | valtype.go:22 |
| 0x75 | f64 | ✅ | valtype.go:23 |
| 0x74 | char | ✅ | valtype.go:24 |
| 0x73 | string | ✅ | valtype.go:25 |
| 0x64 | error-context 📝 | ✅ | valtype.go:26 |

#### 5.3 Composite/Defined Types

| Opcode | Type | Status | Location |
|--------|------|--------|----------|
| 0x72 | record | ✅ | decoder.go:188, types.go:389 |
| 0x71 | variant | ✅ | decoder.go:235, types.go:433 |
| 0x70 | list | ✅ | decoder.go:211, types.go:420 |
| 0x67 | fixed-size list 🔧 | ✅ | decoder.go:343, types.go:673 |
| 0x6f | tuple | ✅ | decoder.go:247, types.go:496 |
| 0x6e | flags | ✅ | decoder.go:259, types.go:518 |
| 0x6d | enum | ✅ | decoder.go:271, types.go:540 |
| 0x6b | option | ✅ | decoder.go:199, types.go:562 |
| 0x6a | result | ✅ | decoder.go:223, types.go:573 |
| 0x69 | own | ✅ | types.go:161 |
| 0x68 | borrow | ✅ | types.go:148 |
| 0x66 | stream 🔀 | ✅ | decoder.go:319, types.go:615 |
| 0x65 | future 🔀 | ✅ | decoder.go:331, types.go:651 |
| 0x63 | map | ❌ | Not in spec (wasmparser has it) |

#### 5.4 Function Types

| Feature | Status | Location |
|---------|--------|----------|
| functype 0x40 (sync) | ✅ | types.go:16, 192 |
| functype 0x43 (async) | ✅ | types.go:17, 198 |
| paramlist | ✅ | types.go:204-226 |
| resultlist 0x00 (single) | ✅ | types.go:234-241 |
| resultlist 0x01 (named/empty) | ✅ | types.go:243-266 |

#### 5.5 Resource Types

| Feature | Status | Location |
|---------|--------|----------|
| resourcetype 0x3f (sync dtor) | ✅ | types.go:20, 692-730 |
| resourcetype 0x3e (async dtor) 🚝 | ❌ | Not implemented |

#### 5.6 Component/Instance Types

| Feature | Status | Location |
|---------|--------|----------|
| componenttype 0x41 | ✅ | component_type.go |
| instancetype 0x42 | ✅ | instance_type.go |
| componentdecl 0x00-0x04 | ✅ | component_type.go:39-88 |
| instancedecl 0x00-0x04 | ✅ | instance_type.go:39-86 |
| typebound 0x00 (eq) | ✅ | instance_type.go:152-158 |
| typebound 0x01 (sub resource) | ✅ | instance_type.go:160-162 |

---

## 6. Canonical Definitions

### Status: ❌ INCOMPLETE (5 of 45+ implemented)

#### 6.1 Implemented Canonicals

| Opcode | Function | Status | Location |
|--------|----------|--------|----------|
| 0x00 0x00 | canon lift | ✅ | canonical.go:42 |
| 0x01 0x00 | canon lower | ✅ | canonical.go:71 |
| 0x02 | resource.new | ✅ | canonical.go:94 |
| 0x03 | resource.drop | ✅ | canonical.go:101 |
| 0x04 | resource.rep | ✅ | canonical.go:108 |

#### 6.2 Missing Canonicals (40+)

| Opcode | Function | Feature Flag |
|--------|----------|--------------|
| 0x05 | task.cancel | 🔀 |
| 0x06 | subtask.cancel | 🔀 |
| 0x07 | resource.drop (async) | 🔀 |
| 0x09 | task.return | 🔀 |
| 0x0a | context.get | 🔀 |
| 0x0b | context.set | 🔀 |
| 0x0c | thread.yield | 🔀 |
| 0x0d | subtask.drop | 🔀 |
| 0x0e | stream.new | 🔀 |
| 0x0f | stream.read | 🔀 |
| 0x10 | stream.write | 🔀 |
| 0x11 | stream.cancel-read | 🔀 |
| 0x12 | stream.cancel-write | 🔀 |
| 0x13 | stream.drop-readable | 🔀 |
| 0x14 | stream.drop-writable | 🔀 |
| 0x15 | future.new | 🔀 |
| 0x16 | future.read | 🔀 |
| 0x17 | future.write | 🔀 |
| 0x18 | future.cancel-read | 🔀 |
| 0x19 | future.cancel-write | 🔀 |
| 0x1a | future.drop-readable | 🔀 |
| 0x1b | future.drop-writable | 🔀 |
| 0x1c | error-context.new | 📝 |
| 0x1d | error-context.debug-message | 📝 |
| 0x1e | error-context.drop | 📝 |
| 0x1f | waitable-set.new | 🔀 |
| 0x20 | waitable-set.wait | 🔀 |
| 0x21 | waitable-set.poll | 🔀 |
| 0x22 | waitable-set.drop | 🔀 |
| 0x23 | waitable.join | 🔀 |
| 0x24 | backpressure.inc | 🔀 |
| 0x25 | backpressure.dec | 🔀 |
| 0x26 | thread.index | 🧵 |
| 0x27 | thread.new-indirect | 🧵 |
| 0x28 | thread.switch-to | 🧵 |
| 0x29 | thread.suspend | 🧵 |
| 0x2a | thread.resume-later | 🧵 |
| 0x2b | thread.yield-to | 🧵 |
| 0x40 | thread.spawn-ref | 🧵② |
| 0x41 | thread.spawn-indirect | 🧵② |
| 0x42 | thread.available-parallelism | 🧵② |

#### 6.3 Canonical Options

| Opcode | Option | Status | Location |
|--------|--------|--------|----------|
| 0x00 | string-encoding=utf8 | ✅ | canonical.go:136 |
| 0x01 | string-encoding=utf16 | ✅ | canonical.go:138 |
| 0x02 | string-encoding=latin1+utf16 | ✅ | canonical.go:140 |
| 0x03 | memory | ✅ | canonical.go:142 |
| 0x04 | realloc | ✅ | canonical.go:148 |
| 0x05 | post-return | ✅ | canonical.go:154 |
| 0x06 | async 🔀 | ❌ | Missing |
| 0x07 | callback 🔀 | ❌ | Missing |
| 0x08 | core-type | ❌ | Missing |
| 0x09 | gc | ❌ | Missing |

---

## 7. Import/Export Definitions

### Status: ⚠️ MOSTLY COMPLETE

#### 7.1 Import Names

| Feature | Status | Location |
|---------|--------|----------|
| 0x00 prefix (plain) | ✅ | import.go:23 |
| 0x01 prefix (versioned) 🔗 | ✅ | import.go:24 |
| Version suffix parsing | ⚠️ Partial | Embedded in name, not parsed |

#### 7.2 Export Names

| Feature | Status | Location |
|---------|--------|----------|
| 0x00 prefix (plain) | ✅ | exports.go:122 |
| 0x01 prefix (versioned) | ✅ | exports.go:127 |
| Version suffix parsing | ⚠️ Skipped | exports.go:133-139 |

#### 7.3 ExternDesc

| Opcode | Kind | Status | Location |
|--------|------|--------|----------|
| 0x00 0x11 | core module | ✅ | import.go:50-63 |
| 0x01 | func | ✅ | import.go:65-69 |
| 0x02 | value 🪙 | ✅ | import.go:72-79 |
| 0x03 | type (bounds) | ✅ | import.go:81-100 |
| 0x04 | component | ✅ | import.go:102-107 |
| 0x05 | instance | ✅ | import.go:109-114 |

---

## 8. Value Definitions (Section 12) 🪙

### Status: ❌ INCOMPLETE

The spec defines a comprehensive value encoding for all types. Current implementation only handles a few primitives.

| Feature | Status | Location |
|---------|--------|----------|
| Primitive values (bool, integers) | ⚠️ Partial | value.go:27-44 |
| Float values (f32, f64) | ❌ | Missing |
| Char values | ❌ | Missing |
| String values | ❌ | Missing |
| Record/variant/list/tuple values | ❌ | Missing |
| Flags/enum/option/result values | ❌ | Missing |

### Gap: Full val(T) encoding per spec lines 432-465

---

## 9. Start Section

### Status: ✅ COMPLETE

| Feature | Status | Location |
|---------|--------|----------|
| funcidx | ✅ | start.go:11 |
| vec(valueidx) args | ✅ | start.go:17-29 |
| result count | ✅ | start.go:31 |

---

## 10. Name Section (Custom)

### Status: ❌ NOT IMPLEMENTED

The spec defines a `component-name` custom section for debugging. Not implemented.

| Feature | Status |
|---------|--------|
| Component name subsection | ❌ |
| Sort names subsection | ❌ |
| Name map | ❌ |

---

## 11. Validation Gaps

The current parser does minimal validation. Key missing validations:

| Validation | Spec Reference |
|------------|----------------|
| Record/variant/tuple/list/flags/enum must have ≥1 element | Binary.md:196-202 |
| Borrow types not allowed in results | Binary.md:252-253 |
| Stream/future element types cannot contain borrow | Binary.md:254-255 |
| Resource destructor must have type [i32] -> [] | Binary.md:256-257 |
| Unique names in records/variants/flags/enums | Binary.md:273-275 |
| Outer alias sort restricted to type/module/component | Binary.md:131-133 |
| Cannot outer-alias resource types | Binary.md:133 |
| Fixed-size list length must be >0 | Binary.md:281 |
| Validation of instantiate args match imports | Binary.md:103-109 |

---

## Prioritized Implementation Plan

### Priority 1: Core Functionality Fixes (High Impact)

1. **Index Space Management** - Add missing counters and update logic
   - Files: component.go, alias.go, decoder.go
   - Effort: Medium

2. **Async Canonical Options** - Add 0x06 (async) and 0x07 (callback)
   - Files: canonical.go, component.go
   - Effort: Low

3. **Core Type 0x00 0x50 Prefix** - Handle non-final sub type disambiguation
   - Files: core_type.go
   - Effort: Low

### Priority 2: Feature Completeness (Medium Impact)

4. **Resource Async Destructor (0x3e)** - Parse async resource destructors
   - Files: types.go
   - Effort: Low

5. **Version Suffix Parsing** - Store parsed version suffixes
   - Files: import.go, exports.go, component.go
   - Effort: Low

6. **Value Section Completion** - Full val(T) encoding
   - Files: value.go
   - Effort: Medium

### Priority 3: Async/Threading Canonicals (Low Priority - Gated)

7. **Async Canonicals (0x05-0x25)** - All 🔀 gated operations
   - Files: canonical.go, component.go
   - Effort: High (parsing only; runtime is separate)

8. **Error Context Canonicals (0x1c-0x1e)** - 📝 gated operations
   - Files: canonical.go
   - Effort: Low

9. **Threading Canonicals (0x26-0x42)** - 🧵 gated operations
   - Files: canonical.go
   - Effort: Medium

### Priority 4: Validation & Polish

10. **Basic Validation** - Type constraints, unique names
    - Files: decoder.go (or new validator.go)
    - Effort: Medium

11. **Name Section Parsing** - Component debugging support
    - Files: new name.go
    - Effort: Low

---

## Test Cases Required

### Existing Test Coverage
- ✅ `calculator_test.go` - add/subtract plugins working

### New Tests Needed

1. **Index space tracking:**
   - Test alias creates correct indices for all sort types
   - Test imports increment correct spaces

2. **Type definitions:**
   - Test async resource destructor (0x3e)
   - Test fixed-size list with length validation
   - Test stream/future with and without payload types

3. **Canonical definitions:**
   - Test async/callback options
   - Test each new canonical opcode (when implemented)

4. **Value section:**
   - Test all primitive value encodings
   - Test composite value encodings

5. **Validation:**
   - Test rejection of empty record/variant/tuple
   - Test rejection of borrow in results
   - Test outer alias sort restrictions

---

## References

- **Spec:** debug-vendored/component-model/design/mvp/Binary.md
- **Explainer:** debug-vendored/component-model/design/mvp/Explainer.md
- **wasmparser (Rust):** debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/
- **wasmtime:** debug-vendored/wasmtime/crates/environ/src/component/
