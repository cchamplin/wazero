// internal/component/instance_test.go

package component

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/component/types"
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

// TestLiftPrimitiveVal_F32_BitPattern tests that F32 bit patterns are preserved during lifting.
func TestLiftPrimitiveVal_F32_BitPattern(t *testing.T) {
	f := &ExportedFunc{}

	// Test NaN bit pattern (0x7FC00000 is a common quiet NaN)
	nanBits := uint64(0x7FC00000)
	result := f.liftPrimitiveVal(nanBits, ValTypeRef{IsPrimitive: true, Primitive: 0x76})

	resultBits := math.Float32bits(result.F32())
	if resultBits != uint32(nanBits) {
		t.Errorf("F32 bit pattern corrupted: expected 0x%08x, got 0x%08x", nanBits, resultBits)
	}
}

// TestLiftPrimitiveVal_F64_BitPattern tests that F64 bit patterns are preserved during lifting.
func TestLiftPrimitiveVal_F64_BitPattern(t *testing.T) {
	f := &ExportedFunc{}

	// Test NaN bit pattern
	nanBits := uint64(0x7FF8000000000000)
	result := f.liftPrimitiveVal(nanBits, ValTypeRef{IsPrimitive: true, Primitive: 0x75})

	resultBits := math.Float64bits(result.F64())
	if resultBits != nanBits {
		t.Errorf("F64 bit pattern corrupted: expected 0x%016x, got 0x%016x", nanBits, resultBits)
	}
}

// TestLiftResolvedPrimitiveVal_F32_BitPattern tests that F32 bit patterns are preserved during lifting.
func TestLiftResolvedPrimitiveVal_F32_BitPattern(t *testing.T) {
	f := &ExportedFunc{}

	// Test infinity bit pattern
	infBits := uint64(0x7F800000)
	result := f.liftResolvedPrimitiveVal(infBits, types.F32{})

	resultBits := math.Float32bits(result.F32())
	if resultBits != uint32(infBits) {
		t.Errorf("F32 bit pattern corrupted: expected 0x%08x, got 0x%08x", infBits, resultBits)
	}
}

// TestLiftResolvedPrimitiveVal_F64_BitPattern tests that F64 bit patterns are preserved during lifting.
func TestLiftResolvedPrimitiveVal_F64_BitPattern(t *testing.T) {
	f := &ExportedFunc{}

	// Test negative infinity
	negInfBits := uint64(0xFFF0000000000000)
	result := f.liftResolvedPrimitiveVal(negInfBits, types.F64{})

	resultBits := math.Float64bits(result.F64())
	if resultBits != negInfBits {
		t.Errorf("F64 bit pattern corrupted: expected 0x%016x, got 0x%016x", negInfBits, resultBits)
	}
}

// mockMemory implements api.Memory for testing list operations
type mockMemory struct {
	internalapi.WazeroOnlyType
	data []byte
}

