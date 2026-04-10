# Session 2 — Complete Component Model & WASI P2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the wazero component model and WASI P2 implementation — cross-instance resources, post-return protocol, full multi-module Instantiate pipeline, spectest runner, public API parity with wasmtime C API, and zero unaddressed test skips.

**Architecture:** Four layers, bottom-up: Foundation (cross-instance resources, post-return, reentrance at canon_lift, handle+list wiring) → Pipeline (multi-module Instantiate, wireExports, canon closures) → Tests (spectest runner, unskip all, real .wasm integration) → Public API (type introspection, resource surface, InstancePre, post-return exposure).

**Tech Stack:** Go, WebAssembly Component Model, Canonical ABI (definitions.py), wasmtime C API as public surface reference.

**Design spec:** `docs/superpowers/specs/2026-04-10-session2-complete-implementation-design.md`

---

## Critical Rules for All Tasks

1. **Spec wins.** Every behavior decision must be verified against `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py`. If the plan and the spec disagree, the spec wins.

2. **No duplicate types or fields.** Before adding ANY new type, field, or function, `grep -rn` the codebase to check it doesn't already exist under a different name or in a different package. The codebase has `runtime/`, `types/`, `abi/`, `component/` subpackages — a concept may already exist in one of them.

3. **Fix defects at source.** If a test reveals a bug in `abi/`, `binary/`, `runtime/`, or `types/`, fix it there. Do not add workarounds in `component/`.

4. **Process for unknown quantities.** When a task says "for each X" (e.g., "for each t.Skip"), the process is:
   - Run the grep/search command specified
   - Process EVERY result found, regardless of count
   - Do not stop because you've hit an expected number
   - Do not skip results because they "look similar" to ones already processed
   - Verify each result individually

5. **No guessing from training data.** For any function signature, field name, or type, verify by reading the actual file before referencing it. Code moves. Names change.

6. **Verify function signatures before calling.** The plan provides pseudocode patterns, but actual signatures may differ. Key examples:
   - `abi.LiftParams` third param is `flat []uint64` (not an iterator)
   - `abi.LowerResults` has extra `stack []uint64` and `needsRetptr bool` params beyond what the plan shows
   - `HostModuleExport` function field is named `Func` (type `api.GoModuleFunc`), not `GoFunc`
   - `canonResourceInfo` fields are lowercase (unexported) — accessed via same-package access
   - Canon resource constants have a `Kind` infix: `CanonKindResourceNew`, `CanonKindResourceRep`, `CanonKindResourceDrop`
   Always `grep -rn` for the actual definition before using any function or field.

---

## Layer 1: Foundation

### Task 1: ResourceStore — Store-Wide Resource Type Registry

**Purpose:** Enable cross-instance resource resolution by providing a shared registry that all instances can query. Currently `LookupResourceType` only walks the parent chain, which fails for sibling instances.

**Files:**
- Create: `internal/component/runtime/resource_store.go`
- Modify: `internal/component/runtime/component_instance.go`
- Modify: `internal/component/component_linker.go` (bindResourceTypes)
- Test: `internal/component/runtime/resource_store_test.go`

**Spec reference:** `definitions.py:256-273` (ComponentInstance shape), `definitions.py:1345` (`h.rt is not t.rt` identity check). The store is an implementation detail enabling the spec's cross-instance identity checks.

- [ ] **Step 1: Read the current `LookupResourceType` implementation**

Read `internal/component/runtime/component_instance.go` starting at the `LookupResourceType` method. Understand what it does and where it fails (parent-chain walk only).

- [ ] **Step 2: Write failing test for sibling instance resource lookup**

Create `internal/component/runtime/resource_store_test.go`:

```go
package runtime

import "testing"

func TestResourceStore_RegisterAndLookup(t *testing.T) {
	store := NewResourceStore()
	inst := NewComponentInstance(1, nil)
	rt := &ResourceType{Impl: inst}

	store.Register(1, 0, rt)

	got := store.Lookup(1, 0)
	if got != rt {
		t.Fatalf("expected resource type %p, got %p", rt, got)
	}
}

func TestResourceStore_SiblingLookup(t *testing.T) {
	store := NewResourceStore()

	// Two sibling instances (neither is parent of the other)
	instA := NewComponentInstance(1, nil)
	instB := NewComponentInstance(2, nil)

	rtA := &ResourceType{Impl: instA}
	store.Register(1, 0, rtA)

	// instB should be able to find instA's resource type via the store
	got := store.Lookup(1, 0)
	if got != rtA {
		t.Fatalf("sibling lookup failed: expected %p, got %p", rtA, got)
	}
	_ = instB // instB doesn't need to be in the store to query it
}

func TestResourceStore_LookupNotFound(t *testing.T) {
	store := NewResourceStore()
	got := store.Lookup(99, 0)
	if got != nil {
		t.Fatalf("expected nil for missing entry, got %p", got)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/component/runtime/ -run TestResourceStore -v`
Expected: compilation error (ResourceStore not defined)

- [ ] **Step 4: Implement ResourceStore**

Create `internal/component/runtime/resource_store.go`:

```go
package runtime

import "sync"

// resourceTypeKey identifies a resource type by the instance that
// defines it and the declaration index within that instance.
type resourceTypeKey struct {
	InstanceID  uint32
	ResourceIdx uint32
}

// ResourceStore is a shared registry of resource types across all
// instances created during a single top-level Instantiate call.
// It enables cross-instance resource resolution where the parent-chain
// walk on ComponentInstance fails (sibling instances).
//
// Thread-safe: multiple goroutines may query concurrently.
type ResourceStore struct {
	mu    sync.RWMutex
	types map[resourceTypeKey]*ResourceType

	// instances maps instance IDs to an opaque wrapper value.
	// The value is interface{} to avoid an import cycle between
	// runtime/ and component/. Callers in component/ type-assert
	// to *component.Instance.
	instances map[uint32]interface{}
}

// NewResourceStore creates an empty resource store.
func NewResourceStore() *ResourceStore {
	return &ResourceStore{
		types:     make(map[resourceTypeKey]*ResourceType),
		instances: make(map[uint32]interface{}),
	}
}

// Register adds a resource type to the store.
func (s *ResourceStore) Register(instanceID, resourceIdx uint32, rt *ResourceType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.types[resourceTypeKey{InstanceID: instanceID, ResourceIdx: resourceIdx}] = rt
}

// Lookup finds a resource type by instance ID and resource index.
// Returns nil if not found.
func (s *ResourceStore) Lookup(instanceID, resourceIdx uint32) *ResourceType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.types[resourceTypeKey{InstanceID: instanceID, ResourceIdx: resourceIdx}]
}

// RegisterInstance associates an instance ID with a wrapper value.
func (s *ResourceStore) RegisterInstance(id uint32, inst interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[id] = inst
}

// GetInstance returns the wrapper instance for the given ID.
func (s *ResourceStore) GetInstance(id uint32) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instances[id]
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/component/runtime/ -run TestResourceStore -v`
Expected: PASS

- [ ] **Step 6: Wire ResourceStore into ComponentInstance**

Modify `internal/component/runtime/component_instance.go`:
- Add a `Store *ResourceStore` field to the `ComponentInstance` struct
- Modify `LookupResourceType` to fall back to `c.Store.Lookup(instanceIdx, resourceIdx)` when the parent-chain walk returns nil
- Modify `NewComponentInstance` to accept an optional `*ResourceStore` parameter (or add a `SetStore` method)

- [ ] **Step 7: Wire ResourceStore into ComponentLinker.Instantiate**

Modify `internal/component/component_linker.go` in the `Instantiate` method:
- After creating the instance (Step 1 of the 14-step pipeline), create a `ResourceStore` and set it on the instance
- In `bindResourceTypes`, after minting each `*ResourceType`, also call `store.Register(inst.rt.ID, resourceIdx, rt)`
- After creating the instance, also call `store.RegisterInstance(inst.rt.ID, inst)` so cross-instance lookups can resolve the wrapper `*Instance`
- In `processNestedInstances`, pass the same store to nested instances and call `store.RegisterInstance` for each nested instance

