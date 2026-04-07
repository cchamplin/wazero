// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "testing"

func TestResourceIdxRoundTrip(t *testing.T) {
	var r ResourceIdx = 5
	if uint32(r) != 5 {
		t.Errorf("ResourceIdx round-trip = %d, want 5", uint32(r))
	}
}

func TestRuntimeComponentInstanceIdxRoundTrip(t *testing.T) {
	var i RuntimeComponentInstanceIdx = 7
	if uint32(i) != 7 {
		t.Errorf("RuntimeComponentInstanceIdx round-trip = %d, want 7", uint32(i))
	}
}

func TestResourceTableIdxRoundTrip(t *testing.T) {
	var idx ResourceTableIdx = 11
	if uint32(idx) != 11 {
		t.Errorf("ResourceTableIdx round-trip = %d, want 11", uint32(idx))
	}
}

func TestTypeResourceTableConcrete(t *testing.T) {
	rt := TypeResourceTable{
		Concrete: true,
		Resource: 3,
		Instance: 1,
	}
	if !rt.Concrete {
		t.Errorf("Concrete = false, want true")
	}
	if rt.Resource != 3 {
		t.Errorf("Resource = %d, want 3", rt.Resource)
	}
	if rt.Instance != 1 {
		t.Errorf("Instance = %d, want 1", rt.Instance)
	}
}

func TestTypeResourceTableAbstract(t *testing.T) {
	rt := TypeResourceTable{
		Concrete:    false,
		AbstractIdx: 42,
	}
	if rt.Concrete {
		t.Errorf("Concrete = true, want false")
	}
	if rt.AbstractIdx != 42 {
		t.Errorf("AbstractIdx = %d, want 42", rt.AbstractIdx)
	}
}

func TestOwnBorrowValType(t *testing.T) {
	// Own and Borrow are encoded as ValType values, not separate structs.
	own := ValType{Kind: TypeKindOwn, Index: 5}
	borrow := ValType{Kind: TypeKindBorrow, Index: 5}
	if own.Kind != TypeKindOwn {
		t.Errorf("own.Kind = %v, want TypeKindOwn", own.Kind)
	}
	if borrow.Kind != TypeKindBorrow {
		t.Errorf("borrow.Kind = %v, want TypeKindBorrow", borrow.Kind)
	}
	if own == borrow {
		t.Errorf("own and borrow at same index should be distinct ValTypes")
	}
}