func (m *mockMemory) Definition() api.MemoryDefinition { return nil }
func (m *mockMemory) Size() uint32                     { return uint32(len(m.data)) }
func (m *mockMemory) Grow(deltaPages uint32) (uint32, bool) {
	prev := uint32(len(m.data)) / 65536
	m.data = append(m.data, make([]byte, deltaPages*65536)...)
	return prev, true
}
func (m *mockMemory) ReadByteAt(offset uint32) (byte, bool) {
	if offset >= uint32(len(m.data)) {
		return 0, false
	}
	return m.data[offset], true
}
func (m *mockMemory) ReadUint16Le(offset uint32) (uint16, bool) {
	if offset+2 > uint32(len(m.data)) {
		return 0, false
	}
	return uint16(m.data[offset]) | uint16(m.data[offset+1])<<8, true
}
func (m *mockMemory) ReadUint32Le(offset uint32) (uint32, bool) {
	if offset+4 > uint32(len(m.data)) {
		return 0, false
	}
	return uint32(m.data[offset]) | uint32(m.data[offset+1])<<8 | uint32(m.data[offset+2])<<16 | uint32(m.data[offset+3])<<24, true
}
func (m *mockMemory) ReadFloat32Le(offset uint32) (float32, bool) {
	v, ok := m.ReadUint32Le(offset)
	return math.Float32frombits(v), ok
}
func (m *mockMemory) ReadUint64Le(offset uint32) (uint64, bool) {
	if offset+8 > uint32(len(m.data)) {
		return 0, false
	}
	lo, _ := m.ReadUint32Le(offset)
	hi, _ := m.ReadUint32Le(offset + 4)
	return uint64(lo) | uint64(hi)<<32, true
}
func (m *mockMemory) ReadFloat64Le(offset uint32) (float64, bool) {
	v, ok := m.ReadUint64Le(offset)
	return math.Float64frombits(v), ok
}
func (m *mockMemory) Read(offset, byteCount uint32) ([]byte, bool) {
	if offset+byteCount > uint32(len(m.data)) {
		return nil, false
	}
	return m.data[offset : offset+byteCount], true
}
func (m *mockMemory) WriteByteAt(offset uint32, v byte) bool {
	if offset >= uint32(len(m.data)) {
		return false
	}
	m.data[offset] = v
	return true
}
func (m *mockMemory) WriteUint16Le(offset uint32, v uint16) bool {
	if offset+2 > uint32(len(m.data)) {
		return false
	}
	m.data[offset] = byte(v)
	m.data[offset+1] = byte(v >> 8)
	return true
}
func (m *mockMemory) WriteUint32Le(offset, v uint32) bool {
	if offset+4 > uint32(len(m.data)) {
		return false
	}
	m.data[offset] = byte(v)
	m.data[offset+1] = byte(v >> 8)
	m.data[offset+2] = byte(v >> 16)
	m.data[offset+3] = byte(v >> 24)
	return true
}
func (m *mockMemory) WriteFloat32Le(offset uint32, v float32) bool {
	return m.WriteUint32Le(offset, math.Float32bits(v))
}
func (m *mockMemory) WriteUint64Le(offset uint32, v uint64) bool {
	if offset+8 > uint32(len(m.data)) {
		return false
	}
	m.WriteUint32Le(offset, uint32(v))
	m.WriteUint32Le(offset+4, uint32(v>>32))
	return true
}
func (m *mockMemory) WriteFloat64Le(offset uint32, v float64) bool {
	return m.WriteUint64Le(offset, math.Float64bits(v))
}
func (m *mockMemory) Write(offset uint32, v []byte) bool {
	if int(offset)+len(v) > len(m.data) {
		return false
	}
	copy(m.data[offset:], v)
	return true
}
func (m *mockMemory) WriteString(offset uint32, v string) bool {
	return m.Write(offset, []byte(v))
}

// mockModule implements api.Module for testing
type mockModule struct {
	internalapi.WazeroOnlyType
	memory api.Memory
}

func (m *mockModule) String() string                                         { return "mockModule" }
func (m *mockModule) Name() string                                           { return "test" }
func (m *mockModule) Memory() api.Memory                                     { return m.memory }
func (m *mockModule) ExportedFunction(name string) api.Function              { return nil }
func (m *mockModule) ExportedFunctionDefinitions() map[string]api.FunctionDefinition {
	return nil
}
func (m *mockModule) ExportedMemory(name string) api.Memory                  { return m.memory }
func (m *mockModule) ExportedMemoryDefinitions() map[string]api.MemoryDefinition {
	return nil
}
func (m *mockModule) ExportedGlobal(name string) api.Global                  { return nil }
func (m *mockModule) CloseWithExitCode(ctx context.Context, exitCode uint32) error {
	return nil
}
func (m *mockModule) Close(ctx context.Context) error { return nil }
func (m *mockModule) IsClosed() bool                  { return false }

