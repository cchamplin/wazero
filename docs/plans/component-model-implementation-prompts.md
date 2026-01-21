# Component Model Implementation Planning Prompts

These prompts are designed to kick off brainstorming and planning sessions to perform complete defect and gap analysis for each major aspect of the WebAssembly Component Model specification. Each session should result in a comprehensive plan to complete or correct the relevant implementation.

**CRITICAL REQUIREMENT FOR ALL SESSIONS:** The existing working test cases in `internal/component/wasip2test/calculator_test.go` (specifically the `add` and `subtract` plugins) **MUST continue to pass** after each major phase of work. These serve as regression tests to ensure we don't break existing functionality while adding new features.

---

## Reference Implementation Guidance

**IMPORTANT:** To accelerate development and ensure correctness, you should actively reference the **wasmtime** reference implementation and **wasm-tools** crates when working on any of these prompts. These are available in the `debug-vendored/` directory:

### Wasmtime Reference Implementation (`debug-vendored/wasmtime/`)

| Directory | Purpose |
|-----------|---------|
| `crates/wasmtime/src/runtime/component/` | Main component model runtime - **start here for runtime behavior** |
| `crates/wasmtime/src/runtime/vm/component/` | Low-level VM component implementation |
| `crates/environ/src/component/` | Component model compilation/environment |
| `crates/wasi/` | WASI Preview 2 implementation - **primary reference for WASI interfaces** |
| `crates/wasi-http/` | WASI HTTP implementation |
| `crates/wasi-io/` | WASI I/O streams implementation |
| `crates/component-util/` | Shared component utilities |
| `tests/all/component_model/` | **Component model test suite - excellent for understanding expected behavior** |
| `tests/misc_testsuite/component-model/` | Additional component model tests |

### Wasm-Tools Reference (`debug-vendored/wasm-tools/`)

| Directory | Purpose |
|-----------|---------|
| `crates/wasmparser/` | Binary format parsing - **reference for binary decoding** |
| `crates/wit-parser/` | WIT file parsing |
| `crates/wit-component/` | WIT to component conversion |
| `crates/wasm-encoder/` | Binary encoding (useful for understanding format) |

### How to Use These References

1. **For understanding spec behavior:** Look at wasmtime tests first (`tests/all/component_model/`)
2. **For binary format questions:** Reference `wasm-tools/crates/wasmparser/`
3. **For runtime semantics:** Reference `wasmtime/crates/wasmtime/src/runtime/component/`
4. **For WASI implementation:** Reference `wasmtime/crates/wasi/` and related crates
5. **For edge cases:** Search wasmtime issues and test files for similar scenarios

---

## Prompt 1: Binary Format & Parsing Conformance

**Focus Area:** Component binary format decoding, section parsing, and validation

**Objective:** Perform a complete gap analysis of the binary format implementation against the Component Model specification (`debug-vendored/component-model/design/mvp/Binary.md`) and develop a plan to achieve full conformance.

