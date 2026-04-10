// internal/component/instance.go
//
// Session 1 Task B3: Instance embeds *runtime.ComponentInstance.
//
// Per-instance runtime state matching the canonical-abi spec's
// ComponentInstance (definitions.py:256-273) lives on the embedded
// *runtime.ComponentInstance. Wrapper-level state (core module instances,
// component-level exports, linker-time index spaces) stays on this struct
// because runtime/ cannot import component/ without an import cycle.
//
// Design: docs/superpowers/specs/2026-04-08-canonical-abi-session1-design.md
//   Decision 3 (design lines 185-253); Instance Layering After (760-827).
// Spec: definitions.py:256-273 (class ComponentInstance).
// Wasmtime parallel: runtime/component/instance.rs:710-833 (Instantiator).
package component

import (
	"context"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
)

// callerInstanceKey is the context key for the caller instance.
type callerInstanceKey struct{}

// GetCallerInstance retrieves the caller instance from context.
// Returns nil if called from host (no caller in context).
func GetCallerInstance(ctx context.Context) *Instance {
	if caller, ok := ctx.Value(callerInstanceKey{}).(*Instance); ok {
		return caller
	}
	return nil
}

// WithCallerInstance returns a context with the caller instance set.
func WithCallerInstance(ctx context.Context, caller *Instance) context.Context {
	return context.WithValue(ctx, callerInstanceKey{}, caller)
}

// Instance is the linker/compile-time wrapper around a running component
// instantiation. Per-instance runtime state matching the canonical-abi
// spec's ComponentInstance (definitions.py:256-273) lives on the embedded
// *runtime.ComponentInstance. Wrapper-level state (core module instances,
// component-level exports, linker-time index spaces) stays on this struct
// because runtime/ cannot import component/ without an import cycle.
//
// Session 1 design: Decision 3 (design lines 185-253).
type Instance struct {
	// rt is the per-instance runtime state. One-to-one with this Instance
	// and non-nil after newInstance.
	rt *runtime.ComponentInstance

	// Linker-time state.
	component      *Component
	coreInstances  []api.Module
	exports        map[string]*ExportedFunc
	componentFuncs map[uint32]ComponentFunc

	// Value index space for start function support.
	values         []types.Val
	valuesConsumed []bool

	// Wrapper-layer instance tree. rt.Parent holds the runtime-layer
	// back-pointer; parent / children hold *component.Instance wrapper
	// pointers so linker code can navigate without going through rt.
	parent   *Instance
	children []*Instance

	// Index spaces for nested component support.
	instanceSpace  []*Instance
	typeSpace      []*TypeDef
	componentSpace []*Component

	// Exported instances for API access.
	exportedInstances map[string]*Instance

	// activeCtx holds the Go context from the current GoModuleFunc invocation.
	// Set by canon.resource.drop (and other GoModuleFuncs) before calling into
	// ResourceDrop, so that HostDestructor closures can use the caller's context
	// instead of context.Background(). Reset to nil after the call completes.
	activeCtx context.Context
}

// newInstance constructs an Instance together with its embedded
// *runtime.ComponentInstance. If parent is non-nil its rt is used as the
// runtime-layer parent pointer; the wrapper-layer parent is set on this
// Instance as well so linker traversal stays within the component/ package.
//
// Spec: definitions.py:256-273 (ComponentInstance shape).
// Design: Decision 3 (design lines 185-253).
func newInstance(c *Component, id uint32, parent *Instance) *Instance {
	var parentRT *runtime.ComponentInstance
	if parent != nil {
		parentRT = parent.rt
	}
	return &Instance{
		component:      c,
		rt:             runtime.NewComponentInstance(id, parentRT),
		coreInstances:  make([]api.Module, 0),
		exports:        make(map[string]*ExportedFunc),
		componentFuncs: make(map[uint32]ComponentFunc),
		parent:         parent,
	}
}

