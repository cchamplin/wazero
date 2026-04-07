// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "testing"

func TestResourceTypePointerIdentity(t *testing.T) {
	// Two distinct ResourceType pointers compare unequal even if their
	// fields are identical. Spec: definitions.py:1336, 1345 (Python `is`).
	rA := &ResourceType{}
	rB := &ResourceType{}
	if rA == rB {
		t.Errorf("two distinct *ResourceType pointers are equal")
	}
	rAAlias := rA
	if rA != rAAlias {
		t.Errorf("aliased *ResourceType pointers compare unequal")
	}
}

func TestResourceTypeHasDestructor(t *testing.T) {
	rt := &ResourceType{}
	if rt.HasDestructor() {
		t.Errorf("HasDestructor() = true on default, want false")
	}
	dtorIdx := uint32(7)
	rt.Dtor = &dtorIdx
	if !rt.HasDestructor() {
		t.Errorf("HasDestructor() = false after setting Dtor, want true")
	}
}
