// internal/component/instance_test.go

package component

import (
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestInstanceStructure(t *testing.T) {
	c := &Component{}
	inst := &Instance{
		component: c,
	}

	require.Same(t, c, inst.Component())
	require.Nil(t, inst.ExportedFunction("nonexistent"))
}

// TestCanonResourceNew tests the canon resource.new operation.
func TestCanonResourceNew(t *testing.T) {
	inst := &Instance{}

	// Create a new resource with a string representation
	handleIdx, err := inst.ResourceNew("my-resource-rep")
	require.NoError(t, err)

	// Handle should be at index 0 (first resource)
	require.Equal(t, uint32(0), handleIdx)

	// Verify resource table was lazily initialized
	require.NotNil(t, inst.resourceTable)

	// Verify the handle is in the table and owned
	h := Handle(handleIdx)
	entry, err := inst.resourceTable.Get(h)
	require.NoError(t, err)
	require.Equal(t, "my-resource-rep", entry.Rep)
	require.True(t, entry.Own)
}

// TestCanonResourceNew_MultipleResources tests creating multiple resources.
func TestCanonResourceNew_MultipleResources(t *testing.T) {
	inst := &Instance{}

	h1, err := inst.ResourceNew("first")
	require.NoError(t, err)
	h2, err := inst.ResourceNew("second")
	require.NoError(t, err)
	h3, err := inst.ResourceNew("third")
	require.NoError(t, err)

	require.Equal(t, uint32(0), h1)
	require.Equal(t, uint32(1), h2)
	require.Equal(t, uint32(2), h3)
}

// TestCanonResourceRep tests the canon resource.rep operation.
func TestCanonResourceRep(t *testing.T) {
	inst := &Instance{}

	// Create a resource
	handleIdx, err := inst.ResourceNew("my-representation")
	require.NoError(t, err)

	// Extract the representation
	rep, err := inst.ResourceRep(handleIdx)
	require.NoError(t, err)
	require.Equal(t, "my-representation", rep)

	// Handle should still be valid after resource.rep
	rep2, err := inst.ResourceRep(handleIdx)
	require.NoError(t, err)
	require.Equal(t, "my-representation", rep2)
}

// TestCanonResourceRep_InvalidHandle tests resource.rep with invalid handle.
func TestCanonResourceRep_InvalidHandle(t *testing.T) {
	inst := &Instance{}

	// No resource table initialized
	_, err := inst.ResourceRep(999)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidHandle))

	// Initialize table but use invalid handle
	inst.resourceTable = NewResourceTable()
	_, err = inst.ResourceRep(999)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidHandle))
}

// TestCanonResourceDrop_Owned tests dropping an owned handle with destructor.
func TestCanonResourceDrop_Owned(t *testing.T) {
	inst := &Instance{}

	// Track destructor call
	var destructorCalled bool
	var destructorArg any

	// Register destructor for resource type 0
	inst.SetDestructor(0, func(rep any) {
		destructorCalled = true
		destructorArg = rep
	})

	// Create an owned resource
	handleIdx, err := inst.ResourceNew("resource-to-drop")
	require.NoError(t, err)

	// Drop the resource
	err = inst.ResourceDrop(handleIdx, 0)
	require.NoError(t, err)

	// Destructor should have been called
	require.True(t, destructorCalled)
	require.Equal(t, "resource-to-drop", destructorArg)

	// Handle should no longer be valid
	_, err = inst.ResourceRep(handleIdx)
	require.Error(t, err)
}

// TestCanonResourceDrop_NoDestructor tests dropping an owned handle without destructor.
func TestCanonResourceDrop_NoDestructor(t *testing.T) {
	inst := &Instance{}

	// Create an owned resource
	handleIdx, err := inst.ResourceNew("resource-without-dtor")
	require.NoError(t, err)

	// Drop without registering destructor - should not error
	err = inst.ResourceDrop(handleIdx, 0)
	require.NoError(t, err)

	// Handle should no longer be valid
	_, err = inst.ResourceRep(handleIdx)
	require.Error(t, err)
}