// NewInstance is the exported constructor for tests and external packages
// that need a properly-wired Instance (with its embedded
// *runtime.ComponentInstance). Production code inside the component
// package should use newInstance; this wrapper exists so conformance
// tests and other sibling packages can migrate off bare
// `&component.Instance{}` struct literals, which no longer have a
// zero-value runtime layer.
//
// Session 1 Task B4.
func NewInstance(c *Component, id uint32, parent *Instance) *Instance {
	return newInstance(c, id, parent)
}

// ComponentFunc represents a callable component-level function.
type ComponentFunc struct {
	// Type is the component function type. The old *FuncType shape has
	// been unified on *types.TypeFunc (Design Decision 9).
	Type *types.TypeFunc

	// Impl is the actual callable. Under Task C3's wasmtime func_new
	// dynamic-host model, Impl carries the HostFunc signature — the
	// second parameter is the component-declared type supplied by the
	// runtime at call time (read from the Type field above).
	// For imports: the HostFunc from the Definition's Callback.
	// For canon lift: the lifted core function.
	Impl HostFunc
}

// GetComponentFunc looks up a component function by its index.
func (i *Instance) GetComponentFunc(funcIdx uint32) (ComponentFunc, bool) {
	if i.componentFuncs == nil {
		return ComponentFunc{}, false
	}
	f, ok := i.componentFuncs[funcIdx]
	return f, ok
}

// Component returns the component this instance was created from.
func (i *Instance) Component() *Component {
	return i.component
}

// ExportedFunction returns the exported function with the given name,
// or nil if not found.
func (i *Instance) ExportedFunction(name string) *ExportedFunc {
	if i.exports == nil {
		return nil
	}
	return i.exports[name]
}

// postReturnState is a shared mutable reference between the HostFunc
// closure built by buildCanonLiftFunc and the ExportedFunc that wraps
// it. The closure writes the post-return function after each Call;
// ExportedFunc.PostReturn reads and invokes it.
//
// This indirection exists because buildCanonLiftFunc creates the
// closure before the ExportedFunc is constructed by wireExportedFunc,
// so the closure cannot capture the ExportedFunc pointer directly.
type postReturnState struct {
	fn              func(ctx context.Context) error
	needsPostReturn bool
}

