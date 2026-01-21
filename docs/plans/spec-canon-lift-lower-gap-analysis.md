# Canon Lift/Lower Gap Analysis

This document provides a comprehensive gap analysis of wazero's Component Model canon lift/lower implementation against the official Component Model specification.

## Reference Materials
- **Primary Spec**: `debug-vendored/component-model/design/mvp/CanonicalABI.md`
- **Reference Python**: `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`
- **Wasmtime Reference**: `debug-vendored/wasmtime/crates/wasmtime/src/runtime/component/func/`

## Executive Summary

The wazero implementation has a solid foundation for canon lift/lower with good coverage of:
- Basic flattening of parameters and results
- String encoding (UTF-8, UTF-16, Latin1+UTF16)
- Memory bounds checking
- Post-return function invocation
- Realloc parameter passing
- Resource handle lifting/lowering

However, there are **critical gaps** that prevent full spec compliance:

| Category | Severity | Gap Description |
|----------|----------|-----------------|
| may_leave Flag | **CRITICAL** | Not implemented - allows reentrance during lowering |
| call_might_be_recursive | **CRITICAL** | Not implemented - no reentrance guard |
| Borrow Scope Tracking | HIGH | Incomplete - lend counting for borrowed handles |
| Result Spilling | HIGH | Limited - only i32 retptr, no tuple layout |
| Async Support | LOW | Not implemented (gated in spec with 🔀) |
| Error Context | LOW | Not implemented (rarely used) |
| Streams/Futures | LOW | Not implemented (async feature) |

---

## 1. Canonical Options (canonopt)

### Spec Requirements
```python
# From CanonicalABI.md
# Options that can appear in canon lift/lower:
- string-encoding: utf8 (default), utf16, latin1+utf16
- memory: linear memory reference
- realloc: allocation function
- post-return: cleanup function (lift only)
- async: enable async (🔀 gated)
- callback: async callback (🔀 gated)
```

### Implementation Status

| Option | Status | Location | Notes |
|--------|--------|----------|-------|
| string-encoding=utf8 | ✅ IMPLEMENTED | `abi/strings.go` | Default encoding |
| string-encoding=utf16 | ✅ IMPLEMENTED | `abi/strings.go` | Full support |
| string-encoding=latin1+utf16 | ✅ IMPLEMENTED | `abi/strings.go` | Dynamic encoding |
| memory | ✅ IMPLEMENTED | `component_linker.go:1107` | Resolved via memSpace |
| realloc | ✅ IMPLEMENTED | `component_linker.go:1115` | Resolved via funcSpace |
| post-return | ✅ IMPLEMENTED | `component_linker.go:1123` | Invoked in `instance.go:265` |
| async | ❌ NOT IMPLEMENTED | - | Spec gated with 🔀 |
| callback | ❌ NOT IMPLEMENTED | - | Spec gated with 🔀 |

### Gaps

**GAP-OPT-1: Async options not supported**
- **Severity**: LOW (spec gated)
- **Impact**: Cannot use async function calls
- **Spec Reference**: Lines 3180-3185 of CanonicalABI.md
- **Recommended Action**: Defer until async MVP stabilizes

---

## 2. Canon Lift Implementation

