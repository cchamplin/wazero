// internal/component/resource_type_id.go

package component

// ResourceTypeID uniquely identifies a resource type within a component instance.
// This corresponds to the 'rt' field in the spec's ResourceHandle.
// A value of 0 is reserved as invalid/unset.
type ResourceTypeID uint32

// NewResourceTypeID creates a ResourceTypeID from a type index.
// Type indices start at 1; 0 is reserved for invalid.
func NewResourceTypeID(typeIndex uint32) ResourceTypeID {
	return ResourceTypeID(typeIndex + 1)
}

// InvalidResourceTypeID returns an invalid ResourceTypeID (zero value).
func InvalidResourceTypeID() ResourceTypeID {
	return ResourceTypeID(0)
}

// Index returns the underlying type index.
// Returns the original index passed to NewResourceTypeID.
func (id ResourceTypeID) Index() uint32 {
	return uint32(id) - 1
}

// IsValid returns true if this is a valid resource type ID.
func (id ResourceTypeID) IsValid() bool {
	return id != 0
}

// ResourceTypeInfo contains extended information about a resource type,
// including which component instance defines it.
// This is needed for the lower_borrow same-instance optimization.
type ResourceTypeInfo struct {
	typeID     ResourceTypeID
	instanceID uint32 // ID of the component instance that defines this type
}

// NewResourceTypeInfo creates a ResourceTypeInfo from a type index and instance ID.
func NewResourceTypeInfo(typeIndex uint32, instanceID uint32) ResourceTypeInfo {
	return ResourceTypeInfo{
		typeID:     NewResourceTypeID(typeIndex),
		instanceID: instanceID,
	}
}

// TypeID returns the ResourceTypeID.
func (r ResourceTypeInfo) TypeID() ResourceTypeID {
	return r.typeID
}

// TypeIndex returns the type index.
func (r ResourceTypeInfo) TypeIndex() uint32 {
	return r.typeID.Index()
}

// InstanceID returns the defining component instance ID.
func (r ResourceTypeInfo) InstanceID() uint32 {
	return r.instanceID
}

// SameInstance returns true if this type is defined in the same instance as other.
func (r ResourceTypeInfo) SameInstance(other ResourceTypeInfo) bool {
	return r.instanceID == other.instanceID
}