// ExportedFunc represents an exported component function.
//
// C8-b Option A: the wrapper carries a HostFunc closure (`impl`) built
// by wireExports from either buildCanonLiftFunc (for canon.lift exports)
// or directly from the component function index space (for imported
// function re-exports). Call simply invokes it — no per-call
// reconstruction of the lift context.
//
// The legacy linker-time fields (coreFunc, canonical, memory,
// reallocFunc, postReturnFunc) were removed: every call-site that
// touched them has been migrated to the closure.
//
// Two-phase post-return protocol (Task 5):
// For canon.lift exports, Call returns results without running the
// post-return function. The caller must invoke PostReturn before
// calling again. CallAndPostReturn is the convenience single-shot API.
// For imported-function re-exports, prRef is nil and PostReturn is a
// no-op.
type ExportedFunc struct {
	name      string
	funcType  *types.TypeFunc
	component *Component
	instance  *Instance
	impl      HostFunc

	// prRef is the shared post-return state written by the HostFunc
	// closure and read by PostReturn. Nil for non-canon.lift exports.
	prRef *postReturnState
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
//
// For canon.lift exports (prRef != nil), Call returns the lifted
// results WITHOUT running the post-return function. The caller MUST
// invoke PostReturn before calling again; calling Call while a
// post-return is pending panics per spec definitions.py:1999-2002.
//
// For imported-function re-exports (prRef == nil), Call behaves as
// a single-shot invocation with no post-return phase.
//
// C8-b: delegates to the per-export HostFunc closure populated by
// wireExports. For canon.lift exports this is the closure built by
// buildCanonLiftFunc (spec canon_lift at definitions.py:1978-2040);
// for imported-function re-exports it is the component function's own
// Impl.
func (f *ExportedFunc) Call(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	if f == nil {
		return nil, fmt.Errorf("ExportedFunc.Call: nil receiver")
	}
	if f.impl == nil {
		return nil, fmt.Errorf("ExportedFunc.Call: %q has no impl (wireExports did not populate it)", f.name)
	}
	// Spec: definitions.py:1999-2002 — cannot re-enter while post-return
	// is pending. This is a programming error (not a runtime trap) so we
	// panic rather than returning an error.
	if f.prRef != nil && f.prRef.needsPostReturn {
		panic("ExportedFunc.Call: PostReturn must be called before calling again (spec: definitions.py:1999-2002)")
	}
	return f.impl(ctx, f.funcType, params)
}

// PostReturn runs the deferred post-return phase for a prior Call.
//
// For canon.lift exports this invokes the core post-return function
// (if declared) and releases the borrow scope / call context. For
// non-canon.lift exports (prRef == nil) this is a no-op.
//
// Spec: definitions.py:2000-2002 — post_return invocation.
// Wasmtime C API: wasmtime_component_func_post_return.
func (f *ExportedFunc) PostReturn(ctx context.Context) error {
	if f == nil {
		return fmt.Errorf("ExportedFunc.PostReturn: nil receiver")
	}
	if f.prRef == nil || !f.prRef.needsPostReturn {
		return nil // no post-return needed
	}
	f.prRef.needsPostReturn = false
	if f.prRef.fn != nil {
		err := f.prRef.fn(ctx)
		f.prRef.fn = nil
		return err
	}
	return nil
}

// CallAndPostReturn is a convenience method that calls Call and then
// PostReturn in sequence. This is the single-shot API for callers
// that do not need to inspect results before post-return cleanup.
//
// This is the recommended default for most callers. The two-phase
// protocol (Call + PostReturn) is for advanced use cases where the
// caller needs to read results from linear memory before cleanup.
func (f *ExportedFunc) CallAndPostReturn(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	results, err := f.Call(ctx, params...)
	if err != nil {
		// On call error, still attempt PostReturn to clean up any
		// partial state (the impl may have set needsPostReturn before
		// the error occurred).
		_ = f.PostReturn(ctx)
		return nil, err
	}
	if postErr := f.PostReturn(ctx); postErr != nil {
		return results, postErr
	}
	return results, nil
}

// NeedsPostReturn reports whether a PostReturn call is pending.
func (f *ExportedFunc) NeedsPostReturn() bool {
	if f == nil || f.prRef == nil {
		return false
	}
	return f.prRef.needsPostReturn
}

// Type returns the function's type.
func (f *ExportedFunc) Type() *types.TypeFunc {
	return f.funcType
}


// --- Resource management surface (per-instance) --------------------------
//
// These wrap the per-instance runtime.Table with the legacy Instance-level
// resource.new / resource.rep / resource.drop entry points. Session 1 folds
// these into the unified runtime.ComponentInstance + abi.LiftContext path.

// ResourceNew is canon.resource.new — spec definitions.py:2134-2138.
//
//	def canon_resource_new(rt, thread, rep):
//	  trap_if(not thread.task.inst.may_leave)
//	  h = ResourceHandle(rt, rep, own = True)
//	  i = thread.task.inst.table.add(h)
//	  return [i]
//
// Wasmtime parallel: runtime/vm/component/resources.rs resource_new32.
func (i *Instance) ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error) {
	// Spec: definitions.py:2135 — trap_if(not may_leave)
	if !i.rt.IsMayLeave() {
		return 0, errMayNotLeave
	}
	if int(resourceIdx) >= len(i.rt.ResourceTypes) {
		return 0, fmt.Errorf("resource.new: resource declaration %d not defined", resourceIdx)
	}
	rt := i.rt.ResourceTypes[resourceIdx]
	if rt == nil {
		return 0, fmt.Errorf("resource.new: resource type %d not concrete", resourceIdx)
	}
	// Spec: definitions.py:2136-2137 — create own handle, add to table.
	h, err := i.rt.Table.NewResourceHandle(rep, true, rt)
	if err != nil {
		return 0, err
	}
	return h.Index(), nil
}

