// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestOwnType(t *testing.T) {
	// own<T> is represented as i32 handle index
	o := Own{ResourceIdx: 0}
	require.Equal(t, uint32(4), o.Size())
	require.Equal(t, uint32(4), o.Align())
	require.Equal(t, 1, o.FlattenCount())
}

func TestBorrowType(t *testing.T) {
	// borrow<T> same layout as own
	b := Borrow{ResourceIdx: 0}
	require.Equal(t, uint32(4), b.Size())
	require.Equal(t, uint32(4), b.Align())
	require.Equal(t, 1, b.FlattenCount())
}

func TestOwnAndBorrowDistinct(t *testing.T) {
	o := Own{ResourceIdx: 5}
	b := Borrow{ResourceIdx: 5}

	// They reference the same resource type but are different handle types
	require.Equal(t, o.ResourceIdx, b.ResourceIdx)

	// Type assertion should work
	var _ ValType = o
	var _ ValType = b
}

func TestResourceType(t *testing.T) {
	// Resource with destructor
	dtorIdx := uint32(42)
	r := ResourceType{
		Destructor: &dtorIdx,
	}
	require.NotNil(t, r.Destructor)
	require.Equal(t, uint32(42), *r.Destructor)

	// Resource without destructor
	r2 := ResourceType{
		Destructor: nil,
	}
	require.Nil(t, r2.Destructor)
}

func TestResourceType_HasDestructor(t *testing.T) {
	// Resource with destructor
	rt := ResourceType{
		Destructor: ptrTo(uint32(5)),
	}
	require.True(t, rt.HasDestructor())

	// Resource without destructor
	rtNoDtor := ResourceType{}
	require.False(t, rtNoDtor.HasDestructor())
}

func TestResourceType_InstanceID(t *testing.T) {
	rt := ResourceType{
		InstanceID: 42,
	}
	require.Equal(t, uint32(42), rt.InstanceID)
}

func TestResourceType_AsyncDestructor(t *testing.T) {
	rt := ResourceType{
		Destructor:   ptrTo(uint32(5)),
		DtorAsync:    true,
		DtorCallback: ptrTo(uint32(6)),
	}
	require.True(t, rt.DtorAsync)
	require.NotNil(t, rt.DtorCallback)
}

func ptrTo(v uint32) *uint32 {
	return &v
}
