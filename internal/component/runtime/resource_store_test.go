// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "testing"

func TestResourceStore_RegisterAndLookup(t *testing.T) {
	store := NewResourceStore()
	inst := NewComponentInstance(1, nil)
	rt := &ResourceType{Impl: inst}
	store.Register(1, 0, rt)
	got := store.Lookup(1, 0)
	if got != rt {
		t.Fatalf("expected resource type %p, got %p", rt, got)
	}
}

func TestResourceStore_SiblingLookup(t *testing.T) {
	store := NewResourceStore()
	instA := NewComponentInstance(1, nil)
	instB := NewComponentInstance(2, nil)
	rtA := &ResourceType{Impl: instA}
	store.Register(1, 0, rtA)
	// instB can resolve instA's resource type through the store
	// even though instA is not in instB's parent chain.
	got := store.Lookup(1, 0)
	if got != rtA {
		t.Fatalf("sibling lookup failed: expected %p, got %p", rtA, got)
	}
	_ = instB
}

func TestResourceStore_LookupNotFound(t *testing.T) {
	store := NewResourceStore()
	got := store.Lookup(99, 0)
	if got != nil {
		t.Fatalf("expected nil for missing entry, got %p", got)
	}
}

func TestResourceStore_MultipleResources(t *testing.T) {
	store := NewResourceStore()
	inst := NewComponentInstance(1, nil)
	rt0 := &ResourceType{Impl: inst}
	rt1 := &ResourceType{Impl: inst}
	store.Register(1, 0, rt0)
	store.Register(1, 1, rt1)

	got0 := store.Lookup(1, 0)
	got1 := store.Lookup(1, 1)
	if got0 != rt0 {
		t.Fatalf("resource 0: expected %p, got %p", rt0, got0)
	}
	if got1 != rt1 {
		t.Fatalf("resource 1: expected %p, got %p", rt1, got1)
	}
	// Different resource types on the same instance must be distinct pointers.
	if got0 == got1 {
		t.Fatal("resource 0 and 1 must be distinct pointers")
	}
}

func TestResourceStore_MultipleInstances(t *testing.T) {
	store := NewResourceStore()
	inst1 := NewComponentInstance(1, nil)
	inst2 := NewComponentInstance(2, nil)
	rt1 := &ResourceType{Impl: inst1}
	rt2 := &ResourceType{Impl: inst2}
	store.Register(1, 0, rt1)
	store.Register(2, 0, rt2)

	got1 := store.Lookup(1, 0)
	got2 := store.Lookup(2, 0)
	if got1 != rt1 {
		t.Fatalf("instance 1: expected %p, got %p", rt1, got1)
	}
	if got2 != rt2 {
		t.Fatalf("instance 2: expected %p, got %p", rt2, got2)
	}
	// Same resource index on different instances must resolve to distinct types.
	if got1 == got2 {
		t.Fatal("resources on different instances must be distinct pointers")
	}
}

func TestResourceStore_RegisterInstance(t *testing.T) {
	store := NewResourceStore()
	inst := NewComponentInstance(1, nil)
	store.RegisterInstance(1, inst)

	got := store.GetInstance(1)
	if got != inst {
		t.Fatalf("expected instance %p, got %p", inst, got)
	}
}

func TestResourceStore_GetInstanceNotFound(t *testing.T) {
	store := NewResourceStore()
	got := store.GetInstance(99)
	if got != nil {
		t.Fatalf("expected nil for missing instance, got %p", got)
	}
}

func TestResourceStore_LookupResourceTypeFallback(t *testing.T) {
	// Simulate two sibling instances that share a ResourceStore.
	// instA defines a resource type; instB needs to look it up.
	store := NewResourceStore()
	instA := NewComponentInstance(1, nil)
	instA.Store = store
	instB := NewComponentInstance(2, nil)
	instB.Store = store

	rt := &ResourceType{Impl: instA}
	instA.ResourceTypes = append(instA.ResourceTypes, rt)
	store.Register(1, 0, rt)

	// instB has no parent chain to instA, so findInstance returns nil.
	// The Store fallback should resolve the resource type.
	got := instB.LookupResourceType(1, 0)
	if got != rt {
		t.Fatalf("LookupResourceType via store fallback: expected %p, got %p", rt, got)
	}
}
