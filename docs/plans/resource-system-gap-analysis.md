# Component Model Resource System Gap Analysis

## Executive Summary

This document provides a comprehensive defect and gap analysis of the wazero Component Model resource system compared to the official Component Model specification and wasmtime reference implementation.

**Critical Finding**: The current implementation provides basic resource table functionality but lacks several critical features required for spec compliance, particularly around type validation, destructor handling, and Task integration.

## Reference Materials

- **Primary Spec**: `debug-vendored/component-model/design/mvp/CanonicalABI.md`
  - Sections: "Resource State", "Resource built-ins" (lines 493-550, 3590-3688)
- **Wasmtime Reference**:
  - `debug-vendored/wasmtime/crates/wasmtime/src/runtime/vm/component/resources.rs`
  - `debug-vendored/wasmtime/tests/all/component_model/resources.rs` (comprehensive test suite)

---

## 1. Resource Handle Types Analysis

### Specification Requirements

From `CanonicalABI.md` (lines 497-511):
```python
class ResourceHandle:
  rt: ResourceType           # Reference to resource type
  rep: int                   # Internal representation (i32)
  own: bool                  # Ownership flag
  borrow_scope: Optional[Task]  # Scope that created this borrow
  num_lends: int             # Number of active borrows from this handle
```

### Current Wazero Implementation

From `internal/component/resource_table.go`:
```go
type HandleEntry struct {
    Rep         any    // The resource representation value
    Own         bool   // True if this is an owning handle
    NumLends    uint32 // Number of active borrows from this handle
    BorrowScope any    // The scope that created this borrow
}
```

### GAP 1.1: Missing ResourceType Reference (CRITICAL)

**Status**: NOT IMPLEMENTED

**Issue**: `HandleEntry` has no `rt` field to track which `ResourceType` the handle belongs to.

**Impact**: Cannot validate that operations use the correct resource type. The spec requires:
- `lift_own`: Trap if `h.rt is not t.rt`
- `lift_borrow`: Trap if `h.rt is not t.rt`
- `resource.drop`: Trap if `h.rt is not rt`
- `resource.rep`: Trap if `h.rt is not rt`

**Wasmtime Reference** (`resources.rs` line 79-102):
```rust
pub enum TypedResource {
    Host(u32),
    Component { rep: u32, ty: TypeResourceTableIndex },
}
```

### GAP 1.2: BorrowScope Type Ambiguity

**Status**: PARTIAL

**Issue**: `BorrowScope` is typed as `any` instead of the spec-required `Optional[Task]`.

**Impact**: Cannot properly integrate with Task lifecycle for borrow validation.

---

## 2. ResourceType Definition Analysis

### Specification Requirements

From `CanonicalABI.md` (lines 537-549):
```python
class ResourceType(Type):
  impl: ComponentInstance     # The component instance that defines this type
  dtor: Optional[Callable]    # Destructor function
  dtor_async: bool            # Whether destructor is async
  dtor_callback: Optional[Callable]  # Callback for async destructor
```

### Current Wazero Implementation

From `internal/component/types/resource.go`:
```go
type ResourceType struct {
    Destructor *uint32  // Index of the destructor function
}
```

### GAP 2.1: Missing Implementation Instance Reference (CRITICAL)

**Status**: NOT IMPLEMENTED

**Issue**: No `impl` field to track which ComponentInstance defined this resource.

**Impact**:
- Cannot implement the `lower_borrow` optimization (return rep directly when lowering to defining instance)
- Cannot properly route destructor calls
- Cannot implement `call_might_be_recursive` check

### GAP 2.2: Missing Async Destructor Support

**Status**: NOT IMPLEMENTED

**Issue**: No `dtor_async` or `dtor_callback` fields.

**Impact**: Cannot support async resource destructors required by the spec.

---

## 3. Canonical Resource Operations Analysis

### 3.1 `canon resource.new`

