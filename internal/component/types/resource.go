// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// ResourceIdx names a resource *declaration* — a `(type $r (resource ...))`
// site in a component's binary. Unique within a single component's type
// section. The runtime nominal layer maps (RuntimeComponentInstanceIdx,
// ResourceIdx) → *runtime.ResourceType for the spec's `is` check at
// definitions.py:1345.
type ResourceIdx uint32

// RuntimeComponentInstanceIdx names an instantiated component instance
// at runtime, assigned monotonically at instantiation time.
type RuntimeComponentInstanceIdx uint32

// ResourceTableIdx is the index of a TypeResourceTable entry in
// ComponentTypes.ResourceTables. TypeKindOwn / TypeKindBorrow ValTypes
// carry this as their Index field.
type ResourceTableIdx uint32

// TypeResourceTable is the structural layer in ComponentTypes.ResourceTables.
// Two variants:
//
//   - Concrete: bound to a specific runtime component instance. Resolves
//     at call time via runtime.ComponentInstance.ResourceTypes (possibly
//     walking to a parent or across instances) to the nominal
//     *runtime.ResourceType for validity checking.
//   - Abstract: lives only inside a not-yet-instantiated component or
//     instance type declaration. Cannot be lifted/lowered at runtime;
//     lift/lower traps if reached at call time.
//
// At end of Session 0 ALL entries are Abstract — Concrete promotion at
// instantiation time is Session 2 work.
//
// Spec: CanonicalABI.md:531-549.
type TypeResourceTable struct {
	Concrete bool

	// Concrete fields (Concrete == true)
	Resource ResourceIdx                 // which nominal declaration
	Instance RuntimeComponentInstanceIdx // which instance defines it

	// Abstract fields (Concrete == false)
	AbstractIdx uint32
}