- [ ] **Step 8: Run full component test suite**

Run: `go test ./internal/component/... -v -count=1 2>&1 | tail -30`
Expected: no new failures

- [ ] **Step 9: Commit**

```bash
git add internal/component/runtime/resource_store.go internal/component/runtime/resource_store_test.go internal/component/runtime/component_instance.go internal/component/component_linker.go
git commit -m "component: add ResourceStore for cross-instance resource type resolution

Spec: definitions.py:1345 (h.rt is not t.rt identity check).
Enables sibling instance resource lookup where parent-chain walk fails."
```

---

### Task 2: Cross-Instance Resource Drop with Destructors

**Purpose:** Replace the three error-returning paths in `instance.go` with real cross-instance resource operations per the spec.

**Files:**
- Modify: `internal/component/instance.go` (ResourceDrop, invokeLocalDestructor)
- Modify: `internal/component/component_linker.go` (bindResourceTypes — wire HostDestructor closure)
- Test: `internal/component/instance_test.go`
- Test: `internal/component/conformance/destructor_test.go`

**Spec reference:** `definitions.py:2142-2165` (canon_resource_drop), `:2151-2160` (cross-instance destructor path). Read this section IN FULL before starting.

- [ ] **Step 1: Read the spec section**

Read `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` lines 2142-2165. Understand the three branches: local own with destructor, cross-instance own with destructor, cross-instance own without destructor, and the borrow branch.

- [ ] **Step 2: Read the current ResourceDrop implementation**

Read `internal/component/instance.go` starting at `ResourceDrop`. Identify the three error-returning paths that need replacement:
1. Cross-instance destructor invocation (around line 344-353)
2. Guest destructor resolution in `invokeLocalDestructor` (around line 376-389)
3. Missing reentrance check for cross-instance no-destructor path (around line 350-352)

- [ ] **Step 3: Read the existing HostDestructor field**

Read `internal/component/runtime/resource_type.go`. Verify that `HostDestructor func(rep uint32) error` exists on `ResourceType`. This field is used for host-declared resource destructors. For guest-declared resources, `HostDestructor` will be set at instantiation time as a closure that captures the core function.

- [ ] **Step 4: Write test for cross-instance own drop with destructor**

Add to `internal/component/conformance/destructor_test.go`, in the existing `TestDestructors` function, update the `CrossInstanceDestructorDeferred` subtest:

```go
t.Run("CrossInstanceDestructorDeferred", func(t *testing.T) {
	// Create two instances — instA defines the resource, instB drops it
	store := runtime.NewResourceStore()
	instA := component.NewInstance(&component.Component{}, 1, nil)
	instA.Runtime().Store = store
	instB := component.NewInstance(&component.Component{}, 2, nil)
	instB.Runtime().Store = store

	var destructorCalled bool
	var destructorRep uint32
	rt := &runtime.ResourceType{
		Impl: instA.Runtime(),
		HostDestructor: func(rep uint32) error {
			destructorCalled = true
			destructorRep = rep
			return nil
		},
	}
	instA.Runtime().ResourceTypes = append(instA.Runtime().ResourceTypes, rt)
	store.Register(1, 0, rt)

	// Create a handle in instB's table (simulating a lowered own handle)
	h, err := instB.Runtime().Table.NewResourceHandle(42, true, rt)
	require.NoError(t, err)

	// Drop from instB — should invoke destructor via cross-instance path
	err = instB.ResourceDrop(types.ResourceIdx(0), h.Index())
	require.NoError(t, err)
	require.True(t, destructorCalled, "cross-instance destructor should have been called")
	require.Equal(t, uint32(42), destructorRep)
})
```

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./internal/component/conformance/ -run TestDestructors/CrossInstance -v`
Expected: FAIL (current code returns error instead of invoking destructor)

- [ ] **Step 6: Implement cross-instance destructor invocation**

**IMPORTANT:** The spec (definitions.py:2154-2160) requires cross-instance destructor invocation to go through a full `canon_lift`/`canon_lower` call, NOT a direct function call. This ensures `may_leave`, reentrance checks, and borrow scope cleanup are enforced on the callee instance. Read the spec lines carefully:

```python
# Spec: definitions.py:2154-2160
callee_opts = CanonicalOptions(async_ = False)
ft = FuncType([U32Type()], [])
callee = partial(canon_lift, callee_opts, rt.impl, ft, rt.dtor)
[] = canon_lower(caller_opts, ft, callee, thread, [h.rep])
```

Modify `internal/component/instance.go` in `ResourceDrop`. The cross-instance destructor path must:
1. Resolve the defining instance from the ResourceStore
2. Construct a `types.TypeFunc` for the destructor signature: `([u32], [])`
3. Build a canon_lift closure targeting `rt.Impl` with the destructor core function
4. Invoke it via the canon_lower path from the calling instance
5. This reuses `buildCanonLiftFunc` and `buildCanonLowerFunc` from the pipeline (Tasks 7/9)

Since Tasks 7/9 haven't landed yet, the initial implementation can use the `HostDestructor` closure as a temporary bridge (which the linker sets at instantiation time), but it MUST be replaced with the full canon_lift/canon_lower path once the pipeline is complete. Add a TODO with the spec citation:

```go
// Cross-instance: invoke destructor on the defining instance.
// Spec: definitions.py:2154-2160 requires canon_lift/canon_lower path.
// For host-declared resources, HostDestructor is a Go closure that
// already enforces host-side semantics. For guest-declared resources,
// this MUST go through canon_lift/canon_lower once the pipeline lands.
if rt.HostDestructor != nil {
	if err := rt.HostDestructor(resEntry.Rep); err != nil {
		return fmt.Errorf("resource.drop: cross-instance destructor: %w", err)
	}
} else if rt.Dtor != nil {
	// TODO(session2-pipeline): replace with canon_lift/canon_lower invocation
	// per spec definitions.py:2154-2160. Requires buildCanonLiftFunc (Task 7).
	return fmt.Errorf("resource.drop: guest cross-instance destructor requires canon_lift/canon_lower pipeline (spec definitions.py:2154-2160)")
}
```

The TODO is resolved in Task 10 when the full pipeline is wired.

- [ ] **Step 7: Implement reentrance check for cross-instance no-destructor path**

In the same `ResourceDrop` method, replace the comment at the no-destructor cross-instance path:

```go
// Spec: definitions.py:2162 — trap_if(call_might_be_recursive(...))
if i.CallMightBeRecursive(getDefiningInstance(i, rt)) {
	return errReentrance
}
```

Where `getDefiningInstance` resolves the defining instance from the ResourceStore:
```go
func getDefiningInstance(caller *Instance, rt *runtime.ResourceType) *Instance {
	if caller.rt.Store == nil {
		return nil
	}
	wrapper := caller.rt.Store.GetInstance(rt.Impl.ID)
	if inst, ok := wrapper.(*Instance); ok {
		return inst
	}
	return nil
}
```

- [ ] **Step 8: Wire HostDestructor for guest resources at instantiation time**

Modify `internal/component/component_linker.go` in `bindResourceTypes`:

After minting each `*ResourceType`, if the resource declaration has a destructor core function index (`rt.Dtor != nil`), create a closure that will invoke it once core modules are instantiated:

```go
// The closure captures a mutable pointer to the core function that will be
// resolved after core module instantiation (Task 9's memory capture step).
type dtorRef struct {
	fn api.Function
}
ref := &dtorRef{}
rt.HostDestructor = func(rep uint32) error {
	if ref.fn == nil {
		return fmt.Errorf("guest destructor not yet resolved (core modules not instantiated)")
	}
	_, err := ref.fn.Call(context.Background(), uint64(rep))
	return err
}
// Add ref to a list of pending destructor resolutions.
// Task 9 (post-instantiation memory capture) resolves these by scanning
// core module exports and matching the destructor core function index
// from rt.Dtor to the exported function.
pendingDtors = append(pendingDtors, pendingDtor{rt: rt, ref: ref, coreFuncIdx: *rt.Dtor})
```

The `pendingDtors` list must be processed in Task 9's post-instantiation step. Add a `pendingDtor` struct:
```go
type pendingDtor struct {
	rt           *runtime.ResourceType
	ref          *dtorRef
	coreFuncIdx  uint32
}
```
In `instantiateCoreModules`, after each core module is instantiated, resolve pending destructors by matching `coreFuncIdx` to the core function index space.

- [ ] **Step 9: Run tests**

Run: `go test ./internal/component/conformance/ -run TestDestructors -v`
Expected: PASS (including the CrossInstance subtest)

Run: `go test ./internal/component/... -v -count=1 2>&1 | tail -30`
Expected: no new failures

- [ ] **Step 10: Commit**

```bash
git add internal/component/instance.go internal/component/component_linker.go internal/component/conformance/destructor_test.go
git commit -m "component: implement cross-instance resource drop with destructors