// ResourceRep is canon.resource.rep — spec definitions.py:2169-2173.
//
//	def canon_resource_rep(rt, thread, i):
//	  h = thread.task.inst.table.get(i)
//	  trap_if(not isinstance(h, ResourceHandle))
//	  trap_if(h.rt is not rt)
//	  return [h.rep]
//
// Wasmtime parallel: runtime/vm/component/resources.rs resource_rep.
func (i *Instance) ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error) {
	if int(resourceIdx) >= len(i.rt.ResourceTypes) {
		return 0, fmt.Errorf("resource.rep: resource declaration %d not defined", resourceIdx)
	}
	rt := i.rt.ResourceTypes[resourceIdx]
	if rt == nil {
		return 0, fmt.Errorf("resource.rep: resource type %d not concrete", resourceIdx)
	}
	// Spec: definitions.py:2170 — h = table.get(i)
	// Use GetByIndex to bridge the Wasm-side u32 index to the runtime's
	// 64-bit generation-tagged Handle.
	_, entry, err := i.rt.Table.GetByIndex(handleIdx)
	if err != nil {
		return 0, err
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return 0, runtime.ErrInvalidHandle
	}
	// Spec: definitions.py:2172 — trap_if(h.rt is not rt)
	if resEntry.RT != rt {
		return 0, fmt.Errorf("resource.rep: type mismatch")
	}
	// Spec: definitions.py:2173 — return [h.rep]
	return resEntry.Rep, nil
}

// ResourceDrop is canon.resource.drop — spec definitions.py:2142-2165.
//
//	def canon_resource_drop(rt, thread, i):
//	  trap_if(not thread.task.inst.may_leave)
//	  inst = thread.task.inst
//	  h = inst.table.remove(i)
//	  trap_if(not isinstance(h, ResourceHandle))
//	  trap_if(h.rt is not rt)
//	  trap_if(h.num_lends != 0)
//	  if h.own:
//	    assert(h.borrow_scope is None)
//	    if inst is rt.impl:
//	      if rt.dtor: rt.dtor(h.rep)
//	    else:
//	      if rt.dtor: [...cross-instance...]
//	      else: trap_if(call_might_be_recursive(thread.task, rt.impl))
//	  else:
//	    h.borrow_scope.num_borrows -= 1
//	  return []
//
// Wasmtime parallel: runtime/vm/component/resources.rs resource_drop.
func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error {
	// Spec: definitions.py:2143 — trap_if(not may_leave)
	if !i.rt.IsMayLeave() {
		return errMayNotLeave
	}
	if int(resourceIdx) >= len(i.rt.ResourceTypes) {
		return fmt.Errorf("resource.drop: resource declaration %d not defined", resourceIdx)
	}
	rt := i.rt.ResourceTypes[resourceIdx]
	if rt == nil {
		return fmt.Errorf("resource.drop: resource type %d not concrete", resourceIdx)
	}
	// Spec: definitions.py:2145 — h = inst.table.remove(i)
	// Implementation note: we GetByIndex first for type checking, then Remove.
	// This reorders relative to the spec (which removes before type-checking)
	// but has the same observable behavior since all failure paths trap.
	h, entry, err := i.rt.Table.GetByIndex(handleIdx)
	if err != nil {
		return err
	}
	resEntry, ok := entry.(*runtime.ResourceHandleEntry)
	if !ok {
		return runtime.ErrInvalidHandle
	}
	// Spec: definitions.py:2147 — trap_if(h.rt is not rt)
	if resEntry.RT != rt {
		return fmt.Errorf("resource.drop: type mismatch")
	}
	// Spec: definitions.py:2145+2148 — remove + trap_if(h.num_lends != 0)
	// Table.Remove validates NumLends==0 before removing.
	if _, err := i.rt.Table.Remove(h); err != nil {
		return err
	}
	if resEntry.Own {
		// Spec: definitions.py:2149-2162 — own branch: invoke destructor.
		if rt.Impl == i.rt {
			// Same instance — invoke local destructor.
			// Spec: definitions.py:2151-2153
			if rt.HasDestructor() {
				if err := invokeLocalDestructor(i, rt, resEntry.Rep); err != nil {
					return fmt.Errorf("resource.drop: destructor: %w", err)
				}
			}
		} else {
			// Cross-instance: invoke destructor on the defining instance.
			// Spec: definitions.py:2154-2160 (canon_lift/canon_lower path)
			//
			// For the trivially flat (u32) -> () destructor signature, the
			// canon_lift/canon_lower simplifies to a reentrance check on
			// the defining instance followed by a direct call. No memory,
			// realloc, or borrow scoping is needed.
			if rt.HostDestructor != nil {
				// Spec: canon_lift at :1979 — trap_if(call_might_be_recursive)
				// on the defining instance before invoking the destructor.
				if rt.Impl.Reentrance.CallMightBeRecursive(rt.Impl.ID) {
					return errReentrance
				}
				if err := rt.HostDestructor(resEntry.Rep); err != nil {
					return fmt.Errorf("resource.drop: cross-instance destructor: %w", err)
				}
			} else {
				// Spec: definitions.py:2162 — trap_if(call_might_be_recursive(...))
				// No destructor: just check reentrance.
				definingInst := getDefiningInstance(i, rt)
				if definingInst != nil && definingInst.CallMightBeRecursive(i) {
					return errReentrance
				}
			}
		}
	}
	if !resEntry.Own {
		// Spec: definitions.py:2163-2164 — borrow branch.
		// Decrement the call context's borrow counter. The context checks this at
		// exit_call to ensure all borrows were dropped before returning.
		if resEntry.CallContext != nil {
			resEntry.CallContext.DecrementBorrows()
		}
	}
	return nil
}