```
I need to perform a comprehensive defect and gap analysis for the wazero component model binary format parser against the official Component Model specification.

**Reference Materials:**
- Primary spec: debug-vendored/component-model/design/mvp/Binary.md
- Supporting spec: debug-vendored/component-model/design/mvp/Explainer.md (Index Spaces section)
- Current implementation: internal/component/binary/

**Wasmtime/Wasm-Tools References (USE THESE TO ACCELERATE DEVELOPMENT):**
- `debug-vendored/wasm-tools/crates/wasmparser/src/readers/component/` - Binary parsing implementation
- `debug-vendored/wasm-tools/crates/wasmparser/src/validator/component/` - Validation logic
- `debug-vendored/wasmtime/crates/environ/src/component/` - Component environment/translation
- `debug-vendored/wasmtime/tests/misc_testsuite/component-model/` - Test WAT files showing expected parsing

When implementing or fixing features, cross-reference how wasmparser handles the same binary structures. The Rust code is well-documented and serves as the reference implementation.

**Analysis Areas:**

1. **Component Preamble & Sections**
   - Verify magic bytes, version (0x0d 0x00), and layer (0x01 0x00) parsing
   - Verify all 13 section types are correctly parsed:
     - Section 0: core:custom
     - Section 1: core:module
     - Section 2: vec(core:instance)
     - Section 3: vec(core:type)
     - Section 4: component
     - Section 5: vec(instance)
     - Section 6: vec(alias)
     - Section 7: vec(type)
     - Section 8: vec(canon)
     - Section 9: start
     - Section 10: vec(import)
     - Section 11: vec(export)
     - Section 12: vec(value) [gated 🪙]

2. **Index Spaces**
   - Verify all 12 index spaces are maintained correctly:
     - Component-level: functions, values, types, component instances, components
     - Core-level (wasm 1.0): functions, tables, memories, globals, types
     - Core-level (extended): module instances, modules

3. **Instance Definitions**
   - core:instance and core:instanceexpr parsing
   - instance and instanceexpr parsing
   - instantiatearg validation

4. **Alias Definitions**
   - Export aliases (component and core)
   - Outer aliases with proper scope validation
   - Sort restrictions for outer aliases

5. **Type Definitions**
   - All value types (primitives, composites, handles, async types)
   - Function types (params, results)
   - Component types and instance types
   - Resource types

6. **Canonical Definitions**
   - canon lift with all options (memory, realloc, post-return, string-encoding, async, callback)
   - canon lower with all options
   - Resource builtins (resource.new, resource.drop, resource.rep)

7. **Import/Export Definitions**
   - Import name formats (plain, interface, depname, hashname, locked-dep, unlocked-dep)
   - Export name validation
   - Type externalization

**Deliverables:**
- Gap analysis document listing missing/incorrect features
- Prioritized implementation plan
- Test cases for each new/fixed feature

**Regression Requirement:**
All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests for `add` and `subtract` plugins continue to pass.
```

---

## Prompt 2: Canonical ABI - Type Lifting & Lowering

**Focus Area:** Value type representation, alignment, loading/storing, and flat representations

**Objective:** Perform a complete gap analysis of the Canonical ABI type lifting and lowering implementation against the specification (`debug-vendored/component-model/design/mvp/CanonicalABI.md`) and develop a plan to achieve full conformance.

```
I need to perform a comprehensive defect and gap analysis for the wazero Canonical ABI type lifting and lowering implementation against the official Component Model specification.

**Reference Materials:**
- Primary spec: debug-vendored/component-model/design/mvp/CanonicalABI.md
- Sections: "Supporting definitions" through "Flat Lowering"
- Current implementation: internal/component/abi/

**Wasmtime/Wasm-Tools References (USE THESE TO ACCELERATE DEVELOPMENT):**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/values.rs` - Value lifting/lowering
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/types/` - Type definitions
- `debug-vendored/wasmtime/crates/component-util/src/` - Canonical ABI utilities (alignment, sizes)
- `debug-vendored/wasmtime/crates/environ/src/component/types.rs` - Type representation
- `debug-vendored/wasmtime/tests/all/component_model/` - Runtime tests for lifting/lowering
- `debug-vendored/component-model/design/mvp/canonical-abi/` - Reference Python implementation

The wasmtime `component-util` crate contains pure functions for alignment and size calculations that can be directly compared. The Python reference implementation in canonical-abi/ is executable and can be used to verify expected behavior.

**Analysis Areas:**

1. **Type Despecialization**
   - Tuple → Record conversion
   - Enum → Variant conversion
   - Option → Variant conversion
   - Result → Variant conversion

2. **Alignment Calculations**
   - Primitive type alignments (bool=1, s8/u8=1, s16/u16=2, s32/u32=4, s64/u64=8, f32=4, f64=8, char=4)
   - String alignment (4)
   - List alignment (fixed-length vs dynamic)
   - Record/tuple alignment (max of field alignments)
   - Variant alignment (max of discriminant + case alignments)
   - Flags alignment (1/2/4 based on count)
   - Handle alignments (own, borrow = 4)
   - Async type alignments (stream, future = 4)

3. **Element Size Calculations**
   - All primitive sizes
   - String size (8 = ptr + len)
   - List size (fixed vs dynamic)
   - Record field layout with padding
   - Variant discriminant sizing and case alignment
   - Flags packing

4. **Loading from Memory**
   - Integer loading (signed/unsigned, all sizes)
   - Boolean conversion (0=false, nonzero=true)
   - Float loading with NaN canonicalization
   - Character validation (valid Unicode scalar)
   - String loading with encoding detection (UTF-8, UTF-16, Latin-1)
   - List loading (fixed and dynamic)
   - Record field loading with alignment
   - Variant discriminant + payload loading
   - Flags bit unpacking

5. **Storing to Memory**
   - All primitive stores
   - String storing with proper encoding
   - List storing
   - Record storing
   - Variant storing
   - Flags packing

6. **Flat Representation**
   - Flattening rules for each type
   - MAX_FLAT_PARAMS (16) and MAX_FLAT_RESULTS (1) limits
   - Core type mapping (i32, i64, f32, f64)
   - Handling of types that exceed flat limits (spill to memory)

7. **String Encoding**
   - UTF-8 encoding/decoding
   - UTF-16 (little-endian) support
   - Latin-1 support
   - Tagged length format for encoding detection on lift

8. **Edge Cases**
   - Empty records, empty variants
   - Deeply nested types
   - Large lists
   - Unicode edge cases (surrogate pairs, invalid sequences)

**Deliverables:**
- Gap analysis document with specific spec deviations
- Implementation plan for each missing/incorrect feature
- Conformance test suite additions

**Regression Requirement:**
All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests for `add` and `subtract` plugins continue to pass.
```