### Spec Algorithm (from `canon_lift` function)
```python
def canon_lift(opts, inst, ft, callee, caller, on_start, on_resolve):
    trap_if(call_might_be_recursive(caller, inst))  # REENTRANCE CHECK
    task = Task(opts, inst, ft, caller, on_resolve)
    # ... create thread_func ...
    cx = LiftLowerContext(opts, inst, task)
    args = on_start()
    flat_args = lower_flat_values(cx, MAX_FLAT_PARAMS, args, ft.param_types())
    # ... call callee ...
    result = lift_flat_values(cx, MAX_FLAT_RESULTS, CoreValueIter(flat_results), ft.result_type())
    task.return_(result)
    if opts.post_return is not None:
        inst.may_leave = False
        call opts.post_return(flat_results)
        inst.may_leave = True
    task.exit()
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Resolve core function | ✅ IMPLEMENTED | `component_linker.go:1075` | Uses funcSpace |
| Resolve memory | ✅ IMPLEMENTED | `component_linker.go:1107` | Uses memSpace |
| Resolve realloc | ✅ IMPLEMENTED | `component_linker.go:1115` | Optional |
| Resolve post-return | ✅ IMPLEMENTED | `component_linker.go:1123` | Optional |
| Call core function | ✅ IMPLEMENTED | `instance.go:255` | Direct call |
| Lift flat results | ✅ IMPLEMENTED | `instance.go:273-515` | Complex logic |
| Call post-return | ✅ IMPLEMENTED | `instance.go:265` | After results lifted |
| Reentrance guard | ❌ NOT IMPLEMENTED | - | **CRITICAL** |
| may_leave during post-return | ❌ NOT IMPLEMENTED | - | **CRITICAL** |
| LiftLowerContext/Task | ⚠️ PARTIAL | `abi/context.go` | Split into LiftContext/LowerContext |
| Borrow scope creation | ⚠️ PARTIAL | `instance.go:100` | Basic BorrowScope exists |

### Gaps

**GAP-LIFT-1: No reentrance guard (call_might_be_recursive)**
- **Severity**: CRITICAL
- **Impact**: Allows undefined behavior from recursive calls into the same component
- **Spec Reference**: Line 3238 - `trap_if(call_might_be_recursive(caller, inst))`
- **Current Code**: No check exists
- **Recommended Fix**:
```go
// Add to Instance struct
type Instance struct {
    // ... existing fields ...
    activeCallDepth int32 // Track call depth for reentrance detection
    activeCaller    *Instance // Track caller for recursive check
}

// Add before function call
func (i *Instance) checkReentrance(caller *Instance) error {
    if i.activeCallDepth > 0 && caller == i {
        return fmt.Errorf("trap: recursive call into same component")
    }
    return nil
}
```

**GAP-LIFT-2: may_leave not enforced during post-return**
- **Severity**: CRITICAL
- **Impact**: Post-return can call out of component, violating spec
- **Spec Reference**: Lines 3287-3289
- **Current Code**: Post-return called but may_leave not set
- **Recommended Fix**:
```go
// In Instance struct
mayLeave bool // Initially true

// Before calling post-return
i.mayLeave = false
defer func() { i.mayLeave = true }()
f.postReturnFunc.Call(ctx, coreResults...)
```

**GAP-LIFT-3: MAX_FLAT_PARAMS not enforced**
- **Severity**: HIGH
- **Impact**: Functions with >16 params not handled via memory
- **Spec Reference**: Line 2735 - `MAX_FLAT_PARAMS = 16`
- **Current Code**: `flatten.go` handles flattening but no param spilling
- **Recommended Fix**: Check param count, use realloc for spilled params

---

## 3. Canon Lower Implementation

### Spec Algorithm (from `canon_lower` function)
```python
def canon_lower(opts, ft, callee, thread, flat_args):
    trap_if(not thread.task.inst.may_leave)  # MAY_LEAVE CHECK
    trap_if(not thread.task.may_block() and ft.async_ and not opts.async_)
    subtask = Subtask()
    cx = LiftLowerContext(opts, thread.task.inst, subtask)
    # ... lift params, call, lower results ...
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Lower parameters | ✅ IMPLEMENTED | `abi/lower.go` | Full type support |
| Create trampoline | ⚠️ PARTIAL | `component_linker.go:192` | canonLowers tracked |
| may_leave check | ❌ NOT IMPLEMENTED | - | **CRITICAL** |
| Subtask management | ❌ NOT IMPLEMENTED | - | No Subtask struct |
| Borrow scope per call | ⚠️ PARTIAL | `instance.go:100` | Basic scope |

### Gaps

**GAP-LOWER-1: may_leave not checked**
- **Severity**: CRITICAL
- **Impact**: Lowered calls can occur when they shouldn't (during lowering)
- **Spec Reference**: Line 3454 - `trap_if(not thread.task.inst.may_leave)`
- **Current Code**: No may_leave field or check
- **Recommended Fix**: Add may_leave to Instance, check before all lowered calls

**GAP-LOWER-2: No Subtask tracking**
- **Severity**: HIGH
- **Impact**: Borrow lifetimes not properly tracked across calls
- **Spec Reference**: Lines 3468-3471
- **Current Code**: BorrowScope exists but no Subtask
- **Recommended Fix**: Implement Subtask struct for per-call state

---

## 4. Realloc Handling