**Spec** (`CanonicalABI.md` lines 3604-3609):
```python
def canon_resource_new(rt, thread, rep):
  trap_if(not thread.task.inst.may_leave)
  h = ResourceHandle(rt, rep, own = True)
  i = thread.task.inst.table.add(h)
  return [i]
```

**Current Implementation** (`resource_table.go` lines 215-220):
```go
func (t *ResourceTable) CreateResourceNewFunc(resourceTypeIdx uint32) func(rep uint32) uint32 {
    return func(rep uint32) uint32 {
        handle := t.New(rep, true)
        return uint32(handle)
    }
}
```

### GAP 3.1.1: Missing may_leave Check

**Status**: NOT IMPLEMENTED

**Issue**: No check for `inst.may_leave` before creating resource.

**Impact**: Can create resources during invalid execution states.

### GAP 3.1.2: ResourceType Not Stored in Handle

**Status**: NOT IMPLEMENTED

**Issue**: The `resourceTypeIdx` parameter is captured but not stored in the handle.

**Impact**: Cannot validate type on subsequent operations.

---

### 3.2 `canon resource.drop`

**Spec** (`CanonicalABI.md` lines 3626-3650):
```python
def canon_resource_drop(rt, thread, i):
  trap_if(not thread.task.inst.may_leave)
  inst = thread.task.inst
  h = inst.table.remove(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not rt)
  trap_if(h.num_lends != 0)
  if h.own:
    assert(h.borrow_scope is None)
    if inst is rt.impl:
      if rt.dtor:
        rt.dtor(h.rep)
    else:
      if rt.dtor:
        # Call destructor via canon_lift/canon_lower
        ...
      else:
        trap_if(call_might_be_recursive(thread.task, rt.impl))
  else:
    h.borrow_scope.num_borrows -= 1
  return []
```

**Current Implementation** (`resource_table.go` lines 225-241):
```go
func (t *ResourceTable) CreateResourceDropFunc(resourceTypeIdx uint32, destructor func(rep uint32)) func(handle uint32) {
    return func(handle uint32) {
        entry, err := t.Remove(Handle(handle))
        if err != nil {
            return // Silently ignore invalid handles per spec  <-- WRONG!
        }
        if destructor != nil && entry.Rep != nil {
            // Convert rep to uint32 and call destructor
            switch rep := entry.Rep.(type) {
            case uint32:
                destructor(rep)
            case int:
                destructor(uint32(rep))
            }
        }
    }
}
```

### GAP 3.2.1: Invalid Handle Silently Ignored (DEFECT)

**Status**: DEFECT - INCORRECT BEHAVIOR

**Issue**: Comment says "Silently ignore invalid handles per spec" but spec says `trap_if(not isinstance(h, ResourceHandle))`.

**Impact**: Masks errors in component code; should trap.

### GAP 3.2.2: Missing Type Validation

**Status**: NOT IMPLEMENTED

**Issue**: No `trap_if(h.rt is not rt)` check.

**Impact**: Can drop handles with wrong resource type.

### GAP 3.2.3: Missing may_leave Check

**Status**: NOT IMPLEMENTED

### GAP 3.2.4: Missing Borrow numBorrows Decrement

**Status**: NOT IMPLEMENTED

**Issue**: For borrowed handles, should decrement `h.borrow_scope.num_borrows`.

### GAP 3.2.5: Missing Destructor Call via canon_lift/canon_lower

**Status**: NOT IMPLEMENTED

**Issue**: Cross-component destructor calls should go through canon_lift/canon_lower.

### GAP 3.2.6: Missing call_might_be_recursive Check

**Status**: NOT IMPLEMENTED

---

### 3.3 `canon resource.rep`

**Spec** (`CanonicalABI.md` lines 3682-3687):
```python
def canon_resource_rep(rt, thread, i):
  h = thread.task.inst.table.get(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not rt)
  return [h.rep]
```

