// internal/component/canon_lower_test.go

package component

import (
	"context"
	"math"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestCanonLower_SimpleFunc(t *testing.T) {
	// Create a simple host function that adds two i32s
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		a := args[0].S32()
		b := args[1].S32()
		return []Val{ValS32(a + b)}, nil
	}

	// Lower it to a core function
	lowered := CanonLower(hostFunc, nil, &CanonicalOptions{
		StringEncoding: StringEncodingUTF8,
	})

	require.NotNil(t, lowered)

	// Call the lowered function with core wasm values
	results, err := lowered.CallWithStack(context.Background(), []uint64{2, 3})
	require.NoError(t, err)

	require.Equal(t, 1, len(results))
	require.Equal(t, uint64(5), results[0])
}

func TestCanonLower_NilOptions(t *testing.T) {
	// Test that nil options defaults to UTF8
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(42)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	require.NotNil(t, lowered)
	require.NotNil(t, lowered.options)
	require.Equal(t, StringEncodingUTF8, lowered.options.StringEncoding)
}

func TestCanonLower_NoArgs(t *testing.T) {
	// Test function with no arguments
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(100)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint64(100), results[0])
}

func TestCanonLower_NoResults(t *testing.T) {
	// Test function with no results
	called := false
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		called = true
		return []Val{}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{42})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, 0, len(results))
}

func TestCanonLower_MultipleResults(t *testing.T) {
	// Test function with multiple results
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		a := args[0].S32()
		return []Val{ValS32(a * 2), ValS32(a * 3)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{5})
	require.NoError(t, err)
	require.Equal(t, 2, len(results))
	require.Equal(t, uint64(10), results[0])
	require.Equal(t, uint64(15), results[1])
}

func TestCanonLower_U32Values(t *testing.T) {
	// Test with unsigned 32-bit values
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		// Args are lifted as S32, convert to U32 for unsigned behavior
		a := uint32(args[0].S32())
		return []Val{ValU32(a + 1)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	// Use a large unsigned value
	input := uint64(0xFFFFFFFF) // max u32
	results, err := lowered.CallWithStack(context.Background(), []uint64{input})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	// 0xFFFFFFFF + 1 = 0 (overflow)
	require.Equal(t, uint64(0), results[0])
}

func TestCanonLower_S64Values(t *testing.T) {
	// Test with 64-bit values using typed function
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		// Without type info, values are lifted as S32
		// This test demonstrates the limitation
		a := int64(args[0].S32())
		b := int64(args[1].S32())
		return []Val{ValS64(a + b)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{100, 200})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int64(300), int64(results[0]))
}

func TestCanonLower_BoolValues(t *testing.T) {
	// Test bool lowering
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValBool(true), ValBool(false)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.NoError(t, err)
	require.Equal(t, 2, len(results))
	require.Equal(t, uint64(1), results[0])
	require.Equal(t, uint64(0), results[1])
}

func TestCanonLower_F32Values(t *testing.T) {
	// Test float32 lowering
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValF32(3.14)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	// Convert result back to float32
	resultF32 := math.Float32frombits(uint32(results[0]))
	// Check within epsilon
	diff := float64(resultF32 - 3.14)
	if diff < 0 {
		diff = -diff
	}
	require.True(t, diff < 0.001)
}

func TestCanonLower_F64Values(t *testing.T) {
	// Test float64 lowering
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValF64(2.718281828)}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	// Convert result back to float64
	resultF64 := math.Float64frombits(results[0])
	// Check within epsilon
	diff := resultF64 - 2.718281828
	if diff < 0 {
		diff = -diff
	}
	require.True(t, diff < 0.0000001)
}

func TestCanonLower_CharValues(t *testing.T) {
	// Test char (Unicode scalar) lowering
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValChar('A'), ValChar('\u4e2d')}, nil // 'A' and Chinese character
	}

	lowered := CanonLower(hostFunc, nil, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.NoError(t, err)
	require.Equal(t, 2, len(results))
	require.Equal(t, uint64('A'), results[0])
	require.Equal(t, uint64('\u4e2d'), results[1])
}

func TestCanonLower_WithTypedParams(t *testing.T) {
	// Test with explicit type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 2, len(args))
		a := args[0].S32()
		b := args[1].S32()
		return []Val{ValS32(a * b)}, nil
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{7, 8})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint64(56), results[0])
}

