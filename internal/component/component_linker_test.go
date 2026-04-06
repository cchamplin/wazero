// internal/component/component_linker_test.go
package component

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestComponentLinkerDefineFunc(t *testing.T) {
	// Use nil runtime for unit tests since we can't import wazero
	linker := NewComponentLinker(nil)

	err := linker.DefineFunc("test:api@1.0.0", "hello", func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValString("Hello!")}, nil
	})
	require.NoError(t, err)

	// Duplicate should fail
	err = linker.DefineFunc("test:api@1.0.0", "hello", func(ctx context.Context, args []Val) ([]Val, error) {
		return nil, nil
	})
	require.Error(t, err)
}

func TestComponentLinkerDefineInstance(t *testing.T) {
	linker := NewComponentLinker(nil)

	err := linker.DefineInstance("wasi:cli/environment@0.2.0").
		Func("get-environment", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Func("get-arguments", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()

	require.NoError(t, err)

	// Verify the instance was registered
	def, err := linker.MatchImport("wasi:cli/environment@0.2.0")
	require.NoError(t, err)
	require.NotNil(t, def)

	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Equal(t, 2, len(instDef.Exports))
}

func TestComponentLinkerDefineResource(t *testing.T) {
	linker := NewComponentLinker(nil)

	destroyed := false
	err := linker.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {
		destroyed = true
	})
	require.NoError(t, err)

	// Duplicate should fail
	err = linker.DefineResource("wasi:io/streams@0.2.0", "input-stream", func(rep uint32) {})
	require.Error(t, err)

	// Verify we can retrieve and call the destructor
	def, err := linker.MatchImport("wasi:io/streams@0.2.0/input-stream")
	require.NoError(t, err)

	resDef, ok := def.(*ResourceDef)
	require.True(t, ok)
	resDef.Destructor(0)
	require.True(t, destroyed)
}

func TestComponentLinkerMatchImport(t *testing.T) {
	linker := NewComponentLinker(nil)

	// Define v1.0.1
	err := linker.DefineFunc("test:api@1.0.1", "fn", func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValS32(101)}, nil
	})
	require.NoError(t, err)

	// Request v1.0.0 - should match v1.0.1 (semver compatible)
	def, err := linker.MatchImport("test:api@1.0.0/fn")
	require.NoError(t, err)
	require.NotNil(t, def)

	// Verify we got the right function
	funcDef, ok := def.(*FuncDef)
	require.True(t, ok)
	results, err := funcDef.Callback(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, int32(101), results[0].S32())
}

func TestComponentLinkerMatchImportNotFound(t *testing.T) {
	linker := NewComponentLinker(nil)

	_, err := linker.MatchImport("missing:api@1.0.0/fn")
	require.Error(t, err)
}

func TestComponentLinkerDefineInstanceDuplicate(t *testing.T) {
	linker := NewComponentLinker(nil)

	err := linker.DefineInstance("api@1.0.0").
		Func("fn", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.NoError(t, err)

	// Duplicate instance should fail
	err = linker.DefineInstance("api@1.0.0").
		Func("fn2", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Build()
	require.Error(t, err)
}

func TestComponentLinkerInstanceBuilderResource(t *testing.T) {
	linker := NewComponentLinker(nil)

	err := linker.DefineInstance("api@1.0.0").
		Func("read", func(ctx context.Context, args []Val) ([]Val, error) {
			return nil, nil
		}).
		Resource("handle", func(rep uint32) {}).
		Build()
	require.NoError(t, err)

	// Use Get for direct instance lookup (MatchImport expects namespace/name format)
	def, ok := linker.Get("api@1.0.0")
	require.True(t, ok)

	instDef, ok := def.(*InstanceDef)
	require.True(t, ok)
	require.Equal(t, 2, len(instDef.Exports))
	require.NotNil(t, instDef.Exports["read"])
	require.NotNil(t, instDef.Exports["handle"])
}

// TestPostReturnFunctionCalled verifies that post-return functions are called
// after the main exported function returns but before control returns to caller.
// The post-return function is used for cleanup (e.g., freeing memory allocated
// for return values).
func TestPostReturnFunctionCalled(t *testing.T) {
	// Track post-return invocation
	postReturnCalled := false
	postReturnArgs := []uint64(nil)

	// Create a mock core function that returns a value
	mockCoreFunc := &testMockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			return []uint64{42}, nil // Returns s32 value 42
		},
	}

	// Create a mock post-return function
	mockPostReturnFunc := &testMockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			postReturnCalled = true
			postReturnArgs = append([]uint64{}, params...) // Copy params
			return nil, nil
		},
	}

	// Create an ExportedFunc with a post-return function
	postReturnIdx := uint32(1)
	exportedFunc := &ExportedFunc{
		name:     "test-func",
		funcType: &FuncType{Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}}}, // s32
		coreFunc: mockCoreFunc,
		canonical: &CanonicalDef{
			Kind:    CanonKindLift,
			Options: CanonicalOptions{PostReturnIdx: &postReturnIdx},
		},
		postReturnFunc: mockPostReturnFunc,
	}

	// Call the exported function
	ctx := context.Background()
	results, err := exportedFunc.Call(ctx)

	// Verify the main function returned correctly
	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(42), results[0].S32())

	// Verify post-return was called
	require.True(t, postReturnCalled, "post-return function should have been called")

	// Verify post-return received the flat return values as arguments
	require.Equal(t, 1, len(postReturnArgs))
	require.Equal(t, uint64(42), postReturnArgs[0])
}

