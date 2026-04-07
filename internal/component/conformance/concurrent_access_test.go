// Package conformance contains conformance tests for the Component Model implementation.
// Concurrent access tests verify thread-safety and behavior under concurrent operations.
package conformance

import (
	"sync"
	"testing"

	"github.com/tetratelabs/wazero/internal/component"
	"github.com/tetratelabs/wazero/internal/component/abi"
	"github.com/tetratelabs/wazero/internal/component/runtime"
	"github.com/tetratelabs/wazero/internal/component/types"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

// TestConcurrent_ResourceTableOperations tests concurrent resource table operations.
// Note: ResourceTable may not be designed for concurrent access, but these tests
// verify it doesn't corrupt data under sequential operations from different goroutines.
func TestConcurrent_ResourceTableOperations(t *testing.T) {
	t.Run("sequential_from_goroutines", func(t *testing.T) {
		table := runtime.NewResourceTable()
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := 10
		handlesPerGoroutine := 100

		handles := make([][]runtime.Handle, numGoroutines)

		// Create handles from multiple goroutines (sequentially with mutex)
		for g := 0; g < numGoroutines; g++ {
			handles[g] = make([]runtime.Handle, handlesPerGoroutine)
			wg.Add(1)
			go func(gIdx int) {
				defer wg.Done()
				for i := 0; i < handlesPerGoroutine; i++ {
					mu.Lock()
					handles[gIdx][i] = table.New(gIdx*1000+i, true)
					mu.Unlock()
				}
			}(g)
		}
		wg.Wait()

		// Verify all handles are accessible
		for g := 0; g < numGoroutines; g++ {
			for i := 0; i < handlesPerGoroutine; i++ {
				entry, err := table.Get(handles[g][i])
				require.NoError(t, err)
				require.Equal(t, g*1000+i, entry.Rep)
			}
		}
	})
}

// TestConcurrent_TypeOperations tests concurrent type property computations.
// Type operations should be safe for concurrent read access.
func TestConcurrent_TypeOperations(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "a", Type: types.U32{}},
			{Name: "b", Type: types.U64{}},
			{Name: "c", Type: types.String{}},
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	// Concurrent reads of type properties should be safe
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				size := recordType.Size()
				align := recordType.Align()
				flatCount := recordType.FlattenCount()
				offsets := recordType.FieldOffsets()

				// Verify consistency
				require.Equal(t, uint32(24), size)
				require.Equal(t, uint32(8), align)
				require.Equal(t, 4, flatCount)
				require.Equal(t, 3, len(offsets))
			}
		}()
	}
	wg.Wait()
}