func TestCanonLower_WithTypedBool(t *testing.T) {
	// Test with explicit bool type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 1, len(args))
		b := args[0].Bool()
		return []Val{ValBool(!b)}, nil
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}, // bool
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7f}}, // bool
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)

	// Test with true (1)
	results, err := lowered.CallWithStack(context.Background(), []uint64{1})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint64(0), results[0]) // !true = false

	// Test with false (0)
	results, err = lowered.CallWithStack(context.Background(), []uint64{0})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint64(1), results[0]) // !false = true
}

func TestCanonLower_ErrorPropagation(t *testing.T) {
	// Test that errors from the callback are propagated
	expectedErr := "test error"
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, &testError{msg: expectedErr}
	}

	lowered := CanonLower(hostFunc, nil, nil)
	_, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.Error(t, err)
	require.Contains(t, err.Error(), expectedErr)
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestCanonLower_SetMemory(t *testing.T) {
	// Test SetMemory
	lowered := CanonLower(func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	}, nil, nil)

	require.Nil(t, lowered.memory)
	// We can't easily create an api.Memory for testing without a full runtime,
	// but we can at least verify the method exists and doesn't panic with nil
	lowered.SetMemory(nil)
	require.Nil(t, lowered.memory)
}

func TestCanonLower_SetInstance(t *testing.T) {
	// Test SetInstance
	lowered := CanonLower(func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	}, nil, nil)

	require.Nil(t, lowered.instance)

	inst := &Instance{
		resourceTable: NewResourceTable(),
	}
	lowered.SetInstance(inst)
	require.Equal(t, inst, lowered.instance)
}

func TestCanonLower_ContextPropagation(t *testing.T) {
	// Test that context is properly propagated to the callback
	type ctxKey string
	testKey := ctxKey("test-key")
	testValue := "test-value"

	var receivedCtx context.Context
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		receivedCtx = ctx
		return []Val{}, nil
	}

	lowered := CanonLower(hostFunc, nil, nil)

	ctx := context.WithValue(context.Background(), testKey, testValue)
	_, err := lowered.CallWithStack(ctx, []uint64{})
	require.NoError(t, err)

	require.NotNil(t, receivedCtx)
	require.Equal(t, testValue, receivedCtx.Value(testKey))
}

func TestLowerValToFlat_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		val      Val
		expected []uint64
	}{
		{"bool true", ValBool(true), []uint64{1}},
		{"bool false", ValBool(false), []uint64{0}},
		{"s8 positive", ValS8(42), []uint64{42}},
		{"s8 negative", ValS8(-1), []uint64{0xFFFFFFFF}}, // sign extended to i32
		{"u8", ValU8(255), []uint64{255}},
		{"s16 positive", ValS16(1000), []uint64{1000}},
		{"s16 negative", ValS16(-1), []uint64{0xFFFFFFFF}},
		{"u16", ValU16(65535), []uint64{65535}},
		{"s32 positive", ValS32(100000), []uint64{100000}},
		{"s32 negative", ValS32(-100), []uint64{0xFFFFFF9C}},
		{"u32", ValU32(0xFFFFFFFF), []uint64{0xFFFFFFFF}},
		{"s64", ValS64(0x123456789ABCDEF0), []uint64{0x123456789ABCDEF0}},
		{"u64", ValU64(0xFEDCBA9876543210), []uint64{0xFEDCBA9876543210}},
		{"char ASCII", ValChar('Z'), []uint64{90}},
		{"char Unicode", ValChar('\u1234'), []uint64{0x1234}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lowerValToFlat(tt.val)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestLowerValToFlat_Floats(t *testing.T) {
	// Test f32
	result, err := lowerValToFlat(ValF32(1.5))
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, math.Float32frombits(uint32(result[0])), float32(1.5))

	// Test f64
	result, err = lowerValToFlat(ValF64(2.5))
	require.NoError(t, err)
	require.Equal(t, 1, len(result))
	require.Equal(t, math.Float64frombits(result[0]), float64(2.5))
}

func TestLowerValToFlat_UnsupportedType(t *testing.T) {
	// Test that unsupported types return an error
	_, err := lowerValToFlat(ValString("hello"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}

func TestFlatIter(t *testing.T) {
	// Test the flat iterator
	iter := newFlatIter([]uint64{1, 2, 3, 4})

	require.Equal(t, uint32(1), iter.nextI32())
	require.Equal(t, uint32(2), iter.nextI32())
	require.Equal(t, uint64(3), iter.nextI64())
	require.Equal(t, uint32(4), iter.nextI32())

	// Out of bounds should return 0
	require.Equal(t, uint32(0), iter.nextI32())
	require.Equal(t, uint64(0), iter.nextI64())
}

func TestFlatIter_Floats(t *testing.T) {
	// Test float reading
	f32bits := math.Float32bits(1.5)
	f64bits := math.Float64bits(2.5)

	iter := newFlatIter([]uint64{uint64(f32bits), f64bits})

	require.Equal(t, float32(1.5), iter.nextF32())
	require.Equal(t, float64(2.5), iter.nextF64())
}

func TestCanonLower_WithTypedS64(t *testing.T) {
	// Test with explicit s64 type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 1, len(args))
		a := args[0].S64()
		return []Val{ValS64(a * 2)}, nil
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x78}}, // s64
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x78}}, // s64
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{0x123456789ABCDEF0})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	// Verify the multiplication (with potential overflow)
	expected := int64(0x123456789ABCDEF0) * 2
	require.Equal(t, uint64(expected), results[0])
}

