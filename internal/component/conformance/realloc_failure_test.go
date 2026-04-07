// Package conformance contains conformance tests for the Component Model implementation.
// Realloc failure handling tests verify graceful error handling when allocation fails.
package conformance

import (
	"errors"
	"testing"

	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestRealloc_StringLowerFailure tests that string lowering fails gracefully when realloc fails.
func TestRealloc_StringLowerFailure(t *testing.T) {
	testCases := []struct {
		name    string
		errMsg  string
		errFunc func(oldPtr, oldSize, align, newSize uint32) (uint32, error)
	}{
		{
			name:   "allocation_failed",
			errMsg: "allocation failed",
			errFunc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, errors.New("allocation failed")
			},
		},
		{
			name:   "out_of_memory",
			errMsg: "out of memory",
			errFunc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, errors.New("out of memory")
			},
		},
		{
			name:   "generic_error",
			errMsg: "realloc error",
			errFunc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, errors.New("realloc error")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMockMemory(1024)
			ctx := &abi.LowerContext{
				Memory:  mem,
				Opts:    &abi.Options{StringEncoding: abi.StringEncodingUTF8},
				Realloc: tc.errFunc,
			}

			_, _, err := abi.LowerString(ctx, "test string")
			require.Error(t, err)
			require.Contains(t, err.Error(), "realloc")
		})
	}
}

// TestRealloc_ListLowerFailure tests that list lowering fails gracefully when realloc fails.
func TestRealloc_ListLowerFailure(t *testing.T) {
	listType := types.List{Element: types.U32{}}
	listVal := types.ValList([]types.Val{
		types.ValU32(1),
		types.ValU32(2),
		types.ValU32(3),
	})

	t.Run("realloc_returns_error", func(t *testing.T) {
		mem := newMockMemory(1024)
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 0, errors.New("allocation failed")
			},
		}

		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc")
	})

	t.Run("no_realloc_provided", func(t *testing.T) {
		ctx := &abi.LowerContext{
			Memory:  newMockMemory(1024),
			Opts:    &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: nil, // No realloc
		}

		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realloc")
	})
}

// TestRealloc_EmptyAllocationsNoRealloc tests that empty allocations don't need realloc.
func TestRealloc_EmptyAllocationsNoRealloc(t *testing.T) {
	reallocCalled := false
	reallocFn := func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
		reallocCalled = true
		return 0, errors.New("should not be called")
	}

	mem := newMockMemory(1024)
	ctx := &abi.LowerContext{
		Memory:  mem,
		Opts:    &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: reallocFn,
	}

	t.Run("empty_string", func(t *testing.T) {
		reallocCalled = false
		_, _, err := abi.LowerString(ctx, "")
		require.NoError(t, err)
		require.False(t, reallocCalled, "realloc should not be called for empty string")
	})

	t.Run("empty_list", func(t *testing.T) {
		reallocCalled = false
		listType := types.List{Element: types.U32{}}
		emptyList := types.ValList([]types.Val{})
		_, err := abi.LowerFlat(ctx, listType, emptyList)
		require.NoError(t, err)
		require.False(t, reallocCalled, "realloc should not be called for empty list")
	})
}

// TestRealloc_PartialFailure tests behavior when realloc fails after partial progress.
func TestRealloc_PartialFailure(t *testing.T) {
	callCount := 0

	// Nested list type that requires multiple allocations
	// list<string> - each string needs allocation, then the list itself
	nestedType := types.List{Element: types.String{}}
	nestedVal := types.ValList([]types.Val{
		types.ValString("hello"),
		types.ValString("world"),
	})

	t.Run("fail_after_first_allocation", func(t *testing.T) {
		callCount = 0
		mem := newMockMemory(4096)
		allocPtr := uint32(256)

		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				callCount++
				if callCount > 1 {
					return 0, errors.New("allocation failed")
				}
				result := allocPtr
				allocPtr += newSize
				return result, nil
			},
		}

		_, err := abi.LowerFlat(ctx, nestedType, nestedVal)
		require.Error(t, err)
	})
}

// TestRealloc_AlignmentRequests tests that realloc receives correct alignment.
func TestRealloc_AlignmentRequests(t *testing.T) {
	var lastAlign uint32

	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			lastAlign = align
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	t.Run("string_alignment", func(t *testing.T) {
		allocPtr = 256
		_, _, err := abi.LowerString(ctx, "test")
		require.NoError(t, err)
		// UTF-8 strings have alignment 1
		require.Equal(t, uint32(1), lastAlign)
	})

	t.Run("list_u32_alignment", func(t *testing.T) {
		allocPtr = 256
		listType := types.List{Element: types.U32{}}
		listVal := types.ValList([]types.Val{types.ValU32(1)})
		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.NoError(t, err)
		// u32 elements have alignment 4
		require.Equal(t, uint32(4), lastAlign)
	})

	t.Run("list_u64_alignment", func(t *testing.T) {
		allocPtr = 256
		listType := types.List{Element: types.U64{}}
		listVal := types.ValList([]types.Val{types.ValU64(1)})
		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.NoError(t, err)
		// u64 elements have alignment 8
		require.Equal(t, uint32(8), lastAlign)
	})
}