// TestConcurrent_LowerFlat tests concurrent lowering operations.
// Each goroutine uses its own context, so operations should not interfere.
func TestConcurrent_LowerFlat(t *testing.T) {
	recordType := types.Record{
		Fields: []types.Field{
			{Name: "x", Type: types.U32{}},
			{Name: "y", Type: types.U32{}},
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	iterations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				val := types.ValRecord(map[string]types.Val{
					"x": types.ValU32(uint32(gIdx)),
					"y": types.ValU32(uint32(i)),
				})

				flat, err := abi.LowerFlat(nil, recordType, val)
				require.NoError(t, err)
				require.Equal(t, 2, len(flat))
				require.Equal(t, uint64(gIdx), flat[0])
				require.Equal(t, uint64(i), flat[1])
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_LiftFlat tests concurrent lifting operations.
func TestConcurrent_LiftFlat(t *testing.T) {
	tupleType := types.Tuple{
		Types: []types.ValType{types.U32{}, types.U32{}, types.U32{}},
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	iterations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				flatVals := []uint64{uint64(gIdx), uint64(i), uint64(gIdx + i)}
				iter := abi.NewFlatIter(flatVals)

				lifted, err := abi.LiftFlat(nil, tupleType, iter)
				require.NoError(t, err)

				elems := lifted.Tuple()
				require.Equal(t, 3, len(elems))
				require.Equal(t, uint32(gIdx), elems[0].U32())
				require.Equal(t, uint32(i), elems[1].U32())
				require.Equal(t, uint32(gIdx+i), elems[2].U32())
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_StringOperations tests concurrent string operations.
func TestConcurrent_StringOperations(t *testing.T) {
	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()

			// Each goroutine has its own memory and context
			mem := newMockMemory(8192)
			allocPtr := uint32(256)
			var mu sync.Mutex

			ctx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					mu.Lock()
					defer mu.Unlock()
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			for i := 0; i < iterations; i++ {
				testStr := "hello world from goroutine"

				ptr, length, err := abi.LowerString(ctx, testStr)
				require.NoError(t, err)
				require.Equal(t, uint32(len(testStr)), length)

				// Lift back
				liftCtx := &abi.LiftContext{
					Memory: mem,
					Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
				}

				iter := abi.NewFlatIter([]uint64{uint64(ptr), uint64(length)})
				lifted, err := abi.LiftFlat(liftCtx, types.String{}, iter)
				require.NoError(t, err)
				require.Equal(t, testStr, lifted.StringVal())
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_MemoryOperations tests concurrent memory read/write operations.
func TestConcurrent_MemoryOperations(t *testing.T) {
	t.Run("separate_regions", func(t *testing.T) {
		// Each goroutine writes to a separate region of memory
		mem := newMockMemory(10000)
		var wg sync.WaitGroup
		numGoroutines := 10
		regionSize := uint32(1000)

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(gIdx int) {
				defer wg.Done()
				baseOffset := uint32(gIdx) * regionSize

				// Write pattern
				for i := uint32(0); i < regionSize; i++ {
					data := []byte{byte(gIdx), byte(i % 256)}
					ok := mem.Write(baseOffset+i, data[:1])
					require.True(t, ok)
				}

				// Read and verify
				for i := uint32(0); i < regionSize; i++ {
					data, ok := mem.Read(baseOffset+i, 1)
					require.True(t, ok)
					require.Equal(t, byte(gIdx), data[0])
				}
			}(g)
		}
		wg.Wait()
	})
}

// TestConcurrent_FlatIterIndependence tests that FlatIter instances are independent.
func TestConcurrent_FlatIterIndependence(t *testing.T) {
	var wg sync.WaitGroup
	numGoroutines := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()

			// Each goroutine creates its own iterator
			values := []uint64{uint64(gIdx), uint64(gIdx * 2), uint64(gIdx * 3)}
			iter := abi.NewFlatIter(values)

			// Read values sequentially
			v1 := iter.NextI32()
			v2 := iter.NextI32()
			v3 := iter.NextI32()

			require.Equal(t, uint32(gIdx), v1)
			require.Equal(t, uint32(gIdx*2), v2)
			require.Equal(t, uint32(gIdx*3), v3)
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_ValConstruction tests concurrent Val construction.
func TestConcurrent_ValConstruction(t *testing.T) {
	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Construct various Val types
				_ = types.ValU32(uint32(gIdx + i))
				_ = types.ValString("test")
				_ = types.ValBool(i%2 == 0)
				_ = types.ValRecord(map[string]types.Val{
					"x": types.ValU32(uint32(i)),
				})
				_ = types.ValTuple([]types.Val{
					types.ValU32(uint32(gIdx)),
					types.ValU32(uint32(i)),
				})
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_HandleConstruction tests concurrent handle construction.
func TestConcurrent_HandleConstruction(t *testing.T) {
	var wg sync.WaitGroup
	numGoroutines := 100
	iterations := 1000

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				h := runtime.MakeHandle(uint32(i), uint32(gIdx))
				require.Equal(t, uint32(i), h.Index())
				require.Equal(t, uint32(gIdx), h.Generation())
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_VariantTypes tests concurrent variant type operations.
func TestConcurrent_VariantTypes(t *testing.T) {
	variantType := types.Variant{
		Cases: []types.Case{
			{Name: "num", Type: types.U32{}},
			{Name: "flag", Type: types.Bool{}},
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	iterations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				var val types.Val
				if i%2 == 0 {
					payload := types.ValU32(uint32(gIdx + i))
					val = types.ValVariant("num", &payload)
				} else {
					payload := types.ValBool(gIdx%2 == 0)
					val = types.ValVariant("flag", &payload)
				}

				flat, err := abi.LowerFlat(nil, variantType, val)
				require.NoError(t, err)

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(nil, variantType, iter)
				require.NoError(t, err)

				caseName, _ := lifted.Variant()
				if i%2 == 0 {
					require.Equal(t, "num", caseName)
				} else {
					require.Equal(t, "flag", caseName)
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_ListOperations tests concurrent list operations.
func TestConcurrent_ListOperations(t *testing.T) {
	listType := types.List{Element: types.U32{}}

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()

			mem := newMockMemory(16384)
			allocPtr := uint32(256)
			var mu sync.Mutex

			ctx := &abi.LowerContext{
				Memory: mem,
				Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
				Realloc: func(oldPtr, oldSize, align, newSize uint32) (uint32, error) {
					mu.Lock()
					defer mu.Unlock()
					result := allocPtr
					allocPtr += newSize
					return result, nil
				},
			}

			for i := 0; i < iterations; i++ {
				// Create a list with varying size
				listSize := (i % 10) + 1
				elements := make([]types.Val, listSize)
				for j := 0; j < listSize; j++ {
					elements[j] = types.ValU32(uint32(gIdx*1000 + i*100 + j))
				}
				val := types.ValList(elements)

				flat, err := abi.LowerFlat(ctx, listType, val)
				require.NoError(t, err)

				liftCtx := &abi.LiftContext{
					Memory: mem,
					Opts:   &abi.Options{StringEncoding: abi.StringEncodingUTF8},
				}

				iter := abi.NewFlatIter(flat)
				lifted, err := abi.LiftFlat(liftCtx, listType, iter)
				require.NoError(t, err)
				require.Equal(t, listSize, len(lifted.List()))
			}
		}(g)
	}
	wg.Wait()
}

// TestConcurrent_NoRace is a sanity test to ensure no race conditions
// are detected when running with -race flag.
func TestConcurrent_NoRace(t *testing.T) {
	// This test is primarily valuable when run with -race
	var wg sync.WaitGroup

	// Mix of different operations
	for g := 0; g < 10; g++ {
		wg.Add(3)

		// Type operations
		go func() {
			defer wg.Done()
			recordType := types.Record{
				Fields: []types.Field{
					{Name: "a", Type: types.U32{}},
				},
			}
			for i := 0; i < 100; i++ {
				_ = recordType.Size()
				_ = recordType.Align()
			}
		}()

		// Val operations
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				val := types.ValU32(uint32(i))
				_ = val.U32()
			}
		}()

		// Handle operations
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				h := runtime.MakeHandle(uint32(i), uint32(i))
				_ = h.Index()
				_ = h.Generation()
			}
		}()
	}

	wg.Wait()
}
