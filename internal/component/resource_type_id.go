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
