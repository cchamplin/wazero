// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

// Own represents an owning handle to a resource.
// When an own<T> is dropped, the resource's destructor is called.
type Own struct {
	ResourceIdx uint32 // Index of the resource type in component's type section
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
type Borrow struct {
	ResourceIdx uint32 // Index of the resource type in component's type section
}

func (Borrow) valType() {}

// Size returns 4 because handles are i32 indices.
func (Borrow) Size() uint32 { return 4 }

// Align returns 4 for i32 alignment.
func (Borrow) Align() uint32 { return 4 }

// FlattenCount returns 1 because a handle is a single i32.
func (Borrow) FlattenCount() int { return 1 }