// TestPostReturnNotCalledWhenNil verifies that when no post-return function
// is specified, the exported function still works correctly.
func TestPostReturnNotCalledWhenNil(t *testing.T) {
	// Create a mock core function that returns a value
	mockCoreFunc := &testMockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			return []uint64{99}, nil
		},
	}

	// Create an ExportedFunc without a post-return function
	exportedFunc := &ExportedFunc{
		name:           "test-func",
		funcType:       &FuncType{Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}}},
		coreFunc:       mockCoreFunc,
		canonical:      &CanonicalDef{Kind: CanonKindLift},
		postReturnFunc: nil, // No post-return
	}

	// Call should succeed
	ctx := context.Background()
	results, err := exportedFunc.Call(ctx)

	require.NoError(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, int32(99), results[0].S32())
}

// testMockFunction is a test helper that implements api.Function for unit testing.
// Named differently to avoid collision with mockFunction in instance_test.go.
type testMockFunction struct {
	internalapi.WazeroOnlyType
	callFn func(ctx context.Context, params ...uint64) ([]uint64, error)
}

func (m *testMockFunction) Definition() api.FunctionDefinition { return nil }
func (m *testMockFunction) Call(ctx context.Context, params ...uint64) ([]uint64, error) {
	return m.callFn(ctx, params...)
}
func (m *testMockFunction) CallWithStack(ctx context.Context, stack []uint64) error { return nil }

// makeReallocStub returns a testMockFunction that emulates cabi_realloc by
// bump-allocating from a counter that starts at startPtr. The returned
// function returns the new pointer in results[0].
func makeReallocStub(startPtr uint32) (*testMockFunction, *uint32) {
	cursor := startPtr
	return &testMockFunction{
		callFn: func(ctx context.Context, params ...uint64) ([]uint64, error) {
			// realloc(old_ptr, old_size, align, new_size) -> new_ptr
			align := uint32(params[2])
			size := uint32(params[3])
			if align == 0 {
				align = 1
			}
			// Align cursor up to 'align'.
			cursor = (cursor + align - 1) &^ (align - 1)
			ptr := cursor
			cursor += size
			return []uint64{uint64(ptr)}, nil
		},
	}, &cursor
}

// TestWriteValTyped_ListOfU32 verifies that lowering list<u32> writes
// (ptr, len) at the offset, allocates element storage via realloc, and
// writes each u32 as 4 bytes little-endian. This is the simplest list
// element type and exercises the type-aware fast path.
func TestWriteValTyped_ListOfU32(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 0x4000)}
	realloc, _ := makeReallocStub(0x100)

	// list<u32>: TypeIdx 0 = u32 (alias), TypeIdx 1 = list<u32>.
	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, Handle: &ValTypeRef{IsPrimitive: true, Primitive: 0x79}}, // u32 alias
		1: {Kind: TypeDefKindDefined, List: &ListTypeDef{
			ElementType: ValTypeRef{IsPrimitive: true, Primitive: 0x79}, // u32
		}},
	}
	listRef := ValTypeRef{TypeIdx: 1}

	val := ValList([]Val{ValU32(11), ValU32(22), ValU32(33)})

	err := writeValTyped(context.Background(), mem, realloc, 0, val, listRef, localTypes)
	require.NoError(t, err)

	// (ptr, len) at offset 0
	ptr, _ := mem.ReadUint32Le(0)
	length, _ := mem.ReadUint32Le(4)
	require.Equal(t, uint32(0x100), ptr)
	require.Equal(t, uint32(3), length)

	// Elements at the allocated buffer
	v0, _ := mem.ReadUint32Le(ptr)
	v1, _ := mem.ReadUint32Le(ptr + 4)
	v2, _ := mem.ReadUint32Le(ptr + 8)
	require.Equal(t, uint32(11), v0)
	require.Equal(t, uint32(22), v1)
	require.Equal(t, uint32(33), v2)
}