---

## Prompt 3: Resource System & Handle Management

**Focus Area:** Resource types (own/borrow), resource tables, lifecycle management, and canonical builtins

**Objective:** Perform a complete gap analysis of the resource handling implementation against the specification and develop a plan for full conformance.

```
I need to perform a comprehensive defect and gap analysis for the wazero component model resource system against the official Component Model specification.

**Reference Materials:**
- Primary spec: debug-vendored/component-model/design/mvp/CanonicalABI.md
- Sections: "Resource State", "Resource built-ins"
- Explainer sections on resource types
- Current implementation: internal/component/resource_table.go, internal/component/borrow_scope.go

**Wasmtime/Wasm-Tools References (USE THESE TO ACCELERATE DEVELOPMENT):**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/resources.rs` - Resource table implementation
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs` - Low-level resource handling
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/types.rs` - Resource type definitions
- `debug-vendored/wasmtime/tests/all/component_model/resources.rs` - **Comprehensive resource tests**
- `debug-vendored/wasmtime/crates/environ/src/component/types.rs` - Resource type validation

The wasmtime resource tests (`tests/all/component_model/resources.rs`) are particularly valuable as they cover edge cases like borrow scopes, drop semantics, and cross-component resource handling.

**Analysis Areas:**

1. **Resource Handle Types**
   - OwnType - represents ownership transfer
   - BorrowType - represents temporary access
   - Handle representation as i32 indices

2. **Resource Table Structure**
   - Table stored per ComponentInstance
   - ResourceHandle structure:
     - rep (i32): internal representation
     - own (bool): ownership flag
     - scope (Optional[Task|Subtask]): for borrows
     - num_lends (int): borrow count tracking
   - Handle allocation and deallocation
   - Invalid handle detection

3. **Resource Lifting**
   - lift_own: transfer ownership from core wasm
   - lift_borrow: create temporary borrow
   - Validation of handle validity
   - Scope tracking for borrows

4. **Resource Lowering**
   - lower_own: transfer ownership to core wasm
   - lower_borrow: provide borrow handle
   - Same-component optimization (return rep directly)
   - Cross-component handle creation

5. **Canonical Builtins**
   - canon resource.new: create new resource
   - canon resource.drop: destroy resource with optional destructor call
   - canon resource.rep: get internal representation
   - Validation requirements for each