// invokeLocalDestructor invokes a resource destructor on the defining
// instance. For host-declared resources, rt.HostDestructor is a Go closure.
// For guest-declared resources, rt.Dtor is a core function index that
// requires the core function index space for resolution.
//
// Spec: definitions.py:2151-2153
//
//	if inst is rt.impl:
//	  if rt.dtor:
//	    rt.dtor(h.rep)
func invokeLocalDestructor(inst *Instance, rt *runtime.ResourceType, rep uint32) error {
	if rt.HostDestructor != nil {
		return rt.HostDestructor(rep)
	}
	if rt.Dtor != nil {
		return fmt.Errorf(
			"invokeLocalDestructor: guest destructor at core function index %d: "+
				"guest destructors require core function index space resolution (Session 2 wiring)",
			*rt.Dtor,
		)
	}
	// No destructor declared — nothing to do.
	return nil
}

// getDefiningInstance resolves the *Instance wrapper for the component
// instance that defines the given resource type. Uses the store-wide
// ResourceStore to look up the instance by its runtime ID (rt.Impl.ID).
// Returns nil if the store is not wired or the instance is not registered.
//
// Spec: definitions.py:2154-2162 — the cross-instance destructor and
// reentrance check paths need the defining instance.
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

// Runtime returns the embedded *runtime.ComponentInstance.
func (i *Instance) Runtime() *runtime.ComponentInstance { return i.rt }

// Table returns the per-instance runtime handle table.
//
// Spec: definitions.py:259, class Table at :303-315.
func (i *Instance) Table() *runtime.Table { return i.rt.Table }

// MayLeave reports the spec may_leave flag. Spec: definitions.py:260.
func (i *Instance) MayLeave() bool { return i.rt.IsMayLeave() }

// SetMayLeave writes the spec may_leave flag.
// Spec: definitions.py:1955, :1973 (lower_flat_values toggles may_leave).
func (i *Instance) SetMayLeave(allowed bool) { i.rt.MayLeave = allowed }

// ActiveCallDepth returns the current reentrance nesting count. Nil-safe
// so host callers that hold a nil *Instance (no instance on the context)
// can query depth without special-casing.
func (i *Instance) ActiveCallDepth() int {
	if i == nil || i.rt == nil {
		return 0
	}
	return i.rt.EnterCount()
}

