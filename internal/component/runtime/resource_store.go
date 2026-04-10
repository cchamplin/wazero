// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "sync"

// resourceTypeKey uniquely identifies a resource type within a store-wide
// registry by combining the defining instance's runtime ID with the
// resource declaration index (position in the component's ResourceTables).
type resourceTypeKey struct {
	InstanceID  uint32
	ResourceIdx uint32
}

// ResourceStore is a store-wide registry that maps (instanceID, resourceIdx)
// pairs to their nominal *ResourceType. It enables cross-instance resource
// resolution — specifically sibling instance lookups that the parent-chain
// walk in ComponentInstance.findInstance cannot reach.
//
// One ResourceStore is created per top-level Instantiate call and shared
// across all instances (including nested ones) created during that
// instantiation. Thread-safe via sync.RWMutex.
//
// Spec: This is an implementation detail enabling the spec's cross-instance
// identity checks (definitions.py:1345 `h.rt is not t.rt`).
type ResourceStore struct {
	mu        sync.RWMutex
	types     map[resourceTypeKey]*ResourceType
	instances map[uint32]interface{}
}

// NewResourceStore creates a new empty ResourceStore.
func NewResourceStore() *ResourceStore {
	return &ResourceStore{
		types:     make(map[resourceTypeKey]*ResourceType),
		instances: make(map[uint32]interface{}),
	}
}

// Register records a *ResourceType under the given (instanceID, resourceIdx)
// key. Called by bindResourceTypes during instantiation.
func (s *ResourceStore) Register(instanceID, resourceIdx uint32, rt *ResourceType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.types[resourceTypeKey{InstanceID: instanceID, ResourceIdx: resourceIdx}] = rt
}

// Lookup retrieves the *ResourceType for the given (instanceID, resourceIdx)
// pair. Returns nil if no entry exists. Called as a fallback by
// ComponentInstance.LookupResourceType when findInstance (parent-chain walk)
// returns nil.
func (s *ResourceStore) Lookup(instanceID, resourceIdx uint32) *ResourceType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.types[resourceTypeKey{InstanceID: instanceID, ResourceIdx: resourceIdx}]
}

// RegisterInstance records an instance (as interface{}) by its runtime ID
// so that cross-instance lookups can resolve the instance without walking
// the parent chain.
func (s *ResourceStore) RegisterInstance(id uint32, inst interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[id] = inst
}

// GetInstance retrieves an instance by its runtime ID. Returns nil if
// not found.
func (s *ResourceStore) GetInstance(id uint32) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instances[id]
}
