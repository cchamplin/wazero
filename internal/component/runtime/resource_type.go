// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// ResourceType is the runtime nominal-identity layer for resource types.
// Equality is POINTER EQUALITY — two *ResourceType values refer to the
// same resource type iff they are literally the same pointer. This
// directly matches the spec's `is` check at definitions.py:1345
// (`trap_if(h.rt is not t.rt)`) and at :2147.
//
// One distinct *ResourceType exists per (ResourceIdx, ComponentInstance)
// pair at runtime, allocated when the instance is instantiated and its
// resource declarations are bound. Two instantiations of the same
// component produce TWO distinct *ResourceType objects — a handle minted
// by the first instance is rejected when presented to a function
// expecting the second instance's type.
//
// Spec: definitions.py:351-361 (Python ResourceType class), :1345, :2147.
type ResourceType struct {
	// Impl is the component instance that defines this resource type.
	// Spec field name: impl (definitions.py:352).
	Impl *ComponentInstance

	// Dtor is the core function index of the destructor in the defining
	// instance, or nil if no destructor was declared.
	Dtor *uint32

	// DtorAsync indicates an async destructor (resource opcode 0x3e).
	DtorAsync bool

	// DtorCallback is the callback function index for async destructors.
	DtorCallback *uint32
}

// HasDestructor reports whether this resource type has a destructor.
func (r *ResourceType) HasDestructor() bool { return r.Dtor != nil }