// EnterCall increments the call-depth counter and registers the instance
// on the ReentranceTracker so CallMightBeRecursive can detect recursive
// re-entries. Spec: definitions.py:290-299 call_might_be_recursive.
// Nil-safe for the same reason as ActiveCallDepth.
func (i *Instance) EnterCall() {
	if i == nil || i.rt == nil {
		return
	}
	i.rt.Enter()
	i.rt.Reentrance.EnterInstance(i.rt.ID)
}

// ExitCall is the inverse of EnterCall. Nil-safe.
func (i *Instance) ExitCall() {
	if i == nil || i.rt == nil {
		return
	}
	i.rt.Reentrance.LeaveInstance(i.rt.ID)
	i.rt.Leave()
}

// CallMightBeRecursive reports whether calling i from caller would be
// recursive per the component-model canonical-ABI spec.
//
// Spec: definitions.py:290-299 call_might_be_recursive checks reflexive
// ancestor overlap between the caller instance and the callee instance —
// the call is recursive iff caller.inst.is_reflexive_ancestor_of(callee_inst)
// OR callee_inst.is_reflexive_ancestor_of(caller.inst). "Reflexive" means
// an instance is its own ancestor, so same-instance calls are always
// recursive.
//
// A nil caller models a host call with no supertask chain; in wazero's
// Session 1 local-only model there is no supertask tree above the host,
// so the spec's host branch reduces to returning false.
//
// The wrapper walks the structural parent chain rather than consulting
// the ReentranceTracker: the tracker models runtime-stack membership for
// the separate concurrency trap at definitions.py:3664-3667, which is a
// different spec check. The tracker must NOT be reintroduced here as a
// substitute for the structural walk — doing so was the B3/B4-initial
// divergence that the B4 corrective removed.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
	if i == nil || caller == nil {
		return false
	}
	return isReflexiveAncestor(caller, i) || isReflexiveAncestor(i, caller)
}

// isReflexiveAncestor reports whether ancestor appears in descendant's
// parent chain (reflexive: descendant qualifies as its own ancestor).
// Spec: definitions.py ComponentInstance.reflexive_ancestors().
func isReflexiveAncestor(ancestor, descendant *Instance) bool {
	for cur := descendant; cur != nil; cur = cur.parent {
		if cur == ancestor {
			return true
		}
	}
	return false
}

// ValidateMayLeave traps if the instance cannot currently leave.
// Spec: definitions.py:2065, :2135, :2143.
func (i *Instance) ValidateMayLeave() error {
	if i == nil || i.rt == nil {
		return nil
	}
	if !i.rt.IsMayLeave() {
		return errMayNotLeave
	}
	return nil
}

// ValidateNotRecursive checks if a call from caller would be recursive reentrance.
func (i *Instance) ValidateNotRecursive(caller *Instance) error {
	if i.CallMightBeRecursive(caller) {
		return errReentrance
	}
	return nil
}

// errMayNotLeave and errReentrance are sentinel errors returned from the
// spec-level validation helpers.
var (
	errMayNotLeave = errors.New("component instance cannot leave (may_leave=false)")
	errReentrance  = errors.New("trap: recursive call into same component instance")
)

// --- Value index space ---------------------------------------------------

// AddValue adds a value to the instance's value index space.
func (i *Instance) AddValue(v types.Val) uint32 {
	if i.values == nil {
		i.values = make([]types.Val, 0)
		i.valuesConsumed = make([]bool, 0)
	}
	idx := uint32(len(i.values))
	i.values = append(i.values, v)
	i.valuesConsumed = append(i.valuesConsumed, false)
	return idx
}

// GetValue retrieves a value from the value index space.
func (i *Instance) GetValue(idx uint32) (types.Val, error) {
	if idx >= uint32(len(i.values)) {
		return types.Val{}, valueIndexError(idx, uint32(len(i.values)))
	}
	return i.values[idx], nil
}