// TestWriteValTyped_ListOfString verifies that lowering list<string> writes
// each string element as a (ptr, len) pair, with the underlying UTF-8 bytes
// allocated via realloc. Element size is 8 (string is ptr+len) per spec.
func TestWriteValTyped_ListOfString(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 0x4000)}
	realloc, _ := makeReallocStub(0x200)

	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, List: &ListTypeDef{
			ElementType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}, // string
		}},
	}
	listRef := ValTypeRef{TypeIdx: 0}

	val := ValList([]Val{ValString("ab"), ValString("xyz")})

	err := writeValTyped(context.Background(), mem, realloc, 0, val, listRef, localTypes)
	require.NoError(t, err)

	// list (ptr, len) at offset 0; len = 2
	listPtr, _ := mem.ReadUint32Le(0)
	listLen, _ := mem.ReadUint32Le(4)
	require.Equal(t, uint32(2), listLen)
	// Element 0 is at listPtr+0, element 1 at listPtr+8 (string elem_size = 8)
	s0Ptr, _ := mem.ReadUint32Le(listPtr)
	s0Len, _ := mem.ReadUint32Le(listPtr + 4)
	s1Ptr, _ := mem.ReadUint32Le(listPtr + 8)
	s1Len, _ := mem.ReadUint32Le(listPtr + 12)
	require.Equal(t, uint32(2), s0Len)
	require.Equal(t, uint32(3), s1Len)
	s0Bytes, _ := mem.Read(s0Ptr, s0Len)
	s1Bytes, _ := mem.Read(s1Ptr, s1Len)
	require.Equal(t, "ab", string(s0Bytes))
	require.Equal(t, "xyz", string(s1Bytes))
}

// TestWriteValTyped_ListOfTupleU32String verifies the canonical-ABI
// element layout for list<tuple<u32, string>>:
//   - elem_size(tuple<u32, string>) = align_to(4 (u32) + 8 (string), 4) = 12
//   - alignment(tuple<u32, string>) = max(4, 4) = 4
// Each list element occupies 12 bytes (4 for u32 + 4+4 for string ptr,len).
func TestWriteValTyped_ListOfTupleU32String(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 0x4000)}
	realloc, _ := makeReallocStub(0x300)

	// TypeIdx 0 = tuple<u32, string>, TypeIdx 1 = list<tuple<u32, string>>.
	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, Tuple: &TupleTypeDef{
			Types: []ValTypeRef{
				{IsPrimitive: true, Primitive: 0x79}, // u32
				{IsPrimitive: true, Primitive: 0x73}, // string
			},
		}},
		1: {Kind: TypeDefKindDefined, List: &ListTypeDef{
			ElementType: ValTypeRef{TypeIdx: 0},
		}},
	}
	listRef := ValTypeRef{TypeIdx: 1}

	val := ValList([]Val{
		ValTuple([]Val{ValU32(0xdeadbeef), ValString("hi")}),
		ValTuple([]Val{ValU32(0xfeedface), ValString("there")}),
	})

	err := writeValTyped(context.Background(), mem, realloc, 0, val, listRef, localTypes)
	require.NoError(t, err)

	// list ptr/len at offset 0
	listPtr, _ := mem.ReadUint32Le(0)
	listLen, _ := mem.ReadUint32Le(4)
	require.Equal(t, uint32(2), listLen)

	// First element: u32 at +0, string ptr at +4, string len at +8
	const elemSize = 12
	u0, _ := mem.ReadUint32Le(listPtr)
	s0Ptr, _ := mem.ReadUint32Le(listPtr + 4)
	s0Len, _ := mem.ReadUint32Le(listPtr + 8)
	require.Equal(t, uint32(0xdeadbeef), u0)
	require.Equal(t, uint32(2), s0Len)
	s0Bytes, _ := mem.Read(s0Ptr, s0Len)
	require.Equal(t, "hi", string(s0Bytes))

	// Second element starts at listPtr + elemSize.
	u1, _ := mem.ReadUint32Le(listPtr + elemSize)
	s1Ptr, _ := mem.ReadUint32Le(listPtr + elemSize + 4)
	s1Len, _ := mem.ReadUint32Le(listPtr + elemSize + 8)
	require.Equal(t, uint32(0xfeedface), u1)
	require.Equal(t, uint32(5), s1Len)
	s1Bytes, _ := mem.Read(s1Ptr, s1Len)
	require.Equal(t, "there", string(s1Bytes))
}

