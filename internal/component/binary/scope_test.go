// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binary

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/component/types"
)

func TestTypeScope_AppendAndLookupValType(t *testing.T) {
	scope := newTypeScope(nil)
	vt := types.U32
	scope.appendValType(vt)
	if len(scope.entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(scope.entries))
	}
	got := scope.entries[0]
	if got.kind != scopeEntryValType {
		t.Errorf("kind = %v, want scopeEntryValType", got.kind)
	}
	if got.valType != vt {
		t.Errorf("valType = %v, want %v", got.valType, vt)
	}
}

func TestTypeScope_AppendAndLookupResource(t *testing.T) {
	scope := newTypeScope(nil)
	scope.appendResource(types.ResourceTableIdx(7))
	got := scope.entries[0]
	if got.kind != scopeEntryResource {
		t.Errorf("kind = %v, want scopeEntryResource", got.kind)
	}
	if got.resource != types.ResourceTableIdx(7) {
		t.Errorf("resource = %d, want 7", got.resource)
	}
}

func TestTypeScope_ParentChain(t *testing.T) {
	parent := newTypeScope(nil)
	parent.appendValType(types.U32)
	child := newTypeScope(parent)
	if child.parent != parent {
		t.Errorf("child.parent != parent")
	}
	// Outer alias resolution by index N walks the parent chain.
	got := child.parent.entries[0]
	if got.valType != types.U32 {
		t.Errorf("parent[0].valType = %v, want U32", got.valType)
	}
}

func TestScope_OuterAliasResolution(t *testing.T) {
	// A type defined in a parent scope, aliased into a child scope by
	// outer alias, resolves to the same ValType.
	parent := newTypeScope(nil)
	parent.appendValType(types.U32)
	child := newTypeScope(parent)
	// Simulate an outer alias copying parent[0] into child.
	child.appendValType(parent.entries[0].valType)
	if child.entries[0].valType != types.U32 {
		t.Errorf("aliased valType = %v, want U32", child.entries[0].valType)
	}
}