Spec: definitions.py:2142-2165 canon_resource_drop.
- Cross-instance destructor invocation via HostDestructor closure
- Reentrance check for cross-instance no-destructor path
- Guest destructor closure wired at bindResourceTypes time"
```

---

### Task 3: lower_borrow Same-Instance Optimization

**Purpose:** Implement the spec-required optimization where `lower_borrow` returns `rep` directly when the calling instance IS the defining instance, skipping the borrow handle allocation.

**Files:**
- Modify: `internal/component/abi/lower.go` (lowerBorrowHandleFlat, lowerBorrowHandleHeap)
- Test: `internal/component/abi/lower_test.go`

**Spec reference:** `definitions.py:1645-1651`:
```python
def lower_borrow(cx, rep, t):
    if cx.inst is t.rt.impl:
        return rep
    h = ResourceHandle(t.rt, rep, own=False, borrow_scope=cx.borrow_scope)
    return cx.inst.table.add(h)
```

- [ ] **Step 1: Read the current lowerBorrowHandle implementation**

Read `internal/component/abi/lower.go` starting at the `lowerBorrowHandleFlat` function (around line 644). Check if it already implements the same-instance optimization. If it does, this task is done — verify with a test and move on.

- [ ] **Step 2: Write failing test if optimization is missing**

Add to `internal/component/abi/lower_test.go`:

```go
func TestLowerBorrowSameInstanceReturnsRep(t *testing.T) {
	// Spec: definitions.py:1645-1647 — if cx.inst is t.rt.impl: return rep
	inst := runtime.NewComponentInstance(1, nil)
	rt := &runtime.ResourceType{Impl: inst}
	inst.ResourceTypes = append(inst.ResourceTypes, rt)

	// The borrow's resource type is defined by the same instance
	// that is doing the lowering. The spec says: return rep directly.
	ctx := &LowerContext{
		Instance: inst,
		Types:    /* minimal types bag with a resource table entry pointing at rt */,
	}

	// Lower a borrow of rep=42 — should get 42 back, not a handle index
	result, err := LowerFlat(ctx, types.ValBorrow(42), /* borrow valtype */)
	require.NoError(t, err)
	require.Equal(t, uint64(42), result[0], "same-instance borrow should return rep directly")

	// Verify no handle was allocated in the table
	require.Equal(t, 0, inst.Table.Len(), "no handle should be allocated for same-instance borrow")
}
```

Note: the exact test setup depends on how `LowerContext` and the types bag are constructed. Read the existing lower_test.go tests for the pattern.

- [ ] **Step 3: Implement the optimization and verify borrow_scope handling**

In `lowerBorrowHandleFlat` (and `lowerBorrowHandleHeap` if it exists), add the same-instance check at the top:

```go
// Spec: definitions.py:1645-1647
// If the calling instance IS the defining instance, return rep directly.
// No borrow handle is allocated, no borrow_scope tracking needed.
if ctx.Instance == rt.Impl {
    return []uint64{uint64(rep)}, nil  // for flat path
}

// Spec: definitions.py:1648-1650
// Non-same-instance: allocate a borrow handle WITH borrow_scope.
// h = ResourceHandle(t.rt, rep, own=False, borrow_scope=cx.borrow_scope)
// h.borrow_scope.num_borrows += 1
// The borrow_scope comes from the LowerContext's CallContext.
```

Also verify that the NON-same-instance path in the existing code correctly:
1. Creates the handle with `own=false` and sets `BorrowScope` from the context
2. Increments `BorrowScope.NumBorrows` (or equivalent)
If either is missing, fix it. Read `runtime/borrow_scope.go` for the increment API.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/component/abi/ -run TestLowerBorrow -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/component/abi/lower.go internal/component/abi/lower_test.go
git commit -m "abi: implement lower_borrow same-instance optimization

Spec: definitions.py:1645-1647. When cx.inst is t.rt.impl,
return rep directly without allocating a borrow handle."
```

---

### Task 4: Reentrance Check at canon_lift Entry

**Purpose:** Enforce the reentrance guard at `canon_lift` entry that prevents recursive calls into a component instance.

**Files:**
- Modify: `internal/component/component_linker.go` (buildCanonLiftFunc)
- Test: `internal/component/conformance/reentrance_test.go`

**Spec reference:** `definitions.py:1978-2002` (canon_lift). **CRITICAL: Read the actual vendored spec file before implementing.** The spec uses `call_might_be_recursive(caller, inst)` at the canon_lift entry point, which is a structural check on the instance ancestry graph (see `definitions.py:290-299`). This is NOT a boolean `may_enter` flag — it is a check that walks the caller's and callee's ancestor chains for overlap.

**Before starting:** Read `definitions.py` lines 1978-2002 in full. Search for `may_enter` in the file — if it does not appear, do NOT add a `MayEnter` field. Use whatever mechanism the spec actually uses. The `CallMightBeRecursive` method already exists on `Instance` at `instance.go:457` and implements the reflexive-ancestor walk from `definitions.py:290-299`.

- [ ] **Step 1: Read the spec and verify the reentrance mechanism**

Read `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` lines 1978-2002. Identify EXACTLY what check is performed at canon_lift entry. Search for `may_enter` — if it exists, use it. If it doesn't, use whatever the spec actually says.

Also read lines 290-299 (`call_might_be_recursive`) to understand the structural check.

- [ ] **Step 2: Read the existing CallMightBeRecursive implementation**

Read `internal/component/instance.go` at `CallMightBeRecursive` (line 457). Verify it matches the spec's `call_might_be_recursive` function.

- [ ] **Step 3: Write failing test**

Add to `internal/component/conformance/reentrance_test.go`:

```go
func TestCanonLiftReentranceCheck(t *testing.T) {
	// Spec: definitions.py canon_lift entry — the spec's reentrance
	// check prevents recursive calls into the same component instance.
	// Verify the check uses the mechanism from the actual spec.
	inst := component.NewInstance(&component.Component{}, 1, nil)

	// A call from an instance to itself should be detected as recursive
	require.True(t, inst.CallMightBeRecursive(inst))

	// A call from nil caller (host) should not be recursive
	require.False(t, inst.CallMightBeRecursive(nil))
}
```

- [ ] **Step 4: Wire into buildCanonLiftFunc**

In `internal/component/component_linker.go`, in `buildCanonLiftFunc`, add the reentrance check at the top of the closure. The exact check depends on what the spec says — use the spec, not this plan, as the authority:

```go
// At entry: check for reentrance per spec canon_lift entry.
// The caller is retrieved from context (set by the calling instance).
caller := GetCallerInstance(ctx)
if inst.CallMightBeRecursive(caller) {
    return nil, errReentrance
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/component/conformance/ -run TestCanonLift -v`
Expected: PASS