6. **Borrow Scope Management**
   - BorrowScope tracking during function calls
   - Borrow validity duration (until call returns)
   - num_lends counter for active borrows
   - Prevention of drop while borrowed

7. **Resource Lifecycle**
   - Creation via resource.new or import
   - Transfer via own handles
   - Temporary access via borrow handles
   - Destruction via resource.drop
   - Destructor invocation

8. **Error Conditions**
   - Invalid handle indices
   - Double-drop
   - Drop while borrowed
   - Use after drop
   - Scope violations for borrows

**Deliverables:**
- Complete gap analysis of resource handling
- Implementation plan for missing features
- Resource lifecycle test suite
- Edge case and error condition tests

**Regression Requirement:**
All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests for `add` and `subtract` plugins continue to pass.
```

---

## Prompt 4: Component Instantiation & Linking

**Focus Area:** Component/module instantiation, import resolution, type matching, and export handling

**Objective:** Perform a complete gap analysis of the instantiation and linking implementation against the specification and develop a plan for full conformance.

```
I need to perform a comprehensive defect and gap analysis for the wazero component model instantiation and linking system against the official Component Model specification.

**Reference Materials:**
- Primary specs:
  - debug-vendored/component-model/design/mvp/Explainer.md (Instance definitions, Import/Export)
  - debug-vendored/component-model/design/mvp/Binary.md (Instance sections)
  - debug-vendored/component-model/design/mvp/Linking.md
- Current implementation: internal/component/linker*.go, internal/component/instantiate*.go

**Wasmtime/Wasm-Tools References (USE THESE TO ACCELERATE DEVELOPMENT):**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/linker.rs` - Component linker implementation
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/instance.rs` - Instance management
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/matching.rs` - Type matching/subtyping
- `debug-vendored/wasmtime/crates/environ/src/component/translate/` - Component translation
- `debug-vendored/wasmtime/tests/all/component_model/import.rs` - Import resolution tests
- `debug-vendored/wasmtime/tests/all/component_model/linking.rs` - Linking tests
- `debug-vendored/wasm-tools/crates/wit-component/src/` - WIT to component linking

Pay special attention to wasmtime's `matching.rs` for type subtyping rules and `linker.rs` for import resolution. The translation layer in `environ/src/component/translate/` shows how components are processed during compilation.

**Analysis Areas:**

1. **Core Module Instantiation**
   - Module binary embedding in components
   - Core import satisfaction from component scope
   - Core instance creation with instantiate arguments
   - Core export access

2. **Component Instantiation**
   - Nested component instantiation
   - Import argument binding (with n si)
   - Type substitution for type imports
   - Component instance state initialization

3. **Import Resolution**
   - Plain name imports
   - Interface imports (wasi:io/streams@0.2.0)
   - Dependency imports (locked-dep, unlocked-dep, hashname)
   - Semver matching (relaxed mode option)
   - Import name normalization

4. **Type Matching & Subtyping**
   - Function type matching (params contravariant, results covariant)
   - Record subtyping (width subtyping with reordering)
   - Variant subtyping (refinement)
   - Resource type equality
   - Instance type matching

5. **Export Handling**
   - Export name validation
   - Export externalization
   - Sort-specific export rules
   - Kebab-case to function name mapping

6. **Alias Resolution**
   - Instance export aliases (component and core)
   - Outer aliases for types, modules, components
   - Alias chain resolution
   - Scope validation for outer aliases

7. **Inline Exports**
   - Creating instances from inline exports
   - Bundling multiple exports

8. **Component Instance State**
   - Resource table initialization
   - Memory and realloc function references
   - may_leave flag management
   - Parent-child component relationships

9. **Start Function**
   - Component start definition handling
   - Start function invocation timing
   - Return value handling

10. **Error Handling**
    - Missing import errors
    - Type mismatch errors
    - Invalid alias targets
    - Instantiation failures

**Deliverables:**
- Gap analysis of instantiation/linking
- Implementation plan with dependency ordering
- Multi-component test scenarios
- Import/export type matching tests

**Regression Requirement:**
All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests for `add` and `subtract` plugins continue to pass.
```