// TestExportedFunc_Call_ListWithRealloc verifies that list memory is properly allocated via realloc.
// This test ensures that lists are written to dynamically allocated memory rather than
// at hardcoded offset 0, which would cause data corruption when multiple lists are passed.
func TestExportedFunc_Call_ListWithRealloc(t *testing.T) {
	// Create mock memory large enough for multiple list allocations
	mem := &mockMemory{data: make([]byte, 4096)}

	// Track all allocations made via realloc
	type allocation struct {
		ptr  uint32
		size uint32
	}
	var allocations []allocation
	nextPtr := uint32(100) // Start allocations at offset 100

	// Mock realloc function - simple bump allocator
	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			// realloc signature: (oldPtr, oldSize, align, newSize) -> ptr
			if len(params) != 4 {
				t.Errorf("realloc called with %d params, expected 4", len(params))
				return []uint64{0}, nil
			}
			newSize := uint32(params[3])
			align := uint32(params[2])

			// Align the pointer
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}

			ptr := nextPtr
			allocations = append(allocations, allocation{ptr: ptr, size: newSize})
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	// Track parameters passed to core function
	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil // Return s32 result
		},
	}

	// Create instance with mock core module
	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	// Create function type with two parameters (the runtime detects lists by Val.Kind())
	// Each list<s32> parameter flattens to (ptr: i32, len: i32)
	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list1", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // placeholder for list<s32>
			{Name: "list2", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // placeholder for list<s32>
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_lists",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Create two lists with different data
	list1 := ValList([]Val{ValS32(1), ValS32(2), ValS32(3)})
	list2 := ValList([]Val{ValS32(4), ValS32(5)})

	// Call function with two lists
	_, err := f.Call(context.Background(), list1, list2)
	require.NoError(t, err)

	// Verify that realloc was called for each list
	require.Equal(t, 2, len(allocations), "expected 2 realloc calls, one for each list")

	// Verify allocations don't overlap
	for i := 0; i < len(allocations); i++ {
		for j := i + 1; j < len(allocations); j++ {
			a, b := allocations[i], allocations[j]
			if a.ptr < b.ptr+b.size && b.ptr < a.ptr+a.size {
				t.Errorf("Allocations overlap: [%d, %d) and [%d, %d)",
					a.ptr, a.ptr+a.size, b.ptr, b.ptr+b.size)
			}
		}
	}

	// Verify the core function received the correct (ptr, len) pairs
	// Each list becomes (ptr, len) = 2 params, so 4 total params
	require.Equal(t, 4, len(capturedParams), "expected 4 core params: ptr1, len1, ptr2, len2")

	ptr1 := uint32(capturedParams[0])
	len1 := uint32(capturedParams[1])
	ptr2 := uint32(capturedParams[2])
	len2 := uint32(capturedParams[3])

	require.Equal(t, uint32(3), len1, "list1 should have length 3")
	require.Equal(t, uint32(2), len2, "list2 should have length 2")

	// Verify the pointers are different (not both at 0)
	require.NotEqual(t, ptr1, ptr2, "list pointers should be different")

	// Verify the allocations match the captured pointers
	require.Equal(t, allocations[0].ptr, ptr1, "first allocation should match ptr1")
	require.Equal(t, allocations[1].ptr, ptr2, "second allocation should match ptr2")

	// Verify list data was written correctly to memory
	val1, ok := mem.ReadUint32Le(ptr1)
	require.True(t, ok)
	require.Equal(t, uint32(1), val1, "list1[0] should be 1")

	val2, ok := mem.ReadUint32Le(ptr1 + 4)
	require.True(t, ok)
	require.Equal(t, uint32(2), val2, "list1[1] should be 2")

	val3, ok := mem.ReadUint32Le(ptr1 + 8)
	require.True(t, ok)
	require.Equal(t, uint32(3), val3, "list1[2] should be 3")

	val4, ok := mem.ReadUint32Le(ptr2)
	require.True(t, ok)
	require.Equal(t, uint32(4), val4, "list2[0] should be 4")

	val5, ok := mem.ReadUint32Le(ptr2 + 4)
	require.True(t, ok)
	require.Equal(t, uint32(5), val5, "list2[1] should be 5")
}

// TestExportedFunc_Call_ListWithoutRealloc verifies that calling with lists but no realloc
// function returns an appropriate error.
func TestExportedFunc_Call_ListWithoutRealloc(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 1024)}

	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // placeholder for list<s32>
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		},
	}

	f := &ExportedFunc{
		name:        "sum_list",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: nil, // No realloc function
	}

	list := ValList([]Val{ValS32(1), ValS32(2)})

	_, err := f.Call(context.Background(), list)
	require.Error(t, err)
	require.Contains(t, err.Error(), "realloc")
}

// TestExportedFunc_Call_EmptyList verifies that empty lists don't require realloc.
func TestExportedFunc_Call_EmptyList(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 1024)}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // placeholder for list<s32>
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}},
		},
	}

	f := &ExportedFunc{
		name:        "sum_list",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: nil, // No realloc - should be OK for empty list
	}

	emptyList := ValList([]Val{})

	_, err := f.Call(context.Background(), emptyList)
	require.NoError(t, err)

	// Empty list should still pass (ptr, len) where len is 0
	require.Equal(t, 2, len(capturedParams))
	require.Equal(t, uint64(0), capturedParams[1], "empty list should have length 0")
}

