// internal/component/instance_test.go

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
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

// mockFunction implements api.Function for testing ExportedFunc.Call
type mockFunction struct {
	internalapi.WazeroOnlyType
	callFn func(ctx context.Context, params ...uint64) ([]uint64, error)
}

func (m *mockFunction) Definition() api.FunctionDefinition { return nil }
func (m *mockFunction) Call(ctx context.Context, params ...uint64) ([]uint64, error) {
	return m.callFn(ctx, params...)
}
func (m *mockFunction) CallWithStack(ctx context.Context, stack []uint64) error { return nil }

// TestExportedFuncCall_OwnArgument tests calling a function with own<T> argument.
func TestExportedFuncCall_OwnArgument(t *testing.T) {
	// Track what was passed to the core function
	var capturedParams []uint64

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{42}, nil
		},
	}

	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	// Create function type with own<T> parameter
	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "handle", ValType: ValTypeRef{IsOwn: true, TypeIdx: 0}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:     "take_ownership",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	// Call with an own handle - the uint32 value is the representation
	// that will be turned into a handle via LowerOwn
	results, err := f.Call(context.Background(), ValOwn(123))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, ValKindS32, results[0].Kind())
	require.Equal(t, int32(42), results[0].S32())

	// The LowerOwn should have created a handle at index 0
	require.Equal(t, 1, len(capturedParams))
	require.Equal(t, uint64(0), capturedParams[0]) // First handle is at index 0
}

// TestExportedFuncCall_BorrowArgument tests calling a function with borrow<T> argument.
func TestExportedFuncCall_BorrowArgument(t *testing.T) {
	// Track what was passed to the core function
	var capturedParams []uint64
	var capturedInst *Instance

	inst := &Instance{
		resourceTable: NewResourceTable(),
	}
	capturedInst = inst

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			// Simulate the callee properly dropping the borrowed handle
			if len(params) > 0 {
				handleIdx := uint32(params[0])
				_ = capturedInst.ResourceDrop(handleIdx, 0)
			}
			return []uint64{99}, nil
		},
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "handle", ValType: ValTypeRef{IsBorrow: true, TypeIdx: 0}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:     "borrow_resource",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	// Call with a borrow handle - the callee drops it before return
	results, err := f.Call(context.Background(), ValBorrow(456))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(99), results[0].S32())

	// The LowerBorrow should have created a borrowed handle at index 0
	require.Equal(t, 1, len(capturedParams))
	require.Equal(t, uint64(0), capturedParams[0])
}

// TestExportedFuncCall_OwnResult tests returning own<T> from a function.
func TestExportedFuncCall_OwnResult(t *testing.T) {
	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	// Pre-create an owned handle that the "callee" will return
	h := inst.resourceTable.New("my-resource-rep", true)

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			// Return the handle index
			return []uint64{uint64(h.Index())}, nil
		},
	}

	funcType := &FuncType{
		Params:  []NamedValType{},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsOwn: true, TypeIdx: 0}},
		},
	}

	f := &ExportedFunc{
		name:     "create_resource",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	results, err := f.Call(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, ValKindOwn, results[0].Kind())
}

// TestExportedFuncCall_OutstandingBorrowTrap tests that outstanding borrows cause a trap.
func TestExportedFuncCall_OutstandingBorrowTrap(t *testing.T) {
	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			// The borrow was created, but won't be dropped before return
			// (simulating callee not dropping the borrowed handle)
			return []uint64{42}, nil
		},
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "handle", ValType: ValTypeRef{IsBorrow: true, TypeIdx: 0}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:     "borrow_and_forget",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	// Call with a borrow - the LowerBorrow will increment borrow count,
	// and since the "callee" doesn't drop it, ValidateReturn should trap
	_, err := f.Call(context.Background(), ValBorrow(789))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrOutstandingBorrows))
}

// TestExportedFuncCall_BorrowDroppedBeforeReturn tests that dropping borrows allows return.
func TestExportedFuncCall_BorrowDroppedBeforeReturn(t *testing.T) {
	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	// Track the instance to drop the borrow during call
	var capturedInst *Instance

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			// Simulate the callee dropping the borrowed handle
			// The handle index is params[0]
			if len(params) > 0 {
				handleIdx := uint32(params[0])
				// Drop the borrowed handle (this decrements the borrow count)
				_ = capturedInst.ResourceDrop(handleIdx, 0)
			}
			return []uint64{42}, nil
		},
	}

	capturedInst = inst

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "handle", ValType: ValTypeRef{IsBorrow: true, TypeIdx: 0}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:     "borrow_and_drop",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	// Call with a borrow - the callee drops it, so ValidateReturn succeeds
	results, err := f.Call(context.Background(), ValBorrow(789))
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(42), results[0].S32())
}

// TestExportedFuncCall_MultipleOwnBorrowParams tests multiple own/borrow parameters.
func TestExportedFuncCall_MultipleOwnBorrowParams(t *testing.T) {
	var capturedParams []uint64

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{100}, nil
		},
	}

	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "owned1", ValType: ValTypeRef{IsOwn: true, TypeIdx: 0}},
			{Name: "borrowed", ValType: ValTypeRef{IsBorrow: true, TypeIdx: 0}},
			{Name: "owned2", ValType: ValTypeRef{IsOwn: true, TypeIdx: 0}},
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:     "multi_handles",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	// The call will fail with outstanding borrow since we don't drop it
	_, err := f.Call(context.Background(), ValOwn(1), ValBorrow(2), ValOwn(3))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrOutstandingBorrows))

	// Verify that all three parameters were passed as handles
	require.Equal(t, 3, len(capturedParams))
}

// TestExportedFuncCall_CallContextRestored tests that call context is restored after call.
func TestExportedFuncCall_CallContextRestored(t *testing.T) {
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			return []uint64{42}, nil
		},
	}

	inst := &Instance{
		resourceTable: NewResourceTable(),
	}

	// Set a pre-existing call context
	prevCtx := NewCallContext()
	inst.SetCallContext(prevCtx)

	funcType := &FuncType{
		Params:  []NamedValType{},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:     "simple",
		funcType: funcType,
		coreFunc: coreFunc,
		instance: inst,
	}

	_, err := f.Call(context.Background())
	require.NoError(t, err)

	// The previous call context should be restored
	require.Same(t, prevCtx, inst.CallContext())
}