**Current Implementation** (`resource_table.go` lines 246-254):
```go
func (t *ResourceTable) CreateResourceRepFunc(resourceTypeIdx uint32) func(handle uint32) uint32 {
    return func(handle uint32) uint32 {
        rep, err := t.Rep(Handle(handle))
        if err != nil {
            return 0 // Return 0 for invalid handles  <-- WRONG!
        }
        return rep
    }
}
```

### GAP 3.3.1: Invalid Handle Returns 0 Instead of Trap (DEFECT)

**Status**: DEFECT - INCORRECT BEHAVIOR

**Issue**: Returns 0 for invalid handles instead of trapping.

### GAP 3.3.2: Missing Type Validation

**Status**: NOT IMPLEMENTED

---

## 4. Lifting and Lowering Analysis

### 4.1 `lift_own`

**Spec** (`CanonicalABI.md` lines 2215-2221):
```python
def lift_own(cx, i, t):
  h = cx.inst.table.remove(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not t.rt)
  trap_if(h.num_lends != 0)
  trap_if(not h.own)
  return h.rep
```

### GAP 4.1: lift_own Not Fully Implemented

**Status**: NOT FULLY IMPLEMENTED

**Location**: Should be in `internal/component/abi/lift.go`

**Missing**:
- Type validation (`h.rt is not t.rt`)
- Check for borrow handle (`trap_if(not h.own)`)

---

### 4.2 `lift_borrow`

**Spec** (`CanonicalABI.md` lines 2234-2240):
```python
def lift_borrow(cx, i, t):
  assert(isinstance(cx.borrow_scope, Subtask))
  h = cx.inst.table.get(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not t.rt)
  cx.borrow_scope.add_lender(h)
  return h.rep
```

### GAP 4.2: lift_borrow Subtask Integration Missing

**Status**: PARTIAL

**Missing**:
- Assertion that borrow_scope is a Subtask
- Type validation

---

### 4.3 `lower_own`

**Spec** (`CanonicalABI.md` lines 2673-2675):
```python
def lower_own(cx, rep, t):
  h = ResourceHandle(t.rt, rep, own = True)
  return cx.inst.table.add(h)
```

### GAP 4.3: lower_own Missing ResourceType Storage

**Status**: NOT FULLY IMPLEMENTED

---

### 4.4 `lower_borrow`

**Spec** (`CanonicalABI.md` lines 2677-2683):
```python
def lower_borrow(cx, rep, t):
  assert(isinstance(cx.borrow_scope, Task))
  if cx.inst is t.rt.impl:
    return rep  # Optimization: return rep directly
  h = ResourceHandle(t.rt, rep, own = False, borrow_scope = cx.borrow_scope)
  h.borrow_scope.num_borrows += 1
  return cx.inst.table.add(h)
```

### GAP 4.4.1: Missing Same-Instance Optimization (CRITICAL)

**Status**: NOT IMPLEMENTED

**Issue**: Should return `rep` directly when lowering to the component that defined the resource type.

**Impact**: Performance penalty and incorrect semantics.

### GAP 4.4.2: Missing Task Assertion

**Status**: NOT IMPLEMENTED

---

## 5. Borrow Scope and Call Context Analysis

### 5.1 Current BorrowScope Implementation

From `internal/component/borrow_scope.go`:
```go
type BorrowScope struct {
    table   *ResourceTable
    lenders []Handle
}
```

### GAP 5.1: Missing Task Reference

**Spec** requires `borrow_scope: Optional[Task]` in ResourceHandle.

---

### 5.2 Current CallContext Implementation

From `internal/component/call_context.go`:
```go
type CallContext struct {
    numBorrows int
}
```

### GAP 5.2: Missing Lenders Tracking

**Spec** (`resources.rs` lines 187-192):
```rust
pub struct CallContext {
    lenders: Vec<TypedResourceIndex>,
    borrow_count: u32,
}
```

**Impact**: Cannot implement `exit_call` which must call `resource_undo_lend` for all lenders.

---

## 6. Wasmtime Test Coverage Analysis

The wasmtime test suite (`tests/all/component_model/resources.rs`) covers critical scenarios:

| Test | Scenario | Wazero Status |
|------|----------|---------------|
| `drop_guest_twice` | Double-drop detection | ✓ Implemented |
| `drop_host_twice` | Host resource double-drop | ✓ Implemented |
| `mismatch_intrinsics` | Type mismatch detection | ✗ NOT IMPLEMENTED |
| `mismatch_resource_types` | Cross-type validation | ✗ NOT IMPLEMENTED |
| `active_borrows_at_end_of_call` | Borrow lifetime validation | ✗ NOT IMPLEMENTED |
| `cannot_use_borrow_for_own` | lift_own from borrow fails | ✗ NOT IMPLEMENTED |
| `passthrough_wrong_type` | Type preservation | ✗ NOT IMPLEMENTED |
| `pass_moved_resource` | Use-after-move detection | Partial |
| `drop_on_owned_resource` | Drop while borrowed trap | ✓ Implemented |
| `intrinsic_trampolines` | resource.rep validation | ✗ NOT IMPLEMENTED |

---

## 7. Implementation Priority Matrix

### P0 - Critical (Spec Compliance)

1. **Add ResourceType to HandleEntry** - Required for all type validation
2. **Fix trap conditions in resource.drop** - Currently incorrect behavior
3. **Fix trap conditions in resource.rep** - Currently incorrect behavior
4. **Implement type validation in all operations**

### P1 - High (Feature Completeness)

5. **Add impl field to ResourceType** - Required for destructor routing
6. **Implement lower_borrow optimization** - Performance and correctness
7. **Add lenders tracking to CallContext** - Required for exit_call
8. **Implement destructor invocation** - Required for resource cleanup

### P2 - Medium (Advanced Features)

9. **Add may_leave checks** - Execution state validation
10. **Implement call_might_be_recursive** - Reentrance protection
11. **Add async destructor support** - Future async component support

### P3 - Low (Optimization)

12. **Handle table MAX_LENGTH enforcement** - `2**28 - 1` limit
13. **Generation overflow handling**

---

## 8. Implementation Plan

### Phase 1: Core Type System (P0)

**Files to modify**:
- `internal/component/resource_table.go`
- `internal/component/types/resource.go`

**Changes**:
1. Add `ResourceType` field to `HandleEntry`:
```go
type HandleEntry struct {
    RT          *ResourceType  // NEW: Resource type reference
    Rep         uint32         // Change from any to uint32
    Own         bool
    NumLends    uint32
    BorrowScope *BorrowScope   // Change from any to typed
}
```

2. Expand `ResourceType`:
```go
type ResourceType struct {
    Impl        ComponentInstanceRef  // NEW: Defining instance
    Destructor  *uint32
    DtorAsync   bool                  // NEW
    DtorCallback *uint32              // NEW
}
```

### Phase 2: Fix Trap Conditions (P0)

**Changes**:
1. `CreateResourceDropFunc`: Return error or panic on invalid handle
2. `CreateResourceRepFunc`: Return error or panic on invalid handle
3. Add type validation to all operations

### Phase 3: Borrow Scope Integration (P1)

**Files to modify**:
- `internal/component/borrow_scope.go`
- `internal/component/call_context.go`

**Changes**:
1. Add `lenders` tracking to CallContext
2. Implement `exit_call` with `resource_undo_lend`
3. Implement `lower_borrow` optimization

### Phase 4: Destructor Support (P1)

**Changes**:
1. Implement destructor invocation on owned handle drop
2. Route cross-component destructor calls through canon_lift/canon_lower

### Phase 5: Advanced Features (P2-P3)

**Changes**:
1. Add may_leave checks
2. Implement call_might_be_recursive
3. Add async destructor support

---

## 9. Test Plan

### New Tests Required

1. **Type Validation Tests**
   - `TestResource_TypeMismatch_Drop`
   - `TestResource_TypeMismatch_Rep`
   - `TestResource_TypeMismatch_LiftOwn`
   - `TestResource_TypeMismatch_LiftBorrow`