// TestExportedFunc_Call_ListOfS64 tests list<s64> element support.
func TestExportedFunc_Call_ListOfS64(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x78}}, // s64
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_s64",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with s64 values including max int64
	list := ValList([]Val{ValS64(1), ValS64(2), ValS64(math.MaxInt64)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 8-byte elements written correctly
	val1, ok := mem.ReadUint64Le(ptr)
	require.True(t, ok)
	require.Equal(t, uint64(1), val1)

	val2, ok := mem.ReadUint64Le(ptr + 8)
	require.True(t, ok)
	require.Equal(t, uint64(2), val2)

	val3, ok := mem.ReadUint64Le(ptr + 16)
	require.True(t, ok)
	require.Equal(t, uint64(math.MaxInt64), val3)
}

// TestExportedFunc_Call_ListOfF32 tests list<f32> element support with proper bit encoding.
func TestExportedFunc_Call_ListOfF32(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x76}}, // f32
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_f32",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with f32 values including infinity
	list := ValList([]Val{ValF32(1.5), ValF32(-3.14), ValF32(float32(math.Inf(1)))})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 4-byte float elements written with correct bit encoding
	bits1, ok := mem.ReadUint32Le(ptr)
	require.True(t, ok)
	require.Equal(t, math.Float32bits(1.5), bits1)

	bits2, ok := mem.ReadUint32Le(ptr + 4)
	require.True(t, ok)
	require.Equal(t, math.Float32bits(-3.14), bits2)

	bits3, ok := mem.ReadUint32Le(ptr + 8)
	require.True(t, ok)
	require.Equal(t, math.Float32bits(float32(math.Inf(1))), bits3)
}

// TestExportedFunc_Call_ListOfF64 tests list<f64> element support with proper bit encoding.
func TestExportedFunc_Call_ListOfF64(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x75}}, // f64
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_f64",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with f64 values including NaN
	list := ValList([]Val{ValF64(1.5), ValF64(-3.14159265358979), ValF64(math.NaN())})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 8-byte float elements written with correct bit encoding
	bits1, ok := mem.ReadUint64Le(ptr)
	require.True(t, ok)
	require.Equal(t, math.Float64bits(1.5), bits1)

	bits2, ok := mem.ReadUint64Le(ptr + 8)
	require.True(t, ok)
	require.Equal(t, math.Float64bits(-3.14159265358979), bits2)

	bits3, ok := mem.ReadUint64Le(ptr + 16)
	require.True(t, ok)
	require.True(t, math.IsNaN(math.Float64frombits(bits3)), "expected NaN")
}