---

## Prompt 5: WASI Preview 2 Interfaces Implementation

**Focus Area:** Complete WASI P2 interface implementations for all standard interfaces

**Objective:** Perform a complete gap analysis of WASI P2 interface implementations and develop a plan to achieve full conformance.

```
I need to perform a comprehensive defect and gap analysis for the wazero WASI Preview 2 interface implementations against the official WASI specifications.

**Reference Materials:**
- WASI specs: debug-vendored/WASI/proposals/
- WIT definitions in each proposal's wit/ directory
- Current implementation: imports/wasip2/

**Wasmtime/Wasm-Tools References (USE THESE TO ACCELERATE DEVELOPMENT):**
- `debug-vendored/wasmtime/crates/wasi/src/` - **Primary WASI P2 implementation reference**
  - `host/io/` - Streams, poll, error implementations
  - `host/clocks/` - Clock implementations
  - `host/random.rs` - Random implementation
  - `host/filesystem/` - Filesystem implementation
  - `host/network/` - Network/socket implementations
  - `host/tcp.rs`, `host/udp.rs` - Socket implementations
- `debug-vendored/wasmtime/crates/wasi-io/src/` - Dedicated I/O streams crate
- `debug-vendored/wasmtime/crates/wasi-http/src/` - HTTP implementation
- `debug-vendored/wasmtime/crates/wasi-preview1-component-adapter/` - P1 to P2 adapter
- `debug-vendored/wasmtime/tests/all/wasi.rs` - WASI integration tests

The wasmtime WASI implementation is the canonical reference. Each interface in `crates/wasi/src/host/` maps directly to a WASI interface. Pay attention to how resources are managed (streams, file descriptors, sockets) and how pollables integrate with the event loop.

**Analysis Areas:**

1. **wasi:io (Core I/O)**
   - wasi:io/error - error type and last-operation-failed
   - wasi:io/poll - pollable resource, poll function, poll-one
   - wasi:io/streams - input-stream, output-stream resources
     - read, blocking-read, skip, subscribe methods
     - write, blocking-write, flush, subscribe methods
     - splice operations

2. **wasi:clocks**
   - wasi:clocks/monotonic-clock
     - now, resolution, subscribe-instant, subscribe-duration
   - wasi:clocks/wall-clock
     - now, resolution
   - Pollable integration for clock subscriptions

3. **wasi:random**
   - wasi:random/random - get-random-bytes, get-random-u64
   - wasi:random/insecure - get-insecure-random-bytes, get-insecure-random-u64
   - wasi:random/insecure-seed - insecure-seed

4. **wasi:filesystem**
   - wasi:filesystem/types - descriptor resource with all methods
     - read-via-stream, write-via-stream
     - append-via-stream
     - advise, sync-data, get-flags, get-type
     - set-size, set-times
     - read, write at offsets
     - read-directory, stat, stat-at
     - open-at, create-directory-at
     - metadata operations
   - wasi:filesystem/preopens - get-directories

5. **wasi:sockets**
   - wasi:sockets/network - network resource, IP types
   - wasi:sockets/tcp - tcp-socket resource
     - start-bind, finish-bind
     - start-connect, finish-connect
     - start-listen, finish-listen
     - accept
     - shutdown
     - socket options
   - wasi:sockets/udp - udp-socket resource
     - start-bind, finish-bind
     - stream, subscribe
     - send, receive datagrams
   - wasi:sockets/ip-name-lookup - resolve-addresses
   - Socket creation functions

6. **wasi:cli**
   - wasi:cli/environment - get-environment, get-arguments, initial-cwd
   - wasi:cli/exit - exit function
   - wasi:cli/stdin, stdout, stderr - get-stdin, get-stdout, get-stderr
   - wasi:cli/terminal-input, terminal-output - terminal resources
   - wasi:cli/terminal-stdin, terminal-stdout, terminal-stderr

7. **wasi:http (if applicable)**
   - wasi:http/types - request, response, headers, body types
   - wasi:http/outgoing-handler - handle function
   - wasi:http/incoming-handler - handle function

8. **Resource Lifecycle for WASI Resources**
   - Proper drop implementation for all resources
   - Stream/pollable lifecycle management
   - File descriptor management

9. **Error Handling**
   - WASI error codes mapping
   - Error context propagation
   - Platform-specific error translation

10. **Platform Compatibility**
    - Cross-platform behavior consistency
    - Platform-specific limitations documentation

**Deliverables:**
- Interface-by-interface gap analysis

**Regression Requirement:**
All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests for `add` and `subtract` plugins continue to pass.
```