func TestCanonLower_WithTypedU64(t *testing.T) {
	// Test with explicit u64 type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 1, len(args))
		a := args[0].U64()
		return []Val{ValU64(a + 1)}, nil
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x77}}, // u64
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x77}}, // u64
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{0xFFFFFFFFFFFFFFFE})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), results[0])
}

func TestCanonLower_WithTypedF32(t *testing.T) {
	// Test with explicit f32 type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 1, len(args))
		a := args[0].F32()
		return []Val{ValF32(a * 2.0)}, nil
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x76}}, // f32
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x76}}, // f32
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	inputF32 := float32(1.5)
	results, err := lowered.CallWithStack(context.Background(), []uint64{uint64(math.Float32bits(inputF32))})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	resultF32 := math.Float32frombits(uint32(results[0]))
	// Check within epsilon
	diff := float64(resultF32 - 3.0)
	if diff < 0 {
		diff = -diff
	}
	require.True(t, diff < 0.001)
}

func TestCanonLower_WithTypedF64(t *testing.T) {
	// Test with explicit f64 type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 1, len(args))
		a := args[0].F64()
		return []Val{ValF64(a * 2.0)}, nil
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x75}}, // f64
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x75}}, // f64
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	inputF64 := float64(1.5)
	results, err := lowered.CallWithStack(context.Background(), []uint64{math.Float64bits(inputF64)})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	resultF64 := math.Float64frombits(results[0])
	// Check within epsilon
	diff := resultF64 - 3.0
	if diff < 0 {
		diff = -diff
	}
	require.True(t, diff < 0.001)
}

func TestCanonLower_WithTypedChar(t *testing.T) {
	// Test with explicit char type information
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		require.Equal(t, 1, len(args))
		c := args[0].Char()
		return []Val{ValChar(c + 1)}, nil // Return next character
	}

	funcType := &FuncType{
		Params: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x74}}, // char
		},
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x74}}, // char
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	results, err := lowered.CallWithStack(context.Background(), []uint64{uint64('A')})
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, uint64('B'), results[0])
}