### Spec Requirements
```python
# realloc(ptr, old_size, align, new_size) -> new_ptr
# Allocation:   realloc(0, 0, align, size)
# Deallocation: realloc(ptr, size, align, 0)
# Resize:       realloc(ptr, old_size, align, new_size)
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Signature (4 params) | ✅ IMPLEMENTED | `instance.go:209` | Correct signature |
| New allocation | ✅ IMPLEMENTED | `abi/strings.go` | ptr=0, old_size=0 |
| OOM trap | ✅ IMPLEMENTED | `conformance/realloc_failure_test.go` | Returns error |
| Alignment passed | ✅ IMPLEMENTED | `conformance/realloc_failure_test.go:188` | Tested |
| Deallocation | ❌ NOT TESTED | - | new_size=0 case |
| Resize | ❌ NOT TESTED | - | Reallocation case |

### Gaps

**GAP-REALLOC-1: Deallocation not tested**
- **Severity**: MEDIUM
- **Impact**: Memory leaks if dealloc path broken
- **Recommended Action**: Add test for `realloc(ptr, size, align, 0)`

**GAP-REALLOC-2: Resize not tested**
- **Severity**: MEDIUM
- **Impact**: String transcoding may fail
- **Spec Reference**: Lines 2477-2484 (resize during UTF-8 transcoding)
- **Recommended Action**: Add test for resize path

---

## 5. Post-Return Cleanup

### Spec Requirements
```python
if opts.post_return is not None:
    inst.may_leave = False
    [] = call_and_trap_on_throw(opts.post_return, thread, flat_results)
    inst.may_leave = True
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Post-return resolved | ✅ IMPLEMENTED | `component_linker.go:1123` | From canon options |
| Post-return called | ✅ IMPLEMENTED | `instance.go:265` | After main call |
| Receives flat results | ✅ IMPLEMENTED | `instance.go:266` | Correct params |
| may_leave = false | ❌ NOT IMPLEMENTED | - | **GAP-LIFT-2** |
| Trap on exception | ⚠️ PARTIAL | - | Go errors propagate |

### Gaps

See **GAP-LIFT-2** above.

---

## 6. Memory Layout for Spilled Values

### Spec Requirements
```python
MAX_FLAT_RESULTS = 1

# When results exceed MAX_FLAT_RESULTS:
# - lift: results returned via i32 pointer
# - lower: retptr passed as additional param

def flatten_functype(opts, ft, context):
    if len(flat_results) > MAX_FLAT_RESULTS:
        match context:
            case 'lift':
                flat_results = ['i32']  # return pointer
            case 'lower':
                flat_params += ['i32']  # retptr param
                flat_results = []
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| MAX_FLAT_RESULTS = 1 | ✅ IMPLEMENTED | `component_linker.go:18` | Correct constant |
| Detect spilling needed | ✅ IMPLEMENTED | `abi/flatten.go:32` | `needsRetptr` returned |
| Add retptr param (lower) | ✅ IMPLEMENTED | `abi/flatten.go:47` | Appends i32 |
| Return i32 (lift) | ⚠️ PARTIAL | `instance.go:449` | String retptr works |
| Tuple layout in memory | ⚠️ PARTIAL | - | Records work, not all tuples |
| Alignment for spilled | ⚠️ PARTIAL | `abi/lift.go` | Per-element alignment |

### Gaps

**GAP-SPILL-1: Tuple spilling incomplete**
- **Severity**: MEDIUM
- **Impact**: Complex return types may fail
- **Spec Reference**: Lines 3117-3120 (TupleType creation)
- **Current Code**: Records handled, tuples may not be
- **Recommended Fix**: Implement `store(cx, tuple_value, TupleType(ts), ptr)`

---

## 7. Calling Convention Compliance

### Spec Requirements
```python
MAX_FLAT_PARAMS = 16
MAX_FLAT_RESULTS = 1
MAX_FLAT_ASYNC_PARAMS = 4  # 🔀 async

# Parameter flattening
def flatten_functype(opts, ft, context):
    flat_params = flatten_types(ft.param_types())
    if not opts.async_:
        if len(flat_params) > MAX_FLAT_PARAMS:
            flat_params = ['i32']  # pointer to params in memory
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| MAX_FLAT_PARAMS = 16 | ❌ NOT ENFORCED | - | Not checked |
| Param spilling | ❌ NOT IMPLEMENTED | - | No param pointer |
| Result spilling | ✅ IMPLEMENTED | `abi/flatten.go:32` | needsRetptr |
| Variant join | ✅ IMPLEMENTED | `abi/flatten.go:134` | Widest type |
| Record flattening | ✅ IMPLEMENTED | `abi/flatten.go:96` | Sequential |