---

## Prompt 6: Canon Lift/Lower, Post-Return & Memory Management

**Focus Area:** canon lift, canon lower, post-return cleanup, realloc handling, and memory management

**Objective:** Perform a complete gap analysis of canonical function handling and memory management against the specification.

```
I need to perform a comprehensive defect and gap analysis for the wazero component model canon lift/lower implementation and memory management against the official Component Model specification.

**Reference Materials:**
- Primary spec: debug-vendored/component-model/design/mvp/CanonicalABI.md
- Sections: "canon lift", "canon lower", canonopt handling
- Current implementation: internal/component/abi/, linker code

**Wasmtime/Wasm-Tools References (USE THESE TO ACCELERATE DEVELOPMENT):**
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/` - canon lift/lower implementation
  - `host.rs` - Host function handling
  - `typed.rs` - Typed function wrappers
  - `options.rs` - Canonical options handling
- `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/` - Low-level VM integration
- `debug-vendored/wasmtime/crates/environ/src/component/dfg.rs` - Data flow graph for canonicals
- `debug-vendored/wasmtime/crates/environ/src/component/translate/adapt.rs` - Adapter generation
- `debug-vendored/wasmtime/tests/all/component_model/func.rs` - Function call tests
- `debug-vendored/wasmtime/tests/all/component_model/post_return.rs` - Post-return tests
- `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` - Reference Python for canon operations

The wasmtime `func/` directory contains the core lifting/lowering trampolines. The `environ/` crate shows how canonical functions are compiled. The Python reference (`definitions.py`) is executable and useful for understanding the exact semantics.

**Analysis Areas:**

1. **Canonical Options (canonopt)**
   - memory: linear memory reference for lifting/lowering
   - realloc: memory allocation function
   - post-return: cleanup function after results lifted
   - string-encoding: utf8 (default), utf16, latin1+utf16
   - async: enable async execution (🔀 gated)
   - callback: async callback function (🔀 gated)

2. **Canon Lift Implementation**
   - Wrapping core wasm function as component function
   - Flat calling convention handling
   - Result spilling when exceeding MAX_FLAT_RESULTS
   - Automatic memory allocation for results
   - Post-return invocation
   - Borrow scope creation and management

3. **Canon Lower Implementation**
   - Creating core wasm function from component function
   - Parameter flattening
   - Result handling
   - Cross-component calls
   - Same-component optimizations

4. **Realloc Handling**
   - realloc(ptr, old_size, align, new_size) signature
   - Allocation (ptr=0, old_size=0)
   - Deallocation (new_size=0)
   - Reallocation (resizing)
   - Out-of-memory trap handling
   - Alignment requirements

5. **Post-Return Cleanup**
   - When post-return is required
   - Parameter passing to post-return
   - Timing relative to result lifting
   - Resource cleanup

6. **Memory Layout for Spilled Values**
   - Return pointer convention
   - Alignment for aggregate returns
   - Multi-value return layout

7. **Calling Convention**
   - MAX_FLAT_PARAMS (16) handling
   - MAX_FLAT_RESULTS (1) handling
   - Parameter/result pointer passing
   - Return area allocation

8. **LiftLowerContext Management**
   - Context creation per call
   - opts, inst, borrow_scope fields
   - Context propagation through nested calls