// TestCanonResourceDrop_DifferentResourceTypes tests destructors for different types.
func TestCanonResourceDrop_DifferentResourceTypes(t *testing.T) {
	inst := &Instance{}

	var type0Called, type1Called bool

	// Register destructors for different resource types
	inst.SetDestructor(0, func(rep any) {
		type0Called = true
	})
	inst.SetDestructor(1, func(rep any) {
		type1Called = true
	})

	// Create resources
	h1, _ := inst.ResourceNew("type0-resource")
	h2, _ := inst.ResourceNew("type1-resource")

	// Drop with correct types
	err := inst.ResourceDrop(h1, 0)
	require.NoError(t, err)
	require.True(t, type0Called)
	require.False(t, type1Called)

	err = inst.ResourceDrop(h2, 1)
	require.NoError(t, err)
	require.True(t, type1Called)
}

// TestCanonResourceDrop_Borrowed tests dropping a borrowed handle.
func TestCanonResourceDrop_Borrowed(t *testing.T) {
	inst := &Instance{}
	inst.resourceTable = NewResourceTable()

	// Create a call context and set initial borrow count
	ctx := NewCallContext()
	ctx.IncrementBorrows() // Simulate receiving a borrowed handle
	inst.SetCallContext(ctx)

	// Verify initial borrow count
	require.Equal(t, 1, ctx.NumBorrows())

	// Create a borrowed handle directly in the table (own=false)
	h := inst.resourceTable.New("borrowed-rep", false)

	// Track destructor call - should NOT be called for borrowed handles
	var destructorCalled bool
	inst.SetDestructor(0, func(rep any) {
		destructorCalled = true
	})

	// Drop the borrowed handle
	err := inst.ResourceDrop(uint32(h), 0)
	require.NoError(t, err)

	// Destructor should NOT have been called
	require.False(t, destructorCalled)

	// Borrow count should have been decremented
	require.Equal(t, 0, ctx.NumBorrows())
}

// TestCanonResourceDrop_BorrowedNoCallContext tests dropping borrowed without context.
func TestCanonResourceDrop_BorrowedNoCallContext(t *testing.T) {
	inst := &Instance{}
	inst.resourceTable = NewResourceTable()

	// Create a borrowed handle directly
	h := inst.resourceTable.New("borrowed-rep", false)

	// Drop without call context - should not error
	err := inst.ResourceDrop(uint32(h), 0)
	require.NoError(t, err)
}

// TestCanonResourceDrop_InvalidHandle tests dropping an invalid handle.
func TestCanonResourceDrop_InvalidHandle(t *testing.T) {
	inst := &Instance{}

	// No resource table
	err := inst.ResourceDrop(999, 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidHandle))

	// With table but invalid handle
	inst.resourceTable = NewResourceTable()
	err = inst.ResourceDrop(999, 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidHandle))
}

// TestCanonResourceDrop_DoubleDrop tests dropping the same handle twice.
func TestCanonResourceDrop_DoubleDrop(t *testing.T) {
	inst := &Instance{}

	handleIdx, err := inst.ResourceNew("resource")
	require.NoError(t, err)

	// First drop succeeds
	err = inst.ResourceDrop(handleIdx, 0)
	require.NoError(t, err)

	// Second drop fails
	err = inst.ResourceDrop(handleIdx, 0)
	require.Error(t, err)
}

// TestInstance_SetCallContext tests call context getter and setter.
func TestInstance_SetCallContext(t *testing.T) {
	inst := &Instance{}

	// Initially nil
	require.Nil(t, inst.CallContext())

	// Set context
	ctx := NewCallContext()
	inst.SetCallContext(ctx)

	// Get context
	require.Same(t, ctx, inst.CallContext())
}

// TestCanonResourceNew_IntRepresentation tests resource.new with int rep.
func TestCanonResourceNew_IntRepresentation(t *testing.T) {
	inst := &Instance{}

	// The spec mentions rep is often an integer (file descriptor, etc.)
	handleIdx, err := inst.ResourceNew(42)
	require.NoError(t, err)

	rep, err := inst.ResourceRep(handleIdx)
	require.NoError(t, err)
	require.Equal(t, 42, rep)
}

// TestCanonResourceNew_StructRepresentation tests resource.new with struct rep.
func TestCanonResourceNew_StructRepresentation(t *testing.T) {
	type FileResource struct {
		fd   int
		path string
	}

	inst := &Instance{}

	resource := &FileResource{fd: 3, path: "/tmp/test"}
	handleIdx, err := inst.ResourceNew(resource)
	require.NoError(t, err)

	rep, err := inst.ResourceRep(handleIdx)
	require.NoError(t, err)
	require.Same(t, resource, rep.(*FileResource))
}