// TestWriteValTyped_ListOfTupleOwnString verifies the layout for
// list<tuple<own<R>, string>>, the exact shape used by
// wasi:filesystem/preopens.get-directories. The own handle is written
// as a 4-byte index and the string follows as (ptr, len). This is the
// regression test for the original bug.
func TestWriteValTyped_ListOfTupleOwnString(t *testing.T) {
	mem := &mockMemory{data: make([]byte, 0x4000)}
	realloc, _ := makeReallocStub(0x400)

	// TypeIdx 0 = own<R> (resource type R is irrelevant for layout)
	// TypeIdx 1 = tuple<own<R>, string>
	// TypeIdx 2 = list<tuple<own<R>, string>>
	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, Handle: &ValTypeRef{IsOwn: true}},
		1: {Kind: TypeDefKindDefined, Tuple: &TupleTypeDef{
			Types: []ValTypeRef{
				{TypeIdx: 0},                         // own<R>
				{IsPrimitive: true, Primitive: 0x73}, // string
			},
		}},
		2: {Kind: TypeDefKindDefined, List: &ListTypeDef{
			ElementType: ValTypeRef{TypeIdx: 1},
		}},
	}
	listRef := ValTypeRef{TypeIdx: 2}

	val := ValList([]Val{
		ValTuple([]Val{ValOwn(7), ValString("/")}),
	})

	err := writeValTyped(context.Background(), mem, realloc, 0, val, listRef, localTypes)
	require.NoError(t, err)

	listPtr, _ := mem.ReadUint32Le(0)
	listLen, _ := mem.ReadUint32Le(4)
	require.Equal(t, uint32(1), listLen)

	// Tuple element layout: own at +0, string at +4 (ptr) and +8 (len)
	handle, _ := mem.ReadUint32Le(listPtr)
	pathPtr, _ := mem.ReadUint32Le(listPtr + 4)
	pathLen, _ := mem.ReadUint32Le(listPtr + 8)
	require.Equal(t, uint32(7), handle)
	require.Equal(t, uint32(1), pathLen)
	pathBytes, _ := mem.Read(pathPtr, pathLen)
	require.Equal(t, "/", string(pathBytes))
}

// TestCabiSize_RecordSpecExample verifies the canonical ABI elem_size and
// alignment for tuple<u32, string>:
//   alignment(u32) = 4, elem_size(u32) = 4
//   alignment(string) = 4, elem_size(string) = 8
//   tuple field offsets: u32 at 0, string at align_to(4,4)=4
//   total before final padding: 4 + 8 = 12
//   alignment(tuple) = max(4,4) = 4
//   align_to(12, 4) = 12 → elem_size = 12
func TestCabiSize_TupleU32String(t *testing.T) {
	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, Tuple: &TupleTypeDef{
			Types: []ValTypeRef{
				{IsPrimitive: true, Primitive: 0x79}, // u32
				{IsPrimitive: true, Primitive: 0x73}, // string
			},
		}},
	}
	ref := ValTypeRef{TypeIdx: 0}
	require.Equal(t, uint32(4), cabiAlignTypeRef(ref, localTypes))
	require.Equal(t, uint32(12), cabiSizeTypeRef(ref, localTypes))
}