9. **May-Leave Flag**
   - Setting may_leave = false during guest execution
   - Trap on reentrance attempts
   - Proper restoration after calls

10. **Memory Bounds Checking**
    - All pointer dereferences validated
    - Alignment checks
    - Buffer overflow prevention

11. **Call Stack Management**
    - Nested component calls
    - Host-to-component calls
    - Component-to-host calls
    - Recursion detection (call_might_be_recursive)

**Deliverables:**
- Gap analysis of canon lift/lower
- Memory management audit
- Post-return test coverage
- Realloc edge case tests cases
- Call stack depth tests cases

**Regression Requirement:**
All changes MUST ensure `internal/component/wasip2test/calculator_test.go` tests for `add` and `subtract` plugins continue to pass.
```

---

## Usage Notes

1. **Session Order:** These prompts can be tackled in any order, but there are natural dependencies:
   - Prompt 1 (Binary Format) should likely come first as it's foundational
   - Prompt 2 (Type Lifting/Lowering) builds on binary parsing
   - Prompt 3 (Resources) can be done independently
   - Prompt 4 (Instantiation) depends on 1, 2, and partially 3
   - Prompt 5 (WASI P2) depends on 2 and 3
   - Prompt 6 (Canon/Memory) ties everything together

2. **Verification:** After each session's implementation phase:
   - Run `go test ./internal/component/wasip2test/...` to verify regression tests pass
   - Run the full component test suite
   - Consider adding new test plugins that exercise the new functionality

3. **Reference Materials:** Each prompt references specific sections of the spec. The full specs are available in:
   - `debug-vendored/component-model/` - Component Model spec
   - `debug-vendored/WASI/` - WASI specifications
   - `debug-vendored/wasmtime/` - Reference implementation
   - `debug-vendored/wasm-tools/` - Tooling reference

4. **Tooling Available:**
   - Rust toolchain with cargo-component
   - TinyGo with WASI P2 support
   - wasm-tools CLI for component manipulation
   - wit-bindgen for code generation

---

## Best Practices for Using Reference Implementations

### When to Reference Wasmtime/Wasm-Tools

1. **Before implementing any feature:** Check how wasmtime handles it first
2. **When debugging unexpected behavior:** Compare against wasmtime's tests
3. **For edge cases:** Search wasmtime's test suite for similar scenarios
4. **For error messages:** Match wasmtime's error handling patterns where appropriate

### Effective Reference Patterns

```
# Find how wasmtime handles a specific feature
grep -r "pattern" debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/

# Find relevant tests
grep -r "test_name" debug-vendored/wasmtime/tests/all/component_model/

# Search wasm-tools for binary format handling
grep -r "section_type" debug-vendored/wasm-tools/crates/wasmparser/
```

### Key Files by Topic

| Topic | Primary Wasmtime File | Primary Wasm-Tools File |
|-------|----------------------|------------------------|
| Binary Parsing | - | `wasm-tools/crates/wasmparser/src/readers/component/` |
| Type System | `wasmtime/.../component/types.rs` | `wasm-tools/crates/wasmparser/src/validator/component/` |
| Lifting/Lowering | `wasmtime/.../component/values.rs` | - |
| Resources | `wasmtime/.../component/resources.rs` | - |
| Linker | `wasmtime/.../component/linker.rs` | - |
| Instantiation | `wasmtime/.../component/instance.rs` | - |
| WASI Interfaces | `wasmtime/crates/wasi/src/host/` | - |

### Translation Tips (Rust → Go)

- Rust's `Result<T, E>` → Go's `(T, error)` return pattern
- Rust's `Option<T>` → Go's pointer types or explicit ok bool
- Rust's traits → Go's interfaces
- Rust's `match` → Go's `switch` with type assertions
- Rust's lifetimes → Go's GC handles this, but watch for resource leaks

### Verifying Against Wasmtime

When implementing a feature, consider:
1. Writing a small WAT test component
2. Running it through both wasmtime and wazero
3. Comparing outputs/behavior
4. Using `wasm-tools print` to inspect component structure
