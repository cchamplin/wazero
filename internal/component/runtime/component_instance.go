// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "github.com/tetratelabs/wazero/internal/component/types"

// ComponentInstance is the runtime state for one instantiated component
// (top-level or nested). Matches the spec's ComponentInstance at
// debug-vendored/component-model/design/mvp/canonical-abi/definitions.py:256-273.
//
// One ComponentInstance per instantiation. For nested instantiation,
// Parent points to the parent instance. For top-level instances, Parent
// is nil. Each instance owns its own Table, its own MayLeave flag, and
// its own ResourceTypes pool.
type ComponentInstance struct {
	// ID is a monotonically-assigned runtime instance identifier.
	ID uint32

	// Parent is the parent component instance for nested instantiation,
	// or nil for top-level instances. Spec field: parent
	// (definitions.py:258).
	Parent *ComponentInstance

	// Table is the unified handle table for this instance. Holds
	// resource handles today; streams, futures, error-contexts, and
	// subtasks share this table when async lands. Handle indices are
	// unique across all handle kinds within this instance.
	// Spec: definitions.py:259, class Table at :303-315.
	Table *Table

	// MayLeave is the may_leave flag. Set to false during canon.task.enter
	// and restored after canon.task.exit. Operations like canon.resource.new
	// trap if !MayLeave. Spec: definitions.py:260, 270, 1955, 1973, 2065,
	// 2135, 2143.
	MayLeave bool

	// enterCount tracks Enter()/Leave() nesting for compatibility with
	// wazero's existing enter/leave tracking. Accessed via methods.
	enterCount int

	// ResourceTypes is the nominal resource type identity pool for
	// resource declarations DEFINED by this instance. Indexed by
	// types.ResourceIdx (the resource's position in the component's
	// type section).
	//
	// Each entry is a *ResourceType with POINTER identity. Two handles
	// are the same resource type iff their *ResourceType pointers are
	// equal — matching the spec's `h.rt is t.rt` check at
	// definitions.py:1345.
	//
	// Populated at instantiation time (Session 2). Empty in Session 0;
	// all TypeResourceTable entries are Abstract and resource lift/lower
	// traps before reaching this pool.
	ResourceTypes []*ResourceType

	// Destructors is this instance's destructor registry.
	Destructors *DestructorRegistry

	// Reentrance tracks call-site reentrance for this instance.
	Reentrance *ReentranceTracker
}

// NewComponentInstance creates a new instance with the given ID and
// optional parent. MayLeave starts true per spec definitions.py:270.
func NewComponentInstance(id uint32, parent *ComponentInstance) *ComponentInstance {
	return &ComponentInstance{
		ID:          id,
		Parent:      parent,
		Table:       NewTable(),
		MayLeave:    true,
		Destructors: NewDestructorRegistry(),
		Reentrance:  NewReentranceTracker(),
	}
}

// Enter marks entry into a region; may_leave does not change but
// enterCount increments.
func (c *ComponentInstance) Enter() { c.enterCount++ }

// Leave decrements enterCount. Paired with Enter.
func (c *ComponentInstance) Leave() {
	if c.enterCount > 0 {
		c.enterCount--
	}
}

// EnterCount returns the current nesting depth.
func (c *ComponentInstance) EnterCount() int { return c.enterCount }

// IsMayLeave returns whether the instance may leave — both MayLeave is
// true and enterCount is zero.
func (c *ComponentInstance) IsMayLeave() bool {
	return c.MayLeave && c.enterCount == 0
}

// LookupResourceType resolves a (RuntimeComponentInstanceIdx, ResourceIdx)
// pair from a TypeResourceTable entry to the nominal *ResourceType.
// Walks the instance tree to find the defining instance, then returns
// the ResourceTypes[ResourceIdx] entry.
//
// Returns nil if the target instance is not found or the resource type
// slot is not yet populated (Session 0 state — Concrete promotion is
// Session 2 work).
func (c *ComponentInstance) LookupResourceType(
	instanceIdx types.RuntimeComponentInstanceIdx,
	resourceIdx types.ResourceIdx,
) *ResourceType {
	target := c.findInstance(uint32(instanceIdx))
	if target == nil {
		return nil
	}
	if int(resourceIdx) >= len(target.ResourceTypes) {
		return nil
	}
	return target.ResourceTypes[resourceIdx]
}

// findInstance walks the parent chain to find the instance with the
// given ID. Returns nil if not found.
func (c *ComponentInstance) findInstance(id uint32) *ComponentInstance {
	for inst := c; inst != nil; inst = inst.Parent {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}