### Gaps

**GAP-CALL-1: Parameter spilling not implemented**
- **Severity**: HIGH
- **Impact**: Functions with >16 flat params will fail
- **Spec Reference**: Lines 2743-2744
- **Recommended Fix**:
```go
func (f *ExportedFunc) Call(ctx context.Context, params ...Val) ([]Val, error) {
    flatParams := flattenParams(params)
    if len(flatParams) > MAX_FLAT_PARAMS {
        // Allocate memory, store params, pass pointer
        ptr, err := f.allocateParams(ctx, flatParams)
        if err != nil {
            return nil, err
        }
        coreParams = []uint64{uint64(ptr)}
    } else {
        coreParams = flatParams
    }
    // ...
}
```

---

## 8. LiftLowerContext Management

### Spec Definition
```python
class LiftLowerContext:
    opts: CanonicalOptions
    inst: ComponentInstance
    borrow_scope: Task|Subtask

    def __init__(self, opts, inst, borrow_scope):
        self.opts = opts
        self.inst = inst
        self.borrow_scope = borrow_scope
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Opts stored | ✅ IMPLEMENTED | `abi/context.go:20` | In LiftContext/LowerContext |
| Memory access | ✅ IMPLEMENTED | `abi/context.go` | Read/Write methods |
| Borrow scope | ⚠️ PARTIAL | `instance.go:100` | BorrowScope exists |
| Instance reference | ⚠️ PARTIAL | `instance.go:76` | Available but not in context |

### Gaps

**GAP-CTX-1: Split context model**
- **Severity**: MEDIUM
- **Impact**: Code duplication between LiftContext and LowerContext
- **Current Code**: Two separate structs with overlapping fields
- **Recommended Fix**: Consider unified LiftLowerContext as spec defines

**GAP-CTX-2: Borrow scope not in context**
- **Severity**: MEDIUM
- **Impact**: Borrow tracking harder to maintain
- **Current Code**: BorrowScope created separately
- **Recommended Fix**: Add borrow_scope field to contexts

---

## 9. May-Leave Flag

### Spec Requirements
```python
# In lower_flat_values:
cx.inst.may_leave = False
# ... lowering operations ...
cx.inst.may_leave = True

# In canon_lower:
trap_if(not thread.task.inst.may_leave)
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| may_leave field | ❌ NOT IMPLEMENTED | - | No field on Instance |
| Set false during lower | ❌ NOT IMPLEMENTED | - | No logic |
| Check in canon_lower | ❌ NOT IMPLEMENTED | - | No check |
| Set false during post-return | ❌ NOT IMPLEMENTED | - | No logic |

### Gaps

**GAP-LEAVE-1: may_leave flag completely missing**
- **Severity**: CRITICAL
- **Impact**: Components can make calls during lowering, causing undefined behavior
- **Spec Reference**: Lines 3133, 3151, 3287-3289, 3454
- **Recommended Fix**:
```go
// Add to Instance
type Instance struct {
    mayLeave bool // True by default
}

// In lower_flat_values equivalent
func (f *ExportedFunc) lowerParams(ctx context.Context, params []Val) ([]uint64, error) {
    f.instance.mayLeave = false
    defer func() { f.instance.mayLeave = true }()
    // ... lowering logic ...
}

// Before any canon_lower call
func (i *Instance) validateMayLeave() error {
    if !i.mayLeave {
        return fmt.Errorf("trap: cannot call while lowering")
    }
    return nil
}
```

---

## 10. Memory Bounds Checking

