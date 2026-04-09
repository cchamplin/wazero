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
type ExportedFunc struct {
	name      string
	funcType  *types.TypeFunc
	component *Component
	instance  *Instance
	impl      HostFunc
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
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
	return f.impl(ctx, f.funcType, params)
}

// Type returns the function's type.
func (f *ExportedFunc) Type() *types.TypeFunc {
	return f.funcType
}

// alignTo rounds up offset to the next multiple of align.
func alignTo(offset, align uint32) uint32 {
	if align == 0 {
		return offset
	}
	return (offset + align - 1) &^ (align - 1)
}

// --- Resource management surface (per-instance) --------------------------
//
// These wrap the per-instance runtime.Table with the legacy Instance-level
// resource.new / resource.rep / resource.drop entry points. Session 1 folds
// these into the unified runtime.ComponentInstance + abi.LiftContext path.

// ResourceNew is canon.resource.new — spec definitions.py:2134-2138.
// Session 1 Task B4: signature in place; body is a placeholder that
// returns a precise error. Task E5 wires the full spec-correct body
// against Table.NewResourceHandle after Tasks E1 (GetByIndex) + E2
// (Rep uint32) + E3 (BorrowScope.ReleaseBorrow) land.
func (i *Instance) ResourceNew(resourceIdx types.ResourceIdx, rep uint32) (uint32, error) {
	_, _ = resourceIdx, rep
	return 0, fmt.Errorf("Instance.ResourceNew: body rebuild in progress (Session 1 Checkpoint E Task E5)")
}

// ResourceRep is canon.resource.rep — spec definitions.py:2169-2173.
func (i *Instance) ResourceRep(resourceIdx types.ResourceIdx, handleIdx uint32) (uint32, error) {
	_, _ = resourceIdx, handleIdx
	return 0, fmt.Errorf("Instance.ResourceRep: body rebuild in progress (Session 1 Checkpoint E Task E5)")
}

// ResourceDrop is canon.resource.drop — spec definitions.py:2142-2165.
func (i *Instance) ResourceDrop(resourceIdx types.ResourceIdx, handleIdx uint32) error {
	_, _ = resourceIdx, handleIdx
	return fmt.Errorf("Instance.ResourceDrop: body rebuild in progress (Session 1 Checkpoint E Task E5)")
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
