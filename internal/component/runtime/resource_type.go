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

	// HostDestructor is the Go-side destructor callback. Called when
	// dropping an owned handle whose ResourceType has this field set.
	// For host-declared resources, this is set directly by the embedder.
	// For guest-declared resources, this is set at bind time as a
	// lazy closure that invokes the core destructor function once
	// core modules are instantiated (see ComponentLinker.bindResourceTypes).
	//
	// Spec: definitions.py:351-361 (ResourceType), :2151-2160 (destructor
	// dispatch in canon_resource_drop).
	HostDestructor func(rep uint32) error
}

// HasDestructor reports whether this resource type has a destructor
// (either a core Wasm destructor index or a host destructor callback).
func (r *ResourceType) HasDestructor() bool { return r.Dtor != nil || r.HostDestructor != nil }