func TestCanonLower_MismatchedResultCount(t *testing.T) {
	// Test error when result count doesn't match type
	hostFunc := func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(1), ValS32(2)}, nil // Return 2 results
	}

	funcType := &FuncType{
		Results: []NamedValType{
			{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // Only 1 result expected
		},
	}

	lowered := CanonLower(hostFunc, funcType, nil)
	_, err := lowered.CallWithStack(context.Background(), []uint64{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected 1 results, got 2")
}

// mockMemoryForLower implements api.Memory for testing string lowering
type mockMemoryForLower struct {
	internalapi.WazeroOnlyType
	data []byte
}

func (m *mockMemoryForLower) Definition() api.MemoryDefinition { return nil }
func (m *mockMemoryForLower) Size() uint32                     { return uint32(len(m.data)) }
func (m *mockMemoryForLower) Grow(deltaPages uint32) (uint32, bool) {
	return 0, false
}
func (m *mockMemoryForLower) ReadByteAt(offset uint32) (byte, bool) {
	if offset >= uint32(len(m.data)) {
		return 0, false
	}
	return m.data[offset], true
}
func (m *mockMemoryForLower) ReadUint16Le(offset uint32) (uint16, bool) {
	if offset+2 > uint32(len(m.data)) {
		return 0, false
	}
	return uint16(m.data[offset]) | uint16(m.data[offset+1])<<8, true
}
func (m *mockMemoryForLower) ReadUint32Le(offset uint32) (uint32, bool) {
	if offset+4 > uint32(len(m.data)) {
		return 0, false
	}
	return uint32(m.data[offset]) | uint32(m.data[offset+1])<<8 | uint32(m.data[offset+2])<<16 | uint32(m.data[offset+3])<<24, true
}
func (m *mockMemoryForLower) ReadFloat32Le(offset uint32) (float32, bool) {
	v, ok := m.ReadUint32Le(offset)
	return math.Float32frombits(v), ok
}
func (m *mockMemoryForLower) ReadUint64Le(offset uint32) (uint64, bool) {
	if offset+8 > uint32(len(m.data)) {
		return 0, false
	}
	lo, _ := m.ReadUint32Le(offset)
	hi, _ := m.ReadUint32Le(offset + 4)
	return uint64(lo) | uint64(hi)<<32, true
}
func (m *mockMemoryForLower) ReadFloat64Le(offset uint32) (float64, bool) {
	v, ok := m.ReadUint64Le(offset)
	return math.Float64frombits(v), ok
}
func (m *mockMemoryForLower) Read(offset, byteCount uint32) ([]byte, bool) {
	if offset+byteCount > uint32(len(m.data)) {
		return nil, false
	}
	return m.data[offset : offset+byteCount], true
}
func (m *mockMemoryForLower) WriteByteAt(offset uint32, v byte) bool {
	if offset >= uint32(len(m.data)) {
		return false
	}
	m.data[offset] = v
	return true
}
func (m *mockMemoryForLower) WriteUint16Le(offset uint32, v uint16) bool {
	if offset+2 > uint32(len(m.data)) {
		return false
	}
	m.data[offset] = byte(v)
	m.data[offset+1] = byte(v >> 8)
	return true
}
func (m *mockMemoryForLower) WriteUint32Le(offset, v uint32) bool {
	if offset+4 > uint32(len(m.data)) {
		return false
	}
	m.data[offset] = byte(v)
	m.data[offset+1] = byte(v >> 8)
	m.data[offset+2] = byte(v >> 16)
	m.data[offset+3] = byte(v >> 24)
	return true
}
func (m *mockMemoryForLower) WriteFloat32Le(offset uint32, v float32) bool {
	return m.WriteUint32Le(offset, math.Float32bits(v))
}
func (m *mockMemoryForLower) WriteUint64Le(offset uint32, v uint64) bool {
	if offset+8 > uint32(len(m.data)) {
		return false
	}
	m.WriteUint32Le(offset, uint32(v))
	m.WriteUint32Le(offset+4, uint32(v>>32))
	return true
}
func (m *mockMemoryForLower) WriteFloat64Le(offset uint32, v float64) bool {
	return m.WriteUint64Le(offset, math.Float64bits(v))
}
func (m *mockMemoryForLower) Write(offset uint32, v []byte) bool {
	if int(offset)+len(v) > len(m.data) {
		return false
	}
	copy(m.data[offset:], v)
	return true
}
func (m *mockMemoryForLower) WriteString(offset uint32, v string) bool {
	return m.Write(offset, []byte(v))
}

// mockFunctionForLower implements api.Function for testing realloc
type mockFunctionForLower struct {
	internalapi.WazeroOnlyType
	callFn func(ctx context.Context, params ...uint64) ([]uint64, error)
}

func (m *mockFunctionForLower) Definition() api.FunctionDefinition { return nil }
func (m *mockFunctionForLower) Call(ctx context.Context, params ...uint64) ([]uint64, error) {
	return m.callFn(ctx, params...)
}
func (m *mockFunctionForLower) CallWithStack(ctx context.Context, stack []uint64) error {
	return nil
}

func TestLoweredFunc_StringLowering(t *testing.T) {
	// Test that string lowering allocates memory and writes UTF-8 bytes
	testStr := "Hello, World!"

	var allocatedPtr, allocatedSize uint32
	mockRealloc := &mockFunctionForLower{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			// realloc(oldPtr, oldSize, align, newSize) -> newPtr
			newSize := uint32(params[3])
			allocatedPtr = 0x1000 // Mock allocation
			allocatedSize = newSize
			return []uint64{uint64(allocatedPtr)}, nil
		},
	}

	memoryData := make([]byte, 0x2000)
	mockMemory := &mockMemoryForLower{data: memoryData}

	f := &LoweredFunc{
		funcType: &FuncType{
			Params: []NamedValType{
				{Name: "s", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
			},
		},
		memory:      mockMemory,
		reallocFunc: mockRealloc,
	}

	// Lower string argument
	results, err := f.lowerValToFlatTyped(ValString(testStr), f.funcType.Params[0].ValType)
	if err != nil {
		t.Fatalf("string lowering failed: %v", err)
	}

	// Should return (ptr, len)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (ptr, len), got %d", len(results))
	}

	ptr := uint32(results[0])
	length := uint32(results[1])

	if length != uint32(len(testStr)) {
		t.Errorf("expected length %d, got %d", len(testStr), length)
	}

	// Verify allocatedSize matches expected
	if allocatedSize != uint32(len(testStr)) {
		t.Errorf("expected allocated size %d, got %d", len(testStr), allocatedSize)
	}

	// Verify UTF-8 bytes were written to memory
	written := string(memoryData[ptr : ptr+length])
	if written != testStr {
		t.Errorf("expected %q, got %q", testStr, written)
	}
}

func TestLoweredFunc_StringLowering_EmptyString(t *testing.T) {
	// Test that empty string returns (0, 0) without calling realloc
	reallocCalled := false
	mockRealloc := &mockFunctionForLower{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			reallocCalled = true
			return []uint64{0x1000}, nil
		},
	}

	memoryData := make([]byte, 0x2000)
	mockMemory := &mockMemoryForLower{data: memoryData}

	f := &LoweredFunc{
		funcType: &FuncType{
			Params: []NamedValType{
				{Name: "s", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
			},
		},
		memory:      mockMemory,
		reallocFunc: mockRealloc,
	}

	// Lower empty string
	results, err := f.lowerValToFlatTyped(ValString(""), f.funcType.Params[0].ValType)
	if err != nil {
		t.Fatalf("empty string lowering failed: %v", err)
	}

	// Should return (0, 0)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (ptr, len), got %d", len(results))
	}

	if results[0] != 0 || results[1] != 0 {
		t.Errorf("expected (0, 0) for empty string, got (%d, %d)", results[0], results[1])
	}

	// Realloc should not be called for empty strings
	if reallocCalled {
		t.Error("realloc should not be called for empty strings")
	}
}

