// internal/component/instance.go
//
// SESSION 0 COMPILE-FIX STUB (Task 17).
//
// The previous implementation of Instance and ExportedFunc — including the
// entire ExportedFunc.Call lift/lower path, liftResolvedType, lowerTyped,
// lowerToMemory, liftResultFromMemory and the resource-table helpers — has
// been reduced to panic stubs so the top-level internal/component/ package
// can compile against the new types.ValType / types.TypeFunc /
// runtime.ComponentInstance shapes.
//
// Every method that depends on the broken lift/lower path panics with a
// precise error pointing at the Session 1 followup note. Session 1 will
// delete these stubs and replace them with direct calls into the rewritten
// internal/component/abi/ package.
//
// Design: docs/superpowers/specs/2026-04-07-canonical-abi-type-unification-design.md
// Work Order: step 15 (compile-fix); V5 caller audit (design lines 1927-1945).
package component

import (
	"context"

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

// Instance represents an instantiated component.
//
// Session 0 compile-fix: the shape is preserved so that other files in the
// package compile. All lift/lower-dependent methods panic.
type Instance struct {
	component     *Component
	coreInstances []api.Module
	exports       map[string]*ExportedFunc

	// componentFuncs maps component function indices to their implementations.
	componentFuncs map[uint32]ComponentFunc

	// Resource management.
	// table holds resource handles and other per-instance handle entries.
	// The old *runtime.ResourceTable type has been unified into runtime.Table.
	table       *runtime.Table
	destructors map[uint32]func(any)
	callContext *runtime.CallContext

	// mayLeaveDisabled mirrors runtime.ComponentInstance.MayLeave for the
	// legacy entry/leave surface. Session 1 will merge this with the runtime
	// ComponentInstance.
	mayLeaveDisabled bool

	// activeCallDepth tracks the number of active calls into this instance
	// for legacy reentrance checks. Session 1 replaces this with
	// runtime.ComponentInstance's Enter/Leave counter.
	activeCallDepth int32

	// Value index space for start function support.
	values         []types.Val
	valuesConsumed []bool

	// Nested component support.
	parent   *Instance
	children []*Instance

	// Index spaces for nested component support.
	instanceSpace  []*Instance
	typeSpace      []*TypeDef
	componentSpace []*Component

	// Exported instances for API access.
	exportedInstances map[string]*Instance
}

// ComponentFunc represents a callable component-level function.
type ComponentFunc struct {
	// Type is the component function type. The old *FuncType shape has
	// been unified on *types.TypeFunc (Design Decision 9).
	Type *types.TypeFunc

	// Impl is the actual callable.
	// For imports: the host-provided Definition
	// For canon lift: the lifted core function
	Impl func(ctx context.Context, args []types.Val) ([]types.Val, error)
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
// Session 0 compile-fix: the shape is preserved so that linker wiring and
// wrapper types compile. The Call method panics because its body depends
// on the deleted FuncType/ValTypeRef helpers and the rewritten abi/ package.
type ExportedFunc struct {
	name           string
	funcType       *types.TypeFunc
	coreFunc       api.Function
	canonical      *CanonicalDef
	component      *Component
	instance       *Instance
	memory         api.Memory
	reallocFunc    api.Function
	postReturnFunc api.Function
}

// Name returns the export name of this function.
func (f *ExportedFunc) Name() string {
	return f.name
}

// Call invokes the exported function with the given arguments.
//
// Session 0 compile-fix: body panics. Session 1 rewrites this around
// abi.LiftContext / abi.LowerContext once those land.
func (f *ExportedFunc) Call(ctx context.Context, params ...types.Val) ([]types.Val, error) {
	_ = ctx
	_ = params
	panic("compile-fix stub: see Session 1 followup note — instance.go lift/lower path scheduled for Session 1 deletion")
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

// ResourceNew implements canon resource.new.
//
// Session 0 compile-fix: body panics. The implementation depended on the
// deleted ResourceTable.New(rep, own) shape; Session 1 migrates to
// runtime.Table.NewResourceHandle with explicit *ResourceType identity.
func (i *Instance) ResourceNew(rep any) (uint32, error) {
	_ = rep
	panic("compile-fix stub: see Session 1 followup note — instance.go resource.new scheduled for Session 1 deletion")
}

// ResourceRep implements canon resource.rep.
//
// Session 0 compile-fix: body panics.
func (i *Instance) ResourceRep(handleIdx uint32) (any, error) {
	_ = handleIdx
	panic("compile-fix stub: see Session 1 followup note — instance.go resource.rep scheduled for Session 1 deletion")
}

// ResourceDrop implements canon resource.drop.
//
// Session 0 compile-fix: body panics.
func (i *Instance) ResourceDrop(handleIdx uint32, resourceTypeIdx uint32) error {
	_ = handleIdx
	_ = resourceTypeIdx
	panic("compile-fix stub: see Session 1 followup note — instance.go resource.drop scheduled for Session 1 deletion")
}

// SetDestructor registers a destructor function for a resource type.
func (i *Instance) SetDestructor(resourceTypeIdx uint32, dtor func(any)) {
	if i.destructors == nil {
		i.destructors = make(map[uint32]func(any))
	}
	i.destructors[resourceTypeIdx] = dtor
}

// SetCallContext sets the current call context for borrow tracking.
func (i *Instance) SetCallContext(ctx *runtime.CallContext) {
	i.callContext = ctx
}

// CallContext returns the current call context.
func (i *Instance) CallContext() *runtime.CallContext {
	return i.callContext
}

// MayLeave returns whether this instance is allowed to call out.
func (i *Instance) MayLeave() bool {
	return !i.mayLeaveDisabled
}

// SetMayLeave sets whether this instance is allowed to call out.
func (i *Instance) SetMayLeave(allowed bool) {
	i.mayLeaveDisabled = !allowed
}

// ActiveCallDepth returns the number of active calls into this instance.
func (i *Instance) ActiveCallDepth() int {
	if i == nil {
		return 0
	}
	return int(i.activeCallDepth)
}

// EnterCall increments the active call depth.
func (i *Instance) EnterCall() {
	if i != nil {
		i.activeCallDepth++
	}
}

// ExitCall decrements the active call depth.
func (i *Instance) ExitCall() {
	if i != nil && i.activeCallDepth > 0 {
		i.activeCallDepth--
	}
}

// CallMightBeRecursive checks if a call from caller into this instance might
// cause recursive reentrance.
func (i *Instance) CallMightBeRecursive(caller *Instance) bool {
	if i == nil || caller == nil {
		return false
	}
	if caller == i && i.activeCallDepth > 0 {
		return true
	}
	return false
}

// ValidateMayLeave checks if this instance is allowed to make outgoing calls.
func (i *Instance) ValidateMayLeave() error {
	if i == nil {
		return nil
	}
	if !i.MayLeave() {
		// Preserve the original error message shape for any caller that
		// scrapes it during assertions. Session 1 replaces this with a
		// direct runtime.ErrMayNotLeave return once the lift/lower path
		// is unified.
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

// errMayNotLeave and errReentrance are sentinel errors preserved from the
// original string shape so that tests relying on message substrings still
// match. Session 1 swaps them for runtime.ErrMayNotLeave / runtime.ErrReentrance.
var (
	errMayNotLeave = instanceError("trap: cannot call out of component while lowering values")
	errReentrance  = instanceError("trap: recursive call into same component instance")
)

type instanceError string

func (e instanceError) Error() string { return string(e) }

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

// Parent returns this instance's parent, or nil if top-level.
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
