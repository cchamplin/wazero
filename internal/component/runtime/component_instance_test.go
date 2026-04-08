// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestComponentInstance_NewDefaults(t *testing.T) {
	c := NewComponentInstance(7, nil)
	if c.ID != 7 {
		t.Errorf("ID = %d, want 7", c.ID)
	}
	if c.Parent != nil {
		t.Errorf("Parent = %v, want nil", c.Parent)
	}
	if c.Table == nil {
		t.Errorf("Table = nil, want non-nil")
	}
	if !c.MayLeave {
		t.Errorf("MayLeave = false, want true (definitions.py:270)")
	}
	if c.EnterCount() != 0 {
		t.Errorf("EnterCount = %d, want 0", c.EnterCount())
	}
}

func TestComponentInstance_EnterLeave(t *testing.T) {
	c := NewComponentInstance(0, nil)
	c.Enter()
	c.Enter()
	if c.EnterCount() != 2 {
		t.Errorf("EnterCount = %d, want 2", c.EnterCount())
	}
	c.Leave()
	if c.EnterCount() != 1 {
		t.Errorf("EnterCount = %d, want 1", c.EnterCount())
	}
	c.Leave()
	c.Leave() // extra leave clamps at 0
	if c.EnterCount() != 0 {
		t.Errorf("EnterCount = %d, want 0", c.EnterCount())
	}
}

// TestComponentInstance_IsMayLeave verifies the spec's may_leave flag is
// independent of enterCount. Session 1 Task B1 (Decision 3 IsMayLeave
// semantic fix, design lines 254-263) decoupled the two fields: the spec
// at definitions.py:260, 270, 1955, 1973, 2065, 2135, 2143 treats
// may_leave as a standalone boolean, with no coupling to reentrance
// tracking. The pre-Session-1 assertion that IsMayLeave() returned false
// during Enter() was a wazero divergence and has been removed.
func TestComponentInstance_IsMayLeave(t *testing.T) {
	c := NewComponentInstance(0, nil)
	if !c.IsMayLeave() {
		t.Errorf("IsMayLeave on fresh instance = false, want true")
	}
	// Session 1 Task B1: IsMayLeave is orthogonal to Enter/Leave.
	c.Enter()
	if !c.IsMayLeave() {
		t.Errorf("IsMayLeave during enter = false, want true (spec: enterCount is orthogonal to may_leave)")
	}
	c.Leave()
	c.MayLeave = false
	if c.IsMayLeave() {
		t.Errorf("IsMayLeave with MayLeave=false = true, want false")
	}
}

func TestComponentInstance_LookupResourceTypeEmpty(t *testing.T) {
	c := NewComponentInstance(0, nil)
	got := c.LookupResourceType(types.RuntimeComponentInstanceIdx(0), types.ResourceIdx(0))
	if got != nil {
		t.Errorf("LookupResourceType on empty instance = %v, want nil", got)
	}
}

func TestComponentInstance_LookupResourceTypeWalksParents(t *testing.T) {
	parent := NewComponentInstance(1, nil)
	rt := &ResourceType{}
	parent.ResourceTypes = []*ResourceType{rt}

	child := NewComponentInstance(2, parent)
	// Child's own resource pool — first findInstance iteration finds
	// the child itself, no walk-up needed.
	childRT := &ResourceType{}
	child.ResourceTypes = []*ResourceType{childRT}
	if got := child.LookupResourceType(types.RuntimeComponentInstanceIdx(2), types.ResourceIdx(0)); got != childRT {
		t.Errorf("LookupResourceType on child = %v, want %v", got, childRT)
	}
	// Lookup of a resource owned by the parent should walk up.
	got := child.LookupResourceType(types.RuntimeComponentInstanceIdx(1), types.ResourceIdx(0))
	if got != rt {
		t.Errorf("LookupResourceType walked-up = %v, want %v", got, rt)
	}
	// Lookup of a non-existent instance ID returns nil.
	missing := child.LookupResourceType(types.RuntimeComponentInstanceIdx(99), types.ResourceIdx(0))
	if missing != nil {
		t.Errorf("LookupResourceType missing = %v, want nil", missing)
	}
}