Run: `go test ./internal/component/... -count=1 2>&1 | tail -10`
Expected: no new failures

- [ ] **Step 6: Commit**

```bash
git commit -m "component: add reentrance check at canon_lift entry

Spec: definitions.py canon_lift entry + call_might_be_recursive at :290-299.
Uses structural ancestor-chain walk, not a boolean flag."
```

---

### Task 5: Post-Return Two-Phase Protocol

**Purpose:** Split ExportedFunc.Call into a two-phase protocol matching the spec and wasmtime C API.

**Files:**
- Modify: `internal/component/instance.go` (ExportedFunc struct, Call, PostReturn)
- Modify: `internal/component/component_linker.go` (buildCanonLiftFunc — separate call from post-return)
- Test: `internal/component/instance_test.go`
- Test: `internal/component/conformance/post_return_test.go`

**Spec reference:** `definitions.py:1999-2002` (post-return invocation), wasmtime C API `func.h` (`func_call` + `func_post_return`).

- [ ] **Step 1: Read the current ExportedFunc.Call implementation**

Read `internal/component/instance.go` at `ExportedFunc` struct (line 166) and `Call` method (line 186). Understand how it currently delegates to `impl`.

- [ ] **Step 2: Read the current post-return handling**

Read `internal/component/component_linker.go` at `buildCanonLiftFunc` (line 1223). Check if post-return is currently handled inside the closure or not at all.

- [ ] **Step 3: Add PostReturn state to ExportedFunc**

Modify `internal/component/instance.go`:

```go
type ExportedFunc struct {
	name      string
	funcType  *types.TypeFunc
	component *Component
	instance  *Instance
	impl      HostFunc

	// Two-phase post-return state.
	// Spec: definitions.py:1999-2002.
	// Wasmtime C API: func_call + func_post_return.
	needsPostReturn bool
	postReturnFunc  func(ctx context.Context) error // set by buildCanonLiftFunc
	pendingResults  []uint64                         // flat results for post-return
}
```

- [ ] **Step 4: Implement PostReturn method**

```go
// PostReturn executes the post-return cleanup for this function.
// Must be called after Call and before the next Call.
// Spec: definitions.py:1999-2002.
func (f *ExportedFunc) PostReturn(ctx context.Context) error {
	if f == nil {
		return fmt.Errorf("ExportedFunc.PostReturn: nil receiver")
	}
	if !f.needsPostReturn {
		return nil // no post-return needed (function has no post-return option)
	}
	f.needsPostReturn = false
	if f.postReturnFunc != nil {
		return f.postReturnFunc(ctx)
	}
	return nil
}
```

- [ ] **Step 5: Add panic guard to Call**

In `ExportedFunc.Call`, add at the top:

```go
if f.needsPostReturn {
	panic("ExportedFunc.Call: PostReturn must be called before calling again (spec: definitions.py:1999-2002)")
}
```

- [ ] **Step 6: Add CallAndPostReturn convenience method**

```go
// CallAndPostReturn invokes the function and immediately runs post-return.
// Convenience for embedders that don't need the two-phase window.
func (f *ExportedFunc) CallAndPostReturn(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	results, err := f.Call(ctx, params...)
	if err != nil {
		return nil, err
	}
	if postErr := f.PostReturn(ctx); postErr != nil {
		return results, postErr
	}
	return results, nil
}
```

- [ ] **Step 7: Update ComponentFuncWrapper in linker_api.go**

**CRITICAL:** The public API's `ComponentFuncWrapper` (at `internal/component/linker_api.go`, search for `ComponentFuncWrapper`) delegates to `ExportedFunc.Call`. After adding the post-return panic guard, the wrapper MUST call `PostReturn` after each `Call`, or the public API breaks for repeated calls. Find the wrapper's `Call` method and update it to call `CallAndPostReturn` instead of `Call`:

```bash
grep -rn "ComponentFuncWrapper" internal/component/ --include="*.go"
```

Read the file, understand the wrapper, and update it.

- [ ] **Step 8: Write tests**

Add tests verifying:
1. Call + PostReturn works in sequence
2. Call + Call without PostReturn panics
3. CallAndPostReturn works
4. PostReturn with no pending post-return is a no-op

- [ ] **Step 9: Run tests and commit**

Run: `go test ./internal/component/... -count=1 2>&1 | tail -10`

```bash
git commit -m "component: add two-phase post-return protocol on ExportedFunc

Spec: definitions.py:1999-2002. Wasmtime C API: func_call + func_post_return.
Call returns results without running post-return. PostReturn runs cleanup.
Panic if Call is invoked again before PostReturn."
```

---

### Task 6: ExportedFunc.Call Handle + List Wiring

**Purpose:** Wire resource handle and list parameters through the abi.LiftParams/LowerParams pipeline so ExportedFunc.Call works with all type kinds.

**Files:**
- Modify: `internal/component/component_linker.go` (buildCanonLiftFunc)
- Modify: `internal/component/instance.go` (if Call needs changes)
- Test: `internal/component/instance_test.go` (unskip handle/list tests)

**Spec reference:** `definitions.py:1978-2040` (canon_lift), `:2064-2130` (canon_lower), `:1943-1975` (lower_flat_values), `:1977-1993` (lift_flat_values).

**Process:** This task requires `buildCanonLiftFunc` to construct proper `LiftContext`/`LowerContext` with memory, realloc, types bag, and call context. The exact implementation depends on the current state of `buildCanonLiftFunc`. Read it first, then determine what's missing.

- [ ] **Step 1: Read buildCanonLiftFunc in full**

Read `internal/component/component_linker.go` starting at line 1223. Understand what the closure currently does and what's missing for handle/list support.

- [ ] **Step 2: Read abi.LiftParams and abi.LowerResults signatures**

Read `internal/component/abi/lift.go` at `LiftParams` (line 771) and `internal/component/abi/lower.go` at `LowerResults` (line 820). Understand what context they need.

- [ ] **Step 3: Read the LiftContext and LowerContext struct definitions**

These are in `internal/component/abi/`. Understand what fields must be populated (Memory, Realloc, Instance, Types, etc.).

- [ ] **Step 4: Enhance buildCanonLiftFunc**

The closure must:
1. Create a `runtime.CallContext` with a fresh `BorrowScope` for each call
2. Construct a `LiftContext` with the instance's memory, realloc, and types
3. Use `abi.LiftParams` to lift core wasm flat values into `[]Val`
4. Invoke the component function
5. Use `abi.LowerResults` to lower `[]Val` results back to flat core values
6. Store flat results for post-return
7. NOT run post-return (that's `PostReturn`'s job)

The exact code depends on the current state. Follow the spec at `definitions.py:1978-2040` line by line.

- [ ] **Step 5: Attempt to unskip handle and list tests**

Search for all `t.Skip` calls in `internal/component/instance_test.go` that mention "handle", "list", "Own", "Borrow", or "CallContext":

```bash
grep -n "t.Skip.*\(handle\|list\|Own\|Borrow\|CallContext\|realloc\)" internal/component/instance_test.go
```

For each one:
1. Remove the `t.Skip` line
2. Run the individual test: `go test ./internal/component/ -run TestName -v`
3. If it passes: keep the skip removed
4. If it fails with an abi/ or runtime/ defect: fix the defect at source
5. If it fails because the closure isn't fully wired yet: investigate what's missing and wire it

- [ ] **Step 6: Run full test suite and commit**

Run: `go test ./internal/component/... -count=1 2>&1 | tail -20`

Commit with a message describing what was wired and how many tests were unskipped.

---

## Layer 2: Pipeline

### Task 7: buildCanonLowerFunc — Canon.Lower Closures

**Purpose:** Implement the canon.lower closure that allows core wasm modules to call imported component functions.