### Spec Requirements
```python
def load(cx, ptr, t):
    assert(ptr == align_to(ptr, alignment(t)))
    assert(ptr + elem_size(t) <= len(cx.opts.memory))
    # ... load value ...
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Bounds check on read | ✅ IMPLEMENTED | `abi/context.go:63` | Returns error |
| Bounds check on write | ✅ IMPLEMENTED | `abi/lower.go` | Via memory API |
| Alignment assertion | ⚠️ PARTIAL | - | No explicit check |
| Overflow protection | ✅ IMPLEMENTED | `conformance/memory_bounds_test.go:211` | Tested |
| List bounds | ✅ IMPLEMENTED | `abi/lift.go:271` | ptr+len*elemSize |
| String bounds | ✅ IMPLEMENTED | `abi/strings.go` | encoding-aware |

### Gaps

**GAP-BOUNDS-1: Alignment not asserted**
- **Severity**: LOW
- **Impact**: Unaligned reads may be slower or incorrect
- **Spec Reference**: Line 1995 - `assert(ptr == align_to(ptr, alignment(t)))`
- **Current Code**: Reads proceed without alignment check
- **Recommended Fix**: Add alignment validation in load/store operations

---

## 11. Call Stack Management

### Spec Requirements
```python
def call_might_be_recursive(caller, inst):
    # Returns true if this call might be recursive reentrance
    # Must trap if true

# Called at start of canon_lift:
trap_if(call_might_be_recursive(caller, inst))
```

### Implementation Status

| Aspect | Status | Location | Notes |
|--------|--------|----------|-------|
| Reentrance detection | ❌ NOT IMPLEMENTED | - | No tracking |
| Caller tracking | ❌ NOT IMPLEMENTED | - | No caller field |
| Call depth tracking | ❌ NOT IMPLEMENTED | - | No depth counter |

### Gaps

**GAP-STACK-1: No reentrance detection**
- **Severity**: CRITICAL
- **Impact**: Recursive calls allowed, causing stack overflow or corruption
- **Spec Reference**: Line 3238
- **Recommended Fix**: See GAP-LIFT-1

---

## Test Coverage Recommendations

### Existing Test Coverage (Good)
- `conformance/realloc_failure_test.go` - Realloc error handling
- `conformance/memory_bounds_test.go` - Memory bounds checking
- `conformance/strings_test.go` - String encoding
- `abi/lift_test.go` - Type lifting
- `abi/lower_test.go` - Type lowering

### Recommended New Tests

1. **Reentrance Test**
```go
func TestReentranceTrapped(t *testing.T) {
    // Call function that calls back into same component
    // Verify trap occurs
}
```

2. **may_leave Test**
```go
func TestMayLeaveDuringLowering(t *testing.T) {
    // Call function with params that trigger lowering
    // During lowering, attempt another call
    // Verify trap occurs
}
```

3. **Post-Return may_leave Test**
```go
func TestPostReturnCannotCallOut(t *testing.T) {
    // Function with post-return that tries to call import
    // Verify trap during post-return
}
```

4. **Param Spilling Test**
```go
func TestParamSpillingOver16(t *testing.T) {
    // Function with >16 flat params
    // Verify params passed via memory pointer
}
```

5. **Realloc Deallocation Test**
```go
func TestReallocDeallocation(t *testing.T) {
    // Verify realloc(ptr, size, align, 0) called for cleanup
}
```

---

## Priority Matrix

| Gap ID | Severity | Effort | Priority |
|--------|----------|--------|----------|
| GAP-LEAVE-1 | CRITICAL | Medium | P0 |
| GAP-LIFT-1 | CRITICAL | Medium | P0 |
| GAP-LIFT-2 | CRITICAL | Low | P0 |
| GAP-LOWER-1 | CRITICAL | Low | P0 |
| GAP-STACK-1 | CRITICAL | Medium | P0 |
| GAP-CALL-1 | HIGH | High | P1 |
| GAP-LOWER-2 | HIGH | Medium | P1 |
| GAP-LIFT-3 | HIGH | Medium | P1 |
| GAP-CTX-1 | MEDIUM | Medium | P2 |
| GAP-CTX-2 | MEDIUM | Low | P2 |
| GAP-SPILL-1 | MEDIUM | Medium | P2 |
| GAP-REALLOC-1 | MEDIUM | Low | P2 |
| GAP-REALLOC-2 | MEDIUM | Low | P2 |
| GAP-BOUNDS-1 | LOW | Low | P3 |
| GAP-OPT-1 | LOW | High | P4 |

---

## Regression Testing

All changes MUST ensure these tests continue to pass:
```
internal/component/wasip2test/calculator_test.go
- TestAdd (add plugin)
- TestSubtract (subtract plugin)
```

Run with:
```bash
go test -v ./internal/component/wasip2test/... -run "TestAdd|TestSubtract"
```