2. **Trap Condition Tests**
   - `TestResource_Drop_InvalidHandle_Traps`
   - `TestResource_Rep_InvalidHandle_Traps`
   - `TestResource_LiftOwn_FromBorrow_Traps`
   - `TestResource_Drop_WhileBorrowed_Traps`

3. **Destructor Tests**
   - `TestResource_Destructor_CalledOnDrop`
   - `TestResource_Destructor_NotCalledOnBorrow`
   - `TestResource_Destructor_CrossComponent`

4. **Optimization Tests**
   - `TestResource_LowerBorrow_SameInstance_ReturnsRep`

### Regression Tests

**CRITICAL**: All changes MUST ensure these tests continue to pass:
- `internal/component/wasip2test/calculator_test.go` - add and subtract plugins

---

## 10. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing tests | Medium | High | Run full test suite before each change |
| ABI incompatibility | Low | High | Follow spec exactly, validate against wasmtime |
| Performance regression | Medium | Medium | Benchmark critical paths |
| Incomplete type tracking | High | High | Add types incrementally with tests |

---

## Appendix A: Spec Code References

### lift_own (CanonicalABI.md:2215-2221)
```python
def lift_own(cx, i, t):
  h = cx.inst.table.remove(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not t.rt)
  trap_if(h.num_lends != 0)
  trap_if(not h.own)
  return h.rep
```

### lift_borrow (CanonicalABI.md:2234-2240)
```python
def lift_borrow(cx, i, t):
  assert(isinstance(cx.borrow_scope, Subtask))
  h = cx.inst.table.get(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not t.rt)
  cx.borrow_scope.add_lender(h)
  return h.rep
```

### lower_own (CanonicalABI.md:2673-2675)
```python
def lower_own(cx, rep, t):
  h = ResourceHandle(t.rt, rep, own = True)
  return cx.inst.table.add(h)
```

### lower_borrow (CanonicalABI.md:2677-2683)
```python
def lower_borrow(cx, rep, t):
  assert(isinstance(cx.borrow_scope, Task))
  if cx.inst is t.rt.impl:
    return rep
  h = ResourceHandle(t.rt, rep, own = False, borrow_scope = cx.borrow_scope)
  h.borrow_scope.num_borrows += 1
  return cx.inst.table.add(h)
```

### canon resource.new (CanonicalABI.md:3604-3609)
```python
def canon_resource_new(rt, thread, rep):
  trap_if(not thread.task.inst.may_leave)
  h = ResourceHandle(rt, rep, own = True)
  i = thread.task.inst.table.add(h)
  return [i]
```

### canon resource.drop (CanonicalABI.md:3626-3650)
```python
def canon_resource_drop(rt, thread, i):
  trap_if(not thread.task.inst.may_leave)
  inst = thread.task.inst
  h = inst.table.remove(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not rt)
  trap_if(h.num_lends != 0)
  if h.own:
    assert(h.borrow_scope is None)
    if inst is rt.impl:
      if rt.dtor:
        rt.dtor(h.rep)
    else:
      if rt.dtor:
        caller_opts = CanonicalOptions(async_ = False)
        callee_opts = CanonicalOptions(async_ = rt.dtor_async, callback = rt.dtor_callback)
        ft = FuncType([U32Type()],[], async_ = False)
        callee = partial(canon_lift, callee_opts, rt.impl, ft, rt.dtor)
        [] = canon_lower(caller_opts, ft, callee, thread, [h.rep])
      else:
        trap_if(call_might_be_recursive(thread.task, rt.impl))
  else:
    h.borrow_scope.num_borrows -= 1
  return []
```

### canon resource.rep (CanonicalABI.md:3682-3687)
```python
def canon_resource_rep(rt, thread, i):
  h = thread.task.inst.table.get(i)
  trap_if(not isinstance(h, ResourceHandle))
  trap_if(h.rt is not rt)
  return [h.rep]
```
