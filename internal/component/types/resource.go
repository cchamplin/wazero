// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// Own represents an owning handle to a resource.
// When an own<T> is dropped, the resource's destructor is called.
//
// TODO: Per spec, should track ResourceType reference for validation during lift/lower.
// This would enable validation that the handle's resource type matches the expected type
// (spec lines 2218-2219).
type Own struct {
	ResourceIdx uint32 // Index of the resource type in component's type section
	// TODO: ResourceType *ResourceType // Reference to the resource type (for validation)
}

func (Own) valType() {}

// Size returns 4 because handles are i32 indices.
func (Own) Size() uint32 { return 4 }

// Align returns 4 for i32 alignment.
func (Own) Align() uint32 { return 4 }

// FlattenCount returns 1 because a handle is a single i32.
func (Own) FlattenCount() int { return 1 }

// Borrow represents a borrowed handle to a resource.
// Borrows do not own the resource and must not outlive the call scope.
//
// TODO: Per spec, should track ResourceType reference for validation during lift/lower.
// This would enable validation that the handle's resource type matches the expected type
// (spec lines 2237-2238).
type Borrow struct {
	ResourceIdx uint32 // Index of the resource type in component's type section
	// TODO: ResourceType *ResourceType // Reference to the resource type (for validation)
}

func (Borrow) valType() {}

// Size returns 4 because handles are i32 indices.
func (Borrow) Size() uint32 { return 4 }

// Align returns 4 for i32 alignment.
func (Borrow) Align() uint32 { return 4 }

// FlattenCount returns 1 because a handle is a single i32.
func (Borrow) FlattenCount() int { return 1 }

// ResourceType represents a resource type definition.
// Resources have an optional destructor that is called when the resource is dropped.
//
// From spec (CanonicalABI.md:537-549):
//
//	class ResourceType(Type):
//	  impl: ComponentInstance
//	  dtor: Optional[Callable]
//	  dtor_async: bool
//	  dtor_callback: Optional[Callable]
type ResourceType struct {
	// InstanceID identifies the component instance that defines this resource type.
	// This corresponds to the 'impl' field in the spec.
	InstanceID uint32

	// Destructor is the index of the destructor function (nil if no destructor).
	// This is the core function index in the defining instance.
	Destructor *uint32

	// DtorAsync indicates if the destructor is an async function.
	DtorAsync bool

	// DtorCallback is the callback function index for async destructors.
	DtorCallback *uint32
}

// HasDestructor returns true if this resource type has a destructor.
func (rt *ResourceType) HasDestructor() bool {
	return rt.Destructor != nil
}
