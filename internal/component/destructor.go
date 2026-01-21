// internal/component/destructor.go

package component

// DestructorFunc is a function that destroys a resource given its representation.
// This is called when an owned handle is dropped.
type DestructorFunc func(rep uint32)

// DestructorRegistry maps resource types to their destructor functions.
// Each component instance has its own registry.
type DestructorRegistry struct {
	destructors map[ResourceTypeID]DestructorFunc
}

// NewDestructorRegistry creates a new destructor registry.
func NewDestructorRegistry() *DestructorRegistry {
	return &DestructorRegistry{
		destructors: make(map[ResourceTypeID]DestructorFunc),
	}
}

// Register associates a destructor function with a resource type.
func (r *DestructorRegistry) Register(rtID ResourceTypeID, dtor DestructorFunc) {
	r.destructors[rtID] = dtor
}

// Unregister removes the destructor for a resource type.
func (r *DestructorRegistry) Unregister(rtID ResourceTypeID) {
	delete(r.destructors, rtID)
}

// Get returns the destructor for a resource type, or nil if none registered.
func (r *DestructorRegistry) Get(rtID ResourceTypeID) DestructorFunc {
	return r.destructors[rtID]
}

// Has returns true if a destructor is registered for the resource type.
func (r *DestructorRegistry) Has(rtID ResourceTypeID) bool {
	_, ok := r.destructors[rtID]
	return ok
}