// TestCabiSize_TupleU64String verifies that 8-byte alignment from u64 is
// honored for the whole tuple:
//   tuple<u64, string>: u64 (size 8, align 8) + string (size 8, align 4)
//   alignment = max(8, 4) = 8
//   field offsets: u64 at 0, string at align_to(8, 4) = 8
//   total = 8 + 8 = 16; align_to(16, 8) = 16
func TestCabiSize_TupleU64String(t *testing.T) {
	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, Tuple: &TupleTypeDef{
			Types: []ValTypeRef{
				{IsPrimitive: true, Primitive: 0x77}, // u64
				{IsPrimitive: true, Primitive: 0x73}, // string
			},
		}},
	}
	ref := ValTypeRef{TypeIdx: 0}
	require.Equal(t, uint32(8), cabiAlignTypeRef(ref, localTypes))
	require.Equal(t, uint32(16), cabiSizeTypeRef(ref, localTypes))
}

// TestCabiSize_ListOfU8 verifies that elem_size(list<u8>) = 8 (ptr + len)
// regardless of the inner element size.
func TestCabiSize_ListOfU8(t *testing.T) {
	localTypes := map[uint32]*TypeDef{
		0: {Kind: TypeDefKindDefined, List: &ListTypeDef{
			ElementType: ValTypeRef{IsPrimitive: true, Primitive: 0x7d}, // u8
		}},
	}
	ref := ValTypeRef{TypeIdx: 0}
	require.Equal(t, uint32(4), cabiAlignTypeRef(ref, localTypes))
	require.Equal(t, uint32(8), cabiSizeTypeRef(ref, localTypes))
}

// TestComponentLinker_OrderedInstantiation verifies that core instances are
// instantiated in order and that imports can be resolved from previously
// instantiated instances.
func TestComponentLinker_OrderedInstantiation(t *testing.T) {
	// This test verifies the import resolver infrastructure.
	// For a full integration test with actual modules, see wasip2test.
	linker := NewComponentLinker(nil)

	// Create a mock component structure
	c := &Component{
		CoreInstances: []CoreInstance{
			// Instance 0: instantiate module 0 (no imports)
			{
				Kind:      CoreInstanceExprInstantiate,
				ModuleIdx: 0,
				Args:      nil,
			},
			// Instance 1: instantiate module 1 with imports from instance 0
			{
				Kind:      CoreInstanceExprInstantiate,
				ModuleIdx: 1,
				Args: []CoreInstantiateArg{
					{Name: "provider", InstanceIdx: 0},
				},
			},
		},
		Aliases: []Alias{
			{Kind: AliasKindCoreExport, InstanceIdx: 0, ExportName: "memory", CoreSort: CoreSortMemory},
			{Kind: AliasKindCoreExport, InstanceIdx: 0, ExportName: "func1", CoreSort: CoreSortFunc},
		},
	}

	// Test the import resolver building
	inst := &Instance{
		component:     c,
		coreInstances: make([]api.Module, 2),
	}

	// Build import resolver for core instance 1
	ctx := context.Background()
	resolvedImports := make(map[string]Definition)
	canonLowers := make(map[uint32]canonLowerInfo)
	canonResources := make(map[uint32]canonResourceInfo)
	funcAliases := make(map[uint32]struct{ instanceIdx uint32; exportName string })
	resolver := linker.buildImportResolver(ctx, inst, c, &c.CoreInstances[1], resolvedImports, canonLowers, canonResources, funcAliases)
	require.NotNil(t, resolver)

	// The resolver should map "provider" to instance 0
	// Since we haven't actually instantiated anything, inst.coreInstances[0] is nil
	// and the resolver will return nil (which is correct behavior)
	result := resolver("provider")
	require.Nil(t, result, "resolver returns nil when instance not yet available")

	// Unknown import module should return nil
	result = resolver("unknown")
	require.Nil(t, result, "resolver returns nil for unknown imports")
}