// TestExportedFunc_Call_ListOfU8 tests list<u8> element support (1-byte elements).
func TestExportedFunc_Call_ListOfU8(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7d}}, // u8
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_u8",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with u8 values
	list := ValList([]Val{ValU8(0), ValU8(127), ValU8(255)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 1-byte elements written correctly
	val1, ok := mem.ReadByteAt(ptr)
	require.True(t, ok)
	require.Equal(t, byte(0), val1)

	val2, ok := mem.ReadByteAt(ptr + 1)
	require.True(t, ok)
	require.Equal(t, byte(127), val2)

	val3, ok := mem.ReadByteAt(ptr + 2)
	require.True(t, ok)
	require.Equal(t, byte(255), val3)
}

// TestExportedFunc_Call_ListOfS8 tests list<s8> element support (1-byte signed elements).
func TestExportedFunc_Call_ListOfS8(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7e}}, // s8
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_s8",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with s8 values including negatives
	list := ValList([]Val{ValS8(-128), ValS8(0), ValS8(127)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 1-byte elements written correctly (s8 stored as 2's complement)
	val1, ok := mem.ReadByteAt(ptr)
	require.True(t, ok)
	require.Equal(t, byte(0x80), val1) // -128 in two's complement

	val2, ok := mem.ReadByteAt(ptr + 1)
	require.True(t, ok)
	require.Equal(t, byte(0), val2)

	val3, ok := mem.ReadByteAt(ptr + 2)
	require.True(t, ok)
	require.Equal(t, byte(127), val3)
}

// TestExportedFunc_Call_ListOfU16 tests list<u16> element support (2-byte elements).
func TestExportedFunc_Call_ListOfU16(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7b}}, // u16
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_u16",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with u16 values
	list := ValList([]Val{ValU16(0), ValU16(32767), ValU16(65535)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 2-byte elements written correctly
	val1, ok := mem.ReadUint16Le(ptr)
	require.True(t, ok)
	require.Equal(t, uint16(0), val1)

	val2, ok := mem.ReadUint16Le(ptr + 2)
	require.True(t, ok)
	require.Equal(t, uint16(32767), val2)

	val3, ok := mem.ReadUint16Le(ptr + 4)
	require.True(t, ok)
	require.Equal(t, uint16(65535), val3)
}

// TestExportedFunc_Call_ListOfS16 tests list<s16> element support (2-byte signed elements).
func TestExportedFunc_Call_ListOfS16(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7c}}, // s16
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_s16",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with s16 values including negatives
	list := ValList([]Val{ValS16(-32768), ValS16(0), ValS16(32767)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 2-byte elements written correctly
	val1, ok := mem.ReadUint16Le(ptr)
	require.True(t, ok)
	require.Equal(t, uint16(0x8000), val1) // -32768 in two's complement

	val2, ok := mem.ReadUint16Le(ptr + 2)
	require.True(t, ok)
	require.Equal(t, uint16(0), val2)

	val3, ok := mem.ReadUint16Le(ptr + 4)
	require.True(t, ok)
	require.Equal(t, uint16(32767), val3)
}

// TestExportedFunc_Call_ListOfU64 tests list<u64> element support.
func TestExportedFunc_Call_ListOfU64(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x77}}, // u64
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "sum_u64",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with u64 values including max uint64
	list := ValList([]Val{ValU64(1), ValU64(2), ValU64(math.MaxUint64)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 8-byte elements written correctly
	val1, ok := mem.ReadUint64Le(ptr)
	require.True(t, ok)
	require.Equal(t, uint64(1), val1)

	val2, ok := mem.ReadUint64Le(ptr + 8)
	require.True(t, ok)
	require.Equal(t, uint64(2), val2)

	val3, ok := mem.ReadUint64Le(ptr + 16)
	require.True(t, ok)
	require.Equal(t, uint64(math.MaxUint64), val3)
}

// TestExportedFunc_Call_ListOfBool tests list<bool> element support (1-byte elements).
func TestExportedFunc_Call_ListOfBool(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}, // bool
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "any_true",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with bool values
	list := ValList([]Val{ValBool(false), ValBool(true), ValBool(false)})

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 1-byte elements written correctly
	val1, ok := mem.ReadByteAt(ptr)
	require.True(t, ok)
	require.Equal(t, byte(0), val1)

	val2, ok := mem.ReadByteAt(ptr + 1)
	require.True(t, ok)
	require.Equal(t, byte(1), val2)

	val3, ok := mem.ReadByteAt(ptr + 2)
	require.True(t, ok)
	require.Equal(t, byte(0), val3)
}

// TestExportedFunc_Call_ListOfChar tests list<char> element support (4-byte elements for Unicode code points).
func TestExportedFunc_Call_ListOfChar(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 4096)}
	nextPtr := uint32(100)

	mockRealloc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			newSize := uint32(params[3])
			align := uint32(params[2])
			if align > 0 && nextPtr%align != 0 {
				nextPtr = ((nextPtr / align) + 1) * align
			}
			ptr := nextPtr
			nextPtr += newSize
			return []uint64{uint64(ptr)}, nil
		},
	}

	var capturedParams []uint64
	coreFunc := &mockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			capturedParams = params
			return []uint64{0}, nil
		},
	}

	mockMod := &mockModule{memory: mem}
	inst := &Instance{
		coreInstances: []api.Module{mockMod},
		resourceTable: NewResourceTable(),
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{Name: "list", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x74}}, // char
		},
		Results: []NamedValType{
			{Name: "", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	f := &ExportedFunc{
		name:        "process_chars",
		funcType:    funcType,
		coreFunc:    coreFunc,
		instance:    inst,
		reallocFunc: mockRealloc,
	}

	// Test with char values including Unicode
	list := ValList([]Val{ValChar('A'), ValChar('Z'), ValChar('\U0001F600')}) // emoji

	_, err := f.Call(context.Background(), list)
	require.NoError(t, err)

	// Verify (ptr, len) were passed
	require.Equal(t, 2, len(capturedParams))
	ptr := uint32(capturedParams[0])
	length := uint32(capturedParams[1])
	require.Equal(t, uint32(3), length)

	// Verify 4-byte char elements written correctly
	val1, ok := mem.ReadUint32Le(ptr)
	require.True(t, ok)
	require.Equal(t, uint32('A'), val1)

	val2, ok := mem.ReadUint32Le(ptr + 4)
	require.True(t, ok)
	require.Equal(t, uint32('Z'), val2)

	val3, ok := mem.ReadUint32Le(ptr + 8)
	require.True(t, ok)
	require.Equal(t, uint32(0x1F600), val3) // grinning face emoji
}