// ConsumeValue retrieves and marks a value as consumed.
func (i *Instance) ConsumeValue(idx uint32) (types.Val, error) {
	if idx >= uint32(len(i.values)) {
		return types.Val{}, valueIndexError(idx, uint32(len(i.values)))
	}
	if i.valuesConsumed[idx] {
		return types.Val{}, valueConsumedError(idx)
	}
	i.valuesConsumed[idx] = true
	return i.values[idx], nil
}

// IsValueConsumed returns whether a value has been consumed.
func (i *Instance) IsValueConsumed(idx uint32) bool {
	if idx >= uint32(len(i.valuesConsumed)) {
		return false
	}
	return i.valuesConsumed[idx]
}

type valueErrorIdx struct{ idx, have uint32 }

func (e valueErrorIdx) Error() string {
	return "value index out of range"
}

type valueErrorConsumed struct{ idx uint32 }

func (e valueErrorConsumed) Error() string {
	return "value already consumed"
}

func valueIndexError(idx, have uint32) error { return valueErrorIdx{idx: idx, have: have} }
func valueConsumedError(idx uint32) error    { return valueErrorConsumed{idx: idx} }

// --- Nested component support --------------------------------------------

// Parent returns the wrapper-layer parent, paired with rt.Parent at
// construction time.
func (i *Instance) Parent() *Instance { return i.parent }

// Children returns this instance's child instances.
func (i *Instance) Children() []*Instance { return i.children }

// AddChild adds a child instance and sets its parent.
func (i *Instance) AddChild(child *Instance) {
	if i.children == nil {
		i.children = make([]*Instance, 0)
	}
	i.children = append(i.children, child)
	child.parent = i
}

// GetAncestor returns the ancestor at the given depth.
func (i *Instance) GetAncestor(depth uint32) *Instance {
	current := i
	for d := uint32(0); d < depth && current != nil; d++ {
		current = current.parent
	}
	return current
}

// AddInstanceToSpace adds an instance to the instance index space.
func (i *Instance) AddInstanceToSpace(inst *Instance) uint32 {
	idx := uint32(len(i.instanceSpace))
	i.instanceSpace = append(i.instanceSpace, inst)
	return idx
}

// GetInstanceFromSpace retrieves an instance from the instance index space.
func (i *Instance) GetInstanceFromSpace(idx uint32) *Instance {
	if idx >= uint32(len(i.instanceSpace)) {
		return nil
	}
	return i.instanceSpace[idx]
}

// AddTypeToSpace adds a type definition to the type index space.
func (i *Instance) AddTypeToSpace(t *TypeDef) uint32 {
	idx := uint32(len(i.typeSpace))
	i.typeSpace = append(i.typeSpace, t)
	return idx
}

// GetTypeFromSpace retrieves a type from the type index space.
func (i *Instance) GetTypeFromSpace(idx uint32) *TypeDef {
	if idx >= uint32(len(i.typeSpace)) {
		return nil
	}
	return i.typeSpace[idx]
}

// AddComponentToSpace adds a component to the component index space.
func (i *Instance) AddComponentToSpace(c *Component) uint32 {
	idx := uint32(len(i.componentSpace))
	i.componentSpace = append(i.componentSpace, c)
	return idx
}

// GetComponentFromSpace retrieves a component from the component index space.
func (i *Instance) GetComponentFromSpace(idx uint32) *Component {
	if idx >= uint32(len(i.componentSpace)) {
		return nil
	}
	return i.componentSpace[idx]
}

// AddExportedInstance adds an instance to the exported instances map.
func (i *Instance) AddExportedInstance(name string, inst *Instance) {
	if i.exportedInstances == nil {
		i.exportedInstances = make(map[string]*Instance)
	}
	i.exportedInstances[name] = inst
}

// GetExportedInstance retrieves an exported instance by name.
func (i *Instance) GetExportedInstance(name string) *Instance {
	if i.exportedInstances == nil {
		return nil
	}
	return i.exportedInstances[name]
}