**Files:**
- Modify: `internal/component/component_linker.go`
- Test: Test via real .wasm component instantiation (Task 11)

**Spec reference:** `definitions.py:2064-2130` (canon_lower). Read this section IN FULL. Also read `definitions.py:1943-1975` (lower_flat_values) for the flattening logic.

- [ ] **Step 1: Read the spec for canon_lower**

Read `debug-vendored/component-model/design/mvp/canonical-abi/definitions.py` lines 2064-2130. Understand the full flow: may_leave check, flatten args, call callee, flatten results.

- [ ] **Step 2: Read the existing createCanonLowerFunc**

Read `internal/component/component_linker.go` at `createCanonLowerFunc` (around line 1332). Understand what it currently does and what's missing.

- [ ] **Step 3: Implement or complete buildCanonLowerFunc**

The canon.lower closure must:
1. Receive core wasm flat params (i32/i64/f32/f64)
2. Construct a `LiftContext` for the callee instance
3. Determine if params are flat or spilled to memory (MAX_FLAT_PARAMS = 16)
4. If flat: call `abi.LiftParams` with the flat values
5. If spilled: read from memory via the retptr
6. Invoke the component function from the func index space
7. Lower results via `abi.LowerResults`
8. At entry: `trap_if(not inst.may_leave)` per spec canon_lower line 2065
9. The `may_leave` toggle (false→true around realloc) happens INSIDE `lower_flat_values` (spec lines 1954-1973), NOT at the top level of canon_lower. If `abi.LowerParams`/`abi.LowerResults` already handle this internally, do NOT duplicate the toggle. Read the abi/ code to verify.
10. Return flat core results

Follow the spec line by line. Every branch in the spec's `canon_lower` must have a corresponding code path.

- [ ] **Step 4: Wire into buildCoreHostModule**

In `buildCoreHostModule`, when processing an inline core instance export that references a `canon.lower` entry:

```go
if cli, ok := canonLowers[coreFuncIdx]; ok {
	fn := l.buildCanonLowerFunc(inst, c, cli)
	exports = append(exports, HostModuleExport{
		Name:   exportName,
		Func: fn, // NOTE: verify field name — was "Func" (type api.GoModuleFunc) at compiled.go:31 as of discovery
	})
	continue
}
```

- [ ] **Step 5: Commit**

Commit with spec citations.

---

### Task 8: buildCanonResourceFunc — Resource Operation Host Functions

**Purpose:** Create host functions for canon.resource.new/rep/drop that core wasm modules import.

**Files:**
- Modify: `internal/component/component_linker.go`

**Spec reference:** `definitions.py:2134-2138` (resource_new), `:2169-2173` (resource_rep), `:2142-2165` (resource_drop).

- [ ] **Step 1: Read the spec for all three resource operations**

Read the three spec sections. Each is a small function. Understand the signatures.

- [ ] **Step 2: Implement buildCanonResourceFunc**

For each canon.resource operation kind:

```go
func (l *ComponentLinker) buildCanonResourceFunc(
	inst *Instance,
	c *Component,
	cri canonResourceInfo,
) api.GoModuleFunction {
	// NOTE: verify actual constant names by reading internal/component/component.go.
	// As of discovery, the constants are CanonKindResourceNew, CanonKindResourceRep,
	// CanonKindResourceDrop (with "Kind" infix). Field access is cri.kind (lowercase,
	// same-package). Verify before using.
	switch cri.kind {
	case CanonKindResourceNew:
		// Spec: definitions.py:2134-2138
		// Core signature: (i32 rep) -> i32 handle
		return /* closure that calls inst.ResourceNew(cri.typeIdx, rep) */

	case CanonKindResourceRep:
		// Spec: definitions.py:2169-2173
		// Core signature: (i32 handle) -> i32 rep
		return /* closure that calls inst.ResourceRep(cri.typeIdx, handle) */

	case CanonKindResourceDrop:
		// Spec: definitions.py:2142-2165
		// Core signature: (i32 handle) -> void
		return /* closure that calls inst.ResourceDrop(cri.typeIdx, handle) */
	}
}
```

- [ ] **Step 3: Wire into buildCoreHostModule**

Same pattern as Task 7 — dispatch in `buildCoreHostModule` for canon.resource exports.

- [ ] **Step 4: Commit**

---

### Task 9: Post-Instantiation Memory and Realloc Capture

**Purpose:** After core modules are instantiated, capture their `memory` and `cabi_realloc` exports and back-patch them into canonical closures.

**Files:**
- Modify: `internal/component/component_linker.go` (instantiateCoreModules, buildCanonLiftFunc, buildCanonLowerFunc)

**Spec reference:** Wasmtime `instance.rs` Instantiator::build flow. The canonical ABI requires access to the core module's linear memory for heap lift/lower paths.

- [ ] **Step 1: Determine the memory capture pattern**

Read the existing `instantiateCoreModules` in `component_linker.go`. After each core module is instantiated (line ~332-339), scan its exports:

```go
mod, err := cmi.InstantiateCoreModule(ctx, compiled)
// ...
inst.coreInstances[ciIdx] = mod

// Capture memory and realloc for canonical closures
if mem := mod.ExportedMemory("memory"); mem != nil {
	// Store for use by buildCanonLiftFunc/buildCanonLowerFunc
}
if realloc := mod.ExportedFunction("cabi_realloc"); realloc != nil {
	// Store for use by canonical closures
}
```

- [ ] **Step 2: Design the back-patching mechanism**

