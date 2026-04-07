// internal/component/runtime/destructor.go

package runtime

// DestructorFunc is a function that destroys a resource given its representation.
// This is called when an owned handle is dropped.
type DestructorFunc func(rep uint32)

// DestructorRegistry maps resource types to their destructor functions.
// Each component instance has its own registry. Keys are *ResourceType
// pointer identities (spec: `h.rt is t.rt` at definitions.py:1345); two
// distinct *ResourceType pointers are always different keys even if the
// underlying resource declarations look identical.
type DestructorRegistry struct {
	destructors map[*ResourceType]DestructorFunc
}

// NewDestructorRegistry creates a new destructor registry.
func NewDestructorRegistry() *DestructorRegistry {
	return &DestructorRegistry{
		destructors: make(map[*ResourceType]DestructorFunc),
	}
}

// Register associates a destructor function with a resource type.
func (r *DestructorRegistry) Register(rt *ResourceType, dtor DestructorFunc) {
	r.destructors[rt] = dtor
}

// Unregister removes the destructor for a resource type.
func (r *DestructorRegistry) Unregister(rt *ResourceType) {
	delete(r.destructors, rt)
}

// Get returns the destructor for a resource type, or nil if none registered.
func (r *DestructorRegistry) Get(rt *ResourceType) DestructorFunc {
	return r.destructors[rt]
}

// Has returns true if a destructor is registered for the resource type.
func (r *DestructorRegistry) Has(rt *ResourceType) bool {
	_, ok := r.destructors[rt]
	return ok
}