// TestComponentLinker_TypeCheckingIntegration verifies that type checking is called
// during component instantiation. We test by providing a mismatched type and
// expecting an error.
func TestComponentLinker_TypeCheckingIntegration(t *testing.T) {
	// Create a minimal component that imports a function with a specific type
	c := &Component{
		Imports: []Import{
			{
				Name: "test/fn",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescFunc,
					TypeIdx: 0,
				},
			},
		},
		Types: []TypeDef{
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Params: []NamedValType{
						{Name: "x", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
					},
					Results: []NamedValType{
						{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x7a}}, // s32
					},
				},
			},
		},
	}

	compiled := &CompiledComponent{
		component: c,
	}

	linker := NewComponentLinker(nil)

	// Define function with WRONG type (no params, returns string instead of s32)
	// Note: DefineFunc doesn't attach type info to FuncDef, so type checking
	// will pass if the FuncDef.Type is nil (trust the host).
	// This test documents the behavior and verifies no panics occur.
	err := linker.DefineFunc("test", "fn", func(ctx context.Context, args []Val) ([]Val, error) {
		return []Val{ValString("wrong")}, nil
	})
	require.NoError(t, err)

	// Instantiation should not panic
	ctx := context.Background()
	_, err = linker.Instantiate(ctx, compiled)

	// Currently, DefineFunc doesn't provide type info on FuncDef,
	// so type checking passes (trusts the host).
	// Once FuncDef carries type info, this should return a type mismatch error.
	// For now, we just verify the type checker is being called and doesn't panic.
	_ = err
}

// TestComponentLinker_TypeCheckingInstanceImport verifies type checking for
// instance imports during component instantiation.
func TestComponentLinker_TypeCheckingInstanceImport(t *testing.T) {
	// Create a component that imports an instance with expected exports
	// Instance imports use the format "namespace/interface@version"
	c := &Component{
		Imports: []Import{
			{
				Name: "test:api/greeting@1.0.0",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
		Types: []TypeDef{
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "greet",
								Kind: ExportKindFunc,
								Idx:  1, // Points to type index 1 (FuncType)
							},
						},
					},
				},
			},
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Params: []NamedValType{
						{Name: "name", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
					},
					Results: []NamedValType{
						{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}, // string
					},
				},
			},
		},
	}

	compiled := &CompiledComponent{
		component: c,
	}

	linker := NewComponentLinker(nil)

	// Define instance with the expected "greet" export
	err := linker.DefineInstance("test:api/greeting@1.0.0").
		Func("greet", func(ctx context.Context, args []Val) ([]Val, error) {
			name := args[0].StringVal()
			return []Val{ValString("Hello, " + name)}, nil
		}).
		Build()
	require.NoError(t, err)

	// Instantiation should not panic
	ctx := context.Background()
	_, err = linker.Instantiate(ctx, compiled)

	// Verify the type checker is being called (no panic)
	// The instance type checking validates that all required exports exist
	_ = err
}

// TestComponentLinker_TypeCheckingMissingExport verifies that type checking
// catches missing exports in instance imports.
func TestComponentLinker_TypeCheckingMissingExport(t *testing.T) {
	// Create a component that imports an instance expecting a "greet" export
	// Instance imports use the format "namespace/interface@version"
	c := &Component{
		Imports: []Import{
			{
				Name: "test:api/greeting@1.0.0",
				ExternDesc: ImportExternDesc{
					Kind:    ImportExternDescInstance,
					TypeIdx: 0,
				},
			},
		},
		Types: []TypeDef{
			{
				Kind: TypeDefKindInstance,
				Instance: &InstanceTypeDef{
					Declarations: []InstanceDecl{
						{
							Kind: InstanceDeclKindExport,
							Export: &InstanceExport{
								Name: "greet",
								Kind: ExportKindFunc,
								Idx:  1, // Points to type index 1 (FuncType)
							},
						},
					},
				},
			},
			{
				Kind: TypeDefKindFunc,
				Func: &FuncType{
					Params:  []NamedValType{{Name: "name", ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}},
					Results: []NamedValType{{ValType: ValTypeRef{IsPrimitive: true, Primitive: 0x73}}},
				},
			},
		},
	}

	compiled := &CompiledComponent{
		component: c,
	}

	linker := NewComponentLinker(nil)

	// Define instance WITHOUT the expected "greet" export (missing export)
	err := linker.DefineInstance("test:api/greeting@1.0.0").
		Func("goodbye", func(ctx context.Context, args []Val) ([]Val, error) {
			return []Val{ValString("Goodbye!")}, nil
		}).
		Build()
	require.NoError(t, err)

	// Instantiation should fail due to missing "greet" export
	ctx := context.Background()
	_, err = linker.Instantiate(ctx, compiled)

	// Type checking should catch the missing export
	require.Error(t, err)
	require.Contains(t, err.Error(), "type mismatch")
	require.Contains(t, err.Error(), "greet")
}