The canonical closures (buildCanonLiftFunc, buildCanonLowerFunc) are created BEFORE core modules are instantiated (they're used as host module exports). They need memory/realloc which isn't available until AFTER instantiation.

Solution: use mutable pointers that the closures capture:

```go
type memoryRef struct {
	memory  api.Memory
	realloc api.Function
}

// Closures capture *memoryRef; back-patch after instantiation
ref := &memoryRef{}
// ... create closures that read ref.memory and ref.realloc ...
// ... after instantiation: ref.memory = mod.ExportedMemory("memory") ...
```

- [ ] **Step 3: Implement and test**

Wire the back-patching into `instantiateCoreModules`. Verify by running the existing component examples:

```bash
go test ./examples/component-... -v
```

- [ ] **Step 4: Commit**

---

### Task 10: buildCoreHostModule Dispatch Completion

**Purpose:** Complete the dispatch logic in `buildCoreHostModule` so all inline core instance export types are handled.

**Files:**
- Modify: `internal/component/component_linker.go` (buildCoreHostModule)

- [ ] **Step 1: Read the current buildCoreHostModule**

Read `internal/component/component_linker.go` at `buildCoreHostModule` (line 365). Understand the current dispatch for inline core instance exports.

- [ ] **Step 2: Ensure all export provenance types are handled**

For each export in an inline core instance, it could be:
1. A `canon.lower` reference → `buildCanonLowerFunc` (Task 7)
2. A `canon.resource.*` reference → `buildCanonResourceFunc` (Task 8)
3. An alias to another core instance's export → forward via SourceModule/SourceName
4. A direct core function → forward to owning core module

Verify each case is handled. If any is missing, add it.

- [ ] **Step 3: Test with a real component**

Try instantiating one of the existing `.wasm` test components that previously skipped:

```bash
go test ./internal/component/wasip2test/ -run TestComponentConvert -v
```

If it still fails, trace the error. It likely reveals a missing dispatch case or a back-patching gap.

- [ ] **Step 4: Commit**

---

### Task 11: wireExports Composite Resolution + funcSpace Population

**Purpose:** Fix wireExports so it can resolve core function indices for composite-typed exports (records, options, results).

**Files:**
- Modify: `internal/component/component_linker.go` (wireExports, Instantiate step ordering)

**Spec reference:** The core function index space must include exports from instantiated core modules, not just pre-instantiation aliases.

- [ ] **Step 1: Read wireExports**

Read `internal/component/component_linker.go` at `wireExports` (line 534). Understand how it resolves core function indices via `funcSpace`.

- [ ] **Step 2: Add post-instantiation funcSpace population**

After `instantiateCoreModules` (Step 12 in the 14-step pipeline) and before `wireExports` (Step 14), add a step that walks each core instance's exports and adds them to `funcSpace`:

```go
// Step 12.5 — Populate funcSpace with core module exports
for ciIdx, mod := range inst.coreInstances {
	if mod == nil {
		continue // inline-kind, no module
	}
	for name, def := range mod.ExportedFunctionDefinitions() {
		funcSpace.AddFromCoreInstance(uint32(ciIdx), name, mod.ExportedFunction(name))
		_ = def
	}
	if mem := mod.ExportedMemory("memory"); mem != nil {
		memSpace.AddFromCoreInstance(uint32(ciIdx), "memory", mem)
	}
}
```

The exact API depends on how `CoreFuncIndexSpace` works. Read `internal/component/index_space.go` to understand the interface.

- [ ] **Step 3: Test by unskipping composite tests**

Search for skips mentioning "wireExports" or "core function index":

```bash
grep -rn "t.Skip.*wireExports\|t.Skip.*core function index\|t.Skip.*core func" internal/component/ --include="*_test.go"
```

For each one: remove the skip, run the test, fix any remaining issues.

- [ ] **Step 4: Commit**

---

## Layer 3: Tests

### Task 12: Spectest Runner — assert_invalid

**Purpose:** Implement `assert_invalid` command in the spectest runner, which will unskip the largest category of ResourcesWast tests.

**Files:**
- Modify: `internal/component/spectest/resources_test.go` (or the runner file)

- [ ] **Step 1: Read the current spectest runner**

Read all files in `internal/component/spectest/`. Understand the command dispatch, the `Command` struct, and how `component` commands work.

- [ ] **Step 2: Implement assert_invalid handler**

`assert_invalid` compiles a component and expects compilation to fail:

```go
case "assert-invalid":
	// Compile the component WAT — expect an error
	_, err := compileComponent(cmd.Wat)
	if err == nil {
		t.Errorf("line %d: expected compilation error containing %q, but compilation succeeded", cmd.Line, cmd.Text)
		return
	}
	if cmd.Text != "" && !strings.Contains(err.Error(), cmd.Text) {
		t.Logf("line %d: compilation error %q does not contain expected %q (non-fatal: error text may differ between implementations)", cmd.Line, err.Error(), cmd.Text)
	}
```

Note: the expected error text may differ between wasmtime and wazero. Log a mismatch as informational, not a failure.

- [ ] **Step 3: Remove skips for assert_invalid tests**

Search for skips related to `assert_invalid` or `assert-invalid`:

```bash
grep -rn "t.Skip.*assert" internal/component/spectest/ --include="*_test.go"
```

For each: remove the skip if `assert_invalid` is now handled. Run the test.

- [ ] **Step 4: Run spectest suite**

```bash
go test ./internal/component/spectest/ -v -count=1
```

Count how many now pass vs skip. Fix any compilation errors in the assert_invalid WAT snippets.

- [ ] **Step 5: Commit**

---

### Task 13: Spectest Runner — Remaining Commands

**Purpose:** Implement module, invoke, assert_return, assert_trap, register commands.

**Files:**
- Modify: `internal/component/spectest/` (runner files)

**Dependency:** Tasks 7-11 (Layer 2 pipeline) must be complete for invoke/assert_return/assert_trap.

- [ ] **Step 1: Implement `module` / `module-definition` handler**

```go
case "module", "module-definition":
	// Compile core wasm module, store in registry
	mod, err := rt.CompileModule(ctx, cmd.Wat)
	if err != nil {
		t.Skipf("line %d: module compilation: %v", cmd.Line, err)
		return
	}
	moduleRegistry[cmd.Name] = mod
```

- [ ] **Step 2: Implement `invoke` handler**

```go
case "invoke":
	// Get current instance, call function
	fn := currentInstance.ExportedFunction(cmd.FuncName)
	if fn == nil {
		t.Errorf("line %d: function %q not exported", cmd.Line, cmd.FuncName)
		return
	}
	args := convertArgs(cmd.Args) // convert to []types.Val
	results, err = fn.CallAndPostReturn(ctx, args...)
```

- [ ] **Step 3: Implement `assert_return` handler**

`assert_return` = invoke + compare results.

- [ ] **Step 4: Implement `assert_trap` handler**

`assert_trap` = invoke + assert error.

- [ ] **Step 5: Implement `register` handler**

Store instance under name for imports by later components.

- [ ] **Step 6: Run and fix iteratively**

Run the full spectest suite. For each remaining skip or failure, trace the root cause. Fix it. Repeat until the spectest suite is clean.

- [ ] **Step 7: Commit**

---

### Task 14: Port Wasmtime Sync WAT Tests

**Purpose:** Add wasmtime's sync WAT test files to the spectest runner.

**Files:**
- Modify: `internal/component/spectest/` (add test driver for each WAT file)
- Reference: `debug-vendored/wasmtime/tests/misc_testsuite/component-model/`

- [ ] **Step 1: List all sync WAT files**

```bash
ls debug-vendored/wasmtime/tests/misc_testsuite/component-model/*.wast | grep -v async
```

- [ ] **Step 2: For each sync WAT file, add a test function**

For each file (simple.wast, resources.wast, types.wast, enums.wast, nested.wast, linking.wast, import.wast, modules.wast, aliasing.wast, tags.wast, enum_discriminant.wast, fixed_length_lists.wast):

1. Read the file to understand what commands it uses
2. Add a test function that runs the spectest runner against it
3. Run the test
4. If commands are unsupported (e.g., `tags.wast` uses exception handling), skip with a documented reason

- [ ] **Step 3: Run all spectest files**

```bash
go test ./internal/component/spectest/ -v -count=1
```

- [ ] **Step 4: Commit**

---

### Task 15: Unskip All Tests — Exhaustive Process

**Purpose:** Find and address every remaining `t.Skip` in the component test suite.

**Files:** All test files under `internal/component/`

**CRITICAL: This task uses a process, not a target count.**

- [ ] **Step 1: Find ALL remaining skips**

```bash
grep -rn "t\.Skip" internal/component/ --include="*_test.go" | grep -v "testcomponents/" | sort
```

Save this list. It is the work queue.

- [ ] **Step 2: For EACH skip in the list, follow this process**

For each line in the grep output:

1. **Read the skip message** — what does it say?
2. **Read the test body** — what does the test exercise?
3. **Determine the root cause** — why is it skipped?
4. **Check if the root cause is fixed** by Layers 1-2:
   - "complex component pipelines" → try removing the skip, run the test
   - "wireExports" → try removing the skip after Task 11
   - "handle params" / "list params" → try removing the skip after Task 6
   - "CallContext" → check if the two-phase protocol (Task 5) addresses it
   - "async" / "subtask" / "stream" / "future" → KEEP the skip, add spec citation
5. **If the test passes:** remove the skip permanently
6. **If the test fails with a new error:** investigate. It's either:
   - A defect in Layers 1-2 → fix it
   - A missing feature → if async, keep skip with citation. If sync, fix it.
7. **If keeping a skip:** update the message to include a spec citation:
   ```go
   t.Skip("async: stream/future/error-context types not implemented (spec definitions.py TypeKindStream/TypeKindFuture/TypeKindErrorContext — deferred, no session scheduled)")
   ```

- [ ] **Step 3: Verify zero unaddressed skips remain**

After processing every entry:

```bash
grep -rn "t\.Skip" internal/component/ --include="*_test.go" | grep -v "testcomponents/" | grep -v "async\|stream\|future\|subtask\|error.context"
```

This should return ZERO results. If any remain, go back to Step 2 for those entries.

- [ ] **Step 4: Count and report final state**

```bash
echo "Total remaining skips:"
grep -rn "t\.Skip" internal/component/ --include="*_test.go" | grep -v "testcomponents/" | wc -l
echo "Of which async deferrals:"
grep -rn "t\.Skip" internal/component/ --include="*_test.go" | grep -v "testcomponents/" | grep -c "async\|stream\|future\|subtask\|error.context"
```

- [ ] **Step 5: Run full test suite**

```bash
go test ./internal/component/... -count=1 -v 2>&1 | grep -E "PASS|FAIL|SKIP" | sort | uniq -c | sort -rn | head -20
```

Expected: ZERO failures. Every SKIP has an async spec citation.

- [ ] **Step 6: Commit**

---

### Task 16: Verify Examples + Conformance Coverage

**Purpose:** Ensure the 4 component examples still work and conformance coverage is complete.

**Files:** Examples under `examples/component-*/`, conformance tests under `internal/component/conformance/`

- [ ] **Step 1: Run all component examples**

```bash
go test ./examples/component-basic/ -v
go test ./examples/component-host-functions/ -v
go test ./examples/component-types/ -v
go test ./examples/component-wasip2/ -v
```

All must pass. If any fail due to public API changes (e.g., `[]any` → `[]Val`), fix the examples.

- [ ] **Step 2: Verify conformance coverage against run_tests.py sync categories**

Read `debug-vendored/component-model/design/mvp/canonical-abi/run_tests.py` and list the sync test categories:
- `test_roundtrips` (scalars, composites)
- `test_nan32` / `test_nan64`
- `test_string`
- `test_heap`
- `test_flatten`
- `test_handles`

For each category, verify a corresponding wazero conformance test exists:

```bash
grep -rn "TestPrimitives\|TestComposites\|TestStrings\|TestFlat\|TestResources\|TestNaN" internal/component/conformance/ --include="*_test.go"
```

If any category has no equivalent, add a conformance test.

- [ ] **Step 3: Run all conformance tests**

```bash
go test ./internal/component/conformance/ -v -count=1
```

- [ ] **Step 4: Run wasip2test suite**

```bash
go test ./internal/component/wasip2test/ -v -count=1 2>&1 | tail -30
```

- [ ] **Step 5: Run the ENTIRE test suite**

```bash
go test ./... 2>&1 | grep -E "^ok|^FAIL" | sort
```

Expected: no FAIL lines (except debug-vendored/ which has known external dependency issues).

- [ ] **Step 6: Commit any fixes**

---

## Layer 4: Public API

**Note on Layer 4 ordering:** Task 20 (ComponentFunc evolution from `[]any` to `[]Val`) should be done FIRST in Layer 4 because it changes the public interface that other Layer 4 tasks build on. If examples break, fix them immediately in Task 20 before proceeding.

### Task 17: Type Introspection — FuncType and ValType

**Purpose:** Add public type introspection to `api/component/` matching wasmtime C API.

**Files:**
- Create: `api/component/types.go`
- Modify: `api/component/component.go`
- Test: `api/component/types_test.go`

**Reference:** Wasmtime C API `types/val.h`, `types/func.h`.

- [ ] **Step 1: Read the internal types**

Read `internal/component/types/composite.go` for `TypeFunc`, and `internal/component/types/types.go` for `ValType`. Understand the existing internal API.

- [ ] **Step 2: Create public wrapper types**

Create `api/component/types.go` with public wrappers that don't expose internals:

```go
package component

import "github.com/tetratelabs/wazero/internal/component/types"

// Param represents a named function parameter or result.
type Param struct {
	Name string
	Type ValType
}

// Field represents a named record field.
type Field struct {
	Name string
	Type ValType
}

// Case represents a variant case with an optional payload type.
type Case struct {
	Name string
	Type *ValType // nil if no payload
}

// FuncTypeInfo provides introspection into a component function's type.
type FuncTypeInfo struct {
	inner *types.TypeFunc
	types *types.ComponentTypes
}

func (f FuncTypeInfo) NumParams() int { return len(f.inner.Params) }
func (f FuncTypeInfo) NumResults() int { return len(f.inner.Results) }
// ... Params(), Results() methods that convert internal types to public Param
```

- [ ] **Step 3: Add ValType introspection methods**

Methods on a `ValTypeInfo` wrapper: `Kind()`, `ListElement()`, `RecordFields()`, `TupleTypes()`, `VariantCases()`, `EnumCases()`, `OptionSome()`, `ResultOk()`, `ResultErr()`, `FlagsNames()`.

Each method reads from the `ComponentTypes` bag using the type index.

**Naming note:** Task 20's `ComponentFunc.Type()` returns `FuncTypeInfo`. Use `FuncTypeInfo` consistently — do not use bare `FuncType` since that's already an alias for `types.TypeFunc` in `api/component/component.go`.

- [ ] **Step 4: Write tests**

- [ ] **Step 5: Commit**

---

### Task 18: Component-Level Import/Export Introspection

**Purpose:** Expose component imports and exports through the public API.

**Files:**
- Modify: `api/component/component.go`
- Reference: Existing `CompiledComponent.Imports()`/`Exports()` at `internal/component/compiled.go`

- [ ] **Step 1: Check existing API**

Read `internal/component/compiled.go` at `Imports()` (line 97) and `Exports()` (line 109). Read the return types `api.ComponentImport` and `api.ComponentExport`. Verify they have sufficient fields (name, kind).

- [ ] **Step 2: Expose through api/component/ if not already exposed**

If these are already accessible through the public `CompiledComponent` wrapper, add type introspection fields (associated `FuncType` for function exports).

- [ ] **Step 3: Test and commit**

---

### Task 19: Resource API Surface

**Purpose:** Add ResourceType, ResourceHandle, and resource operations to the public API.

**Files:**
- Modify: `api/component/component.go`
- Test: `api/component/component_test.go` (if exists, or create)

**Reference:** Wasmtime C API `val.h` (resource_any_t, resource_host_t).

- [ ] **Step 1: Add ResourceType public type**

```go
// ResourceType identifies a resource type with pointer identity.
// Two ResourceTypes are equal iff they refer to the same underlying
// resource declaration from the same component instantiation.
// Spec: definitions.py:351-361.
type ResourceType struct {
	inner *runtime.ResourceType
}

func (r ResourceType) Equal(other ResourceType) bool {
	return r.inner == other.inner
}
```

- [ ] **Step 2: Add ResourceHandle public type**

Per the design spec 4B, add a `ResourceHandle` type that covers wasmtime's `resource_any_t` use case:

```go
// ResourceHandle represents an opaque resource handle (guest or host).
type ResourceHandle struct {
	rt    ResourceType
	owned bool
	rep   uint32
}

func (h ResourceHandle) Type() ResourceType { return h.rt }
func (h ResourceHandle) Owned() bool        { return h.owned }
func (h ResourceHandle) Rep() uint32        { return h.rep }
```

- [ ] **Step 3: Add resource operations**

Expose `ResourceNew`, `ResourceRep`, `ResourceDrop` on the public Instance type, and `GetResource` for resolving exported resource types.

- [ ] **Step 4: Test and commit**

---

### Task 20: Post-Return Public API + ComponentFunc Evolution

**Purpose:** Evolve `api.ComponentFunc` to use `[]Val` and expose `PostReturn`.

**Files:**
- Modify: `api/component.go` (ComponentFunc interface)
- Modify: All 4 examples that use ComponentFunc
- Test: Verify examples still compile and pass

**CAUTION:** This changes a public interface. Update ALL callers.

- [ ] **Step 1: Read the current ComponentFunc interface**

Read `api/component.go` at line 104-119. Note the current `Call(ctx, ...any) ([]any, error)` signature.

- [ ] **Step 2: Change the interface**

```go
type ComponentFunc interface {
	Call(ctx context.Context, params ...Val) ([]Val, error)
	PostReturn(ctx context.Context) error
	CallAndPostReturn(ctx context.Context, params ...Val) ([]Val, error)
	Type() FuncType
}
```

Where `Val = types.Val` and `FuncType` is the public wrapper from Task 17.

- [ ] **Step 3: Find and update ALL callers**

```bash
grep -rn "ComponentFunc\|\.Call(" examples/component-* api/ --include="*.go"
```

Update each caller to use `Val` types instead of `any`.

- [ ] **Step 4: Verify all examples pass**

```bash
go test ./examples/component-... -v
```

- [ ] **Step 5: Commit**

---

### Task 21: InstancePre

**Purpose:** Add pre-computed instantiation pattern.

**Files:**
- Create: `internal/component/instance_pre.go`
- Modify: `internal/component/component_linker.go`
- Expose in: `api/component/component.go`
- Test: `internal/component/instance_pre_test.go`

**Reference:** Wasmtime `instance.rs` InstancePre.

- [ ] **Step 1: Factor out import resolution from Instantiate**

Read `ComponentLinker.Instantiate` and identify the import-resolution + type-checking steps (Steps 3-4 in the 14-step pipeline). The existing code already uses `resolvedImports` as a local `map[string]Definition` — do NOT create a new struct type. Instead, extract into a helper that returns the same types the existing code uses:

```go
func (l *ComponentLinker) resolveImports(c *Component) (
	resolvedImports map[string]Definition,
	instanceToImport map[uint32]string,
	tc *TypeChecker,
	err error,
) {
	// Extract Steps 3-4 from Instantiate
	tc = NewTypeChecker(c)
	resolvedImports = make(map[string]Definition)
	instanceToImport = make(map[uint32]string)
	if err = l.resolveAndCheckImports(c, tc, resolvedImports, instanceToImport); err != nil {
		return nil, nil, nil, err
	}
	return
}
```

- [ ] **Step 2: Create InstancePre**

```go
type InstancePre struct {
	linker   *ComponentLinker
	compiled *CompiledComponent
	resolved *resolvedImports
}

func (l *ComponentLinker) InstantiatePre(compiled *CompiledComponent) (*InstancePre, error) {
	c := compiled.Internal()
	resolved, err := l.resolveImports(c)
	if err != nil {
		return nil, err
	}
	return &InstancePre{linker: l, compiled: compiled, resolved: resolved}, nil
}

func (ip *InstancePre) Instantiate(ctx context.Context) (*Instance, error) {
	// Steps 5-14 from the original Instantiate, using ip.resolved
}
```

- [ ] **Step 3: Update Instantiate to use the helper**

```go
func (l *ComponentLinker) Instantiate(ctx context.Context, compiled *CompiledComponent) (*Instance, error) {
	pre, err := l.InstantiatePre(compiled)
	if err != nil {
		return nil, err
	}
	return pre.Instantiate(ctx)
}
```

- [ ] **Step 4: Write tests verifying reuse**

```go
func TestInstancePre_ReusedInstantiation(t *testing.T) {
	pre, err := linker.InstantiatePre(compiled)
	require.NoError(t, err)

	inst1, err := pre.Instantiate(ctx)
	require.NoError(t, err)

	inst2, err := pre.Instantiate(ctx)
	require.NoError(t, err)

	// Two distinct instances
	require.NotEqual(t, inst1, inst2)
}
```

- [ ] **Step 5: Expose in public API and commit**

---

### Task 22: Export Access + DefineUnknownImportsAsTraps

**Purpose:** Expose export access methods and add the linker convenience API.

**Files:**
- Modify: `api/component/component.go`
- Modify: `internal/component/component_linker.go`

- [ ] **Step 1: Verify ExportedFunction and ExportedInstance are exposed**

Check if the public API wraps `Instance.ExportedFunction()` and `Instance.ExportedInstance()`. If not, add public wrappers.

- [ ] **Step 2: Add ExportedFunctions() iterator**

```go
func (i *Instance) ExportedFunctions() map[string]ComponentFunc {
	// Return a copy of the exports map
}
```

- [ ] **Step 3: Implement DefineUnknownImportsAsTraps**

Add to `ComponentLinker`:

```go
func (l *ComponentLinker) DefineUnknownImportsAsTraps() {
	l.trapUnknownImports = true
}
```

In `resolveAndCheckImports`, when an import is not found and `l.trapUnknownImports` is true:

```go
if !found && l.trapUnknownImports {
	resolvedImports[key] = &FuncDef{
		Callback: func(ctx context.Context, ft *types.TypeFunc, args []types.Val) ([]types.Val, error) {
			return nil, fmt.Errorf("trap: import %q not defined", key)
		},
	}
	continue
}
```

- [ ] **Step 4: Write test for DefineUnknownImportsAsTraps**

```go
func TestDefineUnknownImportsAsTraps(t *testing.T) {
	linker := NewComponentLinker(nil)
	linker.DefineUnknownImportsAsTraps()

	// Compile a component that imports a function we haven't defined
	// The instantiation should succeed (unknown import stubbed with trap)
	// but calling the stubbed function should return an error
	// containing "trap: import ... not defined"
}
```

The exact test depends on having a compiled component with an unsatisfied import. Use one of the existing test components or create a minimal WAT fixture.

- [ ] **Step 5: Commit**

---

### Task 23: Documentation Updates

**Purpose:** Update documentation to reflect all Session 2 changes.

**Files:**
- Modify: `api/component/component.go` (godoc)
- Modify: Example files if API changed
- Modify: Internal package comments referencing "Session 2 deferred" that are now resolved

- [ ] **Step 1: Find all "Session 2" references in code comments**

```bash
grep -rn "Session 2\|session 2\|session2" internal/component/ --include="*.go" | grep -v _test.go | grep -v debug-vendored
```

For each: if the referenced work is now done, update or remove the comment.

- [ ] **Step 2: Update godoc on public API types**

Ensure `api/component/component.go` has clear godoc for:
- `Val`, `TypeFunc`, `ValKind` (already exist)
- `ResourceType`, `ResourceHandle` (new)
- `FuncTypeInfo`, `ValTypeInfo` (new)
- `InstancePre` (new)
- `ComponentFunc` (evolved)
- `DefineUnknownImportsAsTraps` (new)

- [ ] **Step 3: Verify examples have updated comments**

Read each example's `main.go` or test file. Ensure they demonstrate the current API, not a stale one.

- [ ] **Step 4: Commit**

```bash
git commit -m "docs: update documentation for Session 2 changes"
```

---

## Final Verification

### Task 24: Full Test Suite + Demonstration

**Purpose:** Run the complete test suite and verify all acceptance criteria.

- [ ] **Step 1: Run go test ./...**

```bash
go test ./... 2>&1 | grep -E "^ok|^FAIL" | sort
```

Expected: ZERO FAIL lines for non-debug-vendored packages.

- [ ] **Step 2: Count remaining skips**

```bash
go test ./internal/component/... -v 2>&1 | grep -c "SKIP"
```

Report the count. Every skip must have an async spec citation.

- [ ] **Step 3: Verify acceptance criteria checklist**

1. ✅ `go test ./...` passes with zero failures
2. ✅ Real .wasm integration tests running
3. ✅ Spectest runner passing sync commands
4. ✅ Cross-instance resources working end-to-end
5. ✅ Post-return two-phase protocol exposed
6. ✅ Public API covers: type introspection, resources, post-return, InstancePre, exports
7. ✅ No workarounds, shims, or duplicate types
8. ✅ WASI P2 interfaces exercised
9. ✅ 4 component examples passing
10. ✅ may_enter enforced
11. ✅ lower_borrow optimization implemented
12. ✅ Documentation updated

- [ ] **Step 4: Final commit and tag**

```bash
git commit --allow-empty -m "session 2: complete — all acceptance criteria verified"
```