// TestRealloc_SizeRequests tests that realloc receives correct size requests.
func TestRealloc_SizeRequests(t *testing.T) {
	var lastSize uint32

	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			lastSize = newSize
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	t.Run("string_size", func(t *testing.T) {
		allocPtr = 256
		_, _, err := abi.LowerString(ctx, "hello")
		require.NoError(t, err)
		require.Equal(t, uint32(5), lastSize, "should request 5 bytes for 'hello'")
	})

	t.Run("list_u32_size", func(t *testing.T) {
		allocPtr = 256
		listType := types.List{Element: types.U32{}}
		listVal := types.ValList([]types.Val{
			types.ValU32(1),
			types.ValU32(2),
			types.ValU32(3),
		})
		_, err := abi.LowerFlat(ctx, listType, listVal)
		require.NoError(t, err)
		require.Equal(t, uint32(12), lastSize, "should request 12 bytes for 3 u32s")
	})
}

// TestRealloc_UTF16Encoding tests realloc behavior with UTF-16 encoding.
func TestRealloc_UTF16Encoding(t *testing.T) {
	var lastSize uint32

	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF16},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			lastSize = newSize
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	t.Run("utf16_string_size", func(t *testing.T) {
		allocPtr = 256
		_, _, err := abi.LowerString(ctx, "hello")
		require.NoError(t, err)
		// UTF-16: "hello" = 5 code units * 2 bytes = 10 bytes
		require.Equal(t, uint32(10), lastSize)
	})

	t.Run("utf16_emoji_size", func(t *testing.T) {
		allocPtr = 256
		_, _, err := abi.LowerString(ctx, "\U0001F600") // Grinning face emoji
		require.NoError(t, err)
		// UTF-16: emoji requires surrogate pair = 2 code units * 2 bytes = 4 bytes
		require.Equal(t, uint32(4), lastSize)
	})
}

// TestRealloc_ReturnsInvalidPointer tests handling when realloc returns invalid pointer.
func TestRealloc_ReturnsInvalidPointer(t *testing.T) {
	mem := newMockMemory(64) // Small memory

	t.Run("pointer_beyond_memory", func(t *testing.T) {
		ctx := &abi.LowerContext{
			Memory: mem,
			Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
			Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
				return 1000, nil // Beyond 64-byte memory
			},
		}

		_, _, err := abi.LowerString(ctx, "test")
		require.Error(t, err, "should error when realloc returns OOB pointer")
	})
}

// TestRealloc_OldPtrOldSize tests that oldPtr and oldSize are correctly passed.
func TestRealloc_OldPtrOldSize(t *testing.T) {
	var lastOldPtr, lastOldSize uint32

	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			lastOldPtr = oldPtr
			lastOldSize = oldSize
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	// New allocations should have oldPtr=0, oldSize=0
	_, _, err := abi.LowerString(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, uint32(0), lastOldPtr, "new allocation should have oldPtr=0")
	require.Equal(t, uint32(0), lastOldSize, "new allocation should have oldSize=0")
}

// TestRealloc_LowerHeapStringFailure tests LowerHeap string realloc failure.
func TestRealloc_LowerHeapStringFailure(t *testing.T) {
	mem := newMockMemory(1024)
	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, errors.New("allocation failed")
		},
	}

	val := types.ValString("hello")
	err := abi.LowerHeap(ctx, types.String{}, val, 0)
	require.Error(t, err)
}

// TestRealloc_LowerHeapListFailure tests LowerHeap list realloc failure.
func TestRealloc_LowerHeapListFailure(t *testing.T) {
	mem := newMockMemory(1024)
	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			return 0, errors.New("allocation failed")
		},
	}

	listType := types.List{Element: types.U32{}}
	val := types.ValList([]types.Val{types.ValU32(1), types.ValU32(2)})
	err := abi.LowerHeap(ctx, listType, val, 0)
	require.Error(t, err)
}

// TestRealloc_SuccessfulAllocation tests the happy path for allocation.
func TestRealloc_SuccessfulAllocation(t *testing.T) {
	mem := newMockMemory(4096)
	allocPtr := uint32(256)

	ctx := &abi.LowerContext{
		Memory: mem,
		Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
		Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
			result := allocPtr
			allocPtr += newSize
			return result, nil
		},
	}

	t.Run("string_allocation", func(t *testing.T) {
		ptr, length, err := abi.LowerString(ctx, "hello world")
		require.NoError(t, err)
		require.Equal(t, uint32(256), ptr)
		require.Equal(t, uint32(11), length)

		// Verify data was written
		data, ok := mem.Read(256, 11)
		require.True(t, ok)
		require.Equal(t, "hello world", string(data))
	})
}