func TestLoweredFunc_StringLowering_NoMemory(t *testing.T) {
	// Test that string lowering fails without memory
	f := &LoweredFunc{
		funcType: &FuncType{
			Params: []NamedValType{
				{Name: "s", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
			},
		},
		memory:      nil, // No memory
		reallocFunc: nil,
	}

	_, err := f.lowerValToFlatTyped(ValString("test"), f.funcType.Params[0].ValType)
	if err == nil {
		t.Fatal("expected error for string lowering without memory")
	}
}

func TestLoweredFunc_StringLowering_NoRealloc(t *testing.T) {
	// Test that non-empty string lowering fails without realloc
	memoryData := make([]byte, 0x2000)
	mockMemory := &mockMemoryForLower{data: memoryData}

	f := &LoweredFunc{
		funcType: &FuncType{
			Params: []NamedValType{
				{Name: "s", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
			},
		},
		memory:      mockMemory,
		reallocFunc: nil, // No realloc
	}

	_, err := f.lowerValToFlatTyped(ValString("test"), f.funcType.Params[0].ValType)
	if err == nil {
		t.Fatal("expected error for string lowering without realloc")
	}
}

func TestLoweredFunc_StringLowering_Unicode(t *testing.T) {
	// Test that string lowering works with unicode strings
	testStr := "Hello, \u4e16\u754c!" // "Hello, 世界!"

	mockRealloc := &mockFunctionForLower{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			return []uint64{0x1000}, nil
		},
	}

	memoryData := make([]byte, 0x2000)
	mockMemory := &mockMemoryForLower{data: memoryData}

	f := &LoweredFunc{
		funcType: &FuncType{
			Params: []NamedValType{
				{Name: "s", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
			},
		},
		memory:      mockMemory,
		reallocFunc: mockRealloc,
	}

	results, err := f.lowerValToFlatTyped(ValString(testStr), f.funcType.Params[0].ValType)
	if err != nil {
		t.Fatalf("unicode string lowering failed: %v", err)
	}

	ptr := uint32(results[0])
	length := uint32(results[1])

	// Length should be byte length of UTF-8 encoding
	expectedLen := uint32(len([]byte(testStr)))
	if length != expectedLen {
		t.Errorf("expected length %d, got %d", expectedLen, length)
	}

	// Verify UTF-8 bytes were written correctly
	written := string(memoryData[ptr : ptr+length])
	if written != testStr {
		t.Errorf("expected %q, got %q", testStr, written)
	}
}
